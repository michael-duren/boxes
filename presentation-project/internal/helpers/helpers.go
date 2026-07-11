// a simple package of helpers for the presentation
package helpers

import "fmt"

func Usage() {
	fmt.Println(`
About:
		A basic containerized process for educational purposes, runs the rootfs located in the working directory from where the command was run
Usage:
        demo <command> [arguments]
Commands:
        run         start a bug report
Example:
	  demo run /bin/bash         Runs '/bin/bash' in a containerized process
		`)
}
