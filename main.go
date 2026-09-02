// Package main is the entry point for the ipinfo service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ipinfo/internal/config"
	"ipinfo/internal/geoip"
	"ipinfo/internal/lookup"
	"ipinfo/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil {
		slog.Debug("env file not found, using system environment variables")
	}

	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	geoIP := geoip.NewManager(cfg.CityDBPath, cfg.ASNDBPath)
	if err := geoIP.Initialize(ctx); err != nil {
		return fmt.Errorf("initializing geoip databases: %w", err)
	}
	defer func() {
		if err := geoIP.Close(); err != nil {
			slog.Warn("closing geoip databases", "error", err)
		}
	}()

	lookupSvc, err := lookup.NewService(geoIP, cfg.CacheTTL)
	if err != nil {
		return fmt.Errorf("initializing lookup service: %w", err)
	}
	defer lookupSvc.Close()

	geoIP.StartUpdater(ctx, cfg.UpdateInterval)

	appServer := server.NewServer(cfg, lookupSvc)
	if err := appServer.Start(ctx); err != nil {
		return fmt.Errorf("http server: %w", err)
	}

	return nil
}
