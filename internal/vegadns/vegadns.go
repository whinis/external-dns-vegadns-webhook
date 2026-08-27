package VegasDNS

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	vdapi "github.com/whinis/vegadns"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

const (
	// provider specific key to track if PTR record was already created or not for A records
	providerSpecificVegasDNSPtrRecord = "VegasDNS-ptr-record-exists"
	providerSpecificVegasDNSID        = "VegasDNS-record-id"
)

type VegasDNSConfig struct {
	BaseURL    string `env:"VEGADNS_URL,required" envDefault:"localhost"`
	Token      string `env:"VEGADNS_TOKEN,required" envDefault:""`
	Secret     string `env:"VEGADNS_SECRET,required" envDefault:""`
	SSLVerify  bool   `env:"VEGADNS_SSL_VERIFY" envDefault:"true"`
	DryRun     bool   `env:"VEGADNS_DRY_RUN" envDefault:"false"`
	MaxResults int    `env:"VEGADNS_MAX_RESULTS" envDefault:"1500"`
	CreatePTR  bool   `env:"VEGADNS_CREATE_PTR" envDefault:"false"`
	DefaultTTL int    `env:"VEGADNS_DEFAULT_TTL" envDefault:"300"`
	FQDNRegEx  string
	NameRegEx  string
	HTTPClient *http.Client
}

type VegasDNSClient interface {
	ZonesList(config *VegasDNSConfig) ([]*ZoneAuth, error)
	RecordAdd(rr *endpoint.Endpoint) error
	RecordDelete(rr *endpoint.Endpoint) error
	RecordList(Zone ZoneAuth) (endpoints []*endpoint.Endpoint, _ error)
}

func NewVegasDNSAPI(config *VegasDNSConfig, ctx context.Context) (VegasDNSAPI, error) {
	client, err := vdapi.NewClient(config.BaseURL,
		vdapi.WithOAuth(config.Token, config.Secret),
		vdapi.WithHTTPClient(config.HTTPClient),
	)
	if err != nil {
		return VegasDNSAPI{}, fmt.Errorf("vegadns: %w", err)
	}
	return VegasDNSAPI{
		client:  client,
		context: ctx,
	}, nil
}

type VegasDNSAPI struct {
	client  *vdapi.Client
	context context.Context
}

type Provider struct {
	provider.BaseProvider
	client       VegasDNSClient
	domainFilter endpoint.DomainFilter
	context      context.Context
	config       *VegasDNSConfig
}

type ZoneAuth struct {
	Name   string
	ID     int
	status string
}

func NewZoneAuth(zone vdapi.Domain) *ZoneAuth {
	return &ZoneAuth{
		Name:   zone.Domain,
		ID:     zone.DomainID,
		status: zone.Status,
	}
}

// Creates a new VegasDNS provider.
func NewVegasDNSProvider(config *VegasDNSConfig, domainFilter endpoint.DomainFilter) (*Provider, error) {
	ctx := context.Background()
	client, _ := NewVegasDNSAPI(config, ctx)
	provider := &Provider{
		client:       &client,
		domainFilter: domainFilter,
		context:      ctx,
		config:       config,
	}

	return provider, nil
}

func (p *Provider) Zones() ([]*ZoneAuth, error) {
	var result []*ZoneAuth
	zones, err := p.client.ZonesList(p.config)

	if err != nil {
		return nil, err
	}

	for _, zone := range zones {
		if !p.domainFilter.Match(zone.Name) {
			log.Debugf("Ignore zone [%s] by domainFilter", zone.Name)
			continue
		}
		result = append(result, zone)
	}
	return result, nil
}

// Records gets the current records.
func (p *Provider) Records(ctx context.Context) (endpoints []*endpoint.Endpoint, err error) {
	log.Debug("fetching records...")
	p.context = ctx
	zones, err := p.Zones()
	if err != nil {
		return nil, fmt.Errorf("could not fetch zones: %w", err)
	}

	for _, zone := range zones {
		log.Debugf("fetch records from zone '%s'", zone.Name)

		records, err := p.client.RecordList(*zone)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, records...)
	}

	log.Debugf("fetched %d records from VegasDNS", len(endpoints))
	return endpoints, nil
}

// ApplyChanges applies the given changes.
func (p *Provider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	p.context = ctx
	for _, change := range changes.Delete {
		err := p.DeleteChanges(ctx, change)
		if err != nil {
			return err
		}
	}
	for _, change := range changes.UpdateOld {
		err := p.DeleteChanges(ctx, change)
		if err != nil {
			return err
		}
	}
	for _, change := range changes.UpdateNew {
		err := p.CreateChanges(ctx, change)
		if err != nil {
			return err
		}
	}
	for _, change := range changes.Create {
		err := p.CreateChanges(ctx, change)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Provider) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	// Update user specified TTL (0 == disabled)
	for _, ep := range endpoints {
		if !ep.RecordTTL.IsConfigured() {
			ep.RecordTTL = endpoint.TTL(p.config.DefaultTTL)
		}
	}

	if !p.config.CreatePTR {
		return endpoints, nil
	}

	// for all A records, we want to create PTR records
	// so add provider specific property to track if the record was created or not
	for i := range endpoints {
		if endpoints[i].RecordType == endpoint.RecordTypeA {
			found := false
			for j := range endpoints[i].ProviderSpecific {
				if endpoints[i].ProviderSpecific[j].Name == providerSpecificVegasDNSPtrRecord {
					endpoints[i].ProviderSpecific[j].Value = "true"
					found = true
				}
			}
			if !found {
				endpoints[i].WithProviderSpecific(providerSpecificVegasDNSPtrRecord, "true")
			}
		}
	}

	return endpoints, nil
}

func (p *Provider) DeleteChanges(_ context.Context, changes *endpoint.Endpoint) error {
	if p.config.DryRun {
		for _, value := range changes.Targets {
			log.Infof("Would delete %s record named '%s' to '%s' for VegasDNS",
				changes.RecordType,
				changes.DNSName,
				value,
			)
		}
		return nil
	}
	_ = p.client.RecordDelete(changes)
	return nil
}

func (p *Provider) CreateChanges(_ context.Context, changes *endpoint.Endpoint) error {
	if p.config.DryRun {
		for _, value := range changes.Targets {
			log.Infof("Would create %s record named '%s' to '%s' for VegasDNS",
				changes.RecordType,
				changes.DNSName,
				value,
			)
		}
		return nil
	}
	_ = p.client.RecordAdd(changes)
	return nil
}

func (e *VegasDNSAPI) ZonesList(config *VegasDNSConfig) ([]*ZoneAuth, error) {
	zones, err := e.client.GetDomains(e.context, "")

	if err != nil {
		log.Errorf("Error when calling DnsAPI.DnsZoneList: %v\n", err)
		return nil, err
	}
	var result []*ZoneAuth
	for _, zone := range zones {
		result = append(result, NewZoneAuth(zone))
	}

	return result, nil
}

func (e *VegasDNSAPI) RecordList(zone ZoneAuth) (endpoints []*endpoint.Endpoint, _ error) {
	records, err := e.client.GetRecords(e.context, zone.ID)
	if err != nil {
		log.Errorf("Failed to get RRs from zone [%s]: %v", zone.Name, err)
		return nil, err
	}

	Host := make(map[string]*endpoint.Endpoint)
	for _, rr := range records {
		var ttl = rr.TTL
		switch rr.RecordType {
		case "A":
			log.Debugf("Found A Record : %s -> %s", rr.Name, rr.Value)
			if h, found := Host[rr.Name+":"+rr.RecordType]; found {
				h.Targets = append(h.Targets, rr.Value)
			} else {
				var newEndpoint = endpoint.NewEndpointWithTTL(rr.Name, endpoint.RecordTypeA, endpoint.TTL(ttl), rr.Value)
				newEndpoint.WithProviderSpecific(providerSpecificVegasDNSID, strconv.Itoa(rr.RecordID))
				Host[rr.Name+":"+rr.RecordType] = newEndpoint
			}
		case "TXT":
			log.Debugf("Found TXT Record : %s -> %s", rr.Name, rr.Value)
			tmp := endpoint.NewEndpointWithTTL(rr.Name, endpoint.RecordTypeTXT, endpoint.TTL(ttl), rr.Value)
			tmp.WithProviderSpecific(providerSpecificVegasDNSID, strconv.Itoa(rr.RecordID))
			endpoints = append(endpoints, tmp)
		case "CNAME":
			log.Debugf("Found CNAME Record : %s -> %s", rr.Name, rr.Value)
			var newEndpoint = endpoint.NewEndpointWithTTL(rr.Name, rr.RecordType, endpoint.TTL(ttl), rr.Value)
			newEndpoint.WithProviderSpecific(providerSpecificVegasDNSID, strconv.Itoa(rr.RecordID))
			endpoints = append(endpoints, newEndpoint)
		}
	}
	for _, rr := range Host {
		endpoints = append(endpoints, rr)
	}
	return endpoints, nil
}

func (e *VegasDNSAPI) RecordDelete(rr *endpoint.Endpoint) error {
	for _, value := range rr.Targets {
		log.Infof("Deleting %s record named '%s' to '%s' for VegasDNS",
			rr.RecordType,
			rr.DNSName,
			value,
		)
		var recordID int
		var found = false
		log.Debugf("Checking provider specific values for record %s", rr.DNSName)
		for j := range rr.ProviderSpecific {
			log.Debugf("Checking provider specific value %s", rr.ProviderSpecific[j].Name)
			if rr.ProviderSpecific[j].Name == providerSpecificVegasDNSID {
				rr.ProviderSpecific[j].Value = "true"
				found = true
			}
		}
		if !found {
			log.Errorf("Deletion of the RR %v %v -> %v : failed! record id not set", rr.RecordType, rr.DNSName, value)
			return nil
		}
		var err = e.client.DeleteRecord(e.context, recordID)
		if err != nil {
			log.Errorf("Deletion of the RR %v %v -> %v : failed! %v", rr.RecordType, rr.DNSName, value, err)
		}
	}
	return nil
}

func (e *VegasDNSAPI) RecordAdd(rr *endpoint.Endpoint) error {
	zones, err := e.ZonesList(nil)
	if err != nil {
		log.Errorf("Failed to retrieve zone list: %v", err)
		return nil
	}
	for _, value := range rr.Targets {
		log.Infof("Creating %s record named '%s' to '%s' for VegasDNS",
			rr.RecordType,
			rr.DNSName,
			value,
		)
		var domainId = -1
		var foundIndex = 500
		ttl := int(rr.RecordTTL)
		// loop through zones and find the zone that closets matches the dns record.
		for _, zone := range zones {
			var newIndex = strings.Index(rr.DNSName, zone.Name)
			// index can never be -1 in the if statement as it must also be a suffix
			if strings.HasSuffix(rr.DNSName, zone.Name) && newIndex < foundIndex {
				domainId = zone.ID
				foundIndex = newIndex
			}
		}
		if domainId == -1 {
			log.Errorf("Failed to find zone for  RR %v %v  [%v]-> %v", rr.RecordType, rr.DNSName, ttl, value)
			return nil
		}

		err := e.client.CreateRecord(e.context, domainId, rr.RecordType, rr.DNSName, value, ttl)

		if err != nil {
			log.Errorf("Creation of the RR %v %v  [%v]-> %v : failed! %v", rr.RecordType, rr.DNSName, ttl, value, err)
		}
	}
	return nil
}
