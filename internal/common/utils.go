package common

import (
	"net"

	"ipinfo/utils"
)

// ToPtr converts a string to a pointer, returning nil for empty strings.
func ToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// IsBogon checks if the IP is a bogon IP.
func IsBogon(ip net.IP) bool {
	for _, network := range utils.BogonNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
