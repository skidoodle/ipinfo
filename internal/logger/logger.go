package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

// init initializes the logger
func init() {
	Log = slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
