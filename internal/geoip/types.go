package geoip

import (
	"errors"
	"net/netip"

	"github.com/oschwald/maxminddb-golang"
)

// Default database file names and paths
const (
	CityDBName  = "dbip-city-lite"
	ASNDBName   = "dbip-asn-lite"
	DBExtension = ".mmdb"
	CityDBPath  = CityDBName + DBExtension
	ASNDBPath   = ASNDBName + DBExtension
)

// Sentinel errors for geoip lookups
var (
	ErrDatabaseUnavailable = errors.New("database reader is unavailable")
	ErrASNNotFound         = errors.New("asn not found in database")
	ErrCityNotFound        = errors.New("city record not found in database")
	ErrDatabaseOpen        = errors.New("failed to open database")
	ErrDownloadFailed      = errors.New("failed to download database")
)

// ASNRecord represents an entry in the ASN MaxMind database.
type ASNRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// CityRecord represents the location fields extracted from the City MaxMind database.
type CityRecord struct {
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

// Snapshot represents an immutable snapshot of loaded databases and the in-memory ASN prefix map.
type Snapshot struct {
	CityDB       *maxminddb.Reader
	ASNDB        *maxminddb.Reader
	ASNPrefixMap map[uint][]netip.Prefix
}

// Close closes the underlying MaxMind database readers in the snapshot.
func (s *Snapshot) Close() error {
	var errs []error
	if s.CityDB != nil {
		if err := s.CityDB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.ASNDB != nil {
		if err := s.ASNDB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
