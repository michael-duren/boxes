package operations

import (
	"fmt"
	"log/slog"

	"github.com/michael-duren/boxes/internal/container"
	"github.com/michael-duren/boxes/internal/filesystem"
)

type StartOpts struct {
	ID string
}

func Start(opts *StartOpts) error {
	slog.Info("start operation", "id", opts.ID)

	cntr, err := container.Load(opts.ID, filesystem.GetDirs())

	if err != nil {
		slog.Error("failed to load container", "id", opts.ID, "err", err)
		return fmt.Errorf("load container: %w", err)
	}

	if err := cntr.Start(); err != nil {
		slog.Error("failed to start container", "id", opts.ID, "err", err)
		return fmt.Errorf("start container: %w", err)
	}

	if err := cntr.Save(); err != nil {
		slog.Error("failed to save container", "id", opts.ID, "err", err)
		return fmt.Errorf("save container: %w", err)
	}

	slog.Info("start operation complete", "id", opts.ID)
	return nil
}
