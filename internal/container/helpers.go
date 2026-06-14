package container

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/michael-duren/boxes/internal/filesystem"
)

func listenUnix(sockPath string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(sockPath), 0755); err != nil {
		return nil, fmt.Errorf("unable to create socket directory: %w", err)
	}

	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale sock: %w", err)
	}

	listener, err := net.Listen(
		"unix",
		sockPath,
	)

	if err != nil {
		return nil, fmt.Errorf("listen on init sock: %w", err)
	}
	return listener, nil
}

func exists(containerID string) bool {
	dirs := filesystem.GetDirs()
	_, err := os.Stat(filepath.Join(dirs.State, containerID))
	return err == nil
}
