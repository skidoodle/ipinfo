package config

import (
	"cmp"
	"os"
	"time"
)

// Config holds application configuration settings.
type Config struct {
	Addr           string
	UpdateInterval time.Duration
	CacheTTL       time.Duration
	CityDBPath     string
	ASNDBPath      string
}

// Load loads configuration from environment variables with sensible defaults.
func Load() Config {
	port := cmp.Or(os.Getenv("PORT"), "3000")
	host := os.Getenv("HOST")
	addr := host + ":" + port
	if host == "" {
		addr = ":" + port
	}

	updateInterval := 24 * time.Hour
	if v := os.Getenv("UPDATE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			updateInterval = d
		}
	}

	cacheTTL := 10 * time.Minute
	if v := os.Getenv("CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cacheTTL = d
		}
	}

	cityDBPath := cmp.Or(os.Getenv("CITY_DB_PATH"), "dbip-city-lite.mmdb")
	asnDBPath := cmp.Or(os.Getenv("ASN_DB_PATH"), "dbip-asn-lite.mmdb")

	return Config{
		Addr:           addr,
		UpdateInterval: updateInterval,
		CacheTTL:       cacheTTL,
		CityDBPath:     cityDBPath,
		ASNDBPath:      asnDBPath,
	}
}
