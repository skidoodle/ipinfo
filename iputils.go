package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
)

var bogonNets = []*net.IPNet{
	// IPv4
	{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(8, 32)}, // "This" network
	{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)}, // Private-use networks
	{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}, // Carrier-grade NAT
	{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)}, // Loopback
	{IP: net.IPv4(127, 0, 53, 53), Mask: net.CIDRMask(32, 32)}, // Name collision occurrence
	{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)}, // Link-local
	{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)}, // Private-use networks
	{IP: net.IPv4(192, 0, 0, 0), Mask: net.CIDRMask(24, 32)}, // IETF protocol assignments
	{IP: net.IPv4(192, 0, 2, 0), Mask: net.CIDRMask(24, 32)}, // TEST-NET-1
	{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)}, // Private-use networks
	{IP: net.IPv4(198, 18, 0, 0), Mask: net.CIDRMask(15, 32)}, // Network interconnect device benchmark testing
	{IP: net.IPv4(198, 51, 100, 0), Mask: net.CIDRMask(24, 32)}, // TEST-NET-2
	{IP: net.IPv4(203, 0, 113, 0), Mask: net.CIDRMask(24, 32)}, // TEST-NET-3
	{IP: net.IPv4(224, 0, 0, 0), Mask: net.CIDRMask(4, 32)}, // Multicast
	{IP: net.IPv4(240, 0, 0, 0), Mask: net.CIDRMask(4, 32)}, // Reserved for future use
	{IP: net.IPv4(255, 255, 255, 255), Mask: net.CIDRMask(32, 32)}, // Limited broadcast
	// IPv6
	{IP: net.ParseIP("::/128"), Mask: net.CIDRMask(128, 128)}, // Node-scope unicast unspecified address
	{IP: net.ParseIP("::1/128"), Mask: net.CIDRMask(128, 128)}, // Node-scope unicast loopback address
	{IP: net.ParseIP("::ffff:0:0/96"), Mask: net.CIDRMask(96, 128)}, // IPv4-mapped addresses
	{IP: net.ParseIP("::/96"), Mask: net.CIDRMask(96, 128)}, // IPv4-compatible addresses
	{IP: net.ParseIP("100::/64"), Mask: net.CIDRMask(64, 128)}, // Remotely triggered black hole addresses
	{IP: net.ParseIP("2001:10::/28"), Mask: net.CIDRMask(28, 128)}, // Overlay routable cryptographic hash identifiers (ORCHID)
	{IP: net.ParseIP("2001:db8::/32"), Mask: net.CIDRMask(32, 128)}, // Documentation prefix
	{IP: net.ParseIP("fc00::/7"), Mask: net.CIDRMask(7, 128)}, // Unique local addresses (ULA)
	{IP: net.ParseIP("fe80::/10"), Mask: net.CIDRMask(10, 128)}, // Link-local unicast
	{IP: net.ParseIP("fec0::/10"), Mask: net.CIDRMask(10, 128)}, // Site-local unicast (deprecated)
	{IP: net.ParseIP("ff00::/8"), Mask: net.CIDRMask(8, 128)}, // Multicast
	// Additional Bogon Ranges
	{IP: net.ParseIP("2002::/24"), Mask: net.CIDRMask(24, 128)}, // 6to4 bogon (0.0.0.0/8)
	{IP: net.ParseIP("2002:a00::/24"), Mask: net.CIDRMask(24, 128)}, // 6to4 bogon (10.0.0.0/8)
	{IP: net.ParseIP("2002:7f00::/24"), Mask: net.CIDRMask(24, 128)}, // 6to4 bogon (127.0.0.0/8)
	{IP: net.ParseIP("2002:a9fe::/32"), Mask: net.CIDRMask(32, 128)}, // 6to4 bogon (169.254.0.0/16)
	{IP: net.ParseIP("2002:ac10::/28"), Mask: net.CIDRMask(28, 128)}, // 6to4 bogon (172.16.0.0/12)
	{IP: net.ParseIP("2002:c000::/40"), Mask: net.CIDRMask(40, 128)}, // 6to4 bogon (192.0.0.0/24)
	{IP: net.ParseIP("2002:c000:200::/40"), Mask: net.CIDRMask(40, 128)}, // 6to4 bogon (192.0.2.0/24)
	{IP: net.ParseIP("2002:c0a8::/32"), Mask: net.CIDRMask(32, 128)}, // 6to4 bogon (192.168.0.0/16)
	{IP: net.ParseIP("2002:c612::/31"), Mask: net.CIDRMask(31, 128)}, // 6to4 bogon (198.18.0.0/15)
	{IP: net.ParseIP("2002:c633:6400::/40"), Mask: net.CIDRMask(40, 128)}, // 6to4 bogon (198.51.100.0/24)
	{IP: net.ParseIP("2002:cb00:7100::/40"), Mask: net.CIDRMask(40, 128)}, // 6to4 bogon (203.0.113.0/24)
	{IP: net.ParseIP("2002:e000::/20"), Mask: net.CIDRMask(20, 128)}, // 6to4 bogon (224.0.0.0/4)
	{IP: net.ParseIP("2002:f000::/20"), Mask: net.CIDRMask(20, 128)}, // 6to4 bogon (240.0.0.0/4)
	{IP: net.ParseIP("2002:ffff:ffff::/48"), Mask: net.CIDRMask(48, 128)}, // 6to4 bogon (255.255.255.255/32)
	{IP: net.ParseIP("2001::/40"), Mask: net.CIDRMask(40, 128)}, // Teredo bogon (0.0.0.0/8)
	{IP: net.ParseIP("2001:0:a00::/40"), Mask: net.CIDRMask(40, 128)}, // Teredo bogon (10.0.0.0/8)
	{IP: net.ParseIP("2001:0:7f00::/40"), Mask: net.CIDRMask(40, 128)}, // Teredo bogon (127.0.0.0/8)
	{IP: net.ParseIP("2001:0:a9fe::/48"), Mask: net.CIDRMask(48, 128)}, // Teredo bogon (169.254.0.0/16)
	{IP: net.ParseIP("2001:0:ac10::/44"), Mask: net.CIDRMask(44, 128)}, // Teredo bogon (172.16.0.0/12)
	{IP: net.ParseIP("2001:0:c000::/56"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (192.0.0.0/24)
	{IP: net.ParseIP("2001:0:c000:200::/56"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (192.0.2.0/24)
	{IP: net.ParseIP("2001:0:c0a8::/48"), Mask: net.CIDRMask(48, 128)}, // Teredo bogon (192.168.0.0/16)
	{IP: net.ParseIP("2001:0:c612::/47"), Mask: net.CIDRMask(47, 128)}, // Teredo bogon (198.18.0.0/15)
	{IP: net.ParseIP("2001:0:c633:6400::/56"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (198.51.100.0/24)
	{IP: net.ParseIP("2001:0:cb00:7100::/56"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (203.0.113.0/24)
	{IP: net.ParseIP("2001:0:e000::/36"), Mask: net.CIDRMask(36, 128)}, // Teredo bogon (224.0.0.0/4)
	{IP: net.ParseIP("2001:0:f000::/36"), Mask: net.CIDRMask(36, 128)}, // Teredo bogon (240.0.0.0/4)
	{IP: net.ParseIP("2001:0:ffff:ffff::/64"), Mask: net.CIDRMask(64, 128)}, // Teredo bogon (255.255.255.255/32)
}

// Check if the IP is a bogon IP
func isBogon(ip net.IP) bool {
	for _, net := range bogonNets {
		if net.Contains(ip) {
			return true
		}
	}
	return false
}

// Get the real IP address from the request headers
func getRealIP(r *http.Request) string {
	if realIP := r.Header.Get("CF-Connecting-IP"); realIP != "" {
		return realIP
	} else if realIP := r.Header.Get("X-Forwarded-For"); realIP != "" {
		return strings.Split(realIP, ",")[0]
	} else {
		return extractIP(r.RemoteAddr)
	}
}

// Lookup IP data in the databases
func lookupIPData(ip net.IP) *dataStruct {
	dbMtx.RLock()
	defer dbMtx.RUnlock()

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
		Continent struct {
			Code  string            `maxminddb:"code"`
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"continent"`
		Location struct {
			Latitude  float64 `maxminddb:"latitude"`
			Longitude float64 `maxminddb:"longitude"`
		} `maxminddb:"location"`
	}
	err := cityDB.Lookup(ip, &cityRecord)
	if err != nil {
		log.Printf("Error looking up city data: %v\n", err)
		return nil
	}

	var asnRecord struct {
		AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
		AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	}
	err = asnDB.Lookup(ip, &asnRecord)
	if err != nil {
		log.Printf("Error looking up ASN data: %v\n", err)
		return nil
	}

	hostname, err := net.LookupAddr(ip.String())
	if err != nil || len(hostname) == 0 {
		hostname = []string{""}
	}

	var sd *string
	if len(cityRecord.Subdivisions) > 0 {
		name := cityRecord.Subdivisions[0].Names["en"]
		sd = &name
	}

	return &dataStruct{
		IP:        toPtr(ip.String()),
		Hostname:  toPtr(strings.TrimSuffix(hostname[0], ".")),
		ASN:       toPtr(fmt.Sprintf("%d", asnRecord.AutonomousSystemNumber)),
		Org:       toPtr(asnRecord.AutonomousSystemOrganization),
		City:      toPtr(cityRecord.City.Names["en"]),
		Region:    sd,
		Country:   toPtr(cityRecord.Country.Names["en"]),
		Continent: toPtr(cityRecord.Continent.Names["en"]),
		Loc:       toPtr(fmt.Sprintf("%.4f,%.4f", cityRecord.Location.Latitude, cityRecord.Location.Longitude)),
	}
}

// Convert string to pointer
func toPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Validate JSONP callback name
var callbackJSONP = regexp.MustCompile(`^[a-zA-Z_\$][a-zA-Z0-9_\$]*$`)

// Extract the IP address from a string, removing unwanted characters
func extractIP(ip string) string {
	ip = strings.ReplaceAll(ip, "[", "")
	ip = strings.ReplaceAll(ip, "]", "")
	ss := strings.Split(ip, ":")
	return strings.Join(ss[:len(ss)-1], ":")
}