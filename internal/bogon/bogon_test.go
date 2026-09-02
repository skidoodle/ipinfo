package bogon

import (
	"net"
	"net/netip"
	"testing"
)

func TestIsBogon(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		// Bogon IPs
		{"127.0.0.1", true},
		{"10.0.0.5", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"169.254.1.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},

		// Public IPs (not bogons)
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"9.9.9.9", false},
		{"142.250.190.46", false},
		{"2606:4700:4700::1111", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.ip)
			if got := IsBogon(addr); got != tt.want {
				t.Errorf("IsBogon(%s) = %v; want %v", tt.ip, got, tt.want)
			}

			stdIP := net.ParseIP(tt.ip)
			if got := IsBogonIP(stdIP); got != tt.want {
				t.Errorf("IsBogonIP(%s) = %v; want %v", tt.ip, got, tt.want)
			}
		})
	}
}
