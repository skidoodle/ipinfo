package server

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"ipinfo/internal/lookup"
)

var fieldMap = map[string]func(*lookup.IPData) *string{
	"ip":       func(d *lookup.IPData) *string { return d.IP },
	"hostname": func(d *lookup.IPData) *string { return d.Hostname },
	"org":      func(d *lookup.IPData) *string { return d.Org },
	"city":     func(d *lookup.IPData) *string { return d.City },
	"region":   func(d *lookup.IPData) *string { return d.Region },
	"country":  func(d *lookup.IPData) *string { return d.Country },
	"timezone": func(d *lookup.IPData) *string { return d.Timezone },
	"loc":      func(d *lookup.IPData) *string { return d.Loc },
}

func getField(data *lookup.IPData, field string) *string {
	if getter, ok := fieldMap[field]; ok {
		return getter(data)
	}
	return nil
}

// GetRealIP extracts the client IP address from proxy headers, falling back to RemoteAddr.
func GetRealIP(r *http.Request) string {
	headers := []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"}
	for _, h := range headers {
		if val := r.Header.Get(h); val != "" {
			first := strings.TrimSpace(strings.Split(val, ",")[0])
			if addr, err := netip.ParseAddr(first); err == nil {
				return addr.String()
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
