package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ipinfo/internal/config"
	"ipinfo/internal/geoip"
	"ipinfo/internal/lookup"
)

func setupTestServer(t *testing.T) http.Handler {
	t.Helper()

	cityPath := filepath.Join("..", "..", geoip.CityDBPath)
	asnPath := filepath.Join("..", "..", geoip.ASNDBPath)

	if _, err := os.Stat(cityPath); err != nil {
		t.Skip("city db missing, skipping test")
	}
	if _, err := os.Stat(asnPath); err != nil {
		t.Skip("asn db missing, skipping test")
	}

	mgr := geoip.NewManager(cityPath, asnPath)
	if err := mgr.Initialize(context.Background()); err != nil {
		t.Fatalf("failed to init geoip: %v", err)
	}

	svc, err := lookup.NewService(mgr, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to init lookup service: %v", err)
	}

	return newRouter(svc)
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "OK" {
		t.Fatalf("expected OK, got %q", rec.Body.String())
	}
}

func TestFaviconEndpoint(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "image/svg+xml") {
		t.Fatalf("expected image/svg+xml, got %s", ct)
	}
}

func TestBogonIPLookup(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/127.0.0.1", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if res["bogon"] != true {
		t.Fatalf("expected bogon=true, got %v", res["bogon"])
	}
}

func TestValidIPLookup(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/8.8.8.8", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var data lookup.IPData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if data.Country == nil || *data.Country != "US" {
		t.Fatalf("expected US country, got %v", data.Country)
	}
}

func TestFieldLookup(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/8.8.8.8/country", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if res["country"] != "US" {
		t.Fatalf("expected country=US, got %q", res["country"])
	}
}

func TestInvalidFieldLookup(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/8.8.8.8/invalidfield", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestInvalidIP(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/not-an-ip-or-domain", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestASNLookup(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/AS15169", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var data lookup.ASNData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if data.Details.ASN != 15169 {
		t.Fatalf("expected ASN 15169, got %d", data.Details.ASN)
	}
}

func TestNewServer(t *testing.T) {
	cfg := config.Load()
	srv := NewServer(cfg, nil)
	if srv == nil || srv.httpServer == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestMalformedIPRoutes(t *testing.T) {
	router := setupTestServer(t)

	malformedIPs := []string{
		"/212.40.72",
		"/212.40.72.",
		"/0.0.0.256",
		"/1.2.3.4.5",
	}

	for _, p := range malformedIPs {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, p, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("path %s: expected 400, got %d (body: %s)", p, rec.Code, rec.Body.String())
			}

			var res map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
				t.Fatalf("path %s: failed to unmarshal json: %v", p, err)
			}
			if res["error"] != "Please provide a valid IP address." {
				t.Fatalf("path %s: unexpected error message: %q", p, res["error"])
			}
		})
	}
}

func TestMalformedDomainRoutes(t *testing.T) {
	router := setupTestServer(t)

	malformedDomains := []string{
		"/a.b",
		"/test.invalidtld",
	}

	for _, p := range malformedDomains {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, p, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("path %s: expected 400, got %d (body: %s)", p, rec.Code, rec.Body.String())
			}

			var res map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
				t.Fatalf("path %s: failed to unmarshal json: %v", p, err)
			}
			if res["error"] != "Please provide a valid domain name." {
				t.Fatalf("path %s: unexpected error message: %q", p, res["error"])
			}
		})
	}
}

func TestMalformedASNRoutes(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/AS", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUnknownTargetRoutes(t *testing.T) {
	router := setupTestServer(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
