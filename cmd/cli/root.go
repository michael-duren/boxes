package main

import (
	"fmt"

	"github.com/michael-duren/boxes/internal/operations"
	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "box",
		Short:        "boxes is an OCI container runtime",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			containerID := args[0]

			state, err := operations.State(&operations.StateOpts{
				ID: containerID,
			})

			if err != nil {
				return err
			}

			// TODO: do something with 'state'
			fmt.Println(state)

			return nil
		},
	}

	cmd.AddCommand(
		stateCmd(),
		createCmd(),
		startCmd(),
		deleteCmd(),
		killCmd(),
	)

	return cmd
}
