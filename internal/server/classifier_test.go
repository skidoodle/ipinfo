package server

import (
	"testing"
)

func TestClassifyTarget(t *testing.T) {
	tests := []struct {
		input string
		want  TargetKind
	}{
		// Self & field queries
		{"", KindSelf},
		{"city", KindSelfField},
		{"country", KindSelfField},
		{"ip", KindSelfField},

		// Valid IPs
		{"8.8.8.8", KindIP},
		{"1.1.1.1", KindIP},
		{"127.0.0.1", KindIP},
		{"::1", KindIP},
		{"2001:db8::1", KindIP},
		{"[2001:db8::1]", KindIP},

		// Malformed IPs (the exact examples from user)
		{"212.40.72", KindMalformedIP},
		{"212.40.72.", KindMalformedIP},
		{"0.0.0.256", KindMalformedIP},
		{"1.2.3.4.5", KindMalformedIP},
		{"192.168.1.999", KindMalformedIP},
		{"fe80::1::2", KindMalformedIP},
		{".1.2.3", KindMalformedIP},
		{"127.0", KindMalformedIP},

		// ASNs
		{"AS15169", KindASN},
		{"as15169", KindASN},
		{"ASN123", KindASN},
		{"asn123", KindASN},

		// Malformed ASNs
		{"AS", KindMalformedASN},
		{"ASN", KindMalformedASN},
		{"AS0", KindMalformedASN},
		{"AS-1", KindMalformedASN},
		{"ASabc", KindMalformedASN},

		// Valid Domains
		{"example.com", KindDomain},
		{"example.com.", KindDomain},
		{"example.co.uk", KindDomain},
		{"google.com", KindDomain},
		{"asus.com", KindDomain}, // starts with 'as', but is a domain
		{"asia.org", KindDomain}, // starts with 'as', but is a domain
		{"foo.github.io", KindDomain},
		{"example.ai", KindDomain},
		{"example.bar", KindDomain},

		// Malformed Domains
		{"a.b", KindMalformedDomain}, // single-letter TLD 'b' does not exist
		{"test.invalidtld", KindMalformedDomain},
		{"-bad.com", KindMalformedDomain},
		{"bad-.com", KindMalformedDomain},
		{"bad..com", KindMalformedDomain},

		// Unknown (neither IP, ASN, nor domain)
		{"hello", KindUnknown},
		{"12345", KindUnknown},
		{"foobar", KindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := classifyTarget(tt.input)
			if got != tt.want {
				t.Errorf("classifyTarget(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
