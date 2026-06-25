# Boxes

> A minimal Linux container runtime, written in Go.

`box` is the CLI front-end for the Boxes runtime. It follows the OCI-style
lifecycle (`create` → `start` → `kill` → `delete`) and is intended as a
learning-oriented, low-level container runtime in the spirit of `runc` /
`youki`.

> **Status:** early development — APIs, on-disk state layout, and CLI flags
> are unstable and may change without notice.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Configuration](#configuration)
- [Development](#development)
- [Roadmap](#roadmap)
- [License](#license)

## Features

- [ ] OCI-compatible container lifecycle (`create`, `start`, `kill`, `delete`, `state`)
- [ ] Linux namespace isolation (PID, mount, UTS, IPC, network, user)
- [ ] cgroups v2 resource limits (CPU, memory, pids, IO)
- [ ] Rootless containers
- [ ] OCI runtime spec (`config.json`) consumption
- [ ] Image rootfs handling (bring-your-own rootfs for now)

## Architecture

```
┌──────────────┐      ┌──────────────────────┐      ┌─────────────────┐
│  box (CLI)   │ ───► │  internal/operations │ ───► │  Linux kernel   │
│  cmd/cli     │      │  lifecycle handlers  │      │  ns / cgroups   │
└──────────────┘      └──────────────────────┘      └─────────────────┘
```

- `cmd/cli/` — Cobra-based CLI entrypoint and subcommand wiring.
- `internal/operations/` — runtime operations that implement each lifecycle verb.
- _(planned)_ `internal/spec/` — OCI runtime spec parsing.
- _(planned)_ `internal/cgroups/` — cgroups v2 controller.
- _(planned)_ `internal/namespaces/` — namespace setup and the re-exec init process.

## Requirements

- Linux kernel ≥ 5.10 (cgroups v2, user namespaces)
- Go ≥ 1.26
- A rootfs directory (e.g. extracted from `docker export` or a distro bootstrap)
- For rootless mode: `newuidmap` / `newgidmap` and `/etc/subuid` + `/etc/subgid` entries

## Installation

From source:

```sh
git clone https://github.com/michael-duren/boxes.git
cd boxes
make build              # produces ./bin/box
make install            # installs to $HOME/.local/bin/box
```

`PREFIX` can be overridden, e.g. `make install PREFIX=/usr/local`.

## Quick Start

```sh
# 1. Prepare a rootfs
mkdir -p /tmp/mybox/rootfs
# ...extract a base image tarball into rootfs/...

# 2. Create the container
box create mybox --bundle /tmp/mybox

# 3. Start it
box start mybox

# 4. Inspect its state
box state mybox

# 5. Stop and clean up
box kill mybox
box delete mybox
```

## Commands

| Command           | Description                                     |
| ----------------- | ----------------------------------------------- |
| `box create <id>` | Create a container from a bundle (does not run) |
| `box start <id>`  | Start the process inside a created container    |
| `box state <id>`  | Print the current state of a container          |
| `box kill <id>`   | Send a signal to the container's init process   |
| `box delete <id>` | Remove a stopped container and its state        |

Run `box <command> --help` for full flag documentation.

## Configuration

Containers are described by an OCI runtime `config.json` inside the bundle
directory passed to `box create`. A minimal example:

```json
{
  "ociVersion": "1.0.2",
  "process": {
    "args": ["/bin/sh"],
    "cwd": "/"
  },
  "root": { "path": "rootfs" }
}
```

State for running containers is persisted under
`$XDG_RUNTIME_DIR/boxes/<container-id>/` _(planned)_.

## Development

### Cross-compilation environment

Boxes targets Linux. If developing on macOS (or another non-Linux OS), the
project uses [direnv](https://direnv.net/) to automatically set `GOOS=linux`
and `GOARCH=amd64` so that `gopls` and other Go tools resolve Linux-only
symbols (e.g. `syscall.CLONE_*`, `unix.SIGKILL`).

```sh
brew install direnv          # if not already installed
echo 'eval "$(direnv hook zsh)"' >> ~/.zshrc   # or your shell rc
direnv allow                 # approve the project .envrc
```

These variables are scoped to this project directory and do not affect other
Go projects.

### Make targets

```sh
make build      # compile to ./bin/box
make run ARGS="state mybox"
make test       # go test ./...
make vet
make fmt
make lint       # requires golangci-lint
make tidy
make clean
```

Project layout:

```
.
├── cmd/cli/             # CLI entrypoint (Cobra)
├── internal/operations/ # Lifecycle operation implementations
├── Makefile
└── go.mod
```

### OCI conformance

`box` is measured against the upstream
[`opencontainers/runtime-tools`](https://github.com/opencontainers/runtime-tools)
validation suite. See [docs/oci-validation.md](docs/oci-validation.md) for the
CLI contract, suite mechanics, and version pin. To drive one validation test
against the locally-built `box` (expected to fail during development — needs
root + user namespaces):

```sh
./scripts/oci-validation.sh            # runs validation/default
TEST=state ./scripts/oci-validation.sh # run a specific test
```

## Roadmap

- [ ] Wire `create` to fork+exec an init process in new namespaces
- [ ] Parse and honor OCI `config.json`
- [ ] cgroups v2 resource limits
- [ ] Persisted container state under `$XDG_RUNTIME_DIR`
- [ ] Rootless support via user namespaces and `newuidmap`
- [ ] Networking (veth + bridge, then CNI)
- [ ] `exec` into a running container
- [ ] Integration tests with a real rootfs

## License

See [LICENSE](./LICENSE).
