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

## Acceptance criteria

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
