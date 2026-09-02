// Package bogon provides detection for bogon (reserved, private, and unroutable) IP addresses.
package bogon

import (
	"net"
	"net/netip"
)

// bogonPrefixes defines the list of known bogon networks.
// Using netip.Prefix ensures zero heap allocations during containment checks.
var bogonPrefixes = []netip.Prefix{
	// IPv4
	netip.MustParsePrefix("0.0.0.0/8"),          // "This" network
	netip.MustParsePrefix("10.0.0.0/8"),         // Private-use networks
	netip.MustParsePrefix("100.64.0.0/10"),      // Carrier-grade NAT
	netip.MustParsePrefix("127.0.0.0/8"),        // Loopback
	netip.MustParsePrefix("169.254.0.0/16"),     // Link-local
	netip.MustParsePrefix("172.16.0.0/12"),      // Private-use networks
	netip.MustParsePrefix("192.0.0.0/24"),       // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),       // TEST-NET-1
	netip.MustParsePrefix("192.168.0.0/16"),     // Private-use networks
	netip.MustParsePrefix("198.18.0.0/15"),      // Network interconnect device benchmark testing
	netip.MustParsePrefix("198.51.100.0/24"),    // TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),     // TEST-NET-3
	netip.MustParsePrefix("224.0.0.0/4"),        // Multicast
	netip.MustParsePrefix("240.0.0.0/4"),        // Reserved for future use
	netip.MustParsePrefix("255.255.255.255/32"), // Limited broadcast

	// IPv6
	netip.MustParsePrefix("::/128"),        // Node-scope unicast unspecified address
	netip.MustParsePrefix("::1/128"),       // Node-scope unicast loopback address
	netip.MustParsePrefix("100::/64"),      // Remotely triggered black hole addresses
	netip.MustParsePrefix("2001:10::/28"),  // Overlay routable cryptographic hash identifiers (ORCHID)
	netip.MustParsePrefix("2001:db8::/32"), // Documentation prefix
	netip.MustParsePrefix("fc00::/7"),      // Unique local addresses (ULA)
	netip.MustParsePrefix("fe80::/10"),     // Link-local unicast
	netip.MustParsePrefix("fec0::/10"),     // Site-local unicast (deprecated)
	netip.MustParsePrefix("ff00::/8"),      // Multicast

	// 6to4 Bogon Ranges
	netip.MustParsePrefix("2002::/24"),
	netip.MustParsePrefix("2002:a00::/24"),
	netip.MustParsePrefix("2002:7f00::/24"),
	netip.MustParsePrefix("2002:a9fe::/32"),
	netip.MustParsePrefix("2002:ac10::/28"),
	netip.MustParsePrefix("2002:c000::/40"),
	netip.MustParsePrefix("2002:c000:200::/40"),
	netip.MustParsePrefix("2002:c0a8::/32"),
	netip.MustParsePrefix("2002:c612::/31"),
	netip.MustParsePrefix("2002:c633:6400::/40"),
	netip.MustParsePrefix("2002:cb00:7100::/40"),
	netip.MustParsePrefix("2002:e000::/20"),
	netip.MustParsePrefix("2002:f000::/20"),
	netip.MustParsePrefix("2002:ffff:ffff::/48"),

	// Teredo Bogon Ranges
	netip.MustParsePrefix("2001::/40"),
	netip.MustParsePrefix("2001:0:a00::/40"),
	netip.MustParsePrefix("2001:0:7f00::/40"),
	netip.MustParsePrefix("2001:0:a9fe::/48"),
	netip.MustParsePrefix("2001:0:ac10::/44"),
	netip.MustParsePrefix("2001:0:c000::/56"),
	netip.MustParsePrefix("2001:0:c000:200::/56"),
	netip.MustParsePrefix("2001:0:c0a8::/48"),
	netip.MustParsePrefix("2001:0:c612::/47"),
	netip.MustParsePrefix("2001:0:c633:6400::/56"),
	netip.MustParsePrefix("2001:0:cb00:7100::/56"),
	netip.MustParsePrefix("2001:0:e000::/36"),
	netip.MustParsePrefix("2001:0:f000::/36"),
	netip.MustParsePrefix("2001:0:ffff:ffff::/64"),
}

// IsBogon checks whether a netip.Addr falls within any bogon prefix.
func IsBogon(addr netip.Addr) bool {
	// Unmap IPv4-in-IPv6 to check against IPv4 bogons accurately
	unmapped := addr.Unmap()
	for _, prefix := range bogonPrefixes {
		if prefix.Contains(unmapped) {
			return true
		}
	}
	return false
}

// IsBogonIP is a convenience wrapper for standard library net.IP values.
func IsBogonIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	return IsBogon(addr)
}
