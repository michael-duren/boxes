package container_test

import specs "github.com/opencontainers/runtime-spec/specs-go"

// testSpec returns a representative OCI runtime spec for tests, modeled on a
// typical `runc spec` config. It includes the pieces our code actually reads
// (Process.Args, Root, Hostname, namespaces) plus enough surrounding detail to
// exercise (un)marshaling realistically. Tweak the returned value per-test
// rather than editing this helper.
func testSpec() *specs.Spec {
	return &specs.Spec{
		Version:  "1.3.0",
		Hostname: "runc",
		Process: &specs.Process{
			Terminal: true,
			User:     specs.User{UID: 0, GID: 0},
			Args:     []string{"sleep", "60"},
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
			},
			Cwd: "/",
			Capabilities: &specs.LinuxCapabilities{
				Bounding:  []string{"CAP_AUDIT_WRITE", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
				Effective: []string{"CAP_AUDIT_WRITE", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
				Permitted: []string{"CAP_AUDIT_WRITE", "CAP_KILL", "CAP_NET_BIND_SERVICE"},
			},
			Rlimits: []specs.POSIXRlimit{
				{Type: "RLIMIT_NOFILE", Hard: 1024, Soft: 1024},
			},
			NoNewPrivileges: true,
		},
		Root: &specs.Root{
			Path:     "rootfs",
			Readonly: true,
		},
		Mounts: []specs.Mount{
			{Destination: "/proc", Type: "proc", Source: "proc"},
			{
				Destination: "/dev",
				Type:        "tmpfs",
				Source:      "tmpfs",
				Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
			},
			{
				Destination: "/sys",
				Type:        "sysfs",
				Source:      "sysfs",
				Options:     []string{"nosuid", "noexec", "nodev", "ro"},
			},
		},
		Linux: &specs.Linux{
			Resources: &specs.LinuxResources{
				Devices: []specs.LinuxDeviceCgroup{
					{Allow: false, Access: "rwm"},
				},
			},
			Namespaces: []specs.LinuxNamespace{
				{Type: specs.PIDNamespace},
				{Type: specs.NetworkNamespace},
				{Type: specs.IPCNamespace},
				{Type: specs.UTSNamespace},
				{Type: specs.MountNamespace},
				{Type: specs.CgroupNamespace},
			},
			MaskedPaths: []string{
				"/proc/acpi",
				"/proc/kcore",
				"/proc/keys",
				"/sys/firmware",
			},
			ReadonlyPaths: []string{
				"/proc/bus",
				"/proc/fs",
				"/proc/irq",
				"/proc/sys",
				"/proc/sysrq-trigger",
			},
		},
	}
}
