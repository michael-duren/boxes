package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/michael-duren/boxes/internal/assert"
	"github.com/michael-duren/boxes/internal/logger"
)

func TestLevel(t *testing.T) {
	tests := []struct {
		name  string
		debug bool
		want  slog.Level
	}{
		{"debug enabled", true, slog.LevelDebug},
		{"debug disabled", false, slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, logger.Level(tt.debug), tt.want)
		})
	}
}

func TestNewLevelGating(t *testing.T) {
	tests := []struct {
		name string
		// debug is the flag passed to New.
		debug bool
		// emit logs a single record against the constructed logger.
		emit func(l *slog.Logger)
		// wantLevel is the expected "level" field; "" means no output is expected.
		wantLevel string
	}{
		{"info logger emits info", false, func(l *slog.Logger) { l.Info("m") }, "INFO"},
		{"info logger drops debug", false, func(l *slog.Logger) { l.Debug("m") }, ""},
		{"debug logger emits debug", true, func(l *slog.Logger) { l.Debug("m") }, "DEBUG"},
		{"debug logger still emits info", true, func(l *slog.Logger) { l.Info("m") }, "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.emit(logger.New(&buf, tt.debug))

			if tt.wantLevel == "" {
				assert.Equal(t, buf.Len(), 0, "expected no output, got", buf.String())
				return
			}

			var entry map[string]any
			assert.NoError(t, json.Unmarshal(buf.Bytes(), &entry), "output:", buf.String())
			assert.Equal(t, entry["level"], any(tt.wantLevel))
		})
	}
}

func TestNewEmitsStructuredFields(t *testing.T) {
	var buf bytes.Buffer
	logger.New(&buf, false).Info("hello", "key", "value")

	var entry map[string]any
	assert.NoError(t, json.Unmarshal(buf.Bytes(), &entry), "output:", buf.String())
	assert.Equal(t, entry["msg"], any("hello"))
	assert.Equal(t, entry["key"], any("value"))
}

func TestInitCreatesLogFileAndDir(t *testing.T) {
	stateDir := t.TempDir()

	f, err := logger.Init(stateDir, false)
	assert.NoError(t, err)
	defer f.Close()

	wantPath := filepath.Join(stateDir, logger.LogDirName, logger.LogFileName)
	assert.Equal(t, f.Name(), wantPath)

	info, err := os.Stat(wantPath)
	assert.NoError(t, err, "expected log file to exist")
	assert.False(t, info.IsDir(), "expected a file, got a directory")
}

func TestInitWritesToLogFile(t *testing.T) {
	stateDir := t.TempDir()

	f, err := logger.Init(stateDir, false)
	assert.NoError(t, err)
	defer f.Close()

	// Init installs the logger as the slog default, so a package-level call
	// should land in the log file.
	slog.Info("recorded message")

	data, err := os.ReadFile(f.Name())
	assert.NoError(t, err)
	assert.Contains(t, string(data), "recorded message")
}

func TestInitAppendsAcrossCalls(t *testing.T) {
	stateDir := t.TempDir()

	f1, err := logger.Init(stateDir, false)
	assert.NoError(t, err)
	slog.Info("first run")
	f1.Close()

	f2, err := logger.Init(stateDir, false)
	assert.NoError(t, err)
	defer f2.Close()
	slog.Info("second run")

	data, err := os.ReadFile(f2.Name())
	assert.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "first run", "append should preserve earlier runs")
	assert.Contains(t, content, "second run")
}

func TestInitErrorsOnUnwritableStateDir(t *testing.T) {
	// A file (not a directory) at the state path makes MkdirAll fail.
	parent := t.TempDir()
	stateDir := filepath.Join(parent, "state")
	assert.NoError(t, os.WriteFile(stateDir, []byte("not a dir"), 0600))

	_, err := logger.Init(stateDir, false)
	assert.Error(t, err, "expected Init to fail when state dir cannot be created")
}
