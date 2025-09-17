package db

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// StartUpdater starts a background updater for the GeoIP databases.
func (g *GeoIPManager) StartUpdater(ctx context.Context, updateInterval time.Duration) {
	slog.Info("starting database updater", "interval", updateInterval.String())
	ticker := time.NewTicker(updateInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				slog.Info("performing scheduled database update")
				if err := g.UpdateDatabases(); err != nil {
					slog.Error("failed to update databases", "err", err)
				}
			case <-ctx.Done():
				ticker.Stop()
				slog.Info("database updater stopped")
				return
			}
		}
	}()
}

// UpdateDatabases downloads new databases and reloads them into the manager.
func (g *GeoIPManager) UpdateDatabases() error {
	if err := g.DownloadDatabases(context.Background()); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cityDB != nil {
		_ = g.cityDB.Close()
	}
	if g.asnDB != nil {
		_ = g.asnDB.Close()
	}

	var openErr error
	g.cityDB, openErr = maxminddb.Open(CityDBPath)
	if openErr != nil {
		return fmt.Errorf("reopening city database: %w", openErr)
	}

	g.asnDB, openErr = maxminddb.Open(ASNDBPath)
	if openErr != nil {
		return fmt.Errorf("reopening asn database: %w", openErr)
	}

	g.buildASNPrefixMap()
	slog.Info("successfully reloaded databases")
	return nil
}

// DownloadDatabases downloads all configured GeoIP database editions.
func (g *GeoIPManager) DownloadDatabases(ctx context.Context) error {
	accountID := os.Getenv("GEOIPUPDATE_ACCOUNT_ID")
	licenseKey := os.Getenv("GEOIPUPDATE_LICENSE_KEY")
	if accountID == "" || licenseKey == "" {
		return fmt.Errorf("GEOIPUPDATE_ACCOUNT_ID and GEOIPUPDATE_LICENSE_KEY must be set")
	}

	editionIDs := os.Getenv("GEOIPUPDATE_EDITION_IDS")
	if editionIDs == "" {
		editionIDs = "GeoLite2-City GeoLite2-ASN"
	}

	var firstError error
	for _, editionID := range strings.Fields(editionIDs) {
		if err := g.downloadEdition(ctx, accountID, licenseKey, editionID); err != nil {
			slog.Error("failed to download edition", "edition", editionID, "err", err)
			if firstError == nil {
				firstError = err
			}
		}
	}
	return firstError
}

// downloadEdition downloads a specific GeoIP database edition.
func (g *GeoIPManager) downloadEdition(ctx context.Context, accountID, licenseKey, editionID string) error {
	dbPath := editionID + DBExtension
	slog.Info("checking for updates", "database", dbPath)

	hash, err := fileMD5(dbPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not calculate md5 for %s: %w", dbPath, err)
	}

	downloadURL := fmt.Sprintf("https://updates.maxmind.com/geoip/databases/%s/update?db_md5=%s", editionID, hash)
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}
	req.SetBasicAuth(accountID, licenseKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "err", err)
		}
	}()

	if resp.StatusCode == http.StatusNotModified {
		slog.Info("database is already up to date", "database", dbPath)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("received non-200 status code: %d - %s", resp.StatusCode, string(body))
	}

	slog.Info("downloading and decompressing new version", "database", dbPath)

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("could not create gzip reader: %w", err)
	}
	defer func() {
		if err := gzr.Close(); err != nil {
			slog.Error("failed to close gzip reader", "err", err)
		}
	}()

	tmpPath := dbPath + ".tmp"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("could not create temporary file: %w", err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			slog.Error("failed to close output file", "err", err)
		}
	}()

	if _, err := io.Copy(outFile, gzr); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("could not decompress and write db file: %w", err)
	}

	if err := os.Rename(tmpPath, dbPath); err != nil {
		return fmt.Errorf("could not replace database file: %w", err)
	}

	slog.Info("successfully downloaded and updated", "database", dbPath)
	return nil
}
