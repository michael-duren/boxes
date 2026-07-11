package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/michael-duren/boxes/internal/operations"
)

// runCmd is `create` + `start` in a single command. By default it runs the
// container in the foreground and blocks until the container process exits,
// propagating its exit code; with --detach it behaves like create followed by
// start and returns immediately.
func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "run [flags] CONTAINER_ID",
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

			detach, err := cmd.Flags().GetBool("detach")
			if err != nil {
				return err
			}

			cmd.SilenceUsage = true
			return operations.Run(&operations.RunOpts{
				ID:            containerID,
				Bundle:        bundle,
				PidFile:       pidFile,
				ConsoleSocket: consoleSocket,
				Detach:        detach,
			})
		},
	}

	cwd, _ := os.Getwd()
	cmd.Flags().StringP("bundle", "b", cwd, "Path to bundle directory")
	cmd.Flags().String("pid-file", "", "Path to a file to write the container process PID")
	cmd.Flags().String("console-socket", "", "Path to an AF_UNIX socket that receives the pty master when the config requests a terminal")
	cmd.Flags().BoolP("detach", "d", false, "Detach from the container process and return once it is running")

	return cmd
}
