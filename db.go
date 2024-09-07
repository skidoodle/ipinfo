package main

import (
	"log"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

var cityDB *maxminddb.Reader
var asnDB *maxminddb.Reader
var dbMtx = new(sync.RWMutex)

const (
	cityDBPath = "./GeoLite2-City.mmdb"
	asnDBPath  = "./GeoLite2-ASN.mmdb"
)

func initDatabases() {
	var err error

	cityDB, err = maxminddb.Open(cityDBPath)
	if err != nil {
		log.Fatalf("Error opening city database: %v", err)
	}

	asnDB, err = maxminddb.Open(asnDBPath)
	if err != nil {
		log.Fatalf("Error opening ASN database: %v", err)
	}
}

func startUpdater() {
	log.Println("MaxMind GeoIP databases will be updated by geoipupdate automatically.")
}
