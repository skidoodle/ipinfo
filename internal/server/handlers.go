package server

import (
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"ipinfo/internal/common"
	"ipinfo/internal/db"

	"golang.org/x/net/idna"
)

const favicon = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"></svg>`

// faviconHandler handles requests for the favicon.
func faviconHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(favicon))
}

// handleDomainLookup handles domain lookup requests.
func handleDomainLookup(w http.ResponseWriter, _ *http.Request, domain string) {
	punycodeDomain, err := idna.ToASCII(domain)
	if err != nil {
		sendJSONError(w, "Please provide a valid domain name.", http.StatusBadRequest)
		return
	}

	if len(punycodeDomain) > 253 {
		sendJSONError(w, "Please provide a valid domain name.", http.StatusBadRequest)
		return
	}

	data, err := common.LookupDomainData(punycodeDomain)
	if err != nil {
		slog.Error("failed to look up domain data", "domain", punycodeDomain, "error", err)
		sendJSONError(w, "Error retrieving data for domain.", http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, data, http.StatusOK)
}

// handleASNLookup handles ASN lookup requests.
func handleASNLookup(w http.ResponseWriter, _ *http.Request, path string, geoIP *db.GeoIPManager) {
	upperPath := strings.ToUpper(path)
	cleanPath := path

	if strings.HasPrefix(upperPath, "ASN") {
		cleanPath = path[3:]
	} else if strings.HasPrefix(upperPath, "AS") {
		cleanPath = path[2:]
	}

	asnStr := strings.Trim(cleanPath, "/ ")

	asn, err := strconv.ParseUint(asnStr, 10, 32)
	if err != nil || asn == 0 {
		sendJSONError(w, "Invalid ASN: must be a positive number.", http.StatusBadRequest)
		return
	}

	data, err := common.LookupASNData(geoIP, uint(asn))
	if err != nil {
		if strings.Contains(err.Error(), "no prefixes found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			slog.Error("failed to look up asn data", "asn", asn, "error", err)
			sendJSONError(w, "Error retrieving data for ASN.", http.StatusInternalServerError)
		}
		return
	}

	sendJSONResponse(w, data, http.StatusOK)
}

// handleIPLookup handles IP lookup requests.
func handleIPLookup(w http.ResponseWriter, r *http.Request, path string, geoIP *db.GeoIPManager) {
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
		sendJSONError(w, "Invalid request format.", http.StatusBadRequest)
		return
	}

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		sendJSONError(w, "Please provide a valid IP address.", http.StatusBadRequest)
		return
	}

	if field != "" {
		if _, ok := fieldMap[field]; !ok {
			sendJSONError(w, "Please provide a valid field.", http.StatusBadRequest)
			return
		}
	}

	if common.IsBogon(ip) {
		sendJSONResponse(w, bogonDataStruct{IP: ip.String(), Bogon: true}, http.StatusOK)
		return
	}

	data := common.LookupIPData(geoIP, ip)
	if data == nil {
		sendJSONError(w, "Could not retrieve data for the specified IP.", http.StatusNotFound)
		return
	}

	if field != "" {
		value := getField(data, field)
		sendJSONResponse(w, map[string]*string{field: value}, http.StatusOK)
		return
	}

	sendJSONResponse(w, data, http.StatusOK)
}
