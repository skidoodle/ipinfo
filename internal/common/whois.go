package common

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/likexian/whois"
	"github.com/likexian/whois-parser"
)

// performWhoisWithFallback attempts a WHOIS query and falls back to IPv4 if it suspects an IPv6 issue.
func performWhoisWithFallback(domain string) (string, error) {
	result, err := whois.Whois(domain)
	if err == nil {
		return result, nil
	}

	if strings.Contains(err.Error(), "dial tcp [") && strings.Contains(err.Error(), "]:43") {
		slog.Warn("whois failed with potential ipv6 issue, falling back to ipv4", "domain", domain, "err", err)

		serverHost, serverErr := getWhoisServerForDomain(domain)
		if serverErr != nil {
			slog.Error("could not find whois server during fallback", "domain", domain, "err", serverErr)
			return "", err
		}

		ips, resolveErr := net.LookupIP(serverHost)
		if resolveErr != nil {
			slog.Error("could not resolve whois server hostname during fallback", "server", serverHost, "err", resolveErr)
			return "", err
		}

		for _, ip := range ips {
			if ip.To4() != nil {
				ipv4Server := ip.String()
				slog.Info("retrying whois query with explicit ipv4 address", "domain", domain, "server", ipv4Server)
				return queryWhoisServer(domain, ipv4Server)
			}
		}
		slog.Warn("no ipv4 address found for whois server during fallback", "server", serverHost)
	}

	return "", err
}

// getWhoisServerForDomain finds the authoritative WHOIS server for a domain by querying IANA.
func getWhoisServerForDomain(domain string) (string, error) {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid domain: %s", domain)
	}
	tld := parts[len(parts)-1]

	conn, err := net.Dial("tcp", "whois.iana.org:43")
	if err != nil {
		return "", fmt.Errorf("could not connect to iana whois server: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Warn("error closing connection to iana whois server", "err", err)
		}
	}()

	_, err = conn.Write([]byte(tld + "\r\n"))
	if err != nil {
		return "", fmt.Errorf("could not send query to iana whois server: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "whois:") {
			serverParts := strings.Fields(line)
			if len(serverParts) > 1 {
				return serverParts[1], nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading from iana whois server: %w", err)
	}
	return "", fmt.Errorf("could not find whois server for TLD: %s", tld)
}

// queryWhoisServer manually performs a WHOIS query to a specific server IP.
func queryWhoisServer(domain, serverIP string) (string, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(serverIP, "43"), 10*time.Second)
	if err != nil {
		return "", fmt.Errorf("could not connect to %s: %w", serverIP, err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			slog.Warn("error closing connection to whois server", "server", serverIP, "err", err)
		}
	}()

	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	_, err = conn.Write([]byte(domain + "\r\n"))
	if err != nil {
		return "", fmt.Errorf("could not send query to %s: %w", serverIP, err)
	}

	body, err := io.ReadAll(conn)
	if err != nil {
		return "", fmt.Errorf("could not read response from %s: %w", serverIP, err)
	}
	return string(body), nil
}

// formatWhois converts a parsed whois object to the simplified WhoisInfo struct.
func formatWhois(parsed whoisparser.WhoisInfo) WhoisInfo {
	info := WhoisInfo{}
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
		info.Registrant = &WhoisContact{
			ID:           parsed.Registrant.ID,
			Name:         parsed.Registrant.Name,
			Organization: parsed.Registrant.Organization,
			Street:       parsed.Registrant.Street,
			City:         parsed.Registrant.City,
			Province:     parsed.Registrant.Province,
			PostalCode:   parsed.Registrant.PostalCode,
			Country:      parsed.Registrant.Country,
			Phone:        parsed.Registrant.Phone,
			Fax:          parsed.Registrant.Fax,
			Email:        parsed.Registrant.Email,
		}
	}
	if parsed.Administrative != nil {
		info.Admin = &WhoisContact{
			ID:           parsed.Administrative.ID,
			Name:         parsed.Administrative.Name,
			Organization: parsed.Administrative.Organization,
			Street:       parsed.Administrative.Street,
			City:         parsed.Administrative.City,
			Province:     parsed.Administrative.Province,
			PostalCode:   parsed.Administrative.PostalCode,
			Country:      parsed.Administrative.Country,
			Phone:        parsed.Administrative.Phone,
			Fax:          parsed.Administrative.Fax,
			Email:        parsed.Administrative.Email,
		}
	}
	if parsed.Technical != nil {
		info.Tech = &WhoisContact{
			ID:           parsed.Technical.ID,
			Name:         parsed.Technical.Name,
			Organization: parsed.Technical.Organization,
			Street:       parsed.Technical.Street,
			City:         parsed.Technical.City,
			Province:     parsed.Technical.Province,
			PostalCode:   parsed.Technical.PostalCode,
			Country:      parsed.Technical.Country,
			Phone:        parsed.Technical.Phone,
			Fax:          parsed.Technical.Fax,
			Email:        parsed.Technical.Email,
		}
	}
	return info
}
