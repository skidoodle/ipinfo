package internal

import (
	"fmt"
	"log"
	"net"
	"net/http"
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

// Global IP cache with 10 minute TTL
var ipCache = NewIPCache(10 * time.Minute)

type cachedIPData struct {
	data *DataStruct
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

// Set stores an IP in the cache
func (c *IPCache) Set(ipStr string, data *DataStruct) {
	c.cache.Store(ipStr, cachedIPData{
		data: data,
		time: time.Now(),
	})
}

// Get retrieves an IP from the cache if it exists and is not expired
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

// LookupIPData looks up IP data in the databases with caching
func LookupIPData(geoIP *db.GeoIPManager, ip net.IP) *DataStruct {
	// Check cache first
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

	// Get database readers using thread-safe accessor methods
	cityDB := geoIP.GetCityDB()
	err := cityDB.Lookup(ip, &cityRecord)
	if err != nil {
		log.Printf("Error looking up city data: %v", err)
		return nil
	}

	var asnRecord struct {
		AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
		AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	}
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

	// Store in cache
	ipCache.Set(ip.String(), data)

	return data
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
	// Try common proxy headers first
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"} {
		if ip := r.Header.Get(header); ip != "" {
			return strings.TrimSpace(strings.Split(ip, ",")[0])
		}
	}

	// Fall back to remote address
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
