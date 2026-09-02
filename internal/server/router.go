package server

import (
	"net/http"
	"strings"

	"ipinfo/internal/lookup"
)

// newRouter builds the HTTP multiplexer with middleware applied.
func newRouter(svc *lookup.Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/favicon.ico", faviconHandler)
	mux.HandleFunc("/", rootHandler(svc))

	var handler http.Handler = mux
	handler = gzipMiddleware(handler)
	handler = loggingMiddleware(handler)
	handler = recoverMiddleware(handler)

	return handler
}

func rootHandler(svc *lookup.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(r.URL.Path, "/")
		parts := strings.Split(path, "/")
		firstPart := ""
		if len(parts) > 0 {
			firstPart = parts[0]
		}

		kind := classifyTarget(firstPart)

		switch kind {
		case KindSelf:
			handleIPLookup(w, r, svc, "")

		case KindSelfField:
			handleIPLookup(w, r, svc, firstPart)

		case KindIP:
			handleIPLookup(w, r, svc, path)

		case KindMalformedIP:
			sendJSONError(w, http.StatusBadRequest, "Please provide a valid IP address.")

		case KindASN:
			handleASNLookup(w, r, svc, firstPart)

		case KindMalformedASN:
			sendJSONError(w, http.StatusBadRequest, "Invalid ASN: must be a positive number.")

		case KindDomain:
			if len(parts) > 1 {
				sendJSONError(w, http.StatusBadRequest, "Invalid request for domain. Field lookups are not supported.")
				return
			}
			handleDomainLookup(w, r, svc, firstPart)

		case KindMalformedDomain:
			sendJSONError(w, http.StatusBadRequest, "Please provide a valid domain name.")

		case KindUnknown:
			sendJSONError(w, http.StatusBadRequest, "Please provide a valid IP address or domain name.")
		}
	}
}
