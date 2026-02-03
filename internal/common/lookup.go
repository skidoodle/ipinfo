package common

import (
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"sync"

	"ipinfo/internal/db"

	whoisparser "github.com/likexian/whois-parser"
	"github.com/miekg/dns"
	"github.com/ringsaturn/tzf"
	"golang.org/x/net/publicsuffix"
)

var tzFinder tzf.F

func init() {
	var err error
	tzFinder, err = tzf.NewDefaultFinder()
	if err != nil {
		slog.Error("failed to initialize timezone finder", "err", err)
	}
}

// LookupIPData looks up IP data in the databases with caching.
func LookupIPData(geoIP *db.GeoIPManager, ip net.IP) *DataStruct {
	ipStr := ip.String()
	if data, found := cache.Get(ipStr); found {
		return data.(*DataStruct)
	}

	var cityRecord struct {
		City struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"city"`
		Subdivisions []struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"subdivisions"`
		Country struct {
			IsoCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
		Location struct {
			Latitude  float64 `maxminddb:"latitude"`
			Longitude float64 `maxminddb:"longitude"`
		} `maxminddb:"location"`
	}

	if err := geoIP.GetCityDB().Lookup(ip, &cityRecord); err != nil {
		slog.Error("failed to look up city data", "err", err)
		return nil
	}

	var asnRecord db.ASNRecord
	if err := geoIP.GetASNDB().Lookup(ip, &asnRecord); err != nil {
		slog.Error("failed to look up asn data", "err", err)
		return nil
	}

	hostname, _ := net.LookupAddr(ipStr)
	hostnameStr := ""
	if len(hostname) > 0 {
		hostnameStr = strings.TrimSuffix(hostname[0], ".")
	}

	var region *string
	if len(cityRecord.Subdivisions) > 0 {
		region = ToPtr(cityRecord.Subdivisions[0].Names["en"])
	}

	var timezone string
	if tzFinder != nil && (cityRecord.Location.Latitude != 0 || cityRecord.Location.Longitude != 0) {
		timezone = tzFinder.GetTimezoneName(cityRecord.Location.Longitude, cityRecord.Location.Latitude)
	}

	data := &DataStruct{
		IP:       ToPtr(ipStr),
		Hostname: ToPtr(hostnameStr),
		Org:      ToPtr(fmt.Sprintf("AS%d %s", asnRecord.AutonomousSystemNumber, asnRecord.AutonomousSystemOrganization)),
		City:     ToPtr(cityRecord.City.Names["en"]),
		Region:   region,
		Country:  ToPtr(cityRecord.Country.IsoCode),
		Timezone: ToPtr(timezone),
		Loc:      ToPtr(fmt.Sprintf("%.4f,%.4f", cityRecord.Location.Latitude, cityRecord.Location.Longitude)),
	}

	cache.Set(ipStr, data)
	return data
}

// LookupASNData looks up ASN data in the databases with caching.
func LookupASNData(geoIP *db.GeoIPManager, targetASN uint) (*ASNDataResponse, error) {
	if data, found := cache.Get(targetASN); found {
		return data.(*ASNDataResponse), nil
	}

	prefixes := geoIP.GetASNPrefixes(targetASN)
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("no prefixes found for as%d in the database", targetASN)
	}

	var orgName string
	var record db.ASNRecord
	if err := geoIP.GetASNDB().Lookup(prefixes[0].IP, &record); err == nil {
		orgName = record.AutonomousSystemOrganization
	}

	var ipv4Prefixes, ipv6Prefixes []string
	for _, prefix := range prefixes {
		if !IsBogon(prefix.IP) {
			prefixStr := prefix.String()
			if strings.Contains(prefixStr, ":") {
				ipv6Prefixes = append(ipv6Prefixes, prefixStr)
			} else {
				ipv4Prefixes = append(ipv4Prefixes, prefixStr)
			}
		}
	}
	sort.Strings(ipv4Prefixes)
	sort.Strings(ipv6Prefixes)

	response := &ASNDataResponse{
		Details: ASNDetails{
			ASN:  targetASN,
			Name: orgName,
		},
		Prefixes: ASNPrefixInfo{
			IPv4: ipv4Prefixes,
			IPv6: ipv6Prefixes,
		},
	}

	cache.Set(targetASN, response)
	return response, nil
}

// queryDns performs a DNS query for a specific type against a public resolver.
func queryDns(domain string, recordType uint16) ([]dns.RR, error) {
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), recordType)
	m.RecursionDesired = true

	r, _, err := c.Exchange(m, "1.1.1.1:53")
	if err != nil {
		return nil, err
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, nil
	}

	return r.Answer, nil
}

// LookupDomainData looks up domain data with caching.
func LookupDomainData(domain string) (*DomainDataResponse, error) {
	if data, found := cache.Get(domain); found {
		return data.(*DomainDataResponse), nil
	}

	eTLD, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain: %w", err)
	}

	whoisRaw, err := performWhoisWithFallback(eTLD)
	var whoisResult any
	if err != nil {
		slog.Error("whois lookup failed completely", "domain", eTLD, "err", err)
		whoisResult = nil
	} else {
		parsed, parseErr := whoisparser.Parse(whoisRaw)
		if parseErr != nil {
			slog.Warn("failed to parse whois data, returning raw text", "domain", eTLD, "err", parseErr)
			whoisResult = whoisRaw
		} else {
			whoisResult = formatWhois(parsed)
		}
	}

	dnsData := DNSData{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	recordTypes := map[string]uint16{
		"A":     dns.TypeA,
		"AAAA":  dns.TypeAAAA,
		"CNAME": dns.TypeCNAME,
		"MX":    dns.TypeMX,
		"TXT":   dns.TypeTXT,
		"NS":    dns.TypeNS,
		"SOA":   dns.TypeSOA,
		"CAA":   dns.TypeCAA,
	}

	for key, rType := range recordTypes {
		wg.Add(1)
		go func(name string, recordType uint16) {
			defer wg.Done()
			answers, err := queryDns(domain, recordType)
			if err != nil {
				slog.Debug("dns lookup failed for type", "type", name, "domain", domain, "err", err)
				return
			}

			mu.Lock()
			defer mu.Unlock()
			for _, ans := range answers {
				switch rr := ans.(type) {
				case *dns.A:
					dnsData.A = append(dnsData.A, rr.A.String())
				case *dns.AAAA:
					dnsData.AAAA = append(dnsData.AAAA, rr.AAAA.String())
				case *dns.CNAME:
					dnsData.CNAME = strings.TrimSuffix(rr.Target, ".")
				case *dns.MX:
					dnsData.MX = append(dnsData.MX, fmt.Sprintf("%d %s", rr.Preference, strings.TrimSuffix(rr.Mx, ".")))
				case *dns.TXT:
					dnsData.TXT = append(dnsData.TXT, strings.Join(rr.Txt, " "))
				case *dns.NS:
					dnsData.NS = append(dnsData.NS, strings.TrimSuffix(rr.Ns, "."))
				case *dns.SOA:
					soaStr := fmt.Sprintf("%s %s %d %d %d %d %d",
						strings.TrimSuffix(rr.Ns, "."), strings.TrimSuffix(rr.Mbox, "."),
						rr.Serial, rr.Refresh, rr.Retry, rr.Expire, rr.Minttl)
					dnsData.SOA = append(dnsData.SOA, soaStr)
				case *dns.CAA:
					dnsData.CAA = append(dnsData.CAA, fmt.Sprintf(`%d %s "%s"`, rr.Flag, rr.Tag, rr.Value))
				}
			}
		}(key, rType)
	}

	wg.Wait()

	// Sort MX records for consistent output
	sort.Slice(dnsData.MX, func(i, j int) bool {
		var prefI, prefJ int
		_, _ = fmt.Sscanf(dnsData.MX[i], "%d", &prefI)
		_, _ = fmt.Sscanf(dnsData.MX[j], "%d", &prefJ)
		return prefI < prefJ
	})

	response := &DomainDataResponse{
		Whois: whoisResult,
		DNS:   dnsData,
	}

	cache.Set(domain, response)
	return response, nil
}
