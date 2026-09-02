package lookup

import (
	"testing"
)

func TestParseWhoisNotFoundCases(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{
			name: "Hungarian .hu ccTLD no match",
			raw:  "% Whois server 4.1 serving the hu ccTLD\n\n\r\nNincs talalat / No match\n\n% Query time: 343 msec\n% WHEN: Wed Sep 02 23:12:55 CEST 2026\n",
		},
		{
			name: "Verisign .com no match",
			raw:  "No match for domain \"NONEXISTENT12345.COM\".\n>>> Last update of whois database: 2026-09-02T21:15:00Z <<<\n",
		},
		{
			name: "ARIN no match",
			raw:  "No match found for n + 40.72.\n",
		},
		{
			name: "Rate limit exceeded",
			raw:  "% Error: Query rate limit exceeded. Please try again in 60 seconds.\n",
		},
		{
			name: "Access denied",
			raw:  "% Access denied. IP address blocked from whois service.\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseWhoisData(tc.raw, "test.domain")
			if got != nil {
				t.Fatalf("expected nil whois info for %s, got: %+v", tc.name, got)
			}
		})
	}
}

func TestFallbackParseWhois(t *testing.T) {
	raw := `
Domain Name: custom-registry.org
Registrar: Example Registrar Inc.
Creation Date: 2021-05-10T12:00:00Z
Registry Expiry Date: 2027-05-10T12:00:00Z
Name Server: ns1.example.org
Name Server: ns2.example.org
Domain Status: clientTransferProhibited
`

	info := fallbackParseWhois(raw, "custom-registry.org")
	if info == nil || info.Domain == nil {
		t.Fatal("expected non-nil whois info")
	}
	if info.Domain.Domain != "custom-registry.org" {
		t.Errorf("expected domain custom-registry.org, got %s", info.Domain.Domain)
	}
	if info.Registrar == nil || info.Registrar.Name != "Example Registrar Inc." {
		t.Errorf("expected registrar Example Registrar Inc., got %v", info.Registrar)
	}
	if len(info.Domain.NameServers) != 2 {
		t.Errorf("expected 2 nameservers, got %d", len(info.Domain.NameServers))
	}
}
