package container_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"

	"github.com/michael-duren/boxes/internal/assert"
	"github.com/michael-duren/boxes/internal/container"
	"github.com/michael-duren/boxes/internal/filesystem"
)

func TestNew(t *testing.T) {
	t.Run("creates container when it does not exist", func(t *testing.T) {
		dirs := createDirs(t)
		spec := testSpec()

		got, err := container.New(&container.NewContainerOpts{
			ID:     "mycontainer",
			Bundle: "alpinefs",
			Spec:   spec,
			Dirs:   dirs,
		})
		assert.NoError(t, err)

		want := &container.Container{
			State: &specs.State{
				Version: specs.Version,
				ID:      "mycontainer",
				Bundle:  "alpinefs",
				Status:  specs.StateCreating,
			},
			Spec: spec,
			Dirs: dirs,
		}
		assert.DeepEqual(t, got, want)
	})

	t.Run("errors when container already exists", func(t *testing.T) {
		dirs := createDirs(t)
		seedState(t, dirs, "mycontainer")

		_, err := container.New(&container.NewContainerOpts{
			ID:     "mycontainer",
			Bundle: "alpinefs",
			Spec:   testSpec(),
			Dirs:   dirs,
		})
		assert.Error(t, err)
	})

	t.Run("errors when spec is nil", func(t *testing.T) {
		dirs := createDirs(t)

		_, err := container.New(&container.NewContainerOpts{
			ID:     "mycontainer",
			Bundle: "alpinefs",
			Spec:   nil,
			Dirs:   dirs,
		})
		assert.Error(t, err)
	})
}

func TestLoad(t *testing.T) {
	t.Run("returns the saved container", func(t *testing.T) {
		dirs := createDirs(t)
		spec := testSpec()
		bundle := writeBundle(t, spec)
		created := saveContainer(t, dirs, "mycontainer", bundle, spec)

		got, err := container.Load("mycontainer", dirs)
		assert.NoError(t, err)

		want := &container.Container{State: created.State, Spec: spec, Dirs: dirs}
		assert.DeepEqual(t, got, want)
	})

	t.Run("errors when state file is missing", func(t *testing.T) {
		dirs := createDirs(t)

		_, err := container.Load("does-not-exist", dirs)
		assert.Error(t, err)
	})

	t.Run("errors when bundle config is missing", func(t *testing.T) {
		dirs := createDirs(t)
		// Bundle dir exists but has no config.json, so the state read succeeds
		// and the config read fails.
		saveContainer(t, dirs, "mycontainer", t.TempDir(), testSpec())

		_, err := container.Load("mycontainer", dirs)
		assert.Error(t, err)
	})
}

func TestSave(t *testing.T) {
	t.Run("persists the initial state", func(t *testing.T) {
		dirs := createDirs(t)
		c := newContainer(t, dirs, "mycontainer", "alpinefs", testSpec())

		assert.NoError(t, c.Save())

		got := readState(t, dirs, "mycontainer")
		assert.DeepEqual(t, got, c.State)
	})

	t.Run("persists an updated status and pid", func(t *testing.T) {
		dirs := createDirs(t)
		c := newContainer(t, dirs, "mycontainer", "alpinefs", testSpec())

		c.State.Status = specs.StateRunning
		c.State.Pid = 4321

		assert.NoError(t, c.Save())

		got := readState(t, dirs, "mycontainer")
		assert.DeepEqual(t, got, c.State)
	})

	t.Run("overwrites a previously saved state", func(t *testing.T) {
		dirs := createDirs(t)
		c := newContainer(t, dirs, "mycontainer", "alpinefs", testSpec())

		// First save writes the initial Creating state...
		assert.NoError(t, c.Save())

		// ...then a second save must replace it with the new status.
		c.State.Status = specs.StateStopped
		assert.NoError(t, c.Save())

		got := readState(t, dirs, "mycontainer")
		assert.DeepEqual(t, got, c.State)
	})
}

func TestStart(t *testing.T) {
	t.Run("errors when spec has no process", func(t *testing.T) {
		dirs := createDirs(t)
		c := newContainer(t, dirs, "mycontainer", "alpinefs", testSpec())
		c.Spec.Process = nil

		assert.Error(t, c.Start())
	})

	t.Run("errors when not in created state", func(t *testing.T) {
		dirs := createDirs(t)
		c := newContainer(t, dirs, "mycontainer", "alpinefs", testSpec())
		// A freshly built container is in StateCreating, not StateCreated, so
		// Start must reject it before dialing the container socket.

		assert.Error(t, c.Start())
	})

	// TODO(tdd): the happy path dials the container's unix socket expecting a
	// reexec'd process to be listening. Cover it with an acceptance test, or once
	// socket dialing sits behind an injectable seam.
}

func TestKill(t *testing.T) {
	t.Run("errors when container cannot be killed in current state", func(t *testing.T) {
		dirs := createDirs(t)
		c := newContainer(t, dirs, "mycontainer", "alpinefs", testSpec())
		// StateCreating is neither Running nor Created.

		assert.Error(t, c.Kill(unix.SIGTERM))
	})

	t.Run("errors when pid is invalid", func(t *testing.T) {
		dirs := createDirs(t)
		c := newContainer(t, dirs, "mycontainer", "alpinefs", testSpec())
		c.State.Status = specs.StateCreated // passes the state guard...
		c.State.Pid = 0                     // ...but the pid is invalid

		assert.Error(t, c.Kill(unix.SIGTERM))
	})

	// TODO(tdd): killing a live process (the happy path) calls unix.Kill and runs
	// poststop hooks. Exercise it in an acceptance test against a real process.
}

func TestDelete(t *testing.T) {
	t.Run("errors when not stopped and not forced", func(t *testing.T) {
		dirs := createDirs(t)
		c := newContainer(t, dirs, "mycontainer", "alpinefs", testSpec())
		// StateCreating is not deletable without --force.

		assert.Error(t, c.Delete(false))
	})

	// WARNING: do not unit-test the force / Stopped path here. Delete calls
	// os.FindProcess(pid) then Signal(SIGKILL); with the default pid 0 that
	// targets the whole process group and would kill the test runner. Cover
	// deletion in an acceptance test with a real, owned process.
}

func TestInit(t *testing.T) {
	t.Skip("Init reexecs /proc/self/exe and coordinates over unix sockets; cover via acceptance tests")
}

func TestReexec(t *testing.T) {
	t.Skip("Reexec dials unix sockets and ends in syscall.Exec; cover via acceptance tests")
}

func createDirs(t *testing.T) filesystem.Dirs {
	t.Helper()
	dir := t.TempDir()

	return filesystem.Dirs{
		Config:  filepath.Join(dir, ".config"),
		Data:    filepath.Join(dir, ".local/share"),
		State:   filepath.Join(dir, ".local/state"),
		Cache:   filepath.Join(dir, ".cache"),
		Runtime: filepath.Join(dir, "run"),
	}
}

// writeBundle creates a bundle directory containing config.json for spec and
// returns its path, mirroring what Load expects to find on disk.
func writeBundle(t *testing.T, spec *specs.Spec) string {
	t.Helper()
	bundle := t.TempDir()

	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "config.json"), b, 0o644); err != nil {
		t.Fatalf("write config.json: %v", err)
	}
	return bundle
}

// seedState makes id look like an already-created container by creating its
// state directory, without writing a valid state.json.
func seedState(t *testing.T, dirs filesystem.Dirs, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dirs.State, id), 0o755); err != nil {
		t.Fatalf("seed state dir: %v", err)
	}
}

// newContainer constructs a container without persisting it.
func newContainer(
	t *testing.T,
	dirs filesystem.Dirs,
	id,
	bundle string,
	spec *specs.Spec,
) *container.Container {
	t.Helper()
	c, err := container.New(&container.NewContainerOpts{
		ID:     id,
		Bundle: bundle,
		Spec:   spec,
		Dirs:   dirs,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return c
}

// saveContainer constructs and persists a container, returning the in-memory
// value so tests can compare what Load reads back against what was written.
func saveContainer(
	t *testing.T,
	dirs filesystem.Dirs,
	id,
	bundle string,
	spec *specs.Spec,
) *container.Container {
	t.Helper()
	c := newContainer(t, dirs, id, bundle, spec)
	if err := c.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}
	return c
}

// readState reads and decodes the persisted state.json for id.
func readState(t *testing.T, dirs filesystem.Dirs, id string) *specs.State {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dirs.State, id, "state.json"))
	if err != nil {
		t.Fatalf("read saved state: %v", err)
	}
	var s specs.State
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("unmarshal saved state: %v", err)
	}
	return &s
}
