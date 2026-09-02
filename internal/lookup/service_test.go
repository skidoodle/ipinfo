package lookup

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ipinfo/internal/geoip"
)

func TestLookupService(t *testing.T) {
	cityPath := filepath.Join("..", "..", geoip.CityDBPath)
	asnPath := filepath.Join("..", "..", geoip.ASNDBPath)

	if _, err := os.Stat(cityPath); err != nil {
		t.Skip("city db not found locally, skipping test")
	}
	if _, err := os.Stat(asnPath); err != nil {
		t.Skip("asn db not found locally, skipping test")
	}

	mgr := geoip.NewManager(cityPath, asnPath)
	if err := mgr.Initialize(context.Background()); err != nil {
		t.Fatalf("failed to initialize geoip manager: %v", err)
	}
	defer func() {
		_ = mgr.Close()
	}()

	svc, err := NewService(mgr, 10*time.Minute)
	if err != nil {
		t.Fatalf("failed to create lookup service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := svc.LookupIP(ctx, net.ParseIP("8.8.8.8"))
	if err != nil {
		t.Fatalf("LookupIP failed: %v", err)
	}
	if data.Country == nil || *data.Country != "US" {
		t.Errorf("expected Country US, got %v", data.Country)
	}

	asnData, err := svc.LookupASN(ctx, 15169)
	if err != nil {
		t.Fatalf("LookupASN failed: %v", err)
	}
	if asnData.Details.ASN != 15169 {
		t.Errorf("expected ASN 15169, got %d", asnData.Details.ASN)
	}
}
