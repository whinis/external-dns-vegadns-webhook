# external-dns-VegasDNS-webhook

VegasDNS provider based on in-tree provider for ExternalDNS. Supported records:

| Record Type | Status     |
|-------------|------------|
| A           | supported  |
| CNAME       | supported  |
| TXT         | supported  |
| AAAA        | supported  |

## Quick start

To run the provider, you must provide the following Environment Variables:

**VegasDNS Environment Variables**:

| Environment Variable   | Default value | Required |
|------------------------|---------------|----------|
| VEGASDNS_URL           | localhost     | true     |
| VEGASDNS_TOKEN         |               | true     |   
| VEGASDNS_SECRET        |               | true     |
| VEGASDNS_MAX_RESULTS   | 1500          | false    |
| VEGASDNS_SSL_VERIFY    | true          | false    |
| VEGASDNS_DRY_RUN       | false         | false    |
| VEGASDNS_DEFAULT_TTL   | 300           | false    |

Note: The APi authentication requires either VEGASDNS_TOKEN/VEGASDNS_SECRET.

**external-dns-VegasDNS-webhook Environment Variables**:

| Environment Variable           | Default value | Required |
|--------------------------------|---------------|----------|
| SERVER_HOST                    | 0.0.0.0       | true     |
| SERVER_PORT                    | 8888          | true     |   
| SERVER_READ_TIMEOUT            |               | false    |
| SERVER_WRITE_TIMEOUT           |               | false    |
| DOMAIN_FILTER                  |               | false    |
| EXCLUDE_DOMAIN_FILTER          |               | false    |
| REGEXP_DOMAIN_FILTER           |               | false    |
| REGEXP_DOMAIN_FILTER_EXCLUSION |               | false    |
| REGEXP_NAME_FILTER             |               | false    |


## Running locally

To run provider in a local environment, you must provide all required settings through environment variables.
To run locally, set `SERVER_HOST` to `localhost`, otherwise leave it at `0.0.0.0`.
VegasDNS Provider is a simple web server with several clearly defined routers:

| Route            | Method |
|------------------|--------|
| /healthz         | GET    |
| /records         | GET    |
| /records         | POST   |
| /adjustendpoints | POST   |

#### Reading Data
Read data by HTTP GET to `/records`, see:
```shell
curl -H 'Accept: application/external.dns.webhook+json;version=1' localhost:8888/records
```
If you set DOMAIN_FILTER, DNS will return all records from this domain(s). Because the returned data for a given
domain can be large - in some cases tens of thousands of records, it is advisable to use filters to reduce the 
data to the desired result. Filters are specified via environment variables: `DOMAIN_FILTER`,`EXCLUDE_DOMAIN_FILTER`,
`REGEXP_DOMAIN_FILTER`,`REGEXP_DOMAIN_FILTER_EXCLUSION`,`REGEXP_NAME_FILTER`.

The following example demonstrates the use of a filter:
```shell
# We are looking for all records in these two domains. 
# Unfortunately, they may contain tens of thousands of records.
DOMAIN_FILTER=org.eu.cloud.example.com,org-hq.us.cloud.example.com

# If DOMAIN_FILTER is not enough, you can use regex. Once you use REGEXP_DOMAIN_FILTER, DOMAIN_FILTER will be ignored.
# In following example we restrict zones to *.eu.cloud.example.com or *.org-hq.us.cloud.example.com.
REGEXP_DOMAIN_FILTER=(eu.cloud|org-hq.us).cloud.example.com

# Finally, we filter only those records that have `my-project.org-hq` or `.us.cloud` in the name
REGEXP_NAME_FILTER=(my-project.org-hq|.us.cloud)
```

#### Writing Data

Here are the updating rules according to which the data in the DNS server will be updated:

- if updateNew is not part of Update Old , object should be created
- if updateOld is not part of Update New , object should be deleted
- if information is not present (TTL might change) , object should be updated
- if we rename the object, object should be deleted and created


Based on the rules I am providing some examples of `data.json` creating, changing and deleting records in DNS.

```shell
curl -X POST -H 'Accept: application/external.dns.webhook+json;version=1;' -H 'Content-Type: application/external.dns.webhook+json;version=1' -d @data.json localhost:8888/records
```

Create `test.cloud.example.com`
```json
{"Create":null,"UpdateOld":null,"UpdateNew":[{"dnsName":"test.cloud.example.com","targets":["1.3.2.1"],"recordType":"A","recordTTL":300}],"Delete":null}
```

Update `test.cloud.example.com` (DELETE one record `test.cloud.example.com` and CREATE two records `new-test.cloud.example.com`)
```json
{"Create":null,"UpdateOld":[{"dnsName":"test.cloud.example.com","targets":["1.3.2.1"],"recordType":"A","recordTTL":300}],"UpdateNew":[{"dnsName":"new-test.cloud.example.com","targets":["1.2.3.4","4.3.2.1"],"recordType":"A","recordTTL":300}],"Delete":null}
```

Delete `test-new.cloud.example.com`
```json
{"Create":null,"UpdateOld":[{"dnsName":"new-test.cloud.example.","targets":["1.2.3.4","4.3.2.1"],"recordType":"A","recordTTL":300}],"UpdateNew":null,"Delete":null}
```
