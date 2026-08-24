package dnsprovider

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/whinis/external-dns-vegadns-webhook/cmd/webhook/init/configuration"
	VegasDNS "github.com/whinis/external-dns-vegadns-webhook/internal/vegadns"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"

	log "github.com/sirupsen/logrus"
)

// nolint: revive
func Init(config configuration.Config) (provider.Provider, error) {
	var domainFilter endpoint.DomainFilter
	createMsg := "Creating VegasDNS provider with "

	if config.RegexDomainFilter != "" {
		createMsg += fmt.Sprintf("regexp domain filter: '%s', ", config.RegexDomainFilter)
		if config.RegexDomainExclusion != "" {
			createMsg += fmt.Sprintf("with exclusion: '%s', ", config.RegexDomainExclusion)
		}
		domainFilter = endpoint.NewRegexDomainFilter(
			regexp.MustCompile(config.RegexDomainFilter),
			regexp.MustCompile(config.RegexDomainExclusion),
		)
	} else {
		if config.DomainFilter != nil && len(config.DomainFilter) > 0 {
			createMsg += fmt.Sprintf("domain filter: '%s', ", strings.Join(config.DomainFilter, ","))
		}
		if config.ExcludeDomains != nil && len(config.ExcludeDomains) > 0 {
			createMsg += fmt.Sprintf("exclude domain filter: '%s', ", strings.Join(config.ExcludeDomains, ","))
		}
		domainFilter = endpoint.NewDomainFilterWithExclusions(config.DomainFilter, config.ExcludeDomains)
	}

	createMsg = strings.TrimSuffix(createMsg, ", ")
	if strings.HasSuffix(createMsg, "with ") {
		createMsg += "no kind of domain filters"
	}
	log.Info(createMsg)

	vdnsConfig := VegasDNS.VegasDNSConfig{}
	if err := env.Parse(&vdnsConfig); err != nil {
		return nil, fmt.Errorf("reading configuration failed: %v", err)
	} else {
		if vdnsConfig.Token == "" || vdnsConfig.Secret == "" {
			return nil, fmt.Errorf("missing authentication credentials. access token/secret are required")
		}
	}
	vdnsConfig.HTTPClient = &http.Client{}
	vdnsConfig.FQDNRegEx = config.RegexDomainFilter
	vdnsConfig.NameRegEx = config.RegexNameFilter

	return VegasDNS.NewVegasDNSProvider(&vdnsConfig, domainFilter)
}
