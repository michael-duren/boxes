package operations

import (
	"fmt"
	"log/slog"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/michael-duren/boxes/internal/container"
)

type KillOpts struct {
	ID     string
	Signal string
}

func Kill(opts *KillOpts) error {
	slog.Info("kill operation", "id", opts.ID, "signal", opts.Signal)

	cntr, err := container.Load(opts.ID)
	if err != nil {
		slog.Error("failed to load container", "id", opts.ID, "err", err)
		return fmt.Errorf("load container: %w", err)
	}

	sig, err := strconv.Atoi(opts.Signal)
	if err != nil {
		slog.Error("failed to convert signal to int", "id", opts.ID, "signal", opts.Signal, "err", err)
		return fmt.Errorf("convert signal to int: %w", err)
	}

	if err := cntr.Kill(unix.Signal(sig)); err != nil {
		slog.Error("failed to kill container", "id", opts.ID, "signal", sig, "err", err)
		return fmt.Errorf("kill container: %w", err)
	}

	if err := cntr.Save(); err != nil {
		slog.Error("failed to save container", "id", opts.ID, "err", err)
		return fmt.Errorf("save container: %w", err)
	}

	slog.Info("kill operation complete", "id", opts.ID, "signal", sig)
	return nil
}
