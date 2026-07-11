package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/michael-duren/boxes/internal/operations"
)

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "create [flags] CONTAINER_ID",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			containerID := args[0]

			bundle, err := cmd.Flags().GetString("bundle")
			if err != nil {
				return err
			}

			pidFile, err := cmd.Flags().GetString("pid-file")
			if err != nil {
				return err
			}

			consoleSocket, err := cmd.Flags().GetString("console-socket")
			if err != nil {
				return err
			}

			cmd.SilenceUsage = true
			_, err = operations.Create(&operations.CreateOpts{
				ID:            containerID,
				Bundle:        bundle,
				PidFile:       pidFile,
				ConsoleSocket: consoleSocket,
			})
			return err
		},
	}

	cwd, _ := os.Getwd()
	cmd.Flags().StringP("bundle", "b", cwd, "Path to bundle directory")
	cmd.Flags().String("pid-file", "", "Path to a file to write the container process PID")
	cmd.Flags().String("console-socket", "", "Path to an AF_UNIX socket that receives the pty master when the config requests a terminal")

	return cmd
}
