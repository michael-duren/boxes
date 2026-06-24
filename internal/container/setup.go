package container

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func SetupParent(c *Container, config *specs.Spec) error {
	if c == nil || config == nil {
		return fmt.Errorf("nil structs passed to setup parent container: %v, config: %v", c, config)
	}
	return nil
}

func SetupChild(c *Container, config *specs.Spec) error {
	if c == nil || config == nil {
		return fmt.Errorf("nil structs passed to setup parent container: %v, config: %v", c, config)
	}
	return nil
}
