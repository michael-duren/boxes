package operations

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/michael-duren/boxes/internal/container"
	"github.com/michael-duren/boxes/internal/filesystem"
)

type CreateOpts struct {
	// container id
	ID string
	// the path to the os bundle
	Bundle string
	// PidFile, when set, is the path the container process PID is written to
	// after the container is created. This follows the runc/runtime-tools CLI
	// convention (not the OCI runtime spec, which defines no CLI). Empty means
	// do not write a pid file.
	PidFile string
}

func Create(opts *CreateOpts) error {
	slog.Info("create operation", "id", opts.ID, "bundle", opts.Bundle)

	bundle, err := filepath.Abs(opts.Bundle)
	if err != nil {
		slog.Error("failed to resolve absolute bundle path", "id", opts.ID, "bundle", opts.Bundle, "err", err)
		return fmt.Errorf("absolute path from bundle: %w", err)
	}
	slog.Debug("resolved absolute bundle path", "id", opts.ID, "bundle", bundle)

	configPath := filepath.Join(bundle, "config.json")
	slog.Debug("reading config file", "id", opts.ID, "path", configPath)
	config, err := os.ReadFile(configPath)
	if err != nil {
		slog.Error("failed to read config file", "id", opts.ID, "path", configPath, "err", err)
		return fmt.Errorf("read config file: %w", err)
	}
	slog.Debug("read config file", "id", opts.ID, "path", configPath, "bytes", len(config))

	var spec *specs.Spec
	if err := json.Unmarshal(config, &spec); err != nil {
		slog.Error("failed to unmarshal config", "id", opts.ID, "err", err)
		return fmt.Errorf("unmarshall config: %w", err)
	}
	slog.Debug("parsed container config",
		"id", opts.ID,
		"ociVersion", spec.Version,
		"hostname", spec.Hostname,
	)

	cntr, err := container.New(&container.NewContainerOpts{
		ID:     opts.ID,
		Bundle: bundle,
		Spec:   spec,
		Dirs:   filesystem.GetDirs(),
	})

	if err != nil {
		slog.Error("failed to create container", "id", opts.ID, "err", err)
		return fmt.Errorf("create container: %w", err)
	}

	if err := cntr.Save(); err != nil {
		slog.Error("failed to save container", "id", opts.ID, "err", err)
		return fmt.Errorf("save container: %w", err)
	}

	if err := cntr.Init(); err != nil {
		slog.Error("failed to initialize container", "id", opts.ID, "err", err)
		return fmt.Errorf("initialize container: %w", err)
	}

	if err := cntr.Save(); err != nil {
		slog.Error("failed to save container", "id", opts.ID, "err", err)
		return fmt.Errorf("save container: %w", err)
	}

	if opts.PidFile != "" {
		if err := os.WriteFile(opts.PidFile, []byte(strconv.Itoa(cntr.State.Pid)), 0o644); err != nil {
			slog.Error("failed to write pid file", "id", opts.ID, "pidFile", opts.PidFile, "err", err)
			return fmt.Errorf("write pid file: %w", err)
		}
		slog.Debug("wrote pid file", "id", opts.ID, "pidFile", opts.PidFile, "pid", cntr.State.Pid)
	}

	slog.Info("create operation complete", "id", opts.ID)
	return nil
}
