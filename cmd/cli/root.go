package main

import (
	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "box",
		Short:        "boxes is an OCI container runtime",
		SilenceUsage: true,
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
