package operations

import "fmt"

type KillOpts struct {
	ID     string
	Signal string
}

func Kill(opts *KillOpts) error {
	fmt.Println(opts)

	return nil
}
