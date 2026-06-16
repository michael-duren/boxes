package operations

import (
	"fmt"
	"log/slog"

	"github.com/michael-duren/boxes/internal/container"
)

type ReexecOpts struct {
	ID string
}

func Reexec(opts *ReexecOpts) error {
	slog.Info("reexec operation", "id", opts.ID)

	cntr, err := container.Load(opts.ID)
	if err != nil {
		slog.Error("failed to load container", "id", opts.ID, "err", err)
		return fmt.Errorf("load container: %w", err)
	}

	if err := cntr.Reexec(); err != nil {
		slog.Error("failed to reexec container", "id", opts.ID, "err", err)
		return fmt.Errorf("reexec container: %w", err)
	}

	slog.Info("reexec operation complete", "id", opts.ID)
	return nil
}
