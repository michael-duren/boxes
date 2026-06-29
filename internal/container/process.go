package container

import (
	"fmt"
	"log/slog"
	"syscall"

	"golang.org/x/sys/unix"
)

// applyProcessAttrs applies the parts of spec.Process that must be set on this
// process just before it execs the user binary: resource limits, the
// no_new_privileges bit, and the user/group ids.
//
// Order matters. Resource limits and no_new_privileges are set while we still
// have privilege. The user (uid/gid) is dropped LAST, because lowering the uid
// gives up the capabilities needed for the earlier steps (and for the rootfs
// setup that ran before this).
//
// NOTE: capabilities, seccomp, and AppArmor/SELinux labels from the spec are
// not applied yet — see the TODO at the end. They are required for full
// runtimetest conformance but not for the lifecycle/state tests.
func (c *Container) applyProcessAttrs() error {
	p := c.Spec.Process
	if p == nil {
		return fmt.Errorf("spec has no process")
	}

	if err := c.applyRlimits(); err != nil {
		return err
	}

	if p.NoNewPrivileges {
		slog.Debug("setting no_new_privileges", "id", c.State.ID)
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return fmt.Errorf("set no_new_privileges: %w", err)
		}
	}

	if err := c.applyUser(); err != nil {
		return err
	}

	// TODO: apply p.Capabilities, p.Rlimits already done, seccomp
	// (c.Spec.Linux.Seccomp), and apparmor/selinux labels for full OCI
	// process conformance.
	return nil
}

// applyRlimits sets each rlimit named in the spec via setrlimit(2).
func (c *Container) applyRlimits() error {
	for _, rl := range c.Spec.Process.Rlimits {
		resource, ok := rlimitResources[rl.Type]
		if !ok {
			return fmt.Errorf("unknown rlimit type %q", rl.Type)
		}
		limit := unix.Rlimit{Cur: rl.Soft, Max: rl.Hard}
		slog.Debug("setting rlimit", "id", c.State.ID, "type", rl.Type, "soft", rl.Soft, "hard", rl.Hard)
		if err := unix.Setrlimit(resource, &limit); err != nil {
			return fmt.Errorf("set rlimit %s: %w", rl.Type, err)
		}
	}
	return nil
}

// applyUser sets the supplementary groups, gid, and uid from spec.Process.User.
// It uses the syscall package (not x/sys/unix) for Setgroups/Setgid/Setuid
// because the Go runtime makes those apply across every OS thread, which is what
// we need before exec.
func (c *Container) applyUser() error {
	u := c.Spec.Process.User

	if u.AdditionalGids != nil {
		gids := make([]int, len(u.AdditionalGids))
		for i, g := range u.AdditionalGids {
			gids[i] = int(g)
		}
		slog.Debug("setting additional gids", "id", c.State.ID, "gids", gids)
		if err := syscall.Setgroups(gids); err != nil {
			return fmt.Errorf("setgroups: %w", err)
		}
	}

	if u.Umask != nil {
		unix.Umask(int(*u.Umask))
	}

	slog.Debug("setting gid/uid", "id", c.State.ID, "gid", u.GID, "uid", u.UID)
	if err := syscall.Setgid(int(u.GID)); err != nil {
		return fmt.Errorf("setgid %d: %w", u.GID, err)
	}
	if err := syscall.Setuid(int(u.UID)); err != nil {
		return fmt.Errorf("setuid %d: %w", u.UID, err)
	}

	return nil
}

// rlimitResources maps OCI rlimit type names to their setrlimit(2) resource
// constants. See getrlimit(2) for the meaning of each.
var rlimitResources = map[string]int{
	"RLIMIT_AS":         unix.RLIMIT_AS,
	"RLIMIT_CORE":       unix.RLIMIT_CORE,
	"RLIMIT_CPU":        unix.RLIMIT_CPU,
	"RLIMIT_DATA":       unix.RLIMIT_DATA,
	"RLIMIT_FSIZE":      unix.RLIMIT_FSIZE,
	"RLIMIT_LOCKS":      unix.RLIMIT_LOCKS,
	"RLIMIT_MEMLOCK":    unix.RLIMIT_MEMLOCK,
	"RLIMIT_MSGQUEUE":   unix.RLIMIT_MSGQUEUE,
	"RLIMIT_NICE":       unix.RLIMIT_NICE,
	"RLIMIT_NOFILE":     unix.RLIMIT_NOFILE,
	"RLIMIT_NPROC":      unix.RLIMIT_NPROC,
	"RLIMIT_RSS":        unix.RLIMIT_RSS,
	"RLIMIT_RTPRIO":     unix.RLIMIT_RTPRIO,
	"RLIMIT_RTTIME":     unix.RLIMIT_RTTIME,
	"RLIMIT_SIGPENDING": unix.RLIMIT_SIGPENDING,
	"RLIMIT_STACK":      unix.RLIMIT_STACK,
}
