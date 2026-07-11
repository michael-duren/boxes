package operations

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/michael-duren/boxes/internal/container"
)

type RunOpts struct {
	// ID is the container id.
	ID string
	// Bundle is the path to the OCI bundle directory.
	Bundle string
	// PidFile, when set, is where the container process PID is written. Same
	// convention as CreateOpts.PidFile.
	PidFile string
	// ConsoleSocket is the listening AF_UNIX socket the pty master is sent to
	// when the config requests a terminal. See CreateOpts.ConsoleSocket.
	//
	// TODO: decide the foreground-terminal story. runc requires a console
	// socket even for `run` and leaves relaying the master <-> the caller's tty
	// to a helper (recvtty). If we want `box run` to "just work" in a terminal
	// without an external helper, this operation would instead open the pty
	// itself, put the caller's tty into raw mode, and io.Copy between them.
	ConsoleSocket string
	// Detach controls whether we return once the container is running (true) or
	// block until the container process exits (false).
	Detach bool
}

// forwardedSignals is the curated set of signals a foreground run relays to the
// container process while it waits. We deliberately do NOT use a bare
// signal.Notify (which would catch everything, including SIGURG that the Go
// runtime relies on for async preemption, and SIGCHLD). SIGWINCH is included so
// terminal resizes propagate once the pty story is finished.
//
// TODO: runc forwards the full signal set (minus SIGCHLD/SIGURG). Widen this if
// we want that fidelity.
var forwardedSignals = []os.Signal{
	syscall.SIGINT,
	syscall.SIGTERM,
	syscall.SIGQUIT,
	syscall.SIGHUP,
	syscall.SIGWINCH,
}

// Run is `create` + `start` in one call. With Detach it returns as soon as the
// container is running; otherwise it blocks in the foreground until the
// container process exits, forwarding signals to it, then tears the container
// down (mirroring `runc run`).
func Run(opts *RunOpts) error {
	slog.Info("run operation", "id", opts.ID, "bundle", opts.Bundle, "detach", opts.Detach)

	cntr, err := Create(&CreateOpts{
		ID:            opts.ID,
		Bundle:        opts.Bundle,
		PidFile:       opts.PidFile,
		ConsoleSocket: opts.ConsoleSocket,
	})
	if err != nil {
		slog.Error("failed to create container during run", "id", opts.ID, "err", err)
		return fmt.Errorf("run failed to create container: %w", err)
	}

	if err := cntr.Start(); err != nil {
		slog.Error("failed to start container during run", "id", opts.ID, "err", err)
		return fmt.Errorf("run failed to start container: %w", err)
	}

	if err := cntr.Save(); err != nil {
		// The container is running; a failure to persist the Running status
		// shouldn't abort the run, so log and continue.
		slog.Warn("failed to save running state during run", "id", opts.ID, "err", err)
	}

	if opts.Detach {
		slog.Info("run operation complete (detached)", "id", opts.ID, "pid", cntr.State.Pid)
		return nil
	}

	// Foreground: block until the container process exits, forwarding signals to
	// it in the meantime.
	status, err := waitForExit(cntr)
	if err != nil {
		return fmt.Errorf("run wait for container exit: %w", err)
	}

	// runc removes the container once a foreground run exits. Force-delete so we
	// clean up state/runtime files even though the process is already gone.
	if delErr := cntr.Delete(true); delErr != nil {
		slog.Error("failed to delete container after run exit", "id", opts.ID, "err", delErr)
		return fmt.Errorf("run cleanup after exit: %w", delErr)
	}

	slog.Info("run operation complete", "id", opts.ID, "exitCode", status)

	// TODO: propagate the exact exit code. `runc run` exits with the container
	// process's status; doing that faithfully means main() calling os.Exit(code)
	// rather than always exiting 1 on error. For now a non-zero status surfaces
	// as a generic error.
	if status != 0 {
		return fmt.Errorf("container %q exited with status %d", opts.ID, status)
	}
	return nil
}

// waitForExit blocks until the container process reaps and returns its exit
// code (128+signum if it was killed by a signal, matching shell convention).
//
// The container process is a direct child of this process: Init started it with
// exec.Command in the same process that runs create+start. Init calls
// Process.Release(), which frees Go's os.Process bookkeeping but does NOT detach
// the OS parent/child link, so we can still reap it at the syscall level with
// wait4 on the stored pid.
func waitForExit(cntr *container.Container) (int, error) {
	pid := cntr.State.Pid
	if pid <= 0 {
		return 0, fmt.Errorf("invalid container pid %d", pid)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, forwardedSignals...)
	defer signal.Stop(sigCh)

	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case s := <-sigCh:
				sig, ok := s.(syscall.Signal)
				if !ok {
					continue
				}
				slog.Debug("forwarding signal to container", "id", cntr.State.ID, "signal", int(sig))
				if err := unix.Kill(pid, sig); err != nil && err != unix.ESRCH {
					slog.Warn("failed to forward signal to container", "id", cntr.State.ID, "signal", int(sig), "err", err)
				}
			case <-done:
				return
			}
		}
	}()

	slog.Debug("waiting for container process to exit", "id", cntr.State.ID, "pid", pid)
	var ws unix.WaitStatus
	for {
		_, err := unix.Wait4(pid, &ws, 0, nil)
		if err == unix.EINTR {
			// Interrupted by a forwarded signal; resume waiting.
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("wait4 on pid %d: %w", pid, err)
		}
		break
	}

	switch {
	case ws.Exited():
		return ws.ExitStatus(), nil
	case ws.Signaled():
		return 128 + int(ws.Signal()), nil
	default:
		// Stopped/continued shouldn't happen with options=0, but don't hang.
		return 0, nil
	}
}
