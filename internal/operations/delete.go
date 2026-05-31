package operations

import (
	"fmt"

	"github.com/michael-duren/boxes/internal/container"
)

type DeleteOpts struct {
	ID    string
	Force bool
}

func Delete(opts *DeleteOpts) error {
	cntr, err := container.Load(opts.ID)
	if err != nil {
		return fmt.Errorf("load container: %w", err)
	}

	if err := cntr.Delete(opts.Force); err != nil {
		return fmt.Errorf("deleteting container with ID %s: %w", opts.ID, err)
	}

	return nil
}
