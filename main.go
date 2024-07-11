package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// GeoIP database readers
var (
	cityDB *maxminddb.Reader
	asnDB  *maxminddb.Reader
	dbMtx  sync.RWMutex

	currCityFilename = time.Now().Format("2006-01") + "-city.mmdb"
	currASNFilename  = time.Now().Format("2006-01") + "-asn.mmdb"
)

const (
	cityDBURL = "https://download.db-ip.com/free/dbip-city-lite-%s.mmdb.gz"
	asnDBURL  = "https://download.db-ip.com/free/dbip-asn-lite-%s.mmdb.gz"
)

func main() {
	initDBs()
	go runUpdater()
	runServer()
}

// Initialize the databases
func initDBs() {
	var err error
	cityDB, err = openDB(currCityFilename)
	if err != nil {
		log.Printf("Error opening city database: %v", err)
		updateDB(cityDBURL, &cityDB, &currCityFilename)
	}
	asnDB, err = openDB(currASNFilename)
	if err != nil {
		log.Printf("Error opening ASN database: %v", err)
		updateDB(asnDBURL, &asnDB, &currASNFilename)
	}
}

// Open a MaxMind DB file
func openDB(filename string) (*maxminddb.Reader, error) {
	db, err := maxminddb.Open(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return db, nil
}

// Download and set the database
func updateDB(urlTemplate string, db **maxminddb.Reader, currFilename *string) {
	*currFilename = ""
	doUpdate()
	if *db == nil {
		log.Fatalf("Failed to initialize database from %s", urlTemplate)
	}
}

// Periodically update the databases
func runUpdater() {
	for range time.Tick(24 * time.Hour * 7) {
		doUpdate()
	}
}

// Start the HTTP server
func runServer() {
	log.Println("Server listening on :3000")
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":3000", nil))
}

// Fetch and update the GeoIP databases
func doUpdate() {
	log.Println("Fetching updates...")
	currMonth := time.Now().Format("2006-01")
	newCityFilename := currMonth + "-city.mmdb"
	newASNFilename := currMonth + "-asn.mmdb"

	updateDatabase(cityDBURL, newCityFilename, func(newDB *maxminddb.Reader) {
		dbMtx.Lock()
		defer dbMtx.Unlock()
		if cityDB != nil {
			cityDB.Close()
		}
		cityDB = newDB
		currCityFilename = newCityFilename
		log.Printf("City GeoIP database updated to %s\n", currMonth)
	})

	updateDatabase(asnDBURL, newASNFilename, func(newDB *maxminddb.Reader) {
		dbMtx.Lock()
		defer dbMtx.Unlock()
		if asnDB != nil {
			asnDB.Close()
		}
		asnDB = newDB
		currASNFilename = newASNFilename
		log.Printf("ASN GeoIP database updated to %s\n", currMonth)
	})
}

// Update the specified database
func updateDatabase(urlTemplate, dstFilename string, updateFunc func(*maxminddb.Reader)) {
	resp, err := http.Get(fmt.Sprintf(urlTemplate, time.Now().Format("2006-01")))
	if err != nil {
		log.Printf("Error fetching the updated DB: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("Non-200 status code (%d), retry later...\n", resp.StatusCode)
		return
	}

	dst, err := os.Create(dstFilename)
	if err != nil {
		log.Printf("Error creating file: %v\n", err)
		return
	}
	defer dst.Close()

	r, err := gzip.NewReader(resp.Body)
	if err != nil {
		log.Printf("Error creating gzip reader: %v\n", err)
		return
	}
	defer r.Close()

	log.Println("Copying new database...")
	if _, err = io.Copy(dst, r); err != nil {
		log.Printf("Error copying file: %v\n", err)
		return
	}

	newDB, err := maxminddb.Open(dstFilename)
	if err != nil {
		log.Printf("Error opening new DB: %v\n", err)
		return
	}

	updateFunc(newDB)
}

var invalidIPBytes = []byte("Please provide a valid IP address.")

// Struct to hold IP data
type dataStruct struct {
	IP            *string `json:"ip"`
	Hostname      *string `json:"hostname"`
	ASN           *string `json:"asn"`
	Organization  *string `json:"organization"`
	City          *string `json:"city"`
	Region        *string `json:"region"`
	Country       *string `json:"country"`
	CountryFull   *string `json:"country_full"`
	Continent     *string `json:"continent"`
	ContinentFull *string `json:"continent_full"`
	Loc           *string `json:"loc"`
}

type bogonDataStruct struct {
	IP    string `json:"ip"`
	Bogon bool   `json:"bogon"`
}

// List of bogon IP networks
var bogonNets = []*net.IPNet{
	// IPv4
	{IP: net.IPv4(0, 0, 0, 0), Mask: net.CIDRMask(8, 32)}, // "This" network
	{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)}, // Private-use networks
	{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}, // Carrier-grade NAT
	{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)}, // Loopback
	{IP: net.IPv4(127, 0, 53, 53), Mask: net.CIDRMask(32, 32)}, // Name collision occurrence
	{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)}, // Link-local
	{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)}, // Private-use networks
	{IP: net.IPv4(192, 0, 0, 0), Mask: net.CIDRMask(24, 32)}, // IETF protocol assignments
	{IP: net.IPv4(192, 0, 2, 0), Mask: net.CIDRMask(24, 32)}, // TEST-NET-1
	{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)}, // Private-use networks
	{IP: net.IPv4(198, 18, 0, 0), Mask: net.CIDRMask(15, 32)}, // Network interconnect device benchmark testing
	{IP: net.IPv4(198, 51, 100, 0), Mask: net.CIDRMask(24, 32)}, // TEST-NET-2
	{IP: net.IPv4(203, 0, 113, 0), Mask: net.CIDRMask(24, 32)}, // TEST-NET-3
	{IP: net.IPv4(224, 0, 0, 0), Mask: net.CIDRMask(4, 32)}, // Multicast
	{IP: net.IPv4(240, 0, 0, 0), Mask: net.CIDRMask(4, 32)}, // Reserved for future use
	{IP: net.IPv4(255, 255, 255, 255), Mask: net.CIDRMask(32, 32)}, // Limited broadcast
	// IPv6
	{IP: net.ParseIP("::/128"), Mask: net.CIDRMask(128, 128)}, // Node-scope unicast unspecified address
	{IP: net.ParseIP("::1/128"), Mask: net.CIDRMask(128, 128)}, // Node-scope unicast loopback address
	{IP: net.ParseIP("::ffff:0:0/96"), Mask: net.CIDRMask(96, 128)}, // IPv4-mapped addresses
	{IP: net.ParseIP("::/96"), Mask: net.CIDRMask(96, 128)}, // IPv4-compatible addresses
	{IP: net.ParseIP("100::/64"), Mask: net.CIDRMask(64, 128)}, // Remotely triggered black hole addresses
	{IP: net.ParseIP("2001:10::/28"), Mask: net.CIDRMask(28, 128)}, // Overlay routable cryptographic hash identifiers (ORCHID)
	{IP: net.ParseIP("2001:db8::/32"), Mask: net.CIDRMask(32, 128)}, // Documentation prefix
	{IP: net.ParseIP("fc00::/7"), Mask: net.CIDRMask(7, 128)}, // Unique local addresses (ULA)
	{IP: net.ParseIP("fe80::/10"), Mask: net.CIDRMask(10, 128)}, // Link-local unicast
	{IP: net.ParseIP("fec0::/10"), Mask: net.CIDRMask(10, 128)}, // Site-local unicast (deprecated)
	{IP: net.ParseIP("ff00::/8"), Mask: net.CIDRMask(8, 128)}, // Multicast
	// Additional Bogon Ranges
	{IP: net.ParseIP("2002::/24"), Mask: net.CIDRMask(24, 128)}, // 6to4 bogon (0.0.0.0/8)
	{IP: net.ParseIP("2002:a00::/24"), Mask: net.CIDRMask(24, 128)}, // 6to4 bogon (10.0.0.0/8)
	{IP: net.ParseIP("2002:7f00::/24"), Mask: net.CIDRMask(24, 128)}, // 6to4 bogon (127.0.0.0/8)
	{IP: net.ParseIP("2002:a9fe::/32"), Mask: net.CIDRMask(32, 128)}, // 6to4 bogon (169.254.0.0/16)
	{IP: net.ParseIP("2002:ac10::/28"), Mask: net.CIDRMask(28, 128)}, // 6to4 bogon (172.16.0.0/12)
	{IP: net.ParseIP("2002:c000::/40"), Mask: net.CIDRMask(40, 128)}, // 6to4 bogon (192.0.0.0/24)
	{IP: net.ParseIP("2002:c000:200::/40"), Mask: net.CIDRMask(40, 128)}, // 6to4 bogon (192.0.2.0/24)
	{IP: net.ParseIP("2002:c0a8::/32"), Mask: net.CIDRMask(32, 128)}, // 6to4 bogon (192.168.0.0/16)
	{IP: net.ParseIP("2002:c612::/31"), Mask: net.CIDRMask(31, 128)}, // 6to4 bogon (198.18.0.0/15)
	{IP: net.ParseIP("2002:c633:6400::/40"), Mask: net.CIDRMask(40, 128)}, // 6to4 bogon (198.51.100.0/24)
	{IP: net.ParseIP("2002:cb00:7100::/40"), Mask: net.CIDRMask(40, 128)}, // 6to4 bogon (203.0.113.0/24)
	{IP: net.ParseIP("2002:e000::/20"), Mask: net.CIDRMask(20, 128)}, // 6to4 bogon (224.0.0.0/4)
	{IP: net.ParseIP("2002:f000::/20"), Mask: net.CIDRMask(20, 128)}, // 6to4 bogon (240.0.0.0/4)
	{IP: net.ParseIP("2002:ffff:ffff::/48"), Mask: net.CIDRMask(48, 128)}, // 6to4 bogon (255.255.255.255/32)
	{IP: net.ParseIP("2001::/40"), Mask: net.CIDRMask(40, 128)}, // Teredo bogon (0.0.0.0/8)
	{IP: net.ParseIP("2001:0:a00::/40"), Mask: net.CIDRMask(40, 128)}, // Teredo bogon (10.0.0.0/8)
	{IP: net.ParseIP("2001:0:7f00::/40"), Mask: net.CIDRMask(40, 128)}, // Teredo bogon (127.0.0.0/8)
	{IP: net.ParseIP("2001:0:a9fe::/48"), Mask: net.CIDRMask(48, 128)}, // Teredo bogon (169.254.0.0/16)
	{IP: net.ParseIP("2001:0:ac10::/44"), Mask: net.CIDRMask(44, 128)}, // Teredo bogon (172.16.0.0/12)
	{IP: net.ParseIP("2001:0:c000::/56"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (192.0.0.0/24)
	{IP: net.ParseIP("2001:0:c000:200::/56"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (192.0.2.0/24)
	{IP: net.ParseIP("2001:0:c0a8::/48"), Mask: net.CIDRMask(48, 128)}, // Teredo bogon (192.168.0.0/16)
	{IP: net.ParseIP("2001:0:c612::/47"), Mask: net.CIDRMask(47, 128)}, // Teredo bogon (198.18.0.0/15)
	{IP: net.ParseIP("2001:0:c633:6400::/56"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (198.51.100.0/24)
	{IP: net.ParseIP("2001:0:cb00:7100::/56"), Mask: net.CIDRMask(56, 128)}, // Teredo bogon (203.0.113.0/24)
	{IP: net.ParseIP("2001:0:e000::/36"), Mask: net.CIDRMask(36, 128)}, // Teredo bogon (224.0.0.0/4)
	{IP: net.ParseIP("2001:0:f000::/36"), Mask: net.CIDRMask(36, 128)}, // Teredo bogon (240.0.0.0/4)
	{IP: net.ParseIP("2001:0:ffff:ffff::/64"), Mask: net.CIDRMask(64, 128)}, // Teredo bogon (255.255.255.255/32)
}

// Check if the IP is a bogon IP
func isBogon(ip net.IP) bool {
	for _, net := range bogonNets {
		if net.Contains(ip) {
			return true
		}
	}
	return false
}

// HTTP handler
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

	// Check if the IP is a bogon
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
	case "organization":
		return data.Organization
	case "city":
		return data.City
	case "region":
		return data.Region
	case "country":
		return data.Country
	case "country_full":
		return data.CountryFull
	case "continent":
		return data.Continent
	case "continent_full":
		return data.ContinentFull
	case "loc":
		return data.Loc
	default:
		return nil
	}
}

// Get the real IP address from the request headers
func getRealIP(r *http.Request) string {
	if realIP := r.Header.Get("CF-Connecting-IP"); realIP != "" {
		return realIP
	} else if realIP := r.Header.Get("X-Forwarded-For"); realIP != "" {
		return strings.Split(realIP, ",")[0]
	} else {
		return extractIP(r.RemoteAddr)
	}
}

// Lookup IP data in the databases
func lookupIPData(ip net.IP) *dataStruct {
	dbMtx.RLock()
	defer dbMtx.RUnlock()

	var cityRecord struct {
		Country struct {
			IsoCode string            `maxminddb:"iso_code"`
			Names   map[string]string `maxminddb:"names"`
		} `maxminddb:"country"`
		City struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"city"`
		Subdivisions []struct {
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"subdivisions"`
		Continent struct {
			Code  string            `maxminddb:"code"`
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"continent"`
		Location struct {
			Latitude  float64 `maxminddb:"latitude"`
			Longitude float64 `maxminddb:"longitude"`
		} `maxminddb:"location"`
	}
	err := cityDB.Lookup(ip, &cityRecord)
	if err != nil {
		log.Printf("Error looking up city data: %v\n", err)
		return nil
	}

	var asnRecord struct {
		AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
		AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	}
	err = asnDB.Lookup(ip, &asnRecord)
	if err != nil {
		log.Printf("Error looking up ASN data: %v\n", err)
		return nil
	}

	hostname, err := net.LookupAddr(ip.String())
	if err != nil || len(hostname) == 0 {
		hostname = []string{""}
	}

	var sd *string
	if len(cityRecord.Subdivisions) > 0 {
		name := cityRecord.Subdivisions[0].Names["en"]
		sd = &name
	}

	return &dataStruct{
		IP:            toPtr(ip.String()),
		Hostname:      toPtr(strings.TrimSuffix(hostname[0], ".")),
		ASN:           toPtr(fmt.Sprintf("%d", asnRecord.AutonomousSystemNumber)),
		Organization:  toPtr(asnRecord.AutonomousSystemOrganization),
		Country:       toPtr(cityRecord.Country.IsoCode),
		CountryFull:   toPtr(cityRecord.Country.Names["en"]),
		City:          toPtr(cityRecord.City.Names["en"]),
		Region:        sd,
		Continent:     toPtr(cityRecord.Continent.Code),
		ContinentFull: toPtr(cityRecord.Continent.Names["en"]),
		Loc:           toPtr(fmt.Sprintf("%.4f,%.4f", cityRecord.Location.Latitude, cityRecord.Location.Longitude)),
	}
}

// Convert string to pointer
func toPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Validate JSONP callback name
var callbackJSONP = regexp.MustCompile(`^[a-zA-Z_\$][a-zA-Z0-9_\$]*$`)

// Extract the IP address from a string, removing unwanted characters
func extractIP(ip string) string {
	ip = strings.ReplaceAll(ip, "[", "")
	ip = strings.ReplaceAll(ip, "]", "")
	ss := strings.Split(ip, ":")
	return strings.Join(ss[:len(ss)-1], ":")
}
