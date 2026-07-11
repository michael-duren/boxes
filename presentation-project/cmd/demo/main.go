package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/michael-duren/boxes/presentation-project/internal/helpers"
)

func main() {
	if len(os.Args) < 3 {
		helpers.Usage()
		return
	}
	cmd := os.Args[1]
	ctrCmd := os.Args[2]
	cmdArgs := os.Args[3:]
	switch cmd {
	case "run":
		run(ctrCmd, cmdArgs)
	case "reexec":
		reexec(ctrCmd, cmdArgs)
	default:
		helpers.Usage()
	}
}

func run(cmdName string, args []string) {
	fmt.Println("running cmd: ", cmdName, "with args: ", args)
	cmd := exec.Command(cmdName, args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println("error: ", err)
	}
}

func reexec(cmdName string, args []string) {
	fmt.Println("reexecing args: ", args)
	exec.Command(cmdName, args...)
}
