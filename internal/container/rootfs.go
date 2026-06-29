package container

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

const oldRootDir = ".boxes_oldroot"

// setupRootfs turns the container process's view of the filesystem into the
// bundle's rootfs: it makes mount propagation private, mounts everything listed
// in spec.Mounts, and pivot_root(2)s into the bundle root. After it returns the
// process sees the bundle rootfs as / and can exec the bundle's binary (e.g.
// /runtimetest). It must run inside the mount namespace (CLONE_NEWNS).
func (c *Container) setupRootfs() error {
	if c.Spec.Root == nil || c.Spec.Root.Path == "" {
		return fmt.Errorf("spec has no root.path")
	}

	root := c.Spec.Root.Path
	if !filepath.IsAbs(root) {
		root = filepath.Join(c.State.Bundle, root)
	}
	slog.Debug("setting up rootfs", "id", c.State.ID, "root", root)

	// Make the whole mount tree private and recursive so anything we do here
	// does not propagate back to the host mount namespace. See
	// mount_namespaces(7) (shared subtrees).
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make / private: %w", err)
	}

	// pivot_root requires the new root to be a mount point, so bind-mount the
	// rootfs onto itself.
	if err := unix.Mount(root, root, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mount rootfs onto itself: %w", err)
	}

	// Set up the configured mounts (proc, sysfs, tmpfs, devpts, ...) under the
	// new root before we pivot into it.
	for _, m := range c.Spec.Mounts {
		if err := mountInto(root, m); err != nil {
			return fmt.Errorf("mount %q: %w", m.Destination, err)
		}
	}

	if err := pivotRoot(root); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}

	if c.Spec.Root.Readonly {
		if err := unix.Mount("", "/", "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("remount / read-only: %w", err)
		}
		slog.Debug("remounted root read-only", "id", c.State.ID)
	}

	slog.Debug("rootfs ready", "id", c.State.ID)
	return nil
}

// mountInto mounts a single spec.Mount under root, creating its destination
// first. Bind mounts that request read-only are remounted read-only afterwards,
// since the read-only flag is ignored on the initial bind (see mount(2)).
func mountInto(root string, m specs.Mount) error {
	dest := filepath.Join(root, m.Destination)
	flags, data := parseMountOptions(m.Options)

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create mount destination %q: %w", dest, err)
	}

	if err := unix.Mount(m.Source, dest, m.Type, flags, data); err != nil {
		return fmt.Errorf("mount(%q, %q, %q): %w", m.Source, dest, m.Type, err)
	}

	if flags&unix.MS_BIND != 0 && flags&unix.MS_RDONLY != 0 {
		remount := flags | unix.MS_REMOUNT
		if err := unix.Mount(m.Source, dest, m.Type, remount, data); err != nil {
			return fmt.Errorf("remount %q read-only: %w", dest, err)
		}
	}

	return nil
}

// pivotRoot switches the process's root filesystem to root using pivot_root(2),
// then unmounts and removes the temporary old-root directory.
func pivotRoot(root string) error {
	oldRoot := filepath.Join(root, oldRootDir)
	if err := os.MkdirAll(oldRoot, 0o700); err != nil {
		return fmt.Errorf("create old root dir: %w", err)
	}

	if err := unix.PivotRoot(root, oldRoot); err != nil {
		return fmt.Errorf("pivot_root(%q, %q): %w", root, oldRoot, err)
	}

	// We are now in the new root; cd to it and detach the old root.
	if err := unix.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to new root: %w", err)
	}

	pivoted := "/" + oldRootDir
	if err := unix.Unmount(pivoted, unix.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root: %w", err)
	}
	if err := os.RemoveAll(pivoted); err != nil {
		return fmt.Errorf("remove old root dir: %w", err)
	}

	return nil
}

// mountFlags maps OCI mount option strings to their mount(2) flag bits.
// Anything not present here is treated as filesystem-specific data (e.g.
// "mode=755", "size=64m") and passed through as the data argument.
var mountFlags = map[string]uintptr{
	"bind":        unix.MS_BIND,
	"rbind":       unix.MS_BIND | unix.MS_REC,
	"ro":          unix.MS_RDONLY,
	"nosuid":      unix.MS_NOSUID,
	"nodev":       unix.MS_NODEV,
	"noexec":      unix.MS_NOEXEC,
	"sync":        unix.MS_SYNCHRONOUS,
	"dirsync":     unix.MS_DIRSYNC,
	"remount":     unix.MS_REMOUNT,
	"mand":        unix.MS_MANDLOCK,
	"noatime":     unix.MS_NOATIME,
	"nodiratime":  unix.MS_NODIRATIME,
	"relatime":    unix.MS_RELATIME,
	"strictatime": unix.MS_STRICTATIME,
	"private":     unix.MS_PRIVATE,
	"rprivate":    unix.MS_PRIVATE | unix.MS_REC,
	"slave":       unix.MS_SLAVE,
	"rslave":      unix.MS_SLAVE | unix.MS_REC,
	"shared":      unix.MS_SHARED,
	"rshared":     unix.MS_SHARED | unix.MS_REC,
	"unbindable":  unix.MS_UNBINDABLE,
}

// clearingFlags are options that mean "turn this off"; they have no flag bit
// and are not data, so they are simply ignored.
var clearingFlags = map[string]struct{}{
	"rw":       {},
	"dev":      {},
	"exec":     {},
	"suid":     {},
	"async":    {},
	"atime":    {},
	"diratime": {},
}

// parseMountOptions splits an OCI mount's options into mount(2) flag bits and
// the remaining filesystem-specific data string.
func parseMountOptions(options []string) (uintptr, string) {
	var flags uintptr
	var data []string

	for _, opt := range options {
		if bit, ok := mountFlags[opt]; ok {
			flags |= bit
			continue
		}
		if _, ok := clearingFlags[opt]; ok {
			continue
		}
		data = append(data, opt)
	}

	return flags, strings.Join(data, ",")
}
