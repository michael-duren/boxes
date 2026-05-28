package container

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	containerRootDir = "/var/lib/boxes/containers"
)

type Container struct {
	State *specs.State
	Spec  *specs.Spec
}

type NewContainerOpts struct {
	ID     string
	Bundle string
	Spec   *specs.Spec
}

func New(opts *NewContainerOpts) (*Container, error) {
	if exists(opts.ID) {
		return nil, fmt.Errorf("container '%s' exists", opts.ID)
	}

	state := specs.State{
		Version:     specs.Version,
		ID:          opts.ID,
		Bundle:      opts.Bundle,
		Annotations: opts.Spec.Annotations,
		Status:      specs.StateCreating,
	}

	c := Container{
		State: &state,
		Spec:  opts.Spec,
	}

	return &c, nil
}

func exists(containerID string) bool {
	_, err := os.Stat(filepath.Join(containerRootDir, containerID))

	return err == nil
}
