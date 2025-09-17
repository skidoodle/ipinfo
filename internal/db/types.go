package db

import "errors"

// Constants for database names and paths
const (
	CityDBName  = "GeoLite2-City"
	ASNDBName   = "GeoLite2-ASN"
	DBExtension = ".mmdb"
	CityDBPath  = CityDBName + DBExtension
	ASNDBPath   = ASNDBName + DBExtension
)

// Error messages
var (
	ErrDatabaseOpen   = errors.New("failed to open database")
	ErrDownloadFailed = errors.New("failed to download database")
)

// ASNRecord represents a record in the ASN database
type ASNRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}
