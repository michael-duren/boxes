package main

import (
	"fmt"
	"os"

	"github.com/michael-duren/boxes/presentation-project/cmd/internal/helpers"
)

func main() {
	if len(os.Args) < 3 {
		helpers.Usage()
		return
	}
	cmd := os.Args[1]
	cmdArgs := os.Args[2:]
	switch cmd {
	case "run":
		run(cmdArgs)
	case "reexec":
		reexec(cmdArgs)
	default:
		helpers.Usage()
	}
}

func run(args []string ) {
	fmt.Println("running args: ", args)
}

func reexec(args []string) {
	fmt.Println("reexecing args: ", args)
}

