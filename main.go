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

// Fetch and update the GeoIP databases.
func doUpdate() {
	fmt.Fprintln(os.Stderr, "Fetching updates...")
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
		fmt.Fprintf(os.Stderr, "City GeoIP database updated to %s\n", currMonth)
	})

	updateDatabase(asnDBURL, newASNFilename, func(newDB *maxminddb.Reader) {
		dbMtx.Lock()
		defer dbMtx.Unlock()
		if asnDB != nil {
			asnDB.Close()
		}
		asnDB = newDB
		currASNFilename = newASNFilename
		fmt.Fprintf(os.Stderr, "ASN GeoIP database updated to %s\n", currMonth)
	})
}

// Download and update the database file.
func updateDatabase(urlTemplate, dstFilename string, updateFunc func(*maxminddb.Reader)) {
	resp, err := http.Get(fmt.Sprintf(urlTemplate, time.Now().Format("2006-01")))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching the updated DB: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Non-200 status code (%d), retry later...\n", resp.StatusCode)
		return
	}

	dst, err := os.Create(dstFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file: %v\n", err)
		return
	}
	defer dst.Close()

	r, err := gzip.NewReader(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating gzip reader: %v\n", err)
		return
	}
	defer r.Close()

	fmt.Fprintln(os.Stderr, "Copying new database...")
	if _, err = io.Copy(dst, r); err != nil {
		fmt.Fprintf(os.Stderr, "Error copying file: %v\n", err)
		return
	}

	newDB, err := maxminddb.Open(dstFilename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening new DB: %v\n", err)
		return
	}

	updateFunc(newDB)
}

// Periodically update the GeoIP databases every week.
func updater() {
	for range time.Tick(time.Hour * 24 * 7) {
		doUpdate()
	}
}

func main() {
	var err error
	cityDB, err = maxminddb.Open(currCityFilename)
	if err != nil {
		if os.IsNotExist(err) {
			currCityFilename = ""
			doUpdate()
			if cityDB == nil {
				os.Exit(1)
			}
		} else {
			log.Fatal(err)
		}
	}

	asnDB, err = maxminddb.Open(currASNFilename)
	if err != nil {
		if os.IsNotExist(err) {
			currASNFilename = ""
			doUpdate()
			if asnDB == nil {
				os.Exit(1)
			}
		} else {
			log.Fatal(err)
		}
	}

	go updater()

	log.Println("Server listening on :3000")
	http.ListenAndServe(":3000", http.HandlerFunc(handler))
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

var nameToField = map[string]func(dataStruct) string{
	"ip":             func(d dataStruct) string { return d.IP },
	"hostname":       func(d dataStruct) string { return d.Hostname },
	"asn":            func(d dataStruct) string { return d.ASN },
	"organization":   func(d dataStruct) string { return d.Organization },
	"city":           func(d dataStruct) string { return d.City },
	"region":         func(d dataStruct) string { return d.Region },
	"country":        func(d dataStruct) string { return d.Country },
	"country_full":   func(d dataStruct) string { return d.CountryFull },
	"continent":      func(d dataStruct) string { return d.Continent },
	"continent_full": func(d dataStruct) string { return d.ContinentFull },
	"loc":            func(d dataStruct) string { return d.Loc },
}

func handler(w http.ResponseWriter, r *http.Request) {
	requestedThings := strings.Split(r.URL.Path, "/")

	var IPAddress, Which string
	switch len(requestedThings) {
	case 3:
		Which = requestedThings[2]
		fallthrough
	case 2:
		IPAddress = requestedThings[1]
	}

	if IPAddress == "" || IPAddress == "self" {
		if realIP, ok := r.Header["X-Forwarded-For"]; ok && len(realIP) > 0 {
			IPAddress = realIP[0]
		} else {
			IPAddress = extractIP(r.RemoteAddr)
		}
	}

	ip := net.ParseIP(IPAddress)
	if ip == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(invalidIPBytes)
		return
	}

	dbMtx.RLock()
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
	dbMtx.RUnlock()
	if err != nil {
		log.Fatal(err)
	}

	dbMtx.RLock()
	var asnRecord struct {
		AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
		AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
	}
	err = asnDB.Lookup(ip, &asnRecord)
	dbMtx.RUnlock()
	if err != nil {
		log.Fatal(err)
	}

	hostname, err := net.LookupAddr(ip.String())
	if err != nil || len(hostname) == 0 {
		hostname = []string{""}
	}

	var sd string
	if len(cityRecord.Subdivisions) > 0 {
		sd = cityRecord.Subdivisions[0].Names["en"]
	}

	d := dataStruct{
		IP:            ip.String(),
		Hostname:      hostname[0],
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

	if Which == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		callback := r.URL.Query().Get("callback")
		enableJSONP := callback != "" && len(callback) < 2000 && callbackJSONP.MatchString(callback)
		if enableJSONP {
			w.Write([]byte(fmt.Sprintf("/**/ typeof %s === 'function' && %s(", callback, callback)))
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if r.URL.Query().Get("compact") == "true" {
			enc.SetIndent("", "")
		}
		enc.Encode(d)
		if enableJSONP {
			w.Write([]byte(");"))
		}
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if val := nameToField[Which]; val != nil {
			w.Write([]byte(val(d)))
		} else {
			w.Write([]byte("undefined"))
		}
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
