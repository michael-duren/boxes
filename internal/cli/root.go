package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/michael-duren/boxes/internal/filesystem"
	"github.com/michael-duren/boxes/internal/logger"
)

func NewRootCmd() *cobra.Command {
	var logFile *os.File

	cmd := &cobra.Command{
		Use:          "box",
		Short:        "boxes is an OCI container runtime",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			debug, err := cmd.Flags().GetBool("debug")
			if err != nil {
				return err
			}

			f, err := logger.Init(filesystem.GetDirs().State, debug)
			if err != nil {
				return err
			}
			logFile = f
			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if logFile != nil {
				_ = logFile.Close()
			}
		},
	}

	cmd.PersistentFlags().Bool("debug", false, "Enable debug logging")

	// default show usage, nested cmds need to disable this when 
	// runtime errors can occur
	cmd.SilenceUsage = false

	cmd.AddCommand(
		stateCmd(),
		createCmd(),
		startCmd(),
		deleteCmd(),
		killCmd(),
		reexecCmd(),
	)

	return cmd
}
