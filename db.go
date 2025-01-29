package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

var (
	cityDB *maxminddb.Reader
	asnDB  *maxminddb.Reader
	dbMtx  = new(sync.RWMutex)
)

const (
	cityDBPath = "./GeoLite2-City.mmdb"
	asnDBPath  = "./GeoLite2-ASN.mmdb"
)

func downloadDB() error {
	cmd := exec.Command("geoipupdate", "-d", "./")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download database: %v. Ensure geoipupdate is installed and configured", err)
	}
	return nil
}

func initDatabases() {
	var err error

	dbMtx.Lock()
	defer dbMtx.Unlock()

	cityDB, err = maxminddb.Open(cityDBPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("City database not found, attempting to download...")
			if errDownload := downloadDB(); errDownload != nil {
				log.Fatalf("Error downloading city database: %v", errDownload)
			}
			cityDB, err = maxminddb.Open(cityDBPath)
			if err != nil {
				log.Fatalf("Error opening city database after download: %v", err)
			}
		} else {
			log.Fatalf("Error opening city database: %v", err)
		}
	}

	asnDB, err = maxminddb.Open(asnDBPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("ASN database not found, attempting to download...")
			if errDownload := downloadDB(); errDownload != nil {
				log.Fatalf("Error downloading ASN database: %v", errDownload)
			}
			asnDB, err = maxminddb.Open(asnDBPath)
			if err != nil {
				log.Fatalf("Error opening ASN database after download: %v", err)
			}
		} else {
			log.Fatalf("Error opening ASN database: %v", err)
		}
	}
}

func startUpdater() {
	log.Println("MaxMind GeoIP databases will be updated by geoipupdate automatically.")
}
