package db

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// GeoIPManager manages the GeoIP databases
type GeoIPManager struct {
	cityDB       *maxminddb.Reader
	asnDB        *maxminddb.Reader
	asnPrefixMap map[uint][]*net.IPNet
	httpClient   *http.Client
	mu           sync.RWMutex
}

// NewGeoIPManager creates a new GeoIPManager
func NewGeoIPManager() (*GeoIPManager, error) {
	manager := &GeoIPManager{
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
	if err := manager.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing geoip manager: %w", err)
	}
	return manager, nil
}

// Initialize initializes the GeoIPManager by opening the database files.
func (g *GeoIPManager) Initialize() error {
	g.mu.Lock()
	cityErr := g.openDB(CityDBPath)
	asnErr := g.openDB(ASNDBPath)
	g.mu.Unlock()

	if cityErr != nil || asnErr != nil {
		slog.Info("databases missing or invalid, performing initial update")
		if err := g.UpdateDatabases(); err != nil {
			return fmt.Errorf("initial update failed: %w", err)
		}
	} else {
		g.mu.Lock()
		g.buildASNPrefixMap()
		g.mu.Unlock()
	}

	return nil
}

// Close closes the GeoIP database readers
func (g *GeoIPManager) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cityDB != nil {
		if err := g.cityDB.Close(); err != nil {
			slog.Warn("failed to close citydb", "err", err)
		}
	}
	if g.asnDB != nil {
		if err := g.asnDB.Close(); err != nil {
			slog.Warn("failed to close asndb", "err", err)
		}
	}
}

// openDB opens a MaxMind DB file.
func (g *GeoIPManager) openDB(path string) error {
	db, err := maxminddb.Open(path)
	if err != nil {
		return err
	}

	if path == CityDBPath {
		g.cityDB = db
	} else {
		g.asnDB = db
	}
	return nil
}

// buildASNPrefixMap builds a map of ASN prefixes for fast lookups.
func (g *GeoIPManager) buildASNPrefixMap() {
	slog.Info("building asn prefix map for fast lookups")
	startTime := time.Now()
	g.asnPrefixMap = make(map[uint][]*net.IPNet)
	if g.asnDB == nil {
		slog.Warn("asn database is not available, skipping prefix map build")
		return
	}
	networks := g.asnDB.Networks()
	for networks.Next() {
		var record ASNRecord
		subnet, err := networks.Network(&record)
		if err != nil {
			slog.Debug("skipping asn network due to error", "err", err)
			continue
		}
		if record.AutonomousSystemNumber > 0 {
			g.asnPrefixMap[record.AutonomousSystemNumber] = append(g.asnPrefixMap[record.AutonomousSystemNumber], subnet)
		}
	}
	slog.Info("finished building asn prefix map", "duration", time.Since(startTime))
}
