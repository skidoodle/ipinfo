package geoip

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// Manager manages GeoIP databases with thread-safe atomic snapshots.
type Manager struct {
	snapshot   atomic.Pointer[Snapshot]
	httpClient *http.Client
	updateMu   sync.Mutex
	cityPath   string
	asnPath    string
}

// NewManager creates a new GeoIP database manager.
func NewManager(cityPath, asnPath string) *Manager {
	cityPath = cmp.Or(cityPath, CityDBPath)
	asnPath = cmp.Or(asnPath, ASNDBPath)

	return &Manager{
		httpClient: &http.Client{Timeout: 5 * time.Minute},
		cityPath:   cityPath,
		asnPath:    asnPath,
	}
}

// Initialize attempts to load existing databases from disk. If missing or invalid,
// it performs an initial download update.
func (m *Manager) Initialize(ctx context.Context) error {
	snap, err := m.loadSnapshotFromDisk()
	if err != nil {
		slog.Info("databases missing or invalid on disk; performing initial download", "error", err)
		if updateErr := m.UpdateDatabases(ctx); updateErr != nil {
			return fmt.Errorf("initial database update failed: %w", updateErr)
		}
		return nil
	}

	m.snapshot.Store(snap)
	slog.Info("geoip databases loaded successfully")
	return nil
}

// GetSnapshot returns the current database snapshot.
func (m *Manager) GetSnapshot() *Snapshot {
	return m.snapshot.Load()
}

// LookupCity looks up city information for an IP address.
func (m *Manager) LookupCity(ip net.IP, target *CityRecord) error {
	snap := m.snapshot.Load()
	if snap == nil || snap.CityDB == nil {
		return ErrDatabaseUnavailable
	}
	return snap.CityDB.Lookup(ip, target)
}

// LookupASN looks up ASN information for an IP address.
func (m *Manager) LookupASN(ip net.IP, target *ASNRecord) error {
	snap := m.snapshot.Load()
	if snap == nil || snap.ASNDB == nil {
		return ErrDatabaseUnavailable
	}
	return snap.ASNDB.Lookup(ip, target)
}

// GetASNPrefixes retrieves the registered IP prefixes for an ASN.
func (m *Manager) GetASNPrefixes(asn uint) ([]netip.Prefix, bool) {
	snap := m.snapshot.Load()
	if snap == nil || snap.ASNPrefixMap == nil {
		return nil, false
	}
	prefixes, ok := snap.ASNPrefixMap[asn]
	return prefixes, ok
}

// Close closes the currently loaded databases.
func (m *Manager) Close() error {
	snap := m.snapshot.Swap(nil)
	if snap != nil {
		return snap.Close()
	}
	return nil
}

// loadSnapshotFromDisk reads and indexes database files from disk.
func (m *Manager) loadSnapshotFromDisk() (*Snapshot, error) {
	cityDB, err := maxminddb.Open(m.cityPath)
	if err != nil {
		return nil, fmt.Errorf("opening city db (%s): %w", m.cityPath, err)
	}

	asnDB, err := maxminddb.Open(m.asnPath)
	if err != nil {
		_ = cityDB.Close()
		return nil, fmt.Errorf("opening asn db (%s): %w", m.asnPath, err)
	}

	prefixMap := buildPrefixMap(asnDB)

	return &Snapshot{
		CityDB:       cityDB,
		ASNDB:        asnDB,
		ASNPrefixMap: prefixMap,
	}, nil
}

func buildPrefixMap(asnDB *maxminddb.Reader) map[uint][]netip.Prefix {
	start := time.Now()
	slog.Info("building asn prefix index")

	prefixMap := make(map[uint][]netip.Prefix)
	if asnDB == nil {
		return prefixMap
	}

	networks := asnDB.Networks()
	for networks.Next() {
		var record ASNRecord
		subnet, err := networks.Network(&record)
		if err != nil || record.AutonomousSystemNumber == 0 {
			continue
		}

		if prefix, err := netip.ParsePrefix(subnet.String()); err == nil {
			prefixMap[record.AutonomousSystemNumber] = append(prefixMap[record.AutonomousSystemNumber], prefix)
		}
	}

	slog.Info("finished building asn prefix index",
		"asns", len(prefixMap),
		"duration", time.Since(start),
	)
	return prefixMap
}
