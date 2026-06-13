package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
)

type HookEvent string

const (
	// Prestart is a list of hooks to be run before the container process is executed.
	// It is called in the Runtime Namespace.
	//
	// NOTE: this corresponds to the OCI "prestart" hook, which the runtime-spec
	// marks deprecated in favour of CreateRuntime, CreateContainer, and
	// StartContainer. We intentionally still support it because the OCI Runtime
	// integration tests and other tools (e.g. Docker) continue to rely on it.
	// The doc comment deliberately avoids the "Deprecated:" prefix so this
	// project's own symbol does not trip deprecation analyzers; the upstream
	// field access is suppressed separately at its use site.
	Prestart HookEvent = "Prestart"

	// CreateRuntime is a list of hooks to be run after the container has been created but before pivot_root or any equivalent operation has been called
	// It is called in the Runtime Namespace
	CreateRuntime HookEvent = "CreateRuntime"
	// CreateContainer is a list of hooks to be run after the container has been created but before pivot_root or any equivalent operation has been called
	// It is called in the Container Namespace
	CreateContainer HookEvent = "CreateContainer"
	// StartContainer is a list of hooks to be run after the start operation is called but before the container process is started
	// It is called in the Container Namespace
	StartContainer HookEvent = "StartContainer"
	// Poststart is a list of hooks to be run after the container process is started.
	// It is called in the Runtime Namespace
	Poststart HookEvent = "Poststart"
	// Poststop is a list of hooks to be run after the container process exits.
	// It is called in the Runtime Namespace
	Poststop HookEvent = "Poststop"
)

// ExecHooks runs the
// hooks for the current lifecycle
func ExecHooks(hooks []specs.Hook, state *specs.State) error {
	s, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	for _, h := range hooks {
		ctx := context.Background()

		if h.Timeout != nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(
				ctx,
				time.Duration(*h.Timeout)*time.Second,
			)
			defer cancel()
		}

		binary, err := exec.LookPath(h.Path)
		if err != nil {
			return fmt.Errorf("find path of hook binary: %w", err)
		}

		path := filepath.Dir(h.Path)

		cmd := exec.CommandContext(ctx, binary, path)

		cmd.Args = append(h.Args, string(s))
		cmd.Env = h.Env
		cmd.Stdin = strings.NewReader(string(s))

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("execute hook %s: %w", h.Path, err)
		}
	}

	return nil
}
