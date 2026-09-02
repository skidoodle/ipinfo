package server

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"ipinfo/internal/bogon"
	"ipinfo/internal/geoip"
	"ipinfo/internal/lookup"

	"golang.org/x/net/idna"
)

const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"></svg>`

// healthHandler returns a 200 OK health check response.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// faviconHandler returns an empty SVG favicon.
func faviconHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(faviconSVG))
}

func handleDomainLookup(w http.ResponseWriter, r *http.Request, svc *lookup.Service, domain string) {
	cleanDomain := strings.TrimSuffix(strings.TrimSpace(domain), ".")
	punycodeDomain, err := idna.ToASCII(cleanDomain)
	if err != nil || len(punycodeDomain) > 253 {
		sendJSONError(w, http.StatusBadRequest, "Please provide a valid domain name.")
		return
	}

	data, err := svc.LookupDomain(r.Context(), punycodeDomain)
	if err != nil {
		slog.Error("failed domain lookup", "domain", punycodeDomain, "error", err)
		sendJSONError(w, http.StatusInternalServerError, "Error retrieving data for domain.")
		return
	}

	// If domain returned neither WHOIS nor any DNS records, it cannot be resolved
	if data.Whois == nil && isDNSEmpty(data.DNS) {
		sendJSONError(w, http.StatusNotFound, "Could not retrieve data for the specified domain.")
		return
	}

	sendJSONResponse(w, http.StatusOK, data)
}

func isDNSEmpty(d lookup.DNSData) bool {
	return len(d.A) == 0 &&
		len(d.AAAA) == 0 &&
		d.CNAME == "" &&
		len(d.MX) == 0 &&
		len(d.TXT) == 0 &&
		len(d.NS) == 0 &&
		len(d.SOA) == 0 &&
		len(d.CAA) == 0
}

func handleASNLookup(w http.ResponseWriter, r *http.Request, svc *lookup.Service, target string) {
	upper := strings.ToUpper(strings.TrimSpace(target))
	clean := upper
	if strings.HasPrefix(upper, "ASN") {
		clean = upper[3:]
	} else if strings.HasPrefix(upper, "AS") {
		clean = upper[2:]
	}

	asnStr := strings.Trim(clean, "/ ")
	asn, err := strconv.ParseUint(asnStr, 10, 32)
	if err != nil || asn == 0 {
		sendJSONError(w, http.StatusBadRequest, "Invalid ASN: must be a positive number.")
		return
	}

	data, err := svc.LookupASN(r.Context(), uint(asn))
	if err != nil {
		if errors.Is(err, geoip.ErrASNNotFound) {
			sendJSONError(w, http.StatusNotFound, "no prefixes found for as"+asnStr+" in the database")
			return
		}
		slog.Error("failed asn lookup", "asn", asn, "error", err)
		sendJSONError(w, http.StatusInternalServerError, "Error retrieving data for ASN.")
		return
	}

	sendJSONResponse(w, http.StatusOK, data)
}

func handleIPLookup(w http.ResponseWriter, r *http.Request, svc *lookup.Service, path string) {
	parts := strings.Split(path, "/")
	var ipAddress, field string

	switch len(parts) {
	case 0:
		ipAddress = GetRealIP(r)
	case 1:
		if parts[0] == "" {
			ipAddress = GetRealIP(r)
		} else if _, ok := fieldMap[parts[0]]; ok {
			ipAddress = GetRealIP(r)
			field = parts[0]
		} else {
			ipAddress = parts[0]
		}
	case 2:
		ipAddress = parts[0]
		field = parts[1]
	default:
		sendJSONError(w, http.StatusBadRequest, "Invalid request format.")
		return
	}

	cleanIP := strings.TrimSpace(ipAddress)
	if strings.HasPrefix(cleanIP, "[") && strings.HasSuffix(cleanIP, "]") {
		cleanIP = cleanIP[1 : len(cleanIP)-1]
	}

	ip := net.ParseIP(cleanIP)
	if ip == nil {
		sendJSONError(w, http.StatusBadRequest, "Please provide a valid IP address.")
		return
	}

	if field != "" {
		if _, ok := fieldMap[field]; !ok {
			sendJSONError(w, http.StatusBadRequest, "Please provide a valid field.")
			return
		}
	}

	if bogon.IsBogonIP(ip) {
		sendJSONResponse(w, http.StatusOK, lookup.BogonData{
			IP:    ip.String(),
			Bogon: true,
		})
		return
	}

	data, err := svc.LookupIP(r.Context(), ip)
	if err != nil {
		slog.Error("could not retrieve data for ip", "ip", ipAddress, "error", err)
		sendJSONError(w, http.StatusNotFound, "Could not retrieve data for the specified IP.")
		return
	}

	if field != "" {
		val := getField(data, field)
		sendJSONResponse(w, http.StatusOK, map[string]*string{field: val})
		return
	}

	sendJSONResponse(w, http.StatusOK, data)
}
