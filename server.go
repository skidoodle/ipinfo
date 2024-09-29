package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
)

var invalidIPBytes = []byte("Please provide a valid IP address.")

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
	requestedThings := strings.Split(r.URL.Path, "/")

	// Enable gzip compression if requested
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		w = &gzipResponseWriter{Writer: gz, ResponseWriter: w}
	}

	// Extract IP and field
	var IPAddress, field string
	if len(requestedThings) > 1 && net.ParseIP(requestedThings[1]) != nil {
		IPAddress = requestedThings[1]
		if len(requestedThings) > 2 {
			field = requestedThings[2]
		}
	} else if len(requestedThings) > 1 {
		IPAddress = requestedThings[1] // This might be an invalid IP
	}

	// Check if the IP is the client's IP
	if IPAddress == "" {
		IPAddress = getRealIP(r)
	}

	// Validate the IP address
	ip := net.ParseIP(IPAddress)
	if ip == nil {
		// Send 400 Bad Request with invalid IP message
		http.Error(w, string(invalidIPBytes), http.StatusBadRequest)
		return
	}

	// Check if the IP is a bogon IP
	if isBogon(ip) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		bogonData := bogonDataStruct{
			IP:    ip.String(),
			Bogon: true,
		}
		json.NewEncoder(w).Encode(bogonData)
		return
	}

	// Look up IP data
	data := lookupIPData(ip)
	if data == nil {
		// Send 400 Bad Request with invalid IP message
		http.Error(w, string(invalidIPBytes), http.StatusBadRequest)
		return
	}

	// Handle specific field requests
	if field != "" {
		value := getField(data, field)
		if value != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]*string{field: value})
			return
		} else {
			// Handle invalid field request
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]*string{field: nil})
			return
		}
	}

	// If no specific field is requested, return the whole data
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	callback := r.URL.Query().Get("callback")
	enableJSONP := callback != "" && len(callback) < 2000 && callbackJSONP.MatchString(callback)
	if enableJSONP {
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		response := fmt.Sprintf("/**/ typeof %s === 'function' && %s(%s);", callback, callback, jsonData)
		w.Write([]byte(response))
	} else {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if r.URL.Query().Get("compact") == "true" {
			enc.SetIndent("", "")
		}
		enc.Encode(data)
	}
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
