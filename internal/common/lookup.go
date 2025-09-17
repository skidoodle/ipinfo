package common

import (
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"
	"sync"

	"ipinfo/internal/db"

	"github.com/likexian/whois-parser"
	"golang.org/x/net/publicsuffix"
)

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
			Timezone  string  `maxminddb:"time_zone"`
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

	data := &DataStruct{
		IP:       ToPtr(ipStr),
		Hostname: ToPtr(hostnameStr),
		Org:      ToPtr(fmt.Sprintf("AS%d %s", asnRecord.AutonomousSystemNumber, asnRecord.AutonomousSystemOrganization)),
		City:     ToPtr(cityRecord.City.Names["en"]),
		Region:   region,
		Country:  ToPtr(cityRecord.Country.IsoCode),
		Timezone: ToPtr(cityRecord.Location.Timezone),
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
		prefixStr := prefix.String()
		if strings.Contains(prefixStr, ":") {
			ipv6Prefixes = append(ipv6Prefixes, prefixStr)
		} else {
			ipv4Prefixes = append(ipv4Prefixes, prefixStr)
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
	var whoisResult interface{}
	if err != nil {
		slog.Error("whois lookup failed after fallback", "domain", eTLD, "err", err)
		whoisResult = fmt.Sprintf("whois lookup failed: %v", err)
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

	lookupTasks := []func(){
		func() { // A and AAAA records
			ips, err := net.LookupIP(domain)
			if err == nil {
				mu.Lock()
				defer mu.Unlock()
				for _, ip := range ips {
					if ip.To4() != nil {
						dnsData.A = append(dnsData.A, ip.String())
					} else {
						dnsData.AAAA = append(dnsData.AAAA, ip.String())
					}
				}
			}
		},
		func() { // CNAME record
			cname, err := net.LookupCNAME(domain)
			if err == nil && cname != domain+"." && cname != "" {
				mu.Lock()
				defer mu.Unlock()
				dnsData.CNAME = strings.TrimSuffix(cname, ".")
			}
		},
		func() { // MX records
			mxs, err := net.LookupMX(domain)
			if err == nil {
				mu.Lock()
				defer mu.Unlock()
				for _, mx := range mxs {
					dnsData.MX = append(dnsData.MX, fmt.Sprintf("%d %s", mx.Pref, strings.TrimSuffix(mx.Host, ".")))
				}
			}
		},
		func() { // TXT records
			txts, err := net.LookupTXT(domain)
			if err == nil {
				mu.Lock()
				defer mu.Unlock()
				dnsData.TXT = append(dnsData.TXT, txts...)
			}
		},
		func() { // NS records
			nss, err := net.LookupNS(eTLD)
			if err == nil {
				mu.Lock()
				defer mu.Unlock()
				for _, ns := range nss {
					dnsData.NS = append(dnsData.NS, strings.TrimSuffix(ns.Host, "."))
				}
			}
		},
	}

	wg.Add(len(lookupTasks))
	for _, task := range lookupTasks {
		go func(t func()) {
			defer wg.Done()
			t()
		}(task)
	}
	wg.Wait()

	response := &DomainDataResponse{
		Whois: whoisResult,
		DNS:   dnsData,
	}

	cache.Set(domain, response)
	return response, nil
}
