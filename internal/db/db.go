package internal

import (
	"compress/gzip"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"skidoodle/ipinfo/internal/logger"

	"github.com/oschwald/maxminddb-golang"
	"github.com/pkg/errors"
)

// Constants for database names and paths
const (
	CityDBName  = "GeoLite2-City"
	ASNDBName   = "GeoLite2-ASN"
	DBExtension = ".mmdb"
	CityDBPath  = CityDBName + DBExtension
	ASNDBPath   = ASNDBName + DBExtension
)

// Error messages
var (
	ErrDatabaseNotFound = errors.New("database file not found")
	ErrDatabaseOpen     = errors.New("failed to open database")
	ErrDownloadFailed   = errors.New("failed to download database")
)

// ASNRecord represents a record in the ASN database
type ASNRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// GeoIPManager manages the GeoIP databases
type GeoIPManager struct {
	cityDB       *maxminddb.Reader
	asnDB        *maxminddb.Reader
	asnPrefixMap map[uint][]*net.IPNet
	httpClient   *http.Client
	mu           sync.RWMutex
}

// NewGeoIPManager creates a new GeoIPManager
func NewGeoIPManager() (*GeoIPManager, error) {
	manager := &GeoIPManager{
		httpClient: &http.Client{Timeout: 2 * time.Minute},
	}
	if err := manager.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing GeoIP manager: %w", err)
	}
	return manager, nil
}

// Initialize initializes the GeoIPManager by opening the database files
func (g *GeoIPManager) Initialize() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.openDB(CityDBPath); err != nil {
		return err
	}
	if err := g.openDB(ASNDBPath); err != nil {
		return err
	}

	g.buildASNPrefixMap()
	return nil
}

// openDB opens a MaxMind DB file
func (g *GeoIPManager) openDB(path string) error {
	db, err := maxminddb.Open(path)
	if err == nil {
		if path == CityDBPath {
			g.cityDB = db
		} else {
			g.asnDB = db
		}
		return nil
	}

	if !os.IsNotExist(err) {
		return errors.Wrapf(ErrDatabaseOpen, "failed to open %s: %v", path, err)
	}

	logger.Log.Info("Database not found, attempting initial download", "path", path)
	if err := g.DownloadDatabases(context.Background()); err != nil {
		return errors.Wrap(ErrDownloadFailed, err.Error())
	}

	db, err = maxminddb.Open(path)
	if err != nil {
		return errors.Wrapf(ErrDatabaseOpen, "failed to open %s after download: %v", path, err)
	}

	if path == CityDBPath {
		g.cityDB = db
	} else {
		g.asnDB = db
	}
	return nil
}

// buildASNPrefixMap builds a map of ASN prefixes for fast lookups
func (g *GeoIPManager) buildASNPrefixMap() {
	logger.Log.Info("Building ASN prefix map for fast lookups...")
	startTime := time.Now()
	g.asnPrefixMap = make(map[uint][]*net.IPNet)
	if g.asnDB == nil {
		logger.Log.Warn("ASN database is not available, skipping prefix map build")
		return
	}
	networks := g.asnDB.Networks()
	for networks.Next() {
		var record ASNRecord
		subnet, err := networks.Network(&record)
		if err != nil {
			continue
		}
		if record.AutonomousSystemNumber > 0 {
			g.asnPrefixMap[record.AutonomousSystemNumber] = append(g.asnPrefixMap[record.AutonomousSystemNumber], subnet)
		}
	}
	logger.Log.Info("Finished building ASN prefix map", "duration", time.Since(startTime))
}

// DownloadDatabases downloads the GeoIP databases
func (g *GeoIPManager) DownloadDatabases(ctx context.Context) error {
	accountID := os.Getenv("GEOIPUPDATE_ACCOUNT_ID")
	licenseKey := os.Getenv("GEOIPUPDATE_LICENSE_KEY")
	if accountID == "" || licenseKey == "" {
		return errors.New("GEOIPUPDATE_ACCOUNT_ID and GEOIPUPDATE_LICENSE_KEY must be set")
	}

	editionIDs := os.Getenv("GEOIPUPDATE_EDITION_IDS")
	if editionIDs == "" {
		editionIDs = "GeoLite2-City GeoLite2-ASN"
	}

	var firstError error
	for _, editionID := range strings.Fields(editionIDs) {
		if err := g.downloadEdition(ctx, accountID, licenseKey, editionID); err != nil {
			logger.Log.Error("Failed to download edition", "edition", editionID, "error", err)
			if firstError == nil {
				firstError = err
			}
		}
	}
	return firstError
}

// downloadEdition downloads a specific GeoIP database edition
func (g *GeoIPManager) downloadEdition(ctx context.Context, accountID, licenseKey, editionID string) error {
	dbPath := editionID + DBExtension
	logger.Log.Info("Checking for updates", "database", dbPath)

	hash, err := fileMD5(dbPath)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "could not calculate MD5 for %s", dbPath)
	}

	downloadURL := fmt.Sprintf("https://updates.maxmind.com/geoip/databases/%s/update?db_md5=%s", editionID, hash)
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	req.SetBasicAuth(accountID, licenseKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "http request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		logger.Log.Info("Database is already up to date", "database", dbPath)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("received non-200 status code: %d - %s", resp.StatusCode, string(body))
	}

	logger.Log.Info("Downloading and decompressing new version", "database", dbPath)

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return errors.Wrap(err, "could not create gzip reader")
	}
	defer gzr.Close()

	tmpPath := dbPath + ".tmp"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		return errors.Wrap(err, "could not create temporary file")
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, gzr); err != nil {
		os.Remove(tmpPath)
		return errors.Wrap(err, "could not decompress and write db file")
	}

	if err := os.Rename(tmpPath, dbPath); err != nil {
		return errors.Wrap(err, "could not replace database file")
	}

	logger.Log.Info("Successfully downloaded and updated", "database", dbPath)
	return nil
}

// UpdateDatabases updates the GeoIP databases
func (g *GeoIPManager) UpdateDatabases() error {
	if err := g.DownloadDatabases(context.Background()); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cityDB != nil {
		g.cityDB.Close()
	}
	if g.asnDB != nil {
		g.asnDB.Close()
	}

	var openErr error
	g.cityDB, openErr = maxminddb.Open(CityDBPath)
	if openErr != nil {
		return errors.Wrap(openErr, "reopening city database")
	}

	g.asnDB, openErr = maxminddb.Open(ASNDBPath)
	if openErr != nil {
		return errors.Wrap(openErr, "reopening ASN database")
	}

	g.buildASNPrefixMap()
	logger.Log.Info("Successfully reloaded GeoIP databases")
	return nil
}

// StartUpdater starts a background updater for the GeoIP databases
func (g *GeoIPManager) StartUpdater(ctx context.Context, updateInterval time.Duration) {
	logger.Log.Info("Starting MaxMind GeoIP database updater", "interval", updateInterval.String())
	ticker := time.NewTicker(updateInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				logger.Log.Info("Performing scheduled GeoIP database update")
				if err := g.UpdateDatabases(); err != nil {
					logger.Log.Error("Failed to update databases", "error", err)
				}
			case <-ctx.Done():
				ticker.Stop()
				logger.Log.Info("GeoIP database updater stopped")
				return
			}
		}
	}()
}

// Close closes the GeoIP database readers
func (g *GeoIPManager) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cityDB != nil {
		g.cityDB.Close()
	}
	if g.asnDB != nil {
		g.asnDB.Close()
	}
}

// GetCityDB retrieves the city database reader
func (g *GeoIPManager) GetCityDB() *maxminddb.Reader {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.cityDB
}

// GetASNDB retrieves the ASN database reader
func (g *GeoIPManager) GetASNDB() *maxminddb.Reader {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.asnDB
}

// GetASNPrefixes retrieves the list of IP prefixes for a given ASN
func (g *GeoIPManager) GetASNPrefixes(asn uint) []*net.IPNet {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.asnPrefixMap[asn]
}

// fileMD5 calculates the MD5 hash of a file
func fileMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
