package internal

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	db "skidoodle/ipinfo/internal/db"
	iputils "skidoodle/ipinfo/utils/iputils"
)

type DataStruct struct {
	IP       *string `json:"ip"`
	Hostname *string `json:"hostname"`
	Org      *string `json:"org"`
	City     *string `json:"city"`
	Region   *string `json:"region"`
	Country  *string `json:"country"`
	Timezone *string `json:"timezone"`
	Loc      *string `json:"loc"`
}

type ASNDataResponse struct {
	ASNDetails    ASNDetails    `json:"asn_details"`
	Prefixes      PrefixInfo    `json:"prefixes"`
	SourceDetails SourceDetails `json:"source_details"`
}

type ASNDetails struct {
	ASN  uint   `json:"asn"`
	Name string `json:"name"`
}

type PrefixInfo struct {
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
}

type SourceDetails struct {
	Source string `json:"source"`
}

// Global caches with 10 minute TTL
var ipCache = NewIPCache(10 * time.Minute)
var asnCache = NewASNCache(10 * time.Minute)

type cachedIPData struct {
	data *DataStruct
	time time.Time
}

type cachedASNData struct {
	data *ASNDataResponse
	time time.Time
}

// IPCache provides thread-safe caching of IP lookup results
type IPCache struct {
	cache sync.Map
	ttl   time.Duration
}

// NewIPCache creates a new IP cache with the specified TTL
func NewIPCache(ttl time.Duration) *IPCache {
	return &IPCache{
		ttl: ttl,
	}
}

func (c *IPCache) Set(ipStr string, data *DataStruct) {
	c.cache.Store(ipStr, cachedIPData{
		data: data,
		time: time.Now(),
	})
}

func (c *IPCache) Get(ipStr string) (*DataStruct, bool) {
	if cachedData, ok := c.cache.Load(ipStr); ok {
		cached := cachedData.(cachedIPData)
		if time.Since(cached.time) < c.ttl {
			return cached.data, true
		}
		c.cache.Delete(ipStr)
	}
	return nil, false
}

type ASNCache struct {
	cache sync.Map
	ttl   time.Duration
}

func NewASNCache(ttl time.Duration) *ASNCache {
	return &ASNCache{
		ttl: ttl,
	}
}

func (c *ASNCache) Set(asn uint, data *ASNDataResponse) {
	c.cache.Store(asn, cachedASNData{
		data: data,
		time: time.Now(),
	})
}

func (c *ASNCache) Get(asn uint) (*ASNDataResponse, bool) {
	if cachedData, ok := c.cache.Load(asn); ok {
		cached := cachedData.(cachedASNData)
		if time.Since(cached.time) < c.ttl {
			return cached.data, true
		}
		c.cache.Delete(asn)
	}
	return nil, false
}

// LookupIPData looks up IP data in the databases with caching
func LookupIPData(geoIP *db.GeoIPManager, ip net.IP) *DataStruct {
	if data, found := ipCache.Get(ip.String()); found {
		return data
	}

	var cityRecord struct {
		City struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"city"`
		Subdivisions []struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"subdivisions"`
		Country struct {
			IsoCode string            `maxminddb:"iso_code"`
			Names   map[string]string `maxminddb:"names"`
		} `maxminddb:"country"`
		Location struct {
			Latitude  float64 `maxminddb:"latitude"`
			Longitude float64 `maxminddb:"longitude"`
			Timezone  string  `maxminddb:"time_zone"`
		} `maxminddb:"location"`
	}

	cityDB := geoIP.GetCityDB()
	err := cityDB.Lookup(ip, &cityRecord)
	if err != nil {
		log.Printf("Error looking up city data: %v", err)
		return nil
	}

	var asnRecord db.ASNRecord
	asnDB := geoIP.GetASNDB()
	err = asnDB.Lookup(ip, &asnRecord)
	if err != nil {
		log.Printf("Error looking up ASN data: %v", err)
		return nil
	}

	hostname, err := net.LookupAddr(ip.String())
	if err != nil || len(hostname) == 0 {
		hostname = []string{""}
	}

	var sd *string
	if len(cityRecord.Subdivisions) > 0 {
		sd = ToPtr(cityRecord.Subdivisions[0].Names["en"])
	}

	data := &DataStruct{
		IP:       ToPtr(ip.String()),
		Hostname: ToPtr(strings.TrimSuffix(hostname[0], ".")),
		Org:      ToPtr(fmt.Sprintf("AS%d %s", asnRecord.AutonomousSystemNumber, asnRecord.AutonomousSystemOrganization)),
		City:     ToPtr(cityRecord.City.Names["en"]),
		Region:   sd,
		Country:  ToPtr(cityRecord.Country.IsoCode),
		Timezone: ToPtr(cityRecord.Location.Timezone),
		Loc:      ToPtr(fmt.Sprintf("%.4f,%.4f", cityRecord.Location.Latitude, cityRecord.Location.Longitude)),
	}

	ipCache.Set(ip.String(), data)
	return data
}

func LookupASNData(geoIP *db.GeoIPManager, targetASN uint) (*ASNDataResponse, error) {
	if data, found := asnCache.Get(targetASN); found {
		return data, nil
	}

	prefixes := geoIP.GetASNPrefixes(targetASN)
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("no prefixes found for ASN %d in the database", targetASN)
	}

	var orgName string
	var ipv4Prefixes, ipv6Prefixes []string

	var record db.ASNRecord
	if err := geoIP.GetASNDB().Lookup(prefixes[0].IP, &record); err == nil {
		orgName = record.AutonomousSystemOrganization
	}

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
		ASNDetails: ASNDetails{
			ASN:  targetASN,
			Name: orgName,
		},
		Prefixes: PrefixInfo{
			IPv4: ipv4Prefixes,
			IPv6: ipv6Prefixes,
		},
		SourceDetails: SourceDetails{
			Source: "GeoLite2-ASN.mmdb",
		},
	}

	asnCache.Set(targetASN, response)

	return response, nil
}

// ToPtr converts string to pointer
func ToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// IsBogon checks if the IP is a bogon IP
func IsBogon(ip net.IP) bool {
	for _, net := range iputils.BogonNets {
		if net.Contains(ip) {
			return true
		}
	}
	return false
}

// GetRealIP extracts the client's real IP address from request headers
func GetRealIP(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"} {
		if ip := r.Header.Get(header); ip != "" {
			return strings.TrimSpace(strings.Split(ip, ",")[0])
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
