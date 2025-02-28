package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	db "skidoodle/ipinfo/internal/db"
	server "skidoodle/ipinfo/internal/server"
)

func main() {
	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	go handleSignals(cancel)

	// Initialize GeoIP manager from internal/db package
	geoIP, err := db.NewGeoIPManager()
	if err != nil {
		log.Fatalf("Failed to initialize GeoIP databases: %v", err)
	}
	defer geoIP.Close()

	// Start database updater in background (update every 24 hours)
	geoIP.StartUpdater(ctx, 24*time.Hour)

	// Start health check and server
	server.StartServer(ctx, geoIP)
}

// handleSignals gracefully handles termination signals
func handleSignals(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %v, shutting down gracefully", sig)
	cancel()
}
