package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"ipinfo/internal/config"
	"ipinfo/internal/lookup"
)

// Server encapsulates the HTTP server instance.
type Server struct {
	httpServer *http.Server
}

// NewServer creates and configures the HTTP server.
func NewServer(cfg config.Config, svc *lookup.Service) *Server {
	router := newRouter(svc)

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Addr,
			Handler:      router,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Start runs the HTTP server and coordinates graceful shutdown when ctx is canceled.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		slog.Info("server listening", "address", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server failed: %w", err)
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server graceful shutdown failed: %w", err)
	}

	slog.Info("server shut down gracefully")
	return nil
}
