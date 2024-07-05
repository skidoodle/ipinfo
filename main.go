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
var cityDB *maxminddb.Reader
var asnDB *maxminddb.Reader

var (
	currCityFilename = time.Now().Format("2006-01") + "-city.mmdb"
	currASNFilename  = time.Now().Format("2006-01") + "-asn.mmdb"
	dbMtx            = new(sync.RWMutex)
)

const (
	cityDBURL = "https://download.db-ip.com/free/dbip-city-lite-%s.mmdb.gz"
	asnDBURL  = "https://download.db-ip.com/free/dbip-asn-lite-%s.mmdb.gz"
)

func main() {
	initDatabases()
	go startUpdater()
	startServer()
}

func initDatabases() {
	var err error

	cityDB, err = maxminddb.Open(currCityFilename)
	if err != nil {
		if os.IsNotExist(err) {
			currCityFilename = ""
			doUpdate()
			if cityDB == nil {
				log.Fatalf("Failed to initialize city database: %v", err)
			}
		} else {
			log.Fatalf("Error opening city database: %v", err)
		}
	}

	asnDB, err = maxminddb.Open(currASNFilename)
	if err != nil {
		if os.IsNotExist(err) {
			currASNFilename = ""
			doUpdate()
			if asnDB == nil {
				log.Fatalf("Failed to initialize ASN database: %v", err)
			}
		} else {
			log.Fatalf("Error opening ASN database: %v", err)
		}
	}
}

func startUpdater() {
	for range time.Tick(time.Hour * 24 * 7) {
		doUpdate()
	}
}

func startServer() {
	log.Println("Server listening on :3000")
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":3000", nil))
}

// Fetch and update the GeoIP databases.
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

type dataStruct struct {
	IP            string `json:"ip"`
	Hostname      string `json:"hostname"`
	ASN           string `json:"asn"`
	Organization  string `json:"organization"`
	City          string `json:"city"`
	Region        string `json:"region"`
	Country       string `json:"country"`
	CountryFull   string `json:"country_full"`
	Continent     string `json:"continent"`
	ContinentFull string `json:"continent_full"`
	Loc           string `json:"loc"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	requestedThings := strings.Split(r.URL.Path, "/")

	var IPAddress string
	if len(requestedThings) > 1 {
		IPAddress = requestedThings[1]
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

	data := lookupIPData(ip)
	if data == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(invalidIPBytes)
		return
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

func getRealIP(r *http.Request) string {
	if realIP := r.Header.Get("CF-Connecting-IP"); realIP != "" {
		return realIP
	} else if realIP := r.Header.Get("X-Forwarded-For"); realIP != "" {
		return strings.Split(realIP, ",")[0]
	} else {
		return extractIP(r.RemoteAddr)
	}
}

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

	var sd string
	if len(cityRecord.Subdivisions) > 0 {
		sd = cityRecord.Subdivisions[0].Names["en"]
	}

	return &dataStruct{
		IP:            ip.String(),
		Hostname:      strings.TrimSuffix(hostname[0], "."),
		ASN:           fmt.Sprintf("%d", asnRecord.AutonomousSystemNumber),
		Organization:  asnRecord.AutonomousSystemOrganization,
		Country:       cityRecord.Country.IsoCode,
		CountryFull:   cityRecord.Country.Names["en"],
		City:          cityRecord.City.Names["en"],
		Region:        sd,
		Continent:     cityRecord.Continent.Code,
		ContinentFull: cityRecord.Continent.Names["en"],
		Loc:           fmt.Sprintf("%.4f,%.4f", cityRecord.Location.Latitude, cityRecord.Location.Longitude),
	}
}

var callbackJSONP = regexp.MustCompile(`^[a-zA-Z_\$][a-zA-Z0-9_\$]*$`)

// Extract the IP address from a string, removing unwanted characters.
func extractIP(ip string) string {
	ip = strings.ReplaceAll(ip, "[", "")
	ip = strings.ReplaceAll(ip, "]", "")
	ss := strings.Split(ip, ":")
	return strings.Join(ss[:len(ss)-1], ":")
}
