package operations

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

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

	sig, err := parseSignal(opts.Signal)
	if err != nil {
		slog.Error("unable to parse signal from kill command", "id", opts.ID, "signal", opts.Signal)
		return fmt.Errorf("parseSignal: %w", err)
	}

	if err := cntr.Kill(sig); err != nil {
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

func parseSignal(rawSignal string) (unix.Signal, error) {
	s, err := strconv.Atoi(rawSignal)
	if err == nil {
		return unix.Signal(s), nil
	}

	slog.Debug("user passed code rather than int signal", "signal", rawSignal)
	sig := strings.ToUpper(rawSignal)
	if !strings.HasPrefix(sig, "SIG") {
		sig = "SIG" + sig
	}
	signal := unix.SignalNum(sig)
	if signal == 0 {
		slog.Warn("unknown signal passed to kill command", "signal", signal)
		return -1, fmt.Errorf("unknown signal %q", rawSignal)
	}
	return signal, nil
}
