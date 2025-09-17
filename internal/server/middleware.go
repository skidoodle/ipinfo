package server

import (
	"compress/gzip"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// gzipResponseWriter is a wrapper for gzip compression.
type gzipResponseWriter struct {
	http.ResponseWriter
	Writer *gzip.Writer
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w gzipResponseWriter) Close() {
	if err := w.Writer.Close(); err != nil {
		slog.Error("failed to close gzip writer", "error", err)
	}
}

// newGzipResponseWriter wraps the ResponseWriter if the client accepts gzip.
func newGzipResponseWriter(w http.ResponseWriter, r *http.Request) http.ResponseWriter {
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		return gzipResponseWriter{ResponseWriter: w, Writer: gz}
	}
	return w
}

// loggingMiddleware logs the incoming HTTP request and its duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		slog.Info(fmt.Sprintf("%s %s from %s in %s",
			r.Method,
			r.URL.Path,
			GetRealIP(r),
			duration,
		))
	})
}
