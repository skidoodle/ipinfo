package server

import (
	"net"
	"net/netip"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// TargetKind represents the classified type of the requested URL target.
type TargetKind int

const (
	// KindSelf represents a request for the caller's own IP data.
	KindSelf TargetKind = iota
	// KindSelfField represents a request for a single field of the caller's own IP data.
	KindSelfField
	// KindIP represents a valid IPv4 or IPv6 address.
	KindIP
	// KindMalformedIP represents an input that was clearly intended as an IP but is invalid.
	KindMalformedIP
	// KindASN represents an autonomous system query (e.g. AS15169 or ASN15169).
	KindASN
	// KindMalformedASN represents an invalid ASN query (e.g. AS, AS-1, ASabc).
	KindMalformedASN
	// KindDomain represents a valid domain name with a recognized public suffix.
	KindDomain
	// KindMalformedDomain represents an input with dots that fails domain syntax or TLD validation.
	KindMalformedDomain
	// KindUnknown represents input that cannot be resolved as an IP, ASN, or domain.
	KindUnknown
)

// classifyTarget determines the kind of target requested in the first URL path segment.
func classifyTarget(raw string) TargetKind {
	target := strings.TrimSpace(raw)
	if target == "" {
		return KindSelf
	}

	// 1. Check if it's a known field of caller's own IP (e.g. /city, /country)
	if _, ok := fieldMap[strings.ToLower(target)]; ok {
		return KindSelfField
	}

	// 2. Strip optional IPv6 square brackets: [2001:db8::1]
	cleanTarget := target
	if strings.HasPrefix(cleanTarget, "[") && strings.HasSuffix(cleanTarget, "]") {
		cleanTarget = cleanTarget[1 : len(cleanTarget)-1]
	}

	// 3. Check for valid IP address
	if _, err := netip.ParseAddr(cleanTarget); err == nil {
		return KindIP
	}
	if net.ParseIP(cleanTarget) != nil {
		return KindIP
	}

	// 4. Check for ASN: AS<digits> or ASN<digits>
	// Note: Domains like asus.com or asia.org contain dots and must NOT be treated as ASN
	if !strings.Contains(target, ".") {
		upper := strings.ToUpper(target)
		if strings.HasPrefix(upper, "ASN") || strings.HasPrefix(upper, "AS") {
			var numStr string
			if strings.HasPrefix(upper, "ASN") {
				numStr = upper[3:]
			} else {
				numStr = upper[2:]
			}
			if len(numStr) > 0 {
				if val, err := strconv.ParseUint(numStr, 10, 32); err == nil && val > 0 {
					return KindASN
				}
			}
			return KindMalformedASN
		}
	}

	// 5. Check for malformed IP address
	if isMalformedIP(target) {
		return KindMalformedIP
	}

	// 6. Check for Domain name
	if strings.Contains(target, ".") {
		if isValidDomain(target) {
			return KindDomain
		}
		return KindMalformedDomain
	}

	return KindUnknown
}

// isMalformedIP detects attempts to pass an IPv4 or IPv6 address that fail valid parsing.
func isMalformedIP(s string) bool {
	// Colons indicate an IPv6 attempt (valid domain names never contain colons)
	if strings.Contains(s, ":") {
		return true
	}

	// Check if string consists purely of digits and dots:
	// e.g. "212.40.72", "212.40.72.", ".212.40.72", "0.0.0.256", "1.2.3.4.5"
	hasDigit := false
	hasDot := false
	allDigitsOrDots := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			hasDigit = true
		case c == '.':
			hasDot = true
		default:
			allDigitsOrDots = false
		}
	}
	if hasDigit && hasDot && allDigitsOrDots {
		return true
	}

	// Check if the last label after the final dot is purely numeric:
	// e.g. "foo.123", "sub.40.72"
	// No TLD in the DNS can be all-numeric (RFC 1123, RFC 3696)
	trimmed := strings.TrimSuffix(s, ".")
	if idx := strings.LastIndex(trimmed, "."); idx >= 0 {
		tld := trimmed[idx+1:]
		if len(tld) > 0 && isAllDigits(tld) {
			return true
		}
	}

	return false
}

// isValidDomain verifies that a domain conforms to DNS naming standards and has a recognized TLD.
func isValidDomain(s string) bool {
	clean := strings.TrimSuffix(strings.TrimSpace(s), ".")
	if len(clean) == 0 || len(clean) > 253 {
		return false
	}

	// Must contain at least one dot separating name and TLD
	if !strings.Contains(clean, ".") {
		return false
	}

	// Convert IDN to ASCII punycode if needed
	ascii, err := idna.ToASCII(clean)
	if err != nil || len(ascii) == 0 || len(ascii) > 253 {
		return false
	}

	labels := strings.Split(ascii, ".")
	if len(labels) < 2 {
		return false
	}

	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		// Labels cannot start or end with a hyphen
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		// Labels must only contain ASCII alphanumeric characters and hyphens
		for i := 0; i < len(label); i++ {
			c := label[i]
			isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
			if !isAlphaNum && c != '-' {
				return false
			}
		}
	}

	tld := labels[len(labels)-1]
	// TLD cannot be all-numeric (RFC 1123, RFC 3696)
	if isAllDigits(tld) {
		return false
	}
	// TLD must be at least 2 characters (e.g. .com, .io, .ai; single-letter TLDs like .b do not exist)
	if len(tld) < 2 {
		return false
	}

	// Must have a recognized suffix in the Public Suffix List
	suffix, icann := publicsuffix.PublicSuffix(ascii)
	return icann || strings.Contains(suffix, ".")
}

func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
