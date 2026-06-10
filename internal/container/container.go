package container

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/michael-duren/boxes/internal/errs"
	"github.com/michael-duren/boxes/internal/filesystem"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	initSockFilename      = "init.sock"
	containerSockfilename = "container.sock"
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
	dirs := filesystem.GetDirs()
	_, err := os.Stat(filepath.Join(dirs.State, containerID))
	return err == nil
}

func (c *Container) Init() (err error) {
	// 2. configure cntr
	// TODO: configure cntr

	// 3. create ipc socket
	listener, err := net.Listen(
		"unix",
		filepath.Join(filesystem.GetDirs().Runtime, c.State.ID, initSockFilename),
	)

	if err != nil {
		return fmt.Errorf("listen on init sock: %w", err)
	}

	defer errs.WrapDeferedClose(listener, &err)
	// 5. reexec
	// proc filesystem is pseudo-fs, /self/exe is a link
	// to the cntr runtime itself
	cmd := exec.Command("/proc/self/exe", "reexec", c.State.ID)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("reexec container process: %w", err)
	}

	c.State.Pid = cmd.Process.Pid

	// 6. release container process
	if err = cmd.Process.Release(); err != nil {
		return fmt.Errorf("releasing container process: %w", err)
	}

	// 4. listen
	conn, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accept on init sock: %w", err)
	}
	defer errs.WrapDeferedClose(conn, &err)

	b := make([]byte, 128)
	n, err := conn.Read(b)
	if err != nil {
		return fmt.Errorf("read bytes from init sock connection: %w", err)
	}

	// 10. receive ready
	msg := string(b[:n])
	if msg != "ready" {
		return fmt.Errorf("expecting 'ready' but received '%s'", msg)
	}

	c.State.Status = specs.StateCreated

	// 11. exit
	return nil
}

func (c *Container) Save() error {
	if err := os.MkdirAll(
		filepath.Join(filesystem.GetDirs().State, c.State.ID),
		0755,
	); err != nil {
		return fmt.Errorf("create container directory: %w", err)
	}

	state, err := json.Marshal(c.State)
	if err != nil {
		return fmt.Errorf("serialise container state: %w", err)
	}

	if err := os.WriteFile(
		filepath.Join(filesystem.GetDirs().State, c.State.ID, "state.json"),
		state,
		0755,
	); err != nil {
		return fmt.Errorf("write container state: %w", err)
	}

	return nil
}

func Load(id string) (*Container, error) {
	s, err := os.ReadFile(
		filepath.Join(filesystem.GetDirs().State, id, "state.json"),
	)

	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var state *specs.State
	if err := json.Unmarshal(s, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	config, err := os.ReadFile(
		filepath.Join(state.Bundle, "config.json"),
	)

	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var spec *specs.Spec
	if err := json.Unmarshal(config, &spec); err != nil {
		return nil, fmt.Errorf("unmarhsal config: %w", err)
	}

	c := &Container{
		State: state,
		Spec:  spec,
	}

	return c, nil
}

func (c *Container) Delete(force bool) error {
	if !force && !c.canBeDeleted() {
		return fmt.Errorf("container cannot be deleted in current state (%s) try using '--force' if this is intentional", c.State.Status)
	}

	if err := os.RemoveAll(
		filepath.Join(filesystem.GetDirs().State, c.State.ID),
	); err != nil {
		return fmt.Errorf("delete container directory: %w", err)
	}

	return nil
}

func (c *Container) Reexec() error {
	// TODO configure cntr

	// send ready
	dirs := filesystem.GetDirs()
	initConn, err := net.Dial(
		"unix",
		filepath.Join(dirs.Runtime, c.State.ID, initSockFilename),
	)
	if err != nil {
		return fmt.Errorf("dial init sock: %w", err)
	}

	// signal to runtime container is ready
	if _, err := initConn.Write([]byte("ready")); err != nil {
		return fmt.Errorf("write 'ready' msg to init sock: %w", err)
	}

	_ = initConn.Close()

	// open a unix socket
	listener, err := net.Listen(
		"unix",
		filepath.Join(dirs.Runtime, c.State.ID, containerSockfilename),
	)
	if err != nil {
		return fmt.Errorf("listen on container sock: %w", err)
	}

	containerConn, err := listener.Accept()
	if err != nil {
		return fmt.Errorf("accept on container sock: %w", err)
	}

	b := make([]byte, 128)
	n, err := containerConn.Read(b)
	if err != nil {
		return fmt.Errorf("read bytes from container sock: %w", err)
	}

	// if we received msg from runtime to start continue
	msg := string(b[:n])
	if msg != "start" {
		return fmt.Errorf("expecting 'start' but received '%s'", msg)
	}

	_ = containerConn.Close()
	_ = listener.Close()

	// cmd may or may not be an absolute path to bin, so we need to get an abs path to it
	bin, err := exec.LookPath(c.Spec.Process.Args[0])
	if err != nil {
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
	if err := syscall.Exec(bin, args, env); err != nil {
		return fmt.Errorf("execve (%s, %s, %v): %w", bin, args, env, err)
	}

	// NOTE: the process should have overwritten this current process
	// if execve was successful
	panic("the call to execve was not successful and an error was not returned.")
}

func (c *Container) canBeDeleted() bool {
	return c.State.Status == specs.StateStopped
}
