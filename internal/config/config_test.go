package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()
	if cfg.Addr != ":3000" && cfg.Addr != os.Getenv("HOST")+":"+cmpOr(os.Getenv("PORT"), "3000") {
		t.Fatalf("unexpected Addr: %s", cfg.Addr)
	}
	if cfg.UpdateInterval != 24*time.Hour {
		t.Fatalf("expected 24h, got %v", cfg.UpdateInterval)
	}
	if cfg.CacheTTL != 10*time.Minute {
		t.Fatalf("expected 10m, got %v", cfg.CacheTTL)
	}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
