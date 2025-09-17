package utils

import (
	"log/slog"
	"net/http"
)

// HealthCheck Returns a simple health check handler.
func HealthCheck() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			slog.Warn("failed to write healthcheck response",
				"component", "healthcheck",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
		}
	})
}
