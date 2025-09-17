package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"ipinfo/internal/db"
)

// Server represents the HTTP server.
type Server struct {
	server *http.Server
}

// NewServer creates a new HTTP server.
func NewServer(geoIP *db.GeoIPManager) *Server {
	// The router is now created in its own file.
	handler := newRouter(geoIP)

	return &Server{
		server: &http.Server{
			Addr:         ":3000",
			Handler:      handler,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Start starts the HTTP server and handles graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	go func() {
		slog.Info("server listening", "address", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	slog.Info("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "error", err)
		return err
	}

	slog.Info("shutdown complete")
	return nil
}
