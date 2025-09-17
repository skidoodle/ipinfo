package server

import (
	"net"
	"net/http"
	"strings"

	"ipinfo/internal/db"
	"ipinfo/utils"
)

// newRouter creates the main request router and applies middleware.
func newRouter(geoIP *db.GeoIPManager) http.Handler {
	mux := http.NewServeMux()

	// Register handlers
	mux.Handle("/health", utils.HealthCheck())
	mux.HandleFunc("/favicon.ico", faviconHandler)
	mux.HandleFunc("/", rootHandler(geoIP))

	// Chain middleware
	var handler http.Handler = mux
	handler = loggingMiddleware(handler)

	return handler
}

// rootHandler is the main routing logic that inspects the path.
func rootHandler(geoIP *db.GeoIPManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Apply gzip compression where accepted
		w = newGzipResponseWriter(w, r)
		if gw, ok := w.(gzipResponseWriter); ok {
			defer gw.Close()
		}

		path := strings.Trim(r.URL.Path, "/")
		parts := strings.Split(path, "/")
		firstPart := ""
		if len(parts) > 0 {
			firstPart = parts[0]
		}

		// Route to ASN handler
		if strings.HasPrefix(strings.ToLower(firstPart), "as") {
			handleASNLookup(w, r, path, geoIP)
			return
		}

		// Route to Domain handler
		isDomain := strings.Contains(firstPart, ".") && net.ParseIP(firstPart) == nil && firstPart != ""
		if isDomain {
			if len(parts) > 1 {
				sendJSONError(w, "Invalid request for domain. Field lookups are not supported.", http.StatusBadRequest)
				return
			}
			handleDomainLookup(w, r, firstPart)
			return
		}

		// Default to IP handler
		handleIPLookup(w, r, path, geoIP)
	}
}
