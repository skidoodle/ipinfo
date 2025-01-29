package main

import (
	"compress/gzip"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
)

var invalidIPBytes = []byte("Please provide a valid IP address.")
var invalidFieldBytes = []byte("Please provide a valid field.")

type dataStruct struct {
	IP        *string `json:"ip"`
	Hostname  *string `json:"hostname"`
	ASN       *string `json:"asn"`
	Org       *string `json:"org"`
	City      *string `json:"city"`
	Region    *string `json:"region"`
	Country   *string `json:"country"`
	Continent *string `json:"continent"`
	Timezone  *string `json:"timezone"`
	Loc       *string `json:"loc"`
}

type bogonDataStruct struct {
	IP    string `json:"ip"`
	Bogon bool   `json:"bogon"`
}

func startServer() {
	log.Println("Server listening on :3000")
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":3000", nil))
}

type gzipResponseWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func handler(w http.ResponseWriter, r *http.Request) {
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
		IPAddress = getRealIP(r) // Default to visitor's IP
	case 1:
		if requestedThings[0] == "" {
			IPAddress = getRealIP(r) // Handle root page case
		} else if _, ok := fieldMap[requestedThings[0]]; ok {
			IPAddress = getRealIP(r)
			field = requestedThings[0]
		} else if net.ParseIP(requestedThings[0]) != nil {
			IPAddress = requestedThings[0] // Valid IP provided
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": string(invalidIPBytes)})
			return
		}
	case 2:
		IPAddress = requestedThings[0]
		if _, ok := fieldMap[requestedThings[1]]; ok {
			field = requestedThings[1]
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": string(invalidFieldBytes)})
			return
		}
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": string(invalidIPBytes)})
		return
	}

	// Validate the resolved IP
	ip := net.ParseIP(IPAddress)
	if ip == nil {
		http.Error(w, string(invalidIPBytes), http.StatusBadRequest)
		return
	}

	// Check if the IP is a bogon IP
	if isBogon(ip) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(bogonDataStruct{IP: ip.String(), Bogon: true})
		return
	}

	// Look up IP data
	data := lookupIPData(ip)
	if data == nil {
		http.Error(w, string(invalidIPBytes), http.StatusBadRequest)
		return
	}

	// Handle specific field requests
	if field != "" {
		value := getField(data, field)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]*string{field: value})
		return
	}

	// Default case: return full IP data
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

var fieldMap = map[string]func(*dataStruct) *string{
	"ip":        func(d *dataStruct) *string { return d.IP },
	"hostname":  func(d *dataStruct) *string { return d.Hostname },
	"asn":       func(d *dataStruct) *string { return d.ASN },
	"org":       func(d *dataStruct) *string { return d.Org },
	"city":      func(d *dataStruct) *string { return d.City },
	"region":    func(d *dataStruct) *string { return d.Region },
	"country":   func(d *dataStruct) *string { return d.Country },
	"continent": func(d *dataStruct) *string { return d.Continent },
	"timezone":  func(d *dataStruct) *string { return d.Timezone },
	"loc":       func(d *dataStruct) *string { return d.Loc },
}

func getField(data *dataStruct, field string) *string {
	if f, ok := fieldMap[field]; ok {
		return f(data)
	}
	return nil
}
