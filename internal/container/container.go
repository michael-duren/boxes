package container

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"

	"github.com/michael-duren/boxes/internal/hooks"

	"github.com/michael-duren/boxes/internal/errs"
	"github.com/michael-duren/boxes/internal/filesystem"
)

const (
	initSockFilename      = "init.sock"
	containerSockFilename = "container.sock"
	stateFilename         = "state.json"
	configFilename        = "config.json"
)

// Container is the combination of
// configurations and state for a container
// OCI actions like create, start, stop, delete, kill
// are executed against these configurations
type Container struct {
	State *specs.State
	Spec  *specs.Spec
	Dirs  filesystem.Dirs
}

type NewContainerOpts struct {
	ID     string
	Bundle string
	Spec   *specs.Spec
	Dirs   filesystem.Dirs
}

// Creates a new container with
// the opts, fails if the container already exists or
func New(opts *NewContainerOpts) (*Container, error) {
	slog.Debug("creating new container", "id", opts.ID, "bundle", opts.Bundle)

	if _, err := os.Stat(filepath.Join(opts.Dirs.State, opts.ID)); err == nil {
		slog.Warn("container already exists", "id", opts.ID)
		return nil, fmt.Errorf("container '%s' exists", opts.ID)
	}

	if opts.Spec == nil {
		slog.Error("nil pointer as spec.Specs")
		return nil, errors.New("container.New a nil pointer was passed as spec.Specs")
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
		Dirs:  opts.Dirs,
	}

	slog.Debug("container initialized", "id", opts.ID, "status", state.Status)

	return &c, nil
}

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
			// unix time share ns - part of hostname change
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWUTS
		case specs.UserNamespace:
			// TODO: would need to implement rootless container capabilities
			// skipping for talk
			cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWUSER
		default:
			panic(fmt.Sprintf("unexpected specs.LinuxNamespaceType: %#v", ns.Type))
		}
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

func (c *Container) Save() error {
	slog.Debug("saving container state", "id", c.State.ID, "status", c.State.Status)

	if err := os.MkdirAll(
		c.stateDir(),
		0o755,
	); err != nil {
		slog.Error("failed to create container directory", "id", c.State.ID, "err", err)
		return fmt.Errorf("create container directory: %w", err)
	}

	state, err := json.Marshal(c.State)
	if err != nil {
		slog.Error("failed to serialize container state", "id", c.State.ID, "err", err)
		return fmt.Errorf("serialize container state: %w", err)
	}

	if err := os.WriteFile(
		c.statePath(),
		state,
		0o755,
	); err != nil {
		slog.Error("failed to write container state", "id", c.State.ID, "err", err)
		return fmt.Errorf("write container state: %w", err)
	}

	slog.Debug("container state saved", "id", c.State.ID)
	return nil
}

func Load(id string, dirs filesystem.Dirs) (*Container, error) {
	slog.Debug("loading container", "id", id)

	s, err := os.ReadFile(
		filepath.Join(dirs.State, id, stateFilename),
	)

	if err != nil {
		slog.Error("failed to read state file", "id", id, "err", err)
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var state *specs.State
	if err := json.Unmarshal(s, &state); err != nil {
		slog.Error("failed to unmarshal state", "id", id, "err", err)
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	config, err := os.ReadFile(
		filepath.Join(state.Bundle, configFilename),
	)

	if err != nil {
		slog.Error("failed to read config file", "id", id, "bundle", state.Bundle, "err", err)
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var spec *specs.Spec
	if err := json.Unmarshal(config, &spec); err != nil {
		slog.Error("failed to unmarshal config", "id", id, "err", err)
		return nil, fmt.Errorf("unmarhsal config: %w", err)
	}

	c := &Container{
		State: state,
		Spec:  spec,
		Dirs:  dirs,
	}

	slog.Debug("container loaded", "id", id, "status", state.Status, "pid", state.Pid)
	return c, nil
}

func (c *Container) canBeKilled() bool {
	return c.State.Status == specs.StateRunning ||
		c.State.Status == specs.StateCreated
}

// func (c *Container) processIsRunning() bool {
// 	if c.State.Pid <= 0 {
// 		return false
// 	}
// 	// check status
// 	err := unix.Kill(c.State.Pid, 0)
// 	return err == nil || errors.Is(err, unix.EPERM)
// }

func (c *Container) Kill(sig unix.Signal) error {
	slog.Info("killing container", "id", c.State.ID, "signal", int(sig), "status", c.State.Status, "pid", c.State.Pid)

	if !c.canBeKilled() {
		slog.Error("container cannot be killed in current state", "id", c.State.ID, "status", c.State.Status)
		return fmt.Errorf("container cannot be killed in current state (%s)", c.State.Status)
	}

	if c.State.Pid <= 0 {
		slog.Error("invalid process ID for container", "id", c.State.ID, "pid", c.State.Pid)
		return fmt.Errorf("process ID: %d for container ID %s is <= 0", c.State.Pid, c.State.ID)
	}

	slog.Debug("sending signal to container process", "id", c.State.ID, "signal", int(sig), "pid", c.State.Pid)
	if err := unix.Kill(c.State.Pid, sig); err != nil {
		slog.Error("failed to send signal to container process", "id", c.State.ID, "signal", int(sig), "pid", c.State.Pid, "err", err)
		return fmt.Errorf("send signal '%d' to process '%d': %w", sig, c.State.Pid, err)
	}

	// TODO: Create issue and accurately resolve whether or not a
	// process was killed, commenting out for now
	// if c.processIsRunning() {
	// 	slog.Debug("container was sent signal but wasn't stopped", "id", c.State.ID, "signal", int(sig))
	// 	return nil
	// }

	c.State.Status = specs.StateStopped
	slog.Debug("container status updated", "id", c.State.ID, "status", c.State.Status)

	slog.Debug("executing poststop hooks", "id", c.State.ID)
	if err := c.execHooks(hooks.Poststop); err != nil {
		slog.Error("failed to execute poststop hooks", "id", c.State.ID, "err", err)
		return err
	}

	slog.Info("container killed", "id", c.State.ID, "signal", int(sig))
	return nil
}

func (c *Container) canBeDeleted() bool {
	return c.State.Status == specs.StateStopped
}

func (c *Container) Delete(force bool) error {
	slog.Info("deleting container", "id", c.State.ID, "force", force, "status", c.State.Status)

	if !force && !c.canBeDeleted() {
		slog.Warn("container cannot be deleted in current state", "id", c.State.ID, "status", c.State.Status)
		return fmt.Errorf("container cannot be deleted in current state (%s) try using '--force' if this is intentional", c.State.Status)
	}

	process, err := os.FindProcess(c.State.Pid)
	if err != nil {
		slog.Error("failed to find container process to delete", "id", c.State.ID, "pid", c.State.Pid, "err", err)
		return fmt.Errorf("find container process to delete: %w", err)
	}
	// kill process
	if process != nil {
		slog.Debug("killing container process", "id", c.State.ID, "pid", c.State.Pid)
		if err := process.Signal(unix.SIGKILL); err != nil {
			// The process may already be gone; log and continue with cleanup
			// rather than aborting the delete.
			slog.Warn("kill container process during delete",
				"id", c.State.ID, "err", err)
		}
	}

	err = c.execHooks(hooks.Poststop)

	if err != nil {
		slog.Error("error during post stop hook execution", "id", c.State.ID, "error", err)
	}

	slog.Debug("removing container files", "id", c.State.ID)
	return c.removeContainerFiles()
}

func (c *Container) Reexec() error {
	slog.Info("reexec started in container process", "id", c.State.ID, "pid", os.Getpid())

	// TODO: configure cntr

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

// path helpers
func (c *Container) stateDir() string     { return filepath.Join(c.Dirs.State, c.State.ID) }
func (c *Container) statePath() string    { return filepath.Join(c.stateDir(), stateFilename) }
func (c *Container) runtimeDir() string   { return filepath.Join(c.Dirs.Runtime, c.State.ID) }
func (c *Container) initSockPath() string { return filepath.Join(c.runtimeDir(), initSockFilename) }
func (c *Container) containerSockPath() string {
	return filepath.Join(c.runtimeDir(), containerSockFilename)
}

// cleanupOnErr if an error occurs during the creation of a container
// this method kills any started container process and removes its state and
// runtime files. It returns the original error, wrapping it if cleanup fails.
func (c *Container) cleanupOnErr(err error) error {
	slog.Warn("cleaning up container after error", "id", c.State.ID, "err", err)

	// If we already reexec'd a container process, kill it so we don't leak an
	// orphan that outlives its state files. ESRCH means it's already gone.
	if c.State.Pid != 0 {
		if killErr := unix.Kill(c.State.Pid, unix.SIGKILL); killErr != nil && killErr != unix.ESRCH {
			slog.Error("failed to kill container process during cleanup", "id", c.State.ID, "pid", c.State.Pid, "err", killErr)
		}
	}

	rmErr := c.removeContainerFiles()
	if rmErr != nil {
		slog.Error("failed to cleanup container files after error", "id", c.State.ID, "cleanupErr", rmErr, "originalErr", err)
		return fmt.Errorf("cleanup on err unable to cleanup container state and runtime files: %w", err)
	}
	return err
}

// removeContainerFiles - removes state and runtime files of a container
func (c *Container) removeContainerFiles() error {
	slog.Debug("removing container state and runtime files", "id", c.State.ID)

	if err := os.RemoveAll(
		c.stateDir(),
	); err != nil {
		slog.Error("failed to delete container state directory", "id", c.State.ID, "err", err)
		return fmt.Errorf("delete container directory: %w", err)
	}

	dir := c.runtimeDir()
	if err := os.RemoveAll(dir); err != nil {
		slog.Error("failed to remove container runtime directory", "id", c.State.ID, "dir", dir, "err", err)
		return fmt.Errorf("remove container runtime dir: %w", err)
	}

	slog.Debug("container files removed", "id", c.State.ID)
	return nil
}

// execHooks - maps executes the correct hooks depending on the passed event
// execHooks calls the [hooks.ExecHooks] method
func (c *Container) execHooks(he hooks.HookEvent) error {
	if c.Spec.Hooks == nil {
		slog.Debug("no hooks defined, skipping", "id", c.State.ID, "event", he)
		return nil
	}

	var h []specs.Hook
	switch he {
	case hooks.Prestart:
		// SA1019: c.Spec.Hooks.Prestart is deprecated upstream, but still required
		// by OCI Runtime integration tests and used by other tools like Docker.
		h = c.Spec.Hooks.Prestart //nolint:staticcheck
	case hooks.CreateRuntime:
		h = c.Spec.Hooks.CreateRuntime
	case hooks.CreateContainer:
		h = c.Spec.Hooks.CreateContainer
	case hooks.StartContainer:
		h = c.Spec.Hooks.StartContainer
	case hooks.Poststart:
		h = c.Spec.Hooks.Poststart
	case hooks.Poststop:
		h = c.Spec.Hooks.Poststop
	default:
		slog.Error("unknown hook event", "id", c.State.ID, "event", he)
		return fmt.Errorf("execHook use of unknown hook event: %s", he)
	}

	if len(h) == 0 {
		slog.Debug("no hooks for event, skipping", "id", c.State.ID, "event", he)
		return nil
	}

	slog.Debug("executing hooks", "id", c.State.ID, "event", he, "count", len(h))
	if err := hooks.ExecHooks(h, c.State); err != nil {
		slog.Error("hook execution failed", "id", c.State.ID, "event", he, "err", err)
		return err
	}

	slog.Debug("hooks executed", "id", c.State.ID, "event", he)
	return nil
}

// listenUnix creates a listener on the init
// sock path location and returns the listener
func (c *Container) listenUnix() (net.Listener, error) {
	sockPath := c.initSockPath()
	slog.Debug("listening on init sock", "id", c.State.ID, "path", sockPath)
	slog.Debug("creating unix listener", "path", sockPath)

	if err := os.MkdirAll(filepath.Dir(sockPath), 0755); err != nil {
		slog.Error("failed to create socket directory", "path", sockPath, "err", err)
		return nil, fmt.Errorf("unable to create socket directory: %w", err)
	}

	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		slog.Error("failed to remove stale socket", "path", sockPath, "err", err)
		return nil, fmt.Errorf("remove stale sock: %w", err)
	}

	listener, err := net.Listen(
		"unix",
		sockPath,
	)

	if err != nil {
		slog.Error("failed to listen on unix socket", "path", sockPath, "err", err)
		return nil, fmt.Errorf("listen on init sock: %w", err)
	}

	slog.Debug("unix listener created", "path", sockPath)
	return listener, nil
}
