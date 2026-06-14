package container

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/michael-duren/boxes/internal/filesystem"
)

func listenUnix(sockPath string) (net.Listener, error) {
	slog.Debug("creating unix listener", "path", sockPath)

	if err := os.MkdirAll(filepath.Dir(sockPath), 0755); err != nil {
		slog.Error("failed to create socket directory", "path", sockPath, "err", err)
		return nil, fmt.Errorf("unable to create socket directory: %w", err)
	}

	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		slog.Error("failed to remove stale socket", "path", sockPath, "err", err)
		return nil, fmt.Errorf("remove stale sock: %w", err)
	}

	listener, err := net.Listen(
		"unix",
		sockPath,
	)

	if err != nil {
		slog.Error("failed to listen on unix socket", "path", sockPath, "err", err)
		return nil, fmt.Errorf("listen on init sock: %w", err)
	}

	slog.Debug("unix listener created", "path", sockPath)
	return listener, nil
}

func exists(containerID string) bool {
	dirs := filesystem.GetDirs()
	_, err := os.Stat(filepath.Join(dirs.State, containerID))
	found := err == nil
	slog.Debug("checking if container exists", "id", containerID, "exists", found)
	return found
}
