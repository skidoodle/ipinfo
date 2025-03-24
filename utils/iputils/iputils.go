package utils

import (
	"net"
)

// Contains a list of known bogon IP ranges
var BogonNets = []*net.IPNet{
	// IPv4
	{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(8, 32)},          // "This" network
	{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},         // Private-use networks
	{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)},      // Carrier-grade NAT
	{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)},        // Loopback
	{IP: net.IPv4(127, 0, 53, 53), Mask: net.CIDRMask(32, 32)},     // Name collision occurrence
	{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)},     // Link-local
	{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},      // Private-use networks
	{IP: net.IPv4(192, 0, 0, 0), Mask: net.CIDRMask(24, 32)},       // IETF protocol assignments
	{IP: net.IPv4(192, 0, 2, 0), Mask: net.CIDRMask(24, 32)},       // TEST-NET-1
	{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)},     // Private-use networks
	{IP: net.IPv4(198, 18, 0, 0), Mask: net.CIDRMask(15, 32)},      // Network interconnect device benchmark testing
	{IP: net.IPv4(198, 51, 100, 0), Mask: net.CIDRMask(24, 32)},    // TEST-NET-2
	{IP: net.IPv4(203, 0, 113, 0), Mask: net.CIDRMask(24, 32)},     // TEST-NET-3
	{IP: net.IPv4(224, 0, 0, 0), Mask: net.CIDRMask(4, 32)},        // Multicast
	{IP: net.IPv4(240, 0, 0, 0), Mask: net.CIDRMask(4, 32)},        // Reserved for future use
	{IP: net.IPv4(255, 255, 255, 255), Mask: net.CIDRMask(32, 32)}, // Limited broadcast
	// IPv6
	{IP: net.ParseIP("::"), Mask: net.CIDRMask(128, 128)},        // Node-scope unicast unspecified address
	{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},       // Node-scope unicast loopback address
	{IP: net.ParseIP("::"), Mask: net.CIDRMask(96, 128)},         // IPv4-compatible addresses
	{IP: net.ParseIP("100::"), Mask: net.CIDRMask(64, 128)},      // Remotely triggered black hole addresses
	{IP: net.ParseIP("2001:10::"), Mask: net.CIDRMask(28, 128)},  // Overlay routable cryptographic hash identifiers (ORCHID)
	{IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(32, 128)}, // Documentation prefix
	{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)},      // Unique local addresses (ULA)
	{IP: net.ParseIP("fe80::"), Mask: net.CIDRMask(10, 128)},     // Link-local unicast
	{IP: net.ParseIP("fec0::"), Mask: net.CIDRMask(10, 128)},     // Site-local unicast (deprecated)
	{IP: net.ParseIP("ff00::"), Mask: net.CIDRMask(8, 128)},      // Multicast
	// Additional Bogon Ranges
	{IP: net.ParseIP("2002::"), Mask: net.CIDRMask(24, 128)},             // 6to4 bogon (0.0.0.0/8)
	{IP: net.ParseIP("2002:a00::"), Mask: net.CIDRMask(24, 128)},         // 6to4 bogon (10.0.0.0/8)
	{IP: net.ParseIP("2002:7f00::"), Mask: net.CIDRMask(24, 128)},        // 6to4 bogon (127.0.0.0/8)
	{IP: net.ParseIP("2002:a9fe::"), Mask: net.CIDRMask(32, 128)},        // 6to4 bogon (169.254.0.0/16)
	{IP: net.ParseIP("2002:ac10::"), Mask: net.CIDRMask(28, 128)},        // 6to4 bogon (172.16.0.0/12)
	{IP: net.ParseIP("2002:c000::"), Mask: net.CIDRMask(40, 128)},        // 6to4 bogon (192.0.0.0/24)
	{IP: net.ParseIP("2002:c000:200::"), Mask: net.CIDRMask(40, 128)},    // 6to4 bogon (192.0.2.0/24)
	{IP: net.ParseIP("2002:c0a8::"), Mask: net.CIDRMask(32, 128)},        // 6to4 bogon (192.168.0.0/16)
	{IP: net.ParseIP("2002:c612::"), Mask: net.CIDRMask(31, 128)},        // 6to4 bogon (198.18.0.0/15)
	{IP: net.ParseIP("2002:c633:6400::"), Mask: net.CIDRMask(40, 128)},   // 6to4 bogon (198.51.100.0/24)
	{IP: net.ParseIP("2002:cb00:7100::"), Mask: net.CIDRMask(40, 128)},   // 6to4 bogon (203.0.113.0/24)
	{IP: net.ParseIP("2002:e000::"), Mask: net.CIDRMask(20, 128)},        // 6to4 bogon (224.0.0.0/4)
	{IP: net.ParseIP("2002:f000::"), Mask: net.CIDRMask(20, 128)},        // 6to4 bogon (240.0.0.0/4)
	{IP: net.ParseIP("2002:ffff:ffff::"), Mask: net.CIDRMask(48, 128)},   // 6to4 bogon (255.255.255.255/32)
	{IP: net.ParseIP("2001::"), Mask: net.CIDRMask(40, 128)},             // Teredo bogon (0.0.0.0/8)
	{IP: net.ParseIP("2001:0:a00::"), Mask: net.CIDRMask(40, 128)},       // Teredo bogon (10.0.0.0/8)
	{IP: net.ParseIP("2001:0:7f00::"), Mask: net.CIDRMask(40, 128)},      // Teredo bogon (127.0.0.0/8)
	{IP: net.ParseIP("2001:0:a9fe::"), Mask: net.CIDRMask(48, 128)},      // Teredo bogon (169.254.0.0/16)
	{IP: net.ParseIP("2001:0:ac10::"), Mask: net.CIDRMask(44, 128)},      // Teredo bogon (172.16.0.0/12)
	{IP: net.ParseIP("2001:0:c000::"), Mask: net.CIDRMask(56, 128)},      // Teredo bogon (192.0.0.0/24)
	{IP: net.ParseIP("2001:0:c000:200::"), Mask: net.CIDRMask(56, 128)},  // Teredo bogon (192.0.2.0/24)
	{IP: net.ParseIP("2001:0:c0a8::"), Mask: net.CIDRMask(48, 128)},      // Teredo bogon (192.168.0.0/16)
	{IP: net.ParseIP("2001:0:c612::"), Mask: net.CIDRMask(47, 128)},      // Teredo bogon (198.18.0.0/15)
	{IP: net.ParseIP("2001:0:c633:6400::"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (198.51.100.0/24)
	{IP: net.ParseIP("2001:0:cb00:7100::"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (203.0.113.0/24)
	{IP: net.ParseIP("2001:0:e000::"), Mask: net.CIDRMask(36, 128)},      // Teredo bogon (224.0.0.0/4)
	{IP: net.ParseIP("2001:0:f000::"), Mask: net.CIDRMask(36, 128)},      // Teredo bogon (240.0.0.0/4)
	{IP: net.ParseIP("2001:0:ffff:ffff::"), Mask: net.CIDRMask(64, 128)}, // Teredo bogon (255.255.255.255/32)
}
