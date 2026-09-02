package lookup

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// performWhoisWithFallback queries WHOIS with fallback to direct authoritative TLD server lookup.
func performWhoisWithFallback(ctx context.Context, domain string) (string, error) {
	c := whois.NewClient()
	c.SetTimeout(5 * time.Second)

	result, err := c.Whois(domain)
	if err == nil {
		return result, nil
	}

	slog.Warn("standard whois lookup failed, attempting authoritative fallback",
		"domain", domain,
		"error", err,
	)

	serverHost, serverErr := getWhoisServerForDomain(ctx, domain)
	if serverErr != nil {
		slog.Error("could not locate authoritative whois server",
			"domain", domain,
			"error", serverErr,
		)
		return "", fmt.Errorf("resolving whois server: %w", serverErr)
	}

	resolver := net.DefaultResolver
	ips, resolveErr := resolver.LookupIP(ctx, "ip", serverHost)
	if resolveErr != nil {
		slog.Error("could not resolve whois server address",
			"server", serverHost,
			"error", resolveErr,
		)
		return "", fmt.Errorf("resolving whois host %q: %w", serverHost, resolveErr)
	}

	// Try IPv4 addresses first, then IPv6
	for _, ip := range ips {
		if ip.To4() != nil {
			slog.Info("querying whois via ipv4 fallback", "domain", domain, "server", ip.String())
			if res, err := queryWhoisServer(ctx, domain, ip.String()); err == nil {
				return res, nil
			}
		}
	}

	for _, ip := range ips {
		if ip.To4() == nil {
			slog.Info("querying whois via ipv6 fallback", "domain", domain, "server", ip.String())
			if res, err := queryWhoisServer(ctx, domain, ip.String()); err == nil {
				return res, nil
			}
		}
	}

	return "", fmt.Errorf("all whois query attempts failed for %q: %w", domain, err)
}

func getWhoisServerForDomain(ctx context.Context, domain string) (string, error) {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid domain %q: missing tld", domain)
	}
	tld := parts[len(parts)-1]

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", "whois.iana.org:43")
	if err != nil {
		return "", fmt.Errorf("connecting to iana whois server: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err = conn.Write([]byte(tld + "\r\n")); err != nil {
		return "", fmt.Errorf("sending query to iana whois server: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "whois:") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				return fields[1], nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading response from iana whois server: %w", err)
	}

	return "", fmt.Errorf("no authoritative whois server found for tld %q", tld)
}

func queryWhoisServer(ctx context.Context, domain, serverIP string) (string, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(serverIP, "43"))
	if err != nil {
		return "", fmt.Errorf("connecting to whois server %q: %w", serverIP, err)
	}
	defer func() {
		_ = conn.Close()
	}()

	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	if _, err = conn.Write([]byte(domain + "\r\n")); err != nil {
		return "", fmt.Errorf("sending query to whois server %q: %w", serverIP, err)
	}

	body, err := io.ReadAll(conn)
	if err != nil {
		return "", fmt.Errorf("reading response from whois server %q: %w", serverIP, err)
	}

	return string(body), nil
}

// parseWhoisData cleans, validates, and parses raw WHOIS output into a structured WhoisInfo,
// or returns nil if the domain is not found, unavailable, restricted, or rate-limited.
func parseWhoisData(raw, domain string) *WhoisInfo {
	if isNotFoundOrError(raw) {
		return nil
	}

	parsed, parseErr := whoisparser.Parse(raw)
	if parseErr == nil {
		info := formatWhois(parsed)
		if !isWhoisInfoEmpty(info) {
			return &info
		}
	}

	// Standard registry not-found / limit-exceeded errors
	if errors.Is(parseErr, whoisparser.ErrNotFoundDomain) ||
		errors.Is(parseErr, whoisparser.ErrDomainLimitExceed) ||
		errors.Is(parseErr, whoisparser.ErrBlockedDomain) ||
		errors.Is(parseErr, whoisparser.ErrDomainDataInvalid) {
		return nil
	}

	// Attempt structured fallback parsing for atypical ccTLD formats
	return fallbackParseWhois(raw, domain)
}

func isNotFoundOrError(text string) bool {
	lower := strings.ToLower(text)
	indicators := []string{
		"no match",
		"not found",
		"nincs talalat", // Hungarian (.hu) ccTLD "not found"
		"no entries found",
		"no data found",
		"not registered",
		"is available for registration",
		"is available for purchase",
		"object not found",
		"domain not found",
		"status: free",
		"status: available",
		"status: not registered",
		"the queried object does not exist",
		"nothing found",
		"query rate limit exceeded",
		"rate limit exceeded",
		"access denied",
		"ip address blocked",
		"too many requests",
		"connection refused",
		"request rejected",
		"disallowed",
		"quota exceeded",
	}

	for _, ind := range indicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}

func isWhoisInfoEmpty(info WhoisInfo) bool {
	return info.Domain == nil &&
		info.Registrar == nil &&
		info.Registrant == nil &&
		info.Admin == nil &&
		info.Tech == nil
}

func fallbackParseWhois(raw, domain string) *WhoisInfo {
	var (
		d           WhoisDomain
		reg         WhoisRegistrar
		hasDomain   bool
		hasReg      bool
		nameServers []string
		statuses    []string
	)

	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "#") {
			continue
		}

		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}

		switch key {
		case "domain", "domain name":
			d.Domain = val
			hasDomain = true
		case "registrar", "registrar name":
			reg.Name = val
			hasReg = true
		case "name server", "nserver", "nameserver":
			nameServers = append(nameServers, strings.ToLower(val))
		case "status", "domain status":
			statuses = append(statuses, val)
		case "created", "creation date", "created date":
			d.CreatedDate = val
			hasDomain = true
		case "updated", "updated date", "last modified":
			d.UpdatedDate = val
			hasDomain = true
		case "expiry", "expiration date", "registry expiry date":
			d.ExpirationDate = val
			hasDomain = true
		}
	}

	if !hasDomain && !hasReg && len(nameServers) == 0 {
		return nil
	}

	if d.Domain == "" {
		d.Domain = domain
	}
	d.NameServers = nameServers
	d.Status = statuses

	info := &WhoisInfo{
		Domain: &d,
	}
	if hasReg {
		info.Registrar = &reg
	}
	return info
}

func formatWhois(parsed whoisparser.WhoisInfo) WhoisInfo {
	var info WhoisInfo

	if parsed.Domain != nil {
		info.Domain = &WhoisDomain{
			ID:             parsed.Domain.ID,
			Domain:         parsed.Domain.Domain,
			WhoisServer:    parsed.Domain.WhoisServer,
			Status:         parsed.Domain.Status,
			NameServers:    parsed.Domain.NameServers,
			DNSSEC:         parsed.Domain.DNSSec,
			CreatedDate:    parsed.Domain.CreatedDate,
			UpdatedDate:    parsed.Domain.UpdatedDate,
			ExpirationDate: parsed.Domain.ExpirationDate,
		}
	}

	if parsed.Registrar != nil {
		info.Registrar = &WhoisRegistrar{
			ID:          parsed.Registrar.ID,
			Name:        parsed.Registrar.Name,
			Email:       parsed.Registrar.Email,
			Phone:       parsed.Registrar.Phone,
			ReferralURL: parsed.Registrar.ReferralURL,
		}
	}

	if parsed.Registrant != nil {
		info.Registrant = formatContact(parsed.Registrant)
	}
	if parsed.Administrative != nil {
		info.Admin = formatContact(parsed.Administrative)
	}
	if parsed.Technical != nil {
		info.Tech = formatContact(parsed.Technical)
	}

	return info
}

func formatContact(c *whoisparser.Contact) *WhoisContact {
	if c == nil {
		return nil
	}
	return &WhoisContact{
		ID:           c.ID,
		Name:         c.Name,
		Organization: c.Organization,
		Street:       c.Street,
		City:         c.City,
		Province:     c.Province,
		PostalCode:   c.PostalCode,
		Country:      c.Country,
		Phone:        c.Phone,
		Fax:          c.Fax,
		Email:        c.Email,
	}
}
