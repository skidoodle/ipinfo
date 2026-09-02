package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// sendJSONResponse encodes and writes an indented JSON response with the given status code.
func sendJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

// sendJSONError writes a JSON error response with the standard { "error": "message" } format.
func sendJSONError(w http.ResponseWriter, statusCode int, message string) {
	sendJSONResponse(w, statusCode, map[string]string{
		"error": message,
	})
}
