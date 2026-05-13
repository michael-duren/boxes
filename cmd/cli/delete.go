package main

import (
	"github.com/michael-duren/boxes/internal/operations"
	"github.com/spf13/cobra"
)

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "delete [flags] CONTAINER_ID",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			containerID := args[0]

			return operations.Delete(&operations.DeleteOpts{
				ID: containerID,
			})
		},
	}

	return cmd
}
