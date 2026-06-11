package cli

import (
	"github.com/michael-duren/boxes/internal/operations"
	"github.com/spf13/cobra"
)

func deleteCmd() *cobra.Command {
	var delCmd = &cobra.Command{
		Use:  "delete [flags] CONTAINER_ID",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			containerID := args[0]

			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}

			return operations.Delete(&operations.DeleteOpts{
				ID:    containerID,
				Force: force,
			})
		},
	}

	delCmd.Flags().Bool("force", false, "force delete a container")
	return delCmd
}
