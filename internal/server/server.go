package internal

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	common "skidoodle/ipinfo/internal/common"
	db "skidoodle/ipinfo/internal/db"
	utils "skidoodle/ipinfo/utils/health"
)

type bogonDataStruct struct {
	IP    string `json:"ip"`
	Bogon bool   `json:"bogon"`
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// HTTP server and its dependencies
type Server struct {
	geoIP          *db.GeoIPManager
	server         *http.Server
	shutdownSignal chan struct{}
}

// Creates a new server with the given GeoIPManager
func NewServer(geoIP *db.GeoIPManager) *Server {
	s := &Server{
		geoIP:          geoIP,
		shutdownSignal: make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.Handle("/health", utils.HealthCheck()) // Register healthcheck
	mux.HandleFunc("/", s.handler)             // Main handler

	// Create HTTP server
	s.server = &http.Server{
		Addr:         ":3000",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	// Start the server in a goroutine
	go func() {
		log.Println("Server listening on :3000")
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal or context cancellation
	select {
	case <-s.shutdownSignal:
		log.Println("Shutdown requested internally")
	case <-ctx.Done():
		log.Println("Shutdown requested from context")
	}

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server
	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Println("Server shutdown complete")
	return nil
}

// Graceful server shutdown
func (s *Server) Shutdown() {
	close(s.shutdownSignal)
}

// Field access functions map
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

// Retrieves a field from the dataStruct using the fieldMap
func getField(data *common.DataStruct, field string) *string {
	if f, ok := fieldMap[field]; ok {
		return f(data)
	}
	return nil
}

// Processes HTTP requests
func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	requestedThings := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	// Enable gzip compression if requested
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		w = &gzipResponseWriter{Writer: gz, ResponseWriter: w}
	}

	var IPAddress, field string

	// Parse the request URL
	switch len(requestedThings) {
	case 0:
		IPAddress = common.GetRealIP(r) // Default to visitor's IP
	case 1:
		if requestedThings[0] == "" {
			IPAddress = common.GetRealIP(r) // Handle root page case
		} else if _, ok := fieldMap[requestedThings[0]]; ok {
			IPAddress = common.GetRealIP(r)
			field = requestedThings[0]
		} else if net.ParseIP(requestedThings[0]) != nil {
			IPAddress = requestedThings[0] // Valid IP provided
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

	// Validate the resolved IP
	ip := net.ParseIP(IPAddress)
	if ip == nil {
		sendJSONError(w, "Please provide a valid IP address.", http.StatusBadRequest)
		return
	}

	// Check if the IP is bogon
	if common.IsBogon(ip) {
		sendJSONResponse(w, bogonDataStruct{IP: ip.String(), Bogon: true}, http.StatusOK)
		return
	}

	// Look up IP data
	data := common.LookupIPData(s.geoIP, ip)
	if data == nil {
		sendJSONError(w, "Please provide a valid IP address.", http.StatusBadRequest)
		return
	}

	// Handle specific field requests
	if field != "" {
		value := getField(data, field)
		sendJSONResponse(w, map[string]*string{field: value}, http.StatusOK)
		return
	}

	// Default case: return full IP data
	sendJSONResponse(w, data, http.StatusOK)
}

// Sends a JSON response with the given data and status code
func sendJSONResponse(w http.ResponseWriter, data any, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

// Sends a JSON error response
func sendJSONError(w http.ResponseWriter, errMsg string, statusCode int) {
	sendJSONResponse(w, map[string]string{"error": errMsg}, statusCode)
}

// Initializes and starts the server with the given GeoIPManager
func StartServer(ctx context.Context, geoIP *db.GeoIPManager) error {
	// Create new server with the GeoIPManager
	server := NewServer(geoIP)

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Handle shutdown signals
	go func() {
		select {
		case <-sigCh:
			log.Println("Received termination signal")
			server.Shutdown()
		case <-ctx.Done():
			log.Println("Context cancelled")
			server.Shutdown()
		}
	}()

	// Start the server
	return server.Start(ctx)
}
