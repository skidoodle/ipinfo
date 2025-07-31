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
	"strconv"
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

type Server struct {
	geoIP          *db.GeoIPManager
	server         *http.Server
	shutdownSignal chan struct{}
}

func NewServer(geoIP *db.GeoIPManager) *Server {
	s := &Server{
		geoIP:          geoIP,
		shutdownSignal: make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.Handle("/health", utils.HealthCheck())
	mux.HandleFunc("/", s.router)

	s.server = &http.Server{
		Addr:         ":3000",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Start(ctx context.Context) error {
	go func() {
		log.Println("Server listening on :3000")
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	select {
	case <-s.shutdownSignal:
		log.Println("Shutdown requested internally")
	case <-ctx.Done():
		log.Println("Shutdown requested from context")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	log.Println("Server shutdown complete")
	return nil
}

func (s *Server) Shutdown() {
	close(s.shutdownSignal)
}

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

func getField(data *common.DataStruct, field string) *string {
	if f, ok := fieldMap[field]; ok {
		return f(data)
	}
	return nil
}

func (s *Server) router(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		w = &gzipResponseWriter{Writer: gz, ResponseWriter: w}
	}

	path := strings.Trim(r.URL.Path, "/")
	lowerPath := strings.ToLower(path)

	if strings.HasPrefix(lowerPath, "as") {
		s.handleASNLookup(w, r, path)
		return
	}

	s.handleIPLookup(w, r, path)
}

func (s *Server) handleASNLookup(w http.ResponseWriter, _ *http.Request, path string) {
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

	data, err := common.LookupASNData(s.geoIP, uint(asn))
	if err != nil {
		if strings.Contains(err.Error(), "no prefixes found") {
			sendJSONError(w, err.Error(), http.StatusNotFound)
		} else {
			log.Printf("Error looking up ASN data for %d: %v", asn, err)
			sendJSONError(w, "Error retrieving data for ASN.", http.StatusInternalServerError)
		}
		return
	}

	sendJSONResponse(w, data, http.StatusOK)
}

func (s *Server) handleIPLookup(w http.ResponseWriter, r *http.Request, path string) {
	requestedThings := strings.Split(path, "/")
	var IPAddress, field string

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

	ip := net.ParseIP(IPAddress)
	if ip == nil {
		sendJSONError(w, "Please provide a valid IP address.", http.StatusBadRequest)
		return
	}

	if common.IsBogon(ip) {
		sendJSONResponse(w, bogonDataStruct{IP: ip.String(), Bogon: true}, http.StatusOK)
		return
	}

	data := common.LookupIPData(s.geoIP, ip)
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

func sendJSONResponse(w http.ResponseWriter, data any, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func sendJSONError(w http.ResponseWriter, errMsg string, statusCode int) {
	sendJSONResponse(w, map[string]string{"error": errMsg}, statusCode)
}

func StartServer(ctx context.Context, geoIP *db.GeoIPManager) error {
	server := NewServer(geoIP)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

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

	return server.Start(ctx)
}
