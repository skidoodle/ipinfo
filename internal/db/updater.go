package db

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
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
	tmpFiles, err := g.downloadToTemp(context.Background())
	if err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cityDB != nil {
		_ = g.cityDB.Close()
		g.cityDB = nil
	}
	if g.asnDB != nil {
		_ = g.asnDB.Close()
		g.asnDB = nil
	}

	for targetPath, tmpPath := range tmpFiles {
		if err := os.Rename(tmpPath, targetPath); err != nil {
			slog.Error("failed to replace database file", "target", targetPath, "tmp", tmpPath, "err", err)
		}
	}

	var openErr error
	g.cityDB, openErr = maxminddb.Open(CityDBPath)
	if openErr != nil {
		slog.Error("failed to reopen city database", "err", openErr)
	}

	g.asnDB, openErr = maxminddb.Open(ASNDBPath)
	if openErr != nil {
		slog.Error("failed to reopen asn database", "err", openErr)
	}

	g.buildASNPrefixMap()
	slog.Info("successfully updated and reloaded databases")
	return nil
}

// downloadToTemp downloads the current month's DB-IP databases to temporary files.
func (g *GeoIPManager) downloadToTemp(ctx context.Context) (map[string]string, error) {
	now := time.Now()
	dateStr := now.Format("2006-01")

	targets := map[string]string{
		CityDBPath: fmt.Sprintf("dbip-city-lite-%s", dateStr),
		ASNDBPath:  fmt.Sprintf("dbip-asn-lite-%s", dateStr),
	}

	results := make(map[string]string)
	var firstError error

	for localPath, urlName := range targets {
		downloadURL := fmt.Sprintf("https://download.db-ip.com/free/%s.mmdb.gz", urlName)
		tmpPath := localPath + ".tmp"

		if err := g.downloadFile(ctx, downloadURL, tmpPath); err != nil {
			slog.Error("failed to download database", "url", downloadURL, "err", err)
			if firstError == nil {
				firstError = err
			}
			continue
		}
		results[localPath] = tmpPath
	}

	if firstError != nil {
		for _, tmp := range results {
			_ = os.Remove(tmp)
		}
		return nil, firstError
	}

	return results, nil
}

// downloadFile downloads a file from a URL, decompresses it, and saves it to destPath.
func (g *GeoIPManager) downloadFile(ctx context.Context, url, destPath string) error {
	slog.Info("checking for updates", "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body", "err", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	slog.Info("downloading and decompressing", "destination", destPath)

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("could not create gzip reader: %w", err)
	}
	defer func() {
		if err := gzr.Close(); err != nil {
			slog.Error("failed to close gzip reader", "err", err)
		}
	}()

	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("could not create temporary file: %w", err)
	}

	closeFile := func() error {
		if err := outFile.Close(); err != nil {
			return fmt.Errorf("failed to close output file: %w", err)
		}
		return nil
	}

	if _, err := io.Copy(outFile, gzr); err != nil {
		_ = closeFile()
		_ = os.Remove(destPath)
		return fmt.Errorf("could not decompress and write db file: %w", err)
	}

	if err := closeFile(); err != nil {
		return err
	}

	slog.Info("successfully downloaded", "file", destPath)
	return nil
}
