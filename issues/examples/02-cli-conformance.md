---
title: "[Task]: Make box conform to the OCI Runtime Command Line Interface"
labels: task
uploaded:
---

## What needs to be done

Make the `box` CLI conform to the
[OCI Runtime Command Line Interface](https://github.com/opencontainers/runtime-tools/blob/master/docs/command-line-interface.md)
so the validation suite's harness can drive it without custom shims.

This means aligning the existing lifecycle verbs with the spec:

- `box create <id>` — create a container from a bundle (`--bundle`), without
  running the user process; correct exit code on success/failure.
- `box start <id>` — start a previously-created container's process.
- `box state <id>` — emit the OCI state JSON (`ociVersion`, `id`, `status`,
  `pid`, `bundle`, `annotations`) to stdout.
- `box kill <id> <signal>` — send the signal to the container process.
- `box delete <id>` — remove a stopped container's state.

Match the flag names, positional arguments, exit codes, and stdout/stderr
behavior the suite expects (per the research note from the prior task).

## Gaps found during research ({{issue:01-research-suite.md}})

Verified by running the harness against the built `box` (see
[`docs/oci-validation.md` §6](../../docs/oci-validation.md)):

- **`box create` rejects `--pid-file`** with `unknown flag` (exit 1). The harness
  sends `create [--pid-file <path>] --bundle <dir> <id>`. **Add `--pid-file` and
  write the container PID to it** — the `pidfile` test reads the file contents.
- **Logging must stay off stdout** — `state` is the only command allowed to write
  stdout (the state JSON), and `create` (non-terminal) must pass stdout through
  to the container untouched. `box` already routes `slog` to a log file, so this
  holds; keep it that way.
- **Already conformant:** `kill` takes the signal **positionally** (`kill <id>
  <signal>`), which is exactly how the harness invokes it — no `--signal` flag
  needed. `state` already emits the correct `specs.State` fields/status enum.
- **Lower priority:** `--console-socket` (the harness never sends it; only needed
  for `terminal: true` bundles). The bulk of real conformance work is the
  namespace / rootfs / mount setup in `internal/container` so `start` can exec
  the bundle's process (`/runtimetest`).

## Acceptance criteria

- [ ] `box create` accepts `--pid-file` and writes the container PID to it
- [ ] `create`, `start`, `state`, `kill`, `delete` accept the spec's args/flags
- [ ] `box state` outputs valid OCI state JSON matching the runtime-spec schema
- [ ] Exit codes follow the CLI spec (0 on success, non-zero with a message on failure)
- [ ] The container lifecycle (`create → start → kill → delete`) is drivable end-to-end by the harness
- [ ] Behavior verified against the runtime-tools suite for at least the lifecycle/state tests

## Parent / tracking issue

Part of {{issue:00-epic.md}}.

## Blocked by

- {{issue:01-research-suite.md}} (must define the CLI contract first)

## Rough size

L — most of a week
