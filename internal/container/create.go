package container

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/michael-duren/boxes/internal/errs"
	"github.com/michael-duren/boxes/internal/hooks"
)

func (c *Container) Init() (err error) {
	slog.Info("initializing container", "id", c.State.ID, "bundle", c.State.Bundle)

	if c.Spec.Linux == nil {
		return fmt.Errorf("container runtime supports linux only")
	}

	err = c.execHooks(hooks.CreateRuntime)
	if err != nil {
		return err
	}

	listener, err := c.listenUnix()
	if err != nil {
		return c.cleanupOnErr(err)
	}

	defer errs.WrapDeferedClose(listener, &err)

	slog.Debug("reexecing container process", "id", c.State.ID)
	cmd := exec.Command("/proc/self/exe", "reexec", c.State.ID)
	err = c.applyNamespaces(cmd)
	if err != nil {
		return err
	}

	// should figure out where exactly this should go
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 5. reexec
	// proc filesystem is pseudo-fs, /self/exe is a link
	// to the cntr runtime itself
	if err = cmd.Start(); err != nil {
		slog.Error("failed to start reexec container process", "id", c.State.ID, "err", err)
		return c.cleanupOnErr(fmt.Errorf("reexec container process: %w", err))
	}

	c.State.Pid = cmd.Process.Pid
	slog.Debug("reexec container process started", "id", c.State.ID, "pid", c.State.Pid)

	// 6. release container process
	//
	if err = cmd.Process.Release(); err != nil {
		slog.Error("failed to release container process", "id", c.State.ID, "pid", c.State.Pid, "err", err)
		return c.cleanupOnErr(fmt.Errorf("releasing container process: %w", err))
	}
	// set deadline
	if ul, ok := listener.(*net.UnixListener); ok {
		err = ul.SetDeadline(time.Now().Add(10 * time.Second))
		if err != nil {
			slog.Error("failed to set deadline for runtime listener", "id", c.State.ID, "err", err)
			return c.cleanupOnErr(fmt.Errorf("unable to set deadline for runtime listener: %w", err))
		}
	}

	// 4. listen
	slog.Debug("waiting for container process to connect on init sock", "id", c.State.ID)
	conn, err := listener.Accept()
	if err != nil {
		slog.Error("failed to accept on init sock", "id", c.State.ID, "err", err)
		return c.cleanupOnErr(fmt.Errorf("accept on init sock: %w", err))
	}
	defer errs.WrapDeferedClose(conn, &err)

	b := make([]byte, 128)
	n, err := conn.Read(b)
	if err != nil {
		slog.Error("failed to read from init sock connection", "id", c.State.ID, "err", err)
		return c.cleanupOnErr(fmt.Errorf("read bytes from init sock connection: %w", err))
	}

	// 10. receive ready
	msg := string(b[:n])
	if msg != "ready" {
		slog.Error("unexpected message on init sock", "id", c.State.ID, "want", "ready", "got", msg)
		return c.cleanupOnErr(fmt.Errorf("expecting 'ready' but received '%s'", msg))
	}

	c.State.Status = specs.StateCreated
	slog.Info("container created", "id", c.State.ID, "pid", c.State.Pid, "status", c.State.Status)

	// 11. exit
	return nil
}

func (c *Container) applyNamespaces(cmd *exec.Cmd) error {
	for _, ns := range c.Spec.Linux.Namespaces {
		if ns.Path != "" {
			// TODO: research how to handle this. check setns(2)
			// can't go through clone flags, either open ns fd and setns
			// before exec, check how runc handles it
			continue
		}

		switch ns.Type {
		case specs.CgroupNamespace:
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWCGROUP
		case specs.IPCNamespace:
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWIPC
		case specs.MountNamespace:
			// NOTE: the mount-ns flag is newns, it's the oldest namespace before
			// ns conventions
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNS
		case specs.NetworkNamespace:
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
		case specs.PIDNamespace:
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWPID
		case specs.TimeNamespace:
			// TODO: Check spec
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWTIME
		case specs.UTSNamespace:
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWUTS
		case specs.UserNamespace:
			// WARN: would need to implement rootless container capabilities
			// skipping for talk
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWUSER
		default:
			return fmt.Errorf("unexpected specs.LinuxNamespaceType: %#v", ns.Type)
		}
	}
	return nil
}
