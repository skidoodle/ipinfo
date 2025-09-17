package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ipinfo/internal/db"
	"ipinfo/internal/server"

	"github.com/joho/godotenv"
)

// main is the entry point of the application
func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info("env file not found, using system environment variables")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	geoIP, err := db.NewGeoIPManager()
	if err != nil {
		slog.Error("failed to initialize databases", "error", err)
		os.Exit(1)
	}
	defer geoIP.Close()

	geoIP.StartUpdater(ctx, 24*time.Hour)

	slog.Info("starting server")
	appServer := server.NewServer(geoIP)
	if err := appServer.Start(ctx); err != nil {
		slog.Error("server failed to start", "error", err)
		os.Exit(1)
	}
	slog.Info("server shut down gracefully")
}
