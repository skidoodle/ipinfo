package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	db "skidoodle/ipinfo/internal/db"
	logger "skidoodle/ipinfo/internal/logger"
	server "skidoodle/ipinfo/internal/server"

	"github.com/joho/godotenv"
)

// main is the entry point of the application
func main() {
	if err := godotenv.Load(); err != nil {
		logger.Log.Info("No .env file found, using system environment variables")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	geoIP, err := db.NewGeoIPManager()
	if err != nil {
		logger.Log.Error("Failed to initialize GeoIP databases", "error", err)
		os.Exit(1)
	}
	defer geoIP.Close()

	geoIP.StartUpdater(ctx, 24*time.Hour)

	logger.Log.Info("Starting server...")
	if err := server.StartServer(ctx, geoIP); err != nil {
		logger.Log.Error("Server failed", "error", err)
		os.Exit(1)
	}
	logger.Log.Info("Application shut down gracefully")
}
