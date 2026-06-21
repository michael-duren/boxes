# OCI Runtime Validation — design note

> Status: research / design. Output of issue **01 — Research the OCI
> runtime-tools validation suite & CLI contract**. Feeds issues 02–06.

This note explains what the upstream
[`opencontainers/runtime-tools`](https://github.com/opencontainers/runtime-tools)
validation suite expects of a runtime, how the suite is built and driven, which
version we target, and how to integrate it into this repo. It ends with a
runnable command that drives at least one validation test against `box` today
(it fails — that is expected and documented), and a list of concrete gaps the
follow-up tasks should close.

Sources are upstream at commit
[`e5b4542`](https://github.com/opencontainers/runtime-tools/commit/e5b454202754ff211f8dbeb98a398b5c3d346b79)
unless noted; the CLI contract is the
[command-line-interface.md](https://github.com/opencontainers/runtime-tools/blob/master/docs/command-line-interface.md)
document (CLI spec v1.0.1).

---

## TL;DR

- **What the suite expects:** a runtime binary exposing `create` / `start` /
  `state` / `kill` / `delete`, each taking a positional container ID, returning
  `0` on success / non-zero on error, with **only `state` writing to stdout**
  (the OCI state JSON). The suite copies a static `runtimetest` helper into each
  bundle and makes it the container's init process; that helper validates the
  _live_ container against `config.json` from the inside and prints
  [TAP](https://testanything.org/).
- **It is not bats.** The suite emits TAP from self-contained Go test binaries
  (`validation/<feature>/<feature>.t`) and aggregates with a TAP consumer
  (node-tap `tap`, or `prove`). There are no `.bats` files anywhere in the repo.
- **Version:** target **runtime-spec v1.2.0** (`ociVersion` `"1.2.0"`), already
  pinned in `go.mod`. Pin the suite to runtime-tools commit
  [`e5b4542`](https://github.com/opencontainers/runtime-tools/commit/e5b454202754ff211f8dbeb98a398b5c3d346b79)
  (the last commit before master moved to runtime-spec v1.3.0).
- **Integration:** **git submodule** pinned at that commit (rationale below).
- **Run one test today:** `./scripts/oci-validation.sh` (see
  [Running it now](#running-it-now)).

---

## 1. The OCI Runtime Command Line Interface contract

The validation suite drives the runtime purely through its CLI. The contract is
defined by
[`docs/command-line-interface.md`](https://github.com/opencontainers/runtime-tools/blob/master/docs/command-line-interface.md)
(CLI spec **v1.0.1**, versioned independently of the config-format spec).

Invocation shape: `box [global-opts] <COMMAND> [command-opts] <ID>`. Command
names must not start with a hyphen; an unrecognized command must exit non-zero.

| Command  | Positional | Options (per CLI spec)                                                        | Must do                                                                                        |
| -------- | ---------- | ----------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `create` | `<ID>`     | `--bundle <path>` (default CWD), `--console-socket <fd>`, `--pid-file <path>` | Create the container from the bundle. Set up everything **except** running the user `process`. |
| `start`  | `<ID>`     | —                                                                             | Run the user-specified `process.args`.                                                         |
| `state`  | `<ID>`     | —                                                                             | Print the state JSON to **stdout**.                                                            |
| `kill`   | `<ID>`     | `--signal <sig>` (default `TERM`)                                             | Send a signal to the container process. Must support `TERM` and `KILL` with POSIX semantics.   |
| `delete` | `<ID>`     | —                                                                             | Release container resources after the process has exited.                                      |

### Exit codes

Every command: **`0` on success, non-zero on any error.** Uniform across the
five verbs (e.g. `state` returns 0 _"if the state was successfully written to
stdout"_). An error message on stderr is **permitted but not required**, and its
format is unspecified — the only hard signal is the exit code.

### stdout / stderr

- **`state` is the only command that MUST write to stdout** — exactly the state
  JSON, nothing else.
- For `start` / `kill` / `delete`, stdout handling is _unspecified_.
- For `create`, when `process.terminal` is false the runtime MUST pass its
  stdin/stdout/stderr **through to the container process unmodified** — i.e. it
  must not print its own chatter onto those fds. When `terminal` is true, stdout
  handling is unspecified and the PTY master is delivered over `--console-socket`.
- Any command MAY print diagnostics to stderr (unspecified format).

**Implication for `box`:** all logging must go to stderr or the log file, never
stdout, or `state` parsing breaks. `box` already routes `slog` to a log file
(`internal/logger`), so stdout stays clean — good.

### The `state` JSON

The CLI doc only shows one (malformed, `bundlePath`-using) example and defers the
real schema to the
[runtime-spec `state` object](https://github.com/opencontainers/runtime-spec/blob/v1.2.0/runtime.md#state).
The suite parses it into `runtime-spec`'s `specs.State`, so the canonical fields
are:

| Field         | Notes                                                                            |
| ------------- | -------------------------------------------------------------------------------- |
| `ociVersion`  | string, e.g. `"1.2.0"`                                                           |
| `id`          | container ID                                                                     |
| `status`      | one of `creating`, `created`, `running`, `stopped`                               |
| `pid`         | PID of the container process (in the runtime's PID namespace)                    |
| `bundle`      | absolute path to the bundle (note: **`bundle`**, not the CLI doc's `bundlePath`) |
| `annotations` | string→string map                                                                |

**`box` already emits this correctly.** `internal/container.New` builds a
`specs.State{Version, ID, Bundle, Annotations, Status}` and `operations.State`
marshals it straight to stdout, so the field names and status enum come from
`runtime-spec` v1.2.0 for free.

### Contract vs. what the harness actually sends

The harness (`validation/util/container.go`) does **not** exercise the full CLI
spec — it sends a narrower, concrete invocation. Where the two differ, **the
harness is what we must satisfy**:

- **`create`** → `box create [--pid-file <path>] [--bundle <dir>] <ID>`.
  The harness passes `--pid-file` only when a test sets it, and **never passes
  `--console-socket`** (no terminal/detach handling in the harness's `Create`).
- **`kill`** → `box kill <ID> <signal>` — the signal is passed **positionally**,
  not as `--signal` (there's an explicit code comment that runc doesn't accept
  the flag form). `box` already takes the signal positionally, so it matches.
- **`delete`** → `box delete <ID>`; force path is `box delete --force <ID>`.
- **`start`** → `box start <ID>`. **`state`** → `box state <ID>` (output captured
  and JSON-parsed).

---

## 2. Structure of the `validation/` suite

### Layout

One directory per feature: `validation/<feature>/<feature>.go`, each a
`package main` program. Examples present at the pinned commit: `default`,
`create`, `start`, `state`, `kill`, `killsig`, `delete`, `hostname`, `mounts`,
`pidfile`, `process`, `process_capabilities`, `linux_ns_*`, `linux_cgroups_*`,
`linux_seccomp`, `linux_sysctl`, `linux_uid_mappings`, `prestart`/`poststart`/
`poststop` (+ `_fail` variants), `hooks`, … (≈60 dirs).

Each `<feature>.go` compiles to a sibling **`<feature>.t`** executable. A `.t`
binary is self-contained: it reads the `RUNTIME` env var, builds a bundle,
drives the runtime through the lifecycle, and prints **TAP** to stdout. The TAP
comes from the `github.com/mndrix/tap-go` library used by `validation/util`.

### `runtimetest` — the in-container validator

`cmd/runtimetest` builds to a **statically linked** binary (`make runtimetest`).
The suite copies it into the bundle (`fileutils.CopyFile("runtimetest",
<bundle>/runtimetest)`) and sets the container's process args to
`["/runtimetest", "--path=/"]`. So _the container's init process is
`runtimetest`_: from inside the namespace it loads `/config.json`, checks the
live state (mounts, caps, rlimits, hostname, masked paths, …) against what the
config requested, and emits TAP. This is the `RuntimeInsideValidate` path.

A second path, `RuntimeOutsideValidate`, validates from the host (e.g. running
`state` and inspecting the result) without relying on the in-container helper —
used for lifecycle/state assertions.

### Build + run flow (`make localvalidation`)

From the suite's `Makefile`:

```make
RUNTIME ?= runc
TAPTOOL ?= tap
VALIDATION_TESTS ?= $(patsubst %.go,%.t,$(shell find ./validation/ -name *.go | grep -v util))

runtimetest:                 go build $(STATIC_BUILD_FLAGS) -o runtimetest ./cmd/runtimetest
validation-executables:      $(VALIDATION_TESTS)        # foo.go -> foo.t
localvalidation:             # checks binaries exist, then:
    RUNTIME=$(RUNTIME) $(TAPTOOL) $(VALIDATION_TESTS)
```

- `make runtimetest validation-executables` **builds** the helper and every
  `.t` binary. (`localvalidation` does **not** build — it only checks they
  exist, then runs them.)
- `RUNTIME=<bin> make localvalidation` exports `RUNTIME` and hands all `.t`
  binaries to `$(TAPTOOL)` (node-tap `tap` by default; `prove` works too).
- Single test / subset: override `VALIDATION_TESTS`, e.g.
  `sudo make RUNTIME=box VALIDATION_TESTS=validation/state/state.t localvalidation`,
  or run the binary directly: `RUNTIME=box ./validation/state/state.t`.

### How the runtime is located

`validation/util` reads `RUNTIME` from the environment into the package global
`RuntimeCommand` (default `"runc"`), and resolves it with `exec.LookPath`. So
`RUNTIME` can be a name on `PATH` or an absolute path to the built `box`. No
other runtime-selection env var is used.

### Dependencies & environment

- **Go 1.19+** to build the suite and `box`.
- **A TAP consumer:** node-tap (`npm i -g tap`) _or_ `prove` (ships with Perl).
  **Not bats.**
- **`tar`** — the suite shells out to extract `rootfs-<arch>.tar.gz`.
- **Rootfs tarballs are checked into the upstream repo** (`rootfs-amd64.tar.gz`,
  `rootfs-386.tar.gz`) — no Docker/buildroot step needed. The `.t` binary
  extracts them relative to **CWD**, so tests must be run from the suite root.
- **Root on Linux** — most tests create real containers (namespaces, cgroups v2,
  mounts) and need privilege. Rootless runners can only reach the early
  lifecycle before namespace setup fails (see below).

---

## 3. Version alignment

| Component                     | Version                        | Notes                                                                            |
| ----------------------------- | ------------------------------ | -------------------------------------------------------------------------------- |
| `runtime-spec` (our `go.mod`) | **v1.2.0**                     | `ociVersion` `"1.2.0"`; released 2024-02-13. **This is our target.**             |
| runtime-tools latest **tag**  | v0.9.0 (2019)                  | Pre-go-modules; far too old. Ignore.                                             |
| runtime-tools `master`        | tracks runtime-spec **v1.3.0** | master jumped v1.1.0 → **v1.3.0** (2025-11-11), skipping the entire v1.2.x line. |
| **Recommended suite pin**     | commit **`e5b4542`**           | Parent of the v1.3.0 bump; depends on runtime-spec **v1.1.0**.                   |

There is **no runtime-tools tag or commit that pins runtime-spec v1.2.0** — so
we pin a commit. Commit `e5b4542` depends on runtime-spec **v1.1.0**, which is
API-compatible with our **v1.2.0** (v1.2.0 was additive). Current master is
**unsuitable** while we're on v1.2.0: runtime-spec v1.3.0 changed
`LinuxPids.Limit` from `int64` to `*int64`, so master won't build against our
pin, and pulling master would also try to drag our `runtime-spec` up to v1.3.0
under Go's minimal-version-selection.

> **Decision: target runtime-spec v1.2.0; pin runtime-tools at `e5b4542`.**
> Revisit when `box` itself moves to runtime-spec v1.3.0 (then track master).

---

## 4. Integration: vendor vs. submodule vs. fetch-in-CI

The suite is a **separate Go module** built into standalone binaries; we do not
`import` it from `box`'s module. So this is about how the _source tree_ arrives,
not a `go.mod` dependency.

| Option                            | Reproducible        | Repo bloat                                                                                   | Local-dev UX                          | Offline       | Update story                |
| --------------------------------- | ------------------- | -------------------------------------------------------------------------------------------- | ------------------------------------- | ------------- | --------------------------- |
| **git submodule** (pinned commit) | ✅ exact commit     | ✅ only a gitlink in our history                                                             | needs `submodule update --init` once  | ✅ after init | bump the ref, commit        |
| Vendor (copy tree in)             | ✅                  | ❌ ~2 MB rootfs tarballs + foreign sources in _our_ history; risks polluting `go test ./...` | ✅ nothing to fetch                   | ✅            | manual re-copy, noisy diffs |
| Fetch-in-CI (clone at commit)     | ✅ if commit-pinned | ✅ none                                                                                      | ❌ every dev re-clones; needs network | ❌            | change the pinned hash      |

**Recommendation: git submodule, pinned at `e5b4542`.** It pins an exact commit
(reproducible), keeps the foreign module and its checked-in rootfs tarballs out
of _our_ history (just a gitlink), is offline after the one-time init, and
updates by bumping the ref. Vendoring drags a second Go module and ~2 MB of
binaries into our tree and risks `go test ./...` / lint picking up foreign code;
fetch-in-CI is reproducible but forces a network clone on every dev and CI run.

Concretely (for issue 03):

```sh
git submodule add https://github.com/opencontainers/runtime-tools.git \
  third_party/runtime-tools
git -C third_party/runtime-tools checkout e5b454202754ff211f8dbeb98a398b5c3d346b79
git add third_party/runtime-tools .gitmodules && git commit
```

CI then needs `actions/checkout` with `submodules: true`. The bundled
`scripts/oci-validation.sh` (below) currently _clones_ into a gitignored
`.cache/` so the command is runnable **before** the submodule lands; issue 03
should switch it to the submodule path.

---

## 5. Running it now

`scripts/oci-validation.sh` is the documented command required by this task. It:

1. builds `box` (`make build`),
2. clones runtime-tools at the pinned commit into `.cache/runtime-tools`
   (gitignored),
3. builds `runtimetest` + the requested `.t` test(s),
4. runs them with `RUNTIME=<abs path to box>` from the suite root.

```sh
./scripts/oci-validation.sh                 # runs validation/default against box
TEST=state ./scripts/oci-validation.sh      # run a specific test
TEST="state kill delete" ./scripts/oci-validation.sh
```

### Current result (expected: failing)

On a **rootless** dev box (uid ≠ 0), even `runc` can't get past `create`:

```
$ RUNTIME=runc ./validation/default/default.t
failed to create the container
... msg="runc create failed: rootless container requires user namespaces"
```

`box` is driven through the same lifecycle and fails too — the harness _does_
drive it (create → state → delete), which is the point:

```
$ RUNTIME=<.../bin/box> ./validation/default/default.t
failed to create the container
Error: initialize container: accept on init sock: ... i/o timeout
Clean: Delete: ... exit status 1
```

Some host-side checks still run, though — e.g. `TEST=state` drives `box state`
and reports a **passing** subtest even rootless:

```
$ TEST=state ./scripts/oci-validation.sh
validation/state/state.t .. All 1 subtests passed
```

This confirms three things the epic predicted: (a) the harness can invoke `box`
unmodified, (b) `box`'s `state` JSON already satisfies a real validation check,
and (c) the suite needs **root + user namespaces** for almost everything else.
Running the full suite meaningfully requires a privileged Linux host (local
`sudo` or a CI runner) — see issue 05.

> **TAP consumer note:** prefer node-tap's `tap`. `prove` works but its strict
> YAMLish parser trips on the YAML diagnostic blocks `tap-go` emits (you'll see
> `Unsupported YAMLish syntax` / `No plan found` and a spurious `Result: FAIL`
> even when the subtests pass). The script prefers `tap` when installed and falls
> back to `prove`; install node-tap (`npm i -g tap`) for clean parsing.

---

## 6. Gaps found in `box` (feed issue 02)

Verified by running the harness against the built `box` binary:

1. **`box create` rejects `--pid-file`.** The harness passes
   `create --pid-file <path> --bundle <dir> <ID>` for the `pidfile`/`create`
   tests; `box` exits `1` with `unknown flag: --pid-file`. `box` must accept
   `--pid-file` (and write the container PID to it — the `pidfile` test checks
   the file contents).
2. **No `--console-socket`.** The CLI spec defines it for `terminal: true`
   bundles. The _harness_ never sends it (good for now), but the `alpinefs`
   sample config sets `"terminal": true`; conformance bundles generally set
   `terminal: false`, so this is lower priority than `--pid-file`.
3. **`create` must fully prepare the container without running the process** so
   `runtimetest` runs only after `start`. `box`'s `create` re-execs an init
   that blocks on a socket until `start` — structurally correct — but namespace
   / rootfs / mount setup is still `TODO` in `internal/container`, so `start`
   can't exec `/runtimetest` yet. This is the bulk of real conformance work.
4. **Signal handling matches** — `box kill <id> <signal>` already takes the
   signal positionally, as the harness sends it. No change needed.
5. **State output matches** — fields and status enum come from `specs.State`.
   Keep stdout clean (logging already goes to the log file).

---

## 7. Follow-up impact

- **02 (CLI conformance):** add `--pid-file` (+ write PID), keep stdout clean,
  then land namespace/rootfs/mount setup so `start` can exec the bundle process.
  `--console-socket` can wait.
- **03 (integrate suite):** use a **git submodule** at `third_party/runtime-tools`
  pinned to `e5b4542`; document the update (bump-ref) flow; repoint
  `scripts/oci-validation.sh` at the submodule. Note: dependency is **node-tap
  or `prove`**, _not bats_ (the issue text said bats — that's incorrect).
- **04 (`make validation`):** wrap the script as a `make validation` target;
  `TEST=`/`VALIDATION_TESTS=` for subsets; non-fatal on failures.
- **05 (CI):** privileged Linux runner; `continue-on-error`; upload TAP as an
  artifact. Document the rootless limitation.
- **06 (reporting):** parse TAP (node-tap/`prove` both emit it) into a
  pass/fail summary; snapshot a baseline.
