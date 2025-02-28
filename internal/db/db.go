package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// Database paths
const (
	CityDBPath = "./GeoLite2-City.mmdb"
	ASNDBPath  = "./GeoLite2-ASN.mmdb"
)

// Common errors
var (
	ErrDatabaseNotFound = errors.New("database file not found")
	ErrDatabaseOpen     = errors.New("failed to open database")
	ErrDownloadFailed   = errors.New("failed to download database")
)

// Handles MaxMind GeoIP database operations
type GeoIPManager struct {
	cityDB *maxminddb.Reader
	asnDB  *maxminddb.Reader
	mu     sync.RWMutex
}

// Creates and initializes a new GeoIP database
func NewGeoIPManager() (*GeoIPManager, error) {
	manager := &GeoIPManager{}
	if err := manager.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing GeoIP manager: %w", err)
	}
	return manager, nil
}

// Initialize opens the GeoIP databases, downloading them if necessary
func (g *GeoIPManager) Initialize() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.openCityDB(); err != nil {
		return err
	}

	if err := g.openASNDB(); err != nil {
		return err
	}

	return nil
}

// Opens the city database, downloading it if necessary
func (g *GeoIPManager) openCityDB() error {
	var err error
	g.cityDB, err = maxminddb.Open(CityDBPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("City database not found, attempting to download...")
			if err := g.downloadDatabases(); err != nil {
				return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
			}
			g.cityDB, err = maxminddb.Open(CityDBPath)
			if err != nil {
				return fmt.Errorf("%w (city): %v", ErrDatabaseOpen, err)
			}
			return nil
		}
		return fmt.Errorf("%w (city): %v", ErrDatabaseOpen, err)
	}
	return nil
}

// Opens the ASN database, downloading it if necessary
func (g *GeoIPManager) openASNDB() error {
	var err error
	g.asnDB, err = maxminddb.Open(ASNDBPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("ASN database not found, attempting to download...")
			if err := g.downloadDatabases(); err != nil {
				return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
			}
			g.asnDB, err = maxminddb.Open(ASNDBPath)
			if err != nil {
				return fmt.Errorf("%w (ASN): %v", ErrDatabaseOpen, err)
			}
			return nil
		}
		return fmt.Errorf("%w (ASN): %v", ErrDatabaseOpen, err)
	}
	return nil
}

// Downloads both GeoIP databases using geoipupdate
func (g *GeoIPManager) downloadDatabases() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "geoipupdate", "-d", "./")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to download databases: %v. Output: %s. Ensure geoipupdate is installed and configured", err, output)
	}
	return nil
}

// Close properly closes the database readers
func (g *GeoIPManager) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var errs []error

	if g.cityDB != nil {
		if err := g.cityDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing city database: %w", err))
		}
	}

	if g.asnDB != nil {
		if err := g.asnDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("closing ASN database: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing databases: %v", errs)
	}
	return nil
}

// Safely provides read access to the City database
func (g *GeoIPManager) GetCityDB() *maxminddb.Reader {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.cityDB
}

// Safely provides read access to the ASN database
func (g *GeoIPManager) GetASNDB() *maxminddb.Reader {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.asnDB
}

// Sets up automatic database updates
func (g *GeoIPManager) StartUpdater(ctx context.Context, updateInterval time.Duration) {
	log.Printf("Starting MaxMind GeoIP database updater with interval: %s", updateInterval)

	ticker := time.NewTicker(updateInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Println("Performing scheduled GeoIP database update")
				if err := g.UpdateDatabases(); err != nil {
					log.Printf("Failed to update databases: %v", err)
				}
			case <-ctx.Done():
				ticker.Stop()
				log.Println("GeoIP database updater stopped")
				return
			}
		}
	}()
}

// Downloads fresh copies of the databases and reloads them
func (g *GeoIPManager) UpdateDatabases() error {
	// Download new databases
	if err := g.downloadDatabases(); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Close existing databases
	if g.cityDB != nil {
		if err := g.cityDB.Close(); err != nil {
			log.Printf("Warning: error closing city database: %v", err)
		}
	}

	if g.asnDB != nil {
		if err := g.asnDB.Close(); err != nil {
			log.Printf("Warning: error closing ASN database: %v", err)
		}
	}

	// Reopen databases
	var err error
	g.cityDB, err = maxminddb.Open(CityDBPath)
	if err != nil {
		return fmt.Errorf("reopening city database: %w", err)
	}

	g.asnDB, err = maxminddb.Open(ASNDBPath)
	if err != nil {
		return fmt.Errorf("reopening ASN database: %w", err)
	}

	log.Println("Successfully updated and reloaded GeoIP databases")
	return nil
}
