package geoip

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestGeoIPManager(t *testing.T) {
	cityPath := filepath.Join("..", "..", CityDBPath)
	asnPath := filepath.Join("..", "..", ASNDBPath)

	if _, err := os.Stat(cityPath); err != nil {
		t.Skip("city db not found locally, skipping test")
	}
	if _, err := os.Stat(asnPath); err != nil {
		t.Skip("asn db not found locally, skipping test")
	}

	mgr := NewManager(cityPath, asnPath)
	if err := mgr.Initialize(context.Background()); err != nil {
		t.Fatalf("failed to initialize manager: %v", err)
	}
	defer func() {
		_ = mgr.Close()
	}()

	testIP := net.ParseIP("8.8.8.8")
	var cityRecord CityRecord
	if err := mgr.LookupCity(testIP, &cityRecord); err != nil {
		t.Fatalf("LookupCity failed: %v", err)
	}

	var asnRecord ASNRecord
	if err := mgr.LookupASN(testIP, &asnRecord); err != nil {
		t.Fatalf("LookupASN failed: %v", err)
	}

	if asnRecord.AutonomousSystemNumber != 15169 {
		t.Logf("expected AS15169 for Google, got AS%d", asnRecord.AutonomousSystemNumber)
	}

	prefixes, ok := mgr.GetASNPrefixes(asnRecord.AutonomousSystemNumber)
	if !ok || len(prefixes) == 0 {
		t.Logf("expected prefixes for AS%d, found %d", asnRecord.AutonomousSystemNumber, len(prefixes))
	}
}
