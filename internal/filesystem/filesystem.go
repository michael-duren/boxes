package filesystem

import (
	"os"
	"path/filepath"
	"runtime"
)

const containerRuntime = "boxes"

var ContainerRootDir string

func init() {
	d, err := stateDir()
	if err != nil {
		panic(err)
	}
	ContainerRootDir = d
}

func stateDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("LOCALAPPDAATA"); d != "" {
			return filepath.Join(d, containerRuntime, "state"), nil
		}

		home, err := os.UserHomeDir()

		if err != nil {
			return "", err
		}

		return filepath.Join(home, "AppData", containerRuntime, "state"), nil
	default:
		if d := os.Getenv("XDG_STATE_HOME"); d != "" {
			return filepath.Join(d, containerRuntime), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "state", containerRuntime), nil
	}
}
