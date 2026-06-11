package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const containerRuntime = "boxes"

type Dirs struct {
	Config  string
	Data    string
	State   string
	Cache   string
	Runtime string
}

var runtimeDirs Dirs

func init() {
	runtimeDirs = getDirs()
	err := runtimeDirs.ensureAll()
	if err != nil {
		panic(fmt.Sprintf("error occurred initializing runtime: %v", err))
	}
}

func GetDirs() Dirs {
	return runtimeDirs
}

func (d Dirs) ensureAll() error {
	for _, dir := range []string{d.Config, d.Data, d.State, d.Cache, d.Runtime} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func getDirs() Dirs {
	dirs, initErr := resolve()
	if initErr != nil {
		panic(fmt.Sprintf("error occurred initializing runtime: %v", initErr))
	}
	return *dirs
}

func resolve() (*Dirs, error) {
	switch runtime.GOOS {
	case "windows":
		return resolveWindows()
	default:
		return resolveXDG()
	}
}

func resolveXDG() (*Dirs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	xdg := func(envVar, fallback string) string {
		if d := os.Getenv(envVar); d != "" {
			return filepath.Join(d, containerRuntime)
		}
		return filepath.Join(home, fallback, containerRuntime)
	}
	var runtimeDir string
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		runtimeDir = filepath.Join(d, containerRuntime)
	} else {
		runtimeDir = filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", containerRuntime, os.Getuid()))
	}

	return &Dirs{
		Config:  xdg("XDG_CONFIG_HOME", ".config"),
		Data:    xdg("XDG_DATA_HOME", ".local/share"),
		State:   xdg("XDG_STATE_HOME", ".local/state"),
		Cache:   xdg("XDG_CACHE_HOME", ".cache"),
		Runtime: runtimeDir,
	}, nil
}

func resolveWindows() (*Dirs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	winDir := func(envVar, fallback string) string {
		if d := os.Getenv(envVar); d != "" {
			return filepath.Join(d, containerRuntime)
		}
		return filepath.Join(home, fallback, containerRuntime)
	}
	return &Dirs{
		Config:  winDir("APPDATA", filepath.Join("AppData", "Roaming")),
		Data:    winDir("LOCALAPPDATA", filepath.Join("AppData", "Local")),
		State:   winDir("LOCALAPPDATA", filepath.Join("AppData", "Local")),
		Cache:   winDir("TEMP", filepath.Join("AppData", "Local", "Temp")),
		Runtime: winDir("TEMP", filepath.Join("AppData", "Local", "Temp")),
	}, nil
}
