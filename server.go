package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
)

var invalidIPBytes = []byte("Please provide a valid IP address.")

// Struct to hold IP data
type dataStruct struct {
	IP        *string `json:"ip"`
	Hostname  *string `json:"hostname"`
	ASN       *string `json:"asn"`
	Org       *string `json:"org"`
	City      *string `json:"city"`
	Region    *string `json:"region"`
	Country   *string `json:"country"`
	Continent *string `json:"continent"`
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

func handler(w http.ResponseWriter, r *http.Request) {
	requestedThings := strings.Split(r.URL.Path, "/")

	var IPAddress, field string
	if len(requestedThings) > 1 && net.ParseIP(requestedThings[1]) != nil {
		IPAddress = requestedThings[1]
		if len(requestedThings) > 2 {
			field = requestedThings[2]
		}
	} else if len(requestedThings) > 1 {
		field = requestedThings[1]
	}

	if IPAddress == "" || IPAddress == "self" {
		IPAddress = getRealIP(r)
	}
	ip := net.ParseIP(IPAddress)
	if ip == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(invalidIPBytes)
		return
	}

	if isBogon(ip) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		bogonData := bogonDataStruct{
			IP:    ip.String(),
			Bogon: true,
		}
		json.NewEncoder(w).Encode(bogonData)
		return
	}

	data := lookupIPData(ip)
	if data == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(invalidIPBytes)
		return
	}

	if field != "" {
		value := getField(data, field)
		if value != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]*string{field: value})
			return
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]*string{field: nil})
			return
		}
	}

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

// Get specific field from dataStruct
func getField(data *dataStruct, field string) *string {
	switch field {
	case "ip":
		return data.IP
	case "hostname":
		return data.Hostname
	case "asn":
		return data.ASN
	case "org":
		return data.Org
	case "city":
		return data.City
	case "region":
		return data.Region
	case "country":
		return data.Country
	case "continent":
		return data.Continent
	case "loc":
		return data.Loc
	default:
		return nil
	}
}
