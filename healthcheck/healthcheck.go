package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	resp, err := http.Get("http://localhost:3000/health")
	if err != nil {
		slog.Error("error performing healthcheck", "err", err)
		os.Exit(1)
	}

	defer func() {
		if cerr := resp.Body.Close(); err != nil {
			slog.Warn("failed to close response body", "err", cerr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		slog.Error("healthcheck failed", "status", resp.StatusCode)
		os.Exit(1)
	}

	fmt.Println("OK")
	os.Exit(0)
}
