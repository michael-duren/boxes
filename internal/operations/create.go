package operations

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/michael-duren/boxes/internal/container"
)

type CreateOpts struct {
	// container id
	ID string
	// the path to the os bundle
	Bundle string
}

func Create(opts *CreateOpts) error {
	slog.Info("create operation", "id", opts.ID, "bundle", opts.Bundle)

	bundle, err := filepath.Abs(opts.Bundle)
	if err != nil {
		slog.Error("failed to resolve absolute bundle path", "id", opts.ID, "bundle", opts.Bundle, "err", err)
		return fmt.Errorf("absolute path from bundle: %w", err)
	}

	configPath := filepath.Join(bundle, "config.json")
	slog.Debug("reading config file", "id", opts.ID, "path", configPath)
	config, err := os.ReadFile(configPath)
	if err != nil {
		slog.Error("failed to read config file", "id", opts.ID, "path", configPath, "err", err)
		return fmt.Errorf("read config file: %w", err)
	}

	var spec *specs.Spec
	if err := json.Unmarshal(config, &spec); err != nil {
		slog.Error("failed to unmarshal config", "id", opts.ID, "err", err)
		return fmt.Errorf("unmarshall config: %w", err)
	}

	cntr, err := container.New(&container.NewContainerOpts{
		ID:     opts.ID,
		Bundle: bundle,
		Spec:   spec,
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

	slog.Info("create operation complete", "id", opts.ID)
	return nil
}
