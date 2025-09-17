package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// sendJSONResponse sends a JSON response with the given data and status code.
func sendJSONResponse(w http.ResponseWriter, data any, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

// sendJSONError sends a JSON error response with the given message and status code.
func sendJSONError(w http.ResponseWriter, errMsg string, statusCode int) {
	sendJSONResponse(w, map[string]string{"error": errMsg}, statusCode)
}
