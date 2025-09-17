package server

import (
	"net"
	"net/http"
	"strings"

	"ipinfo/internal/common"
)

// fieldMap maps request fields to their corresponding data struct fields.
var fieldMap = map[string]func(*common.DataStruct) *string{
	"ip":       func(d *common.DataStruct) *string { return d.IP },
	"hostname": func(d *common.DataStruct) *string { return d.Hostname },
	"org":      func(d *common.DataStruct) *string { return d.Org },
	"city":     func(d *common.DataStruct) *string { return d.City },
	"region":   func(d *common.DataStruct) *string { return d.Region },
	"country":  func(d *common.DataStruct) *string { return d.Country },
	"timezone": func(d *common.DataStruct) *string { return d.Timezone },
	"loc":      func(d *common.DataStruct) *string { return d.Loc },
}

// getField retrieves the value of a specific field from the data struct.
func getField(data *common.DataStruct, field string) *string {
	if f, ok := fieldMap[field]; ok {
		return f(data)
	}
	return nil
}

// GetRealIP extracts the client's real IP address from request headers.
func GetRealIP(r *http.Request) string {
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For"} {
		if ip := r.Header.Get(header); ip != "" {
			return strings.TrimSpace(strings.Split(ip, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
