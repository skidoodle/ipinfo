package db

import (
	"net"

	"github.com/oschwald/maxminddb-golang"
)

// GetCityDB retrieves the city database reader.
func (g *GeoIPManager) GetCityDB() *maxminddb.Reader {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.cityDB
}

// GetASNDB retrieves the ASN database reader.
func (g *GeoIPManager) GetASNDB() *maxminddb.Reader {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.asnDB
}

// GetASNPrefixes retrieves the list of IP prefixes for a given ASN.
func (g *GeoIPManager) GetASNPrefixes(asn uint) []*net.IPNet {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.asnPrefixMap[asn]
}
