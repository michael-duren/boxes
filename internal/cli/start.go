package cli

import (
	"github.com/spf13/cobra"

	"github.com/michael-duren/boxes/internal/operations"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "start [flags] CONTAINER_ID",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			containerID := args[0]

			return operations.Start(&operations.StartOpts{
				ID: containerID,
			})
		},
	}

	return cmd
}
