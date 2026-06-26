package container

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"syscall"

	"github.com/michael-duren/boxes/internal/hooks"
)

func (c *Container) Reexec() error {
	slog.Info("reexec started in container process", "id", c.State.ID, "pid", os.Getpid())

	var setHostnameErr error
	if c.Spec.Hostname != "" {
		setHostnameErr = syscall.Sethostname([]byte(c.Spec.Hostname))
	} else {
		setHostnameErr = syscall.Sethostname([]byte(c.State.ID))
	}
	if setHostnameErr != nil {
		return fmt.Errorf("set hostname: %w", setHostnameErr)
	}

	// send ready
	slog.Debug("dialing init sock", "id", c.State.ID)
	initConn, err := net.Dial(
		"unix",
		c.initSockPath(),
	)
	if err != nil {
		slog.Error("failed to dial init sock", "id", c.State.ID, "err", err)
		return fmt.Errorf("dial init sock: %w", err)
	}

	// signal to runtime container is ready
	slog.Debug("sending 'ready' to runtime", "id", c.State.ID)
	if _, err := initConn.Write([]byte("ready")); err != nil {
		slog.Error("failed to write 'ready' to init sock", "id", c.State.ID, "err", err)
		return fmt.Errorf("write 'ready' msg to init sock: %w", err)
	}

	_ = initConn.Close()

	// NOTE: after sending ready we are saying it is created
	err = c.execHooks(hooks.CreateContainer)

	if err != nil {
		return err
	}

	// open a unix socket this will continue to listen until the user or system
	// executes start
	slog.Debug("listening on container sock, waiting for start", "id", c.State.ID)
	listener, err := net.Listen(
		"unix",
		c.containerSockPath(),
	)

	if err != nil {
		slog.Error("failed to listen on container sock", "id", c.State.ID, "err", err)
		return fmt.Errorf("listen on container sock: %w", err)
	}

	containerConn, err := listener.Accept()
	if err != nil {
		slog.Error("failed to accept on container sock", "id", c.State.ID, "err", err)
		return fmt.Errorf("accept on container sock: %w", err)
	}

	b := make([]byte, 128)
	n, err := containerConn.Read(b)
	if err != nil {
		slog.Error("failed to read from container sock", "id", c.State.ID, "err", err)
		return fmt.Errorf("read bytes from container sock: %w", err)
	}

	// if we received msg from runtime to start continue
	msg := string(b[:n])
	if msg != "start" {
		slog.Error("unexpected message on container sock", "id", c.State.ID, "want", "start", "got", msg)
		return fmt.Errorf("expecting 'start' but received '%s'", msg)
	}
	slog.Debug("received 'start' from runtime", "id", c.State.ID)

	_ = containerConn.Close()
	_ = listener.Close()

	// NOTE: container hooks now in container namespace
	err = c.execHooks(hooks.StartContainer)

	if err != nil {
		return err
	}

	// cmd may or may not be an absolute path to bin, so we need to get an abs path to it
	bin, err := exec.LookPath(c.Spec.Process.Args[0])
	if err != nil {
		slog.Error("failed to find path of user process binary", "id", c.State.ID, "bin", c.Spec.Process.Args[0], "err", err)
		return fmt.Errorf("find path of user process binary: %w", err)
	}

	// NOTE: any cmd args
	args := c.Spec.Process.Args
	// WARN: this is the same as the host env
	// TODO: fix to
	env := os.Environ()

	// NOTE: calling a system call execve int execve(const char *pathname, char *const argv[], char *const envp[]);
	// execve syscall throws away processes memory img (stack, heap) and loads new one in place, essentially overwriting
	// THIS process with the specified cmd but keeping things like PID the same. Read `man execve` for more info.
	slog.Info("executing user process", "id", c.State.ID, "bin", bin, "args", args)
	if err := syscall.Exec(bin, args, env); err != nil {
		slog.Error("execve failed", "id", c.State.ID, "bin", bin, "err", err)
		return fmt.Errorf("execve (%s, %s, %v): %w", bin, args, env, err)
	}

	// NOTE: the process should have overwritten this current process
	// if execve was successful
	panic("the call to execve was not successful and an error was not returned.")
}
