// Package logger provides a structured logger for the boxes runtime.
//
// Logs are written to the user's state location (XDG_STATE_HOME/boxes/logs)
// as line-delimited JSON. The log level is configurable: callers pass a debug
// flag (wired to the CLI's --debug flag) to enable debug-level output.
package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	// LogDirName is the subdirectory under the state dir that holds log files.
	LogDirName = "logs"
	// LogFileName is the name of the log file boxes writes to.
	LogFileName = "boxes.log"
)

// Level returns the slog level implied by the debug flag.
func Level(debug bool) slog.Level {
	if debug {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

// New builds a structured JSON logger that writes to w at the level implied by
// debug. It performs no I/O of its own, which keeps it cheap to construct and
// easy to test against an in-memory writer.
func New(w io.Writer, debug bool) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: Level(debug),
	})
	return slog.New(handler)
}

// Init creates the log directory under stateDir, opens (or creates) the log
// file for appending, builds a logger, and installs it as the slog default so
// any command can log via the package-level slog functions.
//
// It returns the open file so the caller can close it when the command exits.
func Init(stateDir string, debug bool) (*os.File, error) {
	logDir := filepath.Join(stateDir, LogDirName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(
		filepath.Join(logDir, LogFileName),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0600,
	)
	if err != nil {
		return nil, err
	}

	slog.SetDefault(New(f, debug))
	return f, nil
}
