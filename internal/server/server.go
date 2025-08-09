package internal

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	common "skidoodle/ipinfo/internal/common"
	db "skidoodle/ipinfo/internal/db"
	"skidoodle/ipinfo/internal/logger"
	utils "skidoodle/ipinfo/utils/health"
)

// favicon is the SVG data for the favicon
const favicon = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"></svg>`

// bogonDataStruct represents the response structure for bogon IP queries
type bogonDataStruct struct {
	IP    string `json:"ip"`
	Bogon bool   `json:"bogon"`
}

// gzipResponseWriter is a wrapper for gzip compression
type gzipResponseWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

// Write writes the compressed data to the response
func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// Server represents the HTTP server
type Server struct {
	server *http.Server
}

// NewServer creates a new HTTP server
func NewServer(geoIP *db.GeoIPManager) *Server {
	mux := http.NewServeMux()
	mux.Handle("/health", utils.HealthCheck())
	mux.HandleFunc("/favicon.ico", faviconHandler)
	mux.HandleFunc("/", router(geoIP))

	// Chain the logging middleware
	var handler http.Handler = mux
	handler = loggingMiddleware(handler)

	return &Server{
		server: &http.Server{
			Addr:         ":3000",
			Handler:      handler,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// StartServer starts the HTTP server
func StartServer(ctx context.Context, geoIP *db.GeoIPManager) error {
	server := NewServer(geoIP)

	go func() {
		logger.Log.Info("Server listening", "address", server.server.Addr)
		if err := server.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	logger.Log.Info("Shutdown signal received, shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("Server shutdown failed", "error", err)
		return err
	}

	logger.Log.Info("Server shutdown complete")
	return nil
}

// router returns the HTTP request router for the GeoIP service
func router(geoIP *db.GeoIPManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			w = &gzipResponseWriter{Writer: gz, ResponseWriter: w}
		}

		path := strings.Trim(r.URL.Path, "/")
		lowerPath := strings.ToLower(path)

		if strings.HasPrefix(lowerPath, "as") {
			handleASNLookup(w, r, path, geoIP)
			return
		}

		handleIPLookup(w, r, path, geoIP)
	}
}

// faviconHandler handles requests for the favicon
func faviconHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(favicon))
}

// fieldMap maps request fields to their corresponding data struct fields
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

// getField retrieves the value of a specific field from the data struct
func getField(data *common.DataStruct, field string) *string {
	if f, ok := fieldMap[field]; ok {
		return f(data)
	}
	return nil
}

// handleASNLookup handles ASN lookup requests
func handleASNLookup(w http.ResponseWriter, _ *http.Request, path string, geoIP *db.GeoIPManager) {
	var asnStr string
	lowerPath := strings.ToLower(path)

	if strings.HasPrefix(lowerPath, "asn/") {
		asnStr = path[4:]
	} else if strings.HasPrefix(lowerPath, "as") {
		asnStr = path[2:]
	} else {
		sendJSONError(w, "Invalid ASN query format. Use /asn/<number> or /AS<number>.", http.StatusBadRequest)
		return
	}

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
			logger.Log.Error("Error looking up ASN data", "asn", asn, "error", err)
			sendJSONError(w, "Error retrieving data for ASN.", http.StatusInternalServerError)
		}
		return
	}

	sendJSONResponse(w, data, http.StatusOK)
}

// handleIPLookup handles IP lookup requests
func handleIPLookup(w http.ResponseWriter, r *http.Request, path string, geoIP *db.GeoIPManager) {
	requestedThings := strings.Split(path, "/")
	var IPAddress, field string

	switch len(requestedThings) {
	case 0:
		IPAddress = common.GetRealIP(r)
	case 1:
		if requestedThings[0] == "" {
			IPAddress = common.GetRealIP(r)
		} else if _, ok := fieldMap[requestedThings[0]]; ok {
			IPAddress = common.GetRealIP(r)
			field = requestedThings[0]
		} else if net.ParseIP(requestedThings[0]) != nil {
			IPAddress = requestedThings[0]
		} else {
			sendJSONError(w, "Please provide a valid IP address.", http.StatusBadRequest)
			return
		}
	case 2:
		IPAddress = requestedThings[0]
		if _, ok := fieldMap[requestedThings[1]]; ok {
			field = requestedThings[1]
		} else {
			sendJSONError(w, "Please provide a valid field.", http.StatusBadRequest)
			return
		}
	default:
		sendJSONError(w, "Please provide a valid IP address.", http.StatusBadRequest)
		return
	}

	ip := net.ParseIP(IPAddress)
	if ip == nil {
		sendJSONError(w, "Please provide a valid IP address.", http.StatusBadRequest)
		return
	}

	if common.IsBogon(ip) {
		sendJSONResponse(w, bogonDataStruct{IP: ip.String(), Bogon: true}, http.StatusOK)
		return
	}

	data := common.LookupIPData(geoIP, ip)
	if data == nil {
		sendJSONError(w, "Please provide a valid IP address.", http.StatusBadRequest)
		return
	}

	if field != "" {
		value := getField(data, field)
		sendJSONResponse(w, map[string]*string{field: value}, http.StatusOK)
		return
	}

	sendJSONResponse(w, data, http.StatusOK)
}

// sendJSONResponse sends a JSON response with the given data and status code.
func sendJSONResponse(w http.ResponseWriter, data any, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		logger.Log.Error("Error encoding JSON response", "error", err)
	}
}

// sendJSONError sends a JSON error response with the given message and status code.
func sendJSONError(w http.ResponseWriter, errMsg string, statusCode int) {
	sendJSONResponse(w, map[string]string{"error": errMsg}, statusCode)
}

// loggingMiddleware logs the incoming HTTP request and its duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Log.Info("HTTP request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.Duration("duration", time.Since(start)),
		)
	})
}
