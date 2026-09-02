package geoip

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

// StartUpdater runs a background loop updating databases on a ticker.
func (m *Manager) StartUpdater(ctx context.Context, interval time.Duration) {
	slog.Info("starting database updater", "interval", interval)
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("database updater received stop signal")
				return
			case <-ticker.C:
				slog.Info("running scheduled database update")
				if err := m.UpdateDatabases(ctx); err != nil {
					slog.Error("scheduled database update failed", "error", err)
				}
			}
		}
	}()
}

// UpdateDatabases downloads the latest DB-IP databases, builds an in-memory index,
// and atomically replaces the active snapshot with zero downtime.
func (m *Manager) UpdateDatabases(ctx context.Context) error {
	m.updateMu.Lock()
	defer m.updateMu.Unlock()

	tmpFiles, err := m.downloadToTemp(ctx)
	if err != nil {
		return fmt.Errorf("downloading databases: %w", err)
	}
	defer func() {
		for _, tmp := range tmpFiles {
			_ = os.Remove(tmp)
		}
	}()

	cityTmp := tmpFiles[m.cityPath]
	asnTmp := tmpFiles[m.asnPath]

	// Verify both temp files open cleanly before moving them
	newCityDB, err := maxminddb.Open(cityTmp)
	if err != nil {
		return fmt.Errorf("verifying downloaded city database: %w", err)
	}

	newAsnDB, err := maxminddb.Open(asnTmp)
	if err != nil {
		_ = newCityDB.Close()
		return fmt.Errorf("verifying downloaded asn database: %w", err)
	}

	newPrefixMap := buildPrefixMap(newAsnDB)
	_ = newCityDB.Close()
	_ = newAsnDB.Close()

	// Atomically move temp files over target files
	for targetPath, tmpPath := range tmpFiles {
		if err := os.Rename(tmpPath, targetPath); err != nil {
			return fmt.Errorf("replacing database file %q: %w", targetPath, err)
		}
	}

	// Reopen final files for long-lived snapshot
	finalCityDB, err := maxminddb.Open(m.cityPath)
	if err != nil {
		return fmt.Errorf("reopening updated city database: %w", err)
	}

	finalAsnDB, err := maxminddb.Open(m.asnPath)
	if err != nil {
		_ = finalCityDB.Close()
		return fmt.Errorf("reopening updated asn database: %w", err)
	}

	newSnapshot := &Snapshot{
		CityDB:       finalCityDB,
		ASNDB:        finalAsnDB,
		ASNPrefixMap: newPrefixMap,
	}

	// Atomically swap the active snapshot
	oldSnapshot := m.snapshot.Swap(newSnapshot)

	// Close old readers in the background after a brief grace window to allow in-flight lookups to finish
	if oldSnapshot != nil {
		go func(old *Snapshot) {
			time.Sleep(10 * time.Second)
			_ = old.Close()
		}(oldSnapshot)
	}

	slog.Info("successfully updated and hot-reloaded databases")
	return nil
}

func (m *Manager) downloadToTemp(ctx context.Context) (map[string]string, error) {
	dateStr := time.Now().Format("2006-01")

	targets := map[string]string{
		m.cityPath: fmt.Sprintf("dbip-city-lite-%s", dateStr),
		m.asnPath:  fmt.Sprintf("dbip-asn-lite-%s", dateStr),
	}

	tmpFiles := make(map[string]string, len(targets))
	for localPath, urlName := range targets {
		downloadURL := fmt.Sprintf("https://download.db-ip.com/free/%s.mmdb.gz", urlName)
		tmpPath := localPath + ".tmp"

		if err := m.downloadFile(ctx, downloadURL, tmpPath); err != nil {
			for _, f := range tmpFiles {
				_ = os.Remove(f)
			}
			return nil, fmt.Errorf("downloading %s: %w", urlName, err)
		}
		tmpFiles[localPath] = tmpPath
	}

	return tmpFiles, nil
}

func (m *Manager) downloadFile(ctx context.Context, url, destPath string) error {
	slog.Info("downloading database update", "url", url, "dest", destPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating download request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing download request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("initializing gzip reader: %w", err)
	}
	defer func() {
		_ = gzr.Close()
	}()

	cleanPath := filepath.Clean(destPath)
	outFile, err := os.Create(cleanPath)
	if err != nil {
		return fmt.Errorf("creating temp file %q: %w", destPath, err)
	}

	// Limit to 500MB to guard against decompression bombs (gosec G110)
	const maxDecompressedBytes = 500 * 1024 * 1024
	if _, err := io.Copy(outFile, io.LimitReader(gzr, maxDecompressedBytes)); err != nil {
		_ = outFile.Close()
		_ = os.Remove(destPath)
		return fmt.Errorf("copying database contents: %w", err)
	}

	if err := outFile.Close(); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("closing temp file %q: %w", destPath, err)
	}

	return nil
}
