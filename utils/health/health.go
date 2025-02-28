package utils

import (
	"net/http"
)

// Returns a simple health check handler
func HealthCheck() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Healthy"))
	})
	return mux
}
