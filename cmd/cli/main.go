package main

import (
	"os"

	"github.com/michael-duren/boxes/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
