package container

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/michael-duren/boxes/internal/hooks"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func (c *Container) canBeStarted() bool {
	return c.State.Status == specs.StateCreated
}

func (c *Container) Start() error {
	slog.Info("starting container", "id", c.State.ID, "status", c.State.Status)

	if c.Spec.Process == nil {
		slog.Error("no process in spec, nothing to start", "id", c.State.ID)
		return fmt.Errorf("no process in spec, nothing to start for id: %s", c.State.ID)
	}

	if !c.canBeStarted() {
		slog.Warn("container cannot be started in current state", "id", c.State.ID, "status", c.State.Status)
		return fmt.Errorf("container cannot be started in current state (%s)", c.State.Status)
	}

	err := c.execHooks(hooks.Prestart)

	if err != nil {
		return err
	}

	slog.Debug("dialing container sock to send start", "id", c.State.ID)
	conn, err := net.Dial(
		"unix",
		c.containerSockPath(),
	)
	if err != nil {
		slog.Error("failed to dial container sock", "id", c.State.ID, "err", err)
		return fmt.Errorf("dial container sock: %w", err)
	}

	if _, err := conn.Write([]byte("start")); err != nil {
		slog.Error("failed to write 'start' to container sock", "id", c.State.ID, "err", err)
		return fmt.Errorf("write 'start' msg to container sock: %w", err)
	}

	if err := conn.Close(); err != nil {
		// The start message was already delivered, so the container is running;
		// a failure closing our end of the socket must not fail the start.
		slog.Warn("failed to close connection after start msg", "id", c.State.ID, "err", err)
	}
	c.State.Status = specs.StateRunning

	// OCI: poststart hook failures MUST be logged but MUST NOT cause the start
	// to fail, since the container process is already running.
	if err := c.execHooks(hooks.Poststart); err != nil {
		slog.Error("poststart hook execution failed", "id", c.State.ID, "err", err)
	}

	slog.Info("container started", "id", c.State.ID, "status", c.State.Status)
	return nil
}

func (c *Container) canBeStarted() bool {
	return c.State.Status == specs.StateCreated
}

func (c *Container) Start() error {
	slog.Info("starting container", "id", c.State.ID, "status", c.State.Status)

	if c.Spec.Process == nil {
		slog.Error("no process in spec, nothing to start", "id", c.State.ID)
		return fmt.Errorf("no process in spec, nothing to start for id: %s", c.State.ID)
	}

	if !c.canBeStarted() {
		slog.Warn("container cannot be started in current state", "id", c.State.ID, "status", c.State.Status)
		return fmt.Errorf("container cannot be started in current state (%s)", c.State.Status)
	}

	err := c.execHooks(hooks.Prestart)

	if err != nil {
		return err
	}

	slog.Debug("dialing container sock to send start", "id", c.State.ID)
	conn, err := net.Dial(
		"unix",
		c.containerSockPath(),
	)
	if err != nil {
		slog.Error("failed to dial container sock", "id", c.State.ID, "err", err)
		return fmt.Errorf("dial container sock: %w", err)
	}

	if _, err := conn.Write([]byte("start")); err != nil {
		slog.Error("failed to write 'start' to container sock", "id", c.State.ID, "err", err)
		return fmt.Errorf("write 'start' msg to container sock: %w", err)
	}

	if err := conn.Close(); err != nil {
		// The start message was already delivered, so the container is running;
		// a failure closing our end of the socket must not fail the start.
		slog.Warn("failed to close connection after start msg", "id", c.State.ID, "err", err)
	}
	c.State.Status = specs.StateRunning

	// OCI: poststart hook failures MUST be logged but MUST NOT cause the start
	// to fail, since the container process is already running.
	if err := c.execHooks(hooks.Poststart); err != nil {
		slog.Error("poststart hook execution failed", "id", c.State.ID, "err", err)
	}

	slog.Info("container started", "id", c.State.ID, "status", c.State.Status)
	return nil
}
