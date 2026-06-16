package operations

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/michael-duren/boxes/internal/container"
)

type StateOpts struct {
	ID string
}

func State(opts *StateOpts) (string, error) {
	slog.Debug("state operation", "id", opts.ID)

	cntr, err := container.Load(opts.ID)
	if err != nil {
		slog.Error("failed to load container", "id", opts.ID, "err", err)
		return "", fmt.Errorf("load container: %w", err)
	}
	state, err := json.Marshal(cntr.State)
	if err != nil {
		slog.Error("failed to marshal state", "id", opts.ID, "err", err)
		return "", fmt.Errorf("marshal state: %w", err)
	}

	slog.Debug("state operation complete", "id", opts.ID, "status", cntr.State.Status)
	return string(state), nil
}
