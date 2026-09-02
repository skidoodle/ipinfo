// Package lookup orchestrates IP, ASN, and domain information retrieval with caching.
package lookup

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"ipinfo/internal/bogon"
	"ipinfo/internal/cache"
	"ipinfo/internal/geoip"

	"github.com/miekg/dns"
	"github.com/ringsaturn/tzf"
	"golang.org/x/net/publicsuffix"
)

// Service provides IP, ASN, and domain lookup services with in-memory caching.
type Service struct {
	geoIP       *geoip.Manager
	tzFinder    tzf.F
	ipCache     *cache.Cache[string, *IPData]
	asnCache    *cache.Cache[uint, *ASNData]
	domainCache *cache.Cache[string, *DomainData]
	dnsServer   string
}

// NewService creates and initializes a new lookup Service.
func NewService(geoIP *geoip.Manager, cacheTTL time.Duration) (*Service, error) {
	tzFinder, err := tzf.NewDefaultFinder()
	if err != nil {
		return nil, fmt.Errorf("initializing timezone finder: %w", err)
	}

	return &Service{
		geoIP:       geoIP,
		tzFinder:    tzFinder,
		ipCache:     cache.New[string, *IPData](cacheTTL),
		asnCache:    cache.New[uint, *ASNData](cacheTTL),
		domainCache: cache.New[string, *DomainData](cacheTTL),
		dnsServer:   "1.1.1.1:53",
	}, nil
}

// Close stops the background cache eviction workers.
func (s *Service) Close() {
	s.ipCache.Stop()
	s.asnCache.Stop()
	s.domainCache.Stop()
}

// LookupIP retrieves geolocation, ASN, reverse DNS, and timezone information for an IP address.
func (s *Service) LookupIP(ctx context.Context, ip net.IP) (*IPData, error) {
	ipStr := ip.String()
	if cached, ok := s.ipCache.Get(ipStr); ok {
		return cached, nil
	}

	var cityRecord geoip.CityRecord
	if err := s.geoIP.LookupCity(ip, &cityRecord); err != nil {
		slog.Error("failed looking up city data", "ip", ipStr, "error", err)
		return nil, fmt.Errorf("looking up city for %s: %w", ipStr, err)
	}

	var asnRecord geoip.ASNRecord
	if err := s.geoIP.LookupASN(ip, &asnRecord); err != nil {
		slog.Error("failed looking up asn data", "ip", ipStr, "error", err)
		return nil, fmt.Errorf("looking up asn for %s: %w", ipStr, err)
	}

	// Reverse DNS with a bounded timeout so slow DNS doesn't stall the request
	hostnameStr := ""
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	names, err := net.DefaultResolver.LookupAddr(lookupCtx, ipStr)
	cancel()
	if err == nil && len(names) > 0 {
		hostnameStr = strings.TrimSuffix(names[0], ".")
	}

	var region *string
	if len(cityRecord.Subdivisions) > 0 {
		region = StringPtr(cityRecord.Subdivisions[0].Names["en"])
	}

	var timezone string
	if s.tzFinder != nil && (cityRecord.Location.Latitude != 0 || cityRecord.Location.Longitude != 0) {
		timezone = s.tzFinder.GetTimezoneName(cityRecord.Location.Longitude, cityRecord.Location.Latitude)
	}

	orgStr := ""
	if asnRecord.AutonomousSystemNumber > 0 {
		orgStr = fmt.Sprintf("AS%d %s", asnRecord.AutonomousSystemNumber, asnRecord.AutonomousSystemOrganization)
	}

	locStr := fmt.Sprintf("%.4f,%.4f", cityRecord.Location.Latitude, cityRecord.Location.Longitude)

	data := &IPData{
		IP:       StringPtr(ipStr),
		Hostname: StringPtr(hostnameStr),
		Org:      StringPtr(orgStr),
		City:     StringPtr(cityRecord.City.Names["en"]),
		Region:   region,
		Country:  StringPtr(cityRecord.Country.IsoCode),
		Timezone: StringPtr(timezone),
		Loc:      StringPtr(locStr),
	}

	s.ipCache.Set(ipStr, data)
	return data, nil
}

// LookupASN retrieves organization and advertised prefix information for an autonomous system number.
func (s *Service) LookupASN(_ context.Context, asn uint) (*ASNData, error) {
	if cached, ok := s.asnCache.Get(asn); ok {
		return cached, nil
	}

	prefixes, ok := s.geoIP.GetASNPrefixes(asn)
	if !ok || len(prefixes) == 0 {
		return nil, geoip.ErrASNNotFound
	}

	var orgName string
	var record geoip.ASNRecord
	firstIP := net.ParseIP(prefixes[0].Addr().String())
	if err := s.geoIP.LookupASN(firstIP, &record); err == nil {
		orgName = record.AutonomousSystemOrganization
	}

	ipv4Prefixes := make([]string, 0, len(prefixes))
	ipv6Prefixes := make([]string, 0, len(prefixes))

	for _, p := range prefixes {
		if bogon.IsBogon(p.Addr()) {
			continue
		}

		pStr := p.String()
		if p.Addr().Is6() {
			ipv6Prefixes = append(ipv6Prefixes, pStr)
		} else {
			ipv4Prefixes = append(ipv4Prefixes, pStr)
		}
	}

	slices.Sort(ipv4Prefixes)
	slices.Sort(ipv6Prefixes)

	res := &ASNData{
		Details: ASNDetails{
			ASN:  asn,
			Name: orgName,
		},
		Prefixes: PrefixInfo{
			IPv4: ipv4Prefixes,
			IPv6: ipv6Prefixes,
		},
	}

	s.asnCache.Set(asn, res)
	return res, nil
}

// LookupDomain retrieves WHOIS and DNS records for a domain.
func (s *Service) LookupDomain(ctx context.Context, domain string) (*DomainData, error) {
	if cached, ok := s.domainCache.Get(domain); ok {
		return cached, nil
	}

	eTLD, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return nil, fmt.Errorf("invalid domain name %q: %w", domain, err)
	}

	var whoisResult *WhoisInfo
	whoisRaw, err := performWhoisWithFallback(ctx, eTLD)
	if err != nil {
		slog.Warn("whois query failed", "domain", eTLD, "error", err)
	} else {
		whoisResult = parseWhoisData(whoisRaw, eTLD)
	}

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

	var (
		dnsData DNSData
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for key, rType := range recordTypes {
		wg.Add(1)
		go func(typeName string, recordType uint16) {
			defer wg.Done()

			answers, qErr := s.queryDNS(ctx, domain, recordType)
			if qErr != nil {
				slog.Debug("dns query failed", "type", typeName, "domain", domain, "error", qErr)
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

	// Sort MX records by preference
	slices.SortFunc(dnsData.MX, func(a, b string) int {
		var prefA, prefB int
		_, _ = fmt.Sscanf(a, "%d", &prefA)
		_, _ = fmt.Sscanf(b, "%d", &prefB)
		return cmp.Compare(prefA, prefB)
	})

	res := &DomainData{
		Whois: whoisResult,
		DNS:   dnsData,
	}

	s.domainCache.Set(domain, res)
	return res, nil
}

func (s *Service) queryDNS(ctx context.Context, domain string, recordType uint16) ([]dns.RR, error) {
	c := &dns.Client{
		Timeout: 3 * time.Second,
	}

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), recordType)
	m.RecursionDesired = true

	r, _, err := c.ExchangeContext(ctx, m, s.dnsServer)
	if err != nil {
		return nil, err
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, nil
	}

	return r.Answer, nil
}
