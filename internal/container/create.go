package container

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"

	"github.com/michael-duren/boxes/internal/errs"
	"github.com/michael-duren/boxes/internal/hooks"
)

// Init creates the container process. consoleSocket is the path to a listening
// AF_UNIX socket supplied via the runtime's --console-socket flag; it is only
// used when spec.Process.Terminal is true, in which case the PTY master is sent
// to whoever is listening there so they can drive the container's terminal
// after create returns. Pass "" when no terminal is requested.
func (c *Container) Init(consoleSocket string) (err error) {
	slog.Info("initializing container", "id", c.State.ID, "bundle", c.State.Bundle)

	if c.Spec.Linux == nil {
		slog.Warn("only linux is supported, was passed an empty linux spec")
		return fmt.Errorf("container runtime supports linux only")
	}

	err = c.execHooks(hooks.CreateRuntime)
	if err != nil {
		slog.Warn("error trying to execute create runtime hoook", "error", err)
		return err
	}

	listener, err := c.listenUnix()
	if err != nil {
		return c.cleanupOnErr(err)
	}
	slog.Debug("init sock listener ready", "id", c.State.ID, "addr", listener.Addr())

	defer errs.WrapDeferedClose(listener, &err)

	slog.Debug("reexecing container process", "id", c.State.ID)
	cmd := exec.Command("/proc/self/exe", "reexec", c.State.ID)
	err = c.applyNamespaces(cmd)
	if err != nil {
		slog.Error("failed to apply namespaces", "id", c.State.ID, "err", err)
		return err
	}

	// Wire up the container process's stdio. With a terminal, that means a PTY
	// whose slave the child inherits as its controlling terminal and whose
	// master we hand off over the console socket; otherwise we just pass our own
	// std streams through.
	var consoleMaster, consoleSlave *os.File
	if c.Spec.Process != nil && c.Spec.Process.Terminal {
		if consoleSocket == "" {
			return c.cleanupOnErr(errors.New("process.terminal is true but no --console-socket was provided"))
		}

		consoleMaster, consoleSlave, err = openPTY()
		if err != nil {
			return c.cleanupOnErr(fmt.Errorf("open pty: %w", err))
		}
		// We always drop our copy of the master: on error we abandon it, on
		// success we close it once it has been duplicated across the socket.
		defer errs.WrapDeferedClose(consoleMaster, &err)

		// The child's std streams are the slave end of the PTY.
		cmd.Stdin = consoleSlave
		cmd.Stdout = consoleSlave
		cmd.Stderr = consoleSlave

		// Setsid puts the child in a new session (so it has no controlling
		// terminal yet); Setctty then makes Ctty its controlling terminal. Ctty
		// is the child's own fd number, which is 0 because the slave is stdin.
		cmd.SysProcAttr.Setsid = true
		cmd.SysProcAttr.Setctty = true
		cmd.SysProcAttr.Ctty = 0
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// proc filesystem is pseudo-fs, /self/exe is a link
	// to the cntr runtime itself
	if err = cmd.Start(); err != nil {
		slog.Error("failed to start reexec container process", "id", c.State.ID, "err", err)
		return c.cleanupOnErr(fmt.Errorf("reexec container process: %w", err))
	}

	c.State.Pid = cmd.Process.Pid
	slog.Debug("reexec container process started", "id", c.State.ID, "pid", c.State.Pid)

	if consoleSlave != nil {
		// The child now owns the slave as fds 0/1/2, so release our copy.
		if closeErr := consoleSlave.Close(); closeErr != nil {
			slog.Warn("failed to close pty slave after start", "id", c.State.ID, "err", closeErr)
		}

		// Hand the master to whoever is listening on the console socket. Once
		// they receive it (SCM_RIGHTS duplicates the fd into their process) they
		// can read/write the container's terminal independently of this process.
		slog.Debug("sending pty master over console socket", "id", c.State.ID, "socket", consoleSocket)
		if err = c.sendConsoleMaster(consoleSocket, consoleMaster); err != nil {
			slog.Error("failed to send pty master over console socket", "id", c.State.ID, "err", err)
			return c.cleanupOnErr(fmt.Errorf("send console master: %w", err))
		}
	}

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
	slog.Debug("applying namespaces", "id", c.State.ID, "count", len(c.Spec.Linux.Namespaces))
	if err := c.validateNamespacePrivellages(); err != nil {
		return err
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	for _, ns := range c.Spec.Linux.Namespaces {
		if ns.Path != "" {
			// TODO: research how to handle this. check setns(2)
			// can't go through clone flags, either open ns fd and setns
			// before exec, check how runc handles it
			slog.Debug("skipping namespace with explicit path", "id", c.State.ID, "type", ns.Type, "path", ns.Path)
			continue
		}

		slog.Debug("adding namespace clone flag", "id", c.State.ID, "type", ns.Type)
		switch ns.Type {
		case specs.CgroupNamespace:
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWCGROUP
		case specs.IPCNamespace:
			// INFO: isolates System V IPC objects and POSIX message queues. See
			// ipc_namespaces(7).
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
			slog.Error("unexpected namespace type", "id", c.State.ID, "type", ns.Type)
			return fmt.Errorf("unexpected specs.LinuxNamespaceType: %#v", ns.Type)
		}
	}

	slog.Debug("namespaces applied", "id", c.State.ID, "cloneflags", cmd.SysProcAttr.Cloneflags)
	return nil
}

func (c *Container) validateNamespacePrivellages() error {
	for _, ns := range c.Spec.Linux.Namespaces {
		if ns.Type == specs.UserNamespace {
			return nil
		}
	}

	if os.Getuid() == 0 {
		return nil
	}

	return errors.New("must be a user who has CAP_SYS_ADMIN (sudo) if not creating user ns")
}

// openPTY allocates a new pseudo-terminal pair and returns the master and slave
// ends. It talks to the kernel PTY multiplexer directly rather than pulling in a
// dependency: open /dev/ptmx (which mints a fresh master), unlock the matching
// slave, ask which /dev/pts/N it is, then open that slave. O_NOCTTY keeps this
// process from accidentally acquiring the new terminal as its own controlling
// tty — that job belongs to the container process.
func openPTY() (master, slave *os.File, err error) {
	master, err = os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open /dev/ptmx: %w", err)
	}
	defer func() {
		// If anything after this fails, don't leak the master fd.
		if err != nil {
			_ = master.Close()
		}
	}()

	// unlockpt(3): clear the slave's lock bit so it can be opened.
	if err = unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		return nil, nil, fmt.Errorf("unlock pty slave (TIOCSPTLCK): %w", err)
	}

	// ptsname(3): ask the kernel which /dev/pts entry pairs with this master.
	n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		return nil, nil, fmt.Errorf("get pty slave number (TIOCGPTN): %w", err)
	}

	slavePath := fmt.Sprintf("/dev/pts/%d", n)
	slave, err = os.OpenFile(slavePath, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open pty slave %s: %w", slavePath, err)
	}

	return master, slave, nil
}

// sendConsoleMaster dials the listening console socket and passes the PTY master
// fd across it using SCM_RIGHTS ancillary data, which duplicates the descriptor
// into the receiving process. A single payload byte accompanies it because a
// message carrying ancillary data cannot be empty; runc-style tooling reads the
// terminal name from it.
func (c *Container) sendConsoleMaster(socketPath string, master *os.File) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return fmt.Errorf("dial console socket: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			slog.Warn("failed to close console socket connection", "id", c.State.ID, "err", closeErr)
		}
	}()

	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("console socket %q is not a unix connection", socketPath)
	}

	rights := unix.UnixRights(int(master.Fd()))
	if _, _, err := uc.WriteMsgUnix([]byte(master.Name()), rights, nil); err != nil {
		return fmt.Errorf("write master fd to console socket: %w", err)
	}

	return nil
}
