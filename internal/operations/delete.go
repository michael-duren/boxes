package operations

import (
	"fmt"
	"log/slog"

	"github.com/michael-duren/boxes/internal/container"
	"github.com/michael-duren/boxes/internal/filesystem"
)

type DeleteOpts struct {
	ID    string
	Force bool
}

func Delete(opts *DeleteOpts) error {
	slog.Info("delete operation", "id", opts.ID, "force", opts.Force)

	cntr, err := container.Load(opts.ID, filesystem.GetDirs())
	if err != nil {
		slog.Error("failed to load container", "id", opts.ID, "err", err)
		return fmt.Errorf("load container: %w", err)
	}

	if err := cntr.Delete(opts.Force); err != nil {
		slog.Error("failed to delete container", "id", opts.ID, "err", err)
		return fmt.Errorf("deleteting container with ID %s: %w", opts.ID, err)
	}

	slog.Info("delete operation complete", "id", opts.ID)
	return nil
}
