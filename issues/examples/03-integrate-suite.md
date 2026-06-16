---
title: "[Task]: Integrate the runtime-tools validation suite into the repo"
labels: task
uploaded:
---

## What needs to be done

Bring the `opencontainers/runtime-tools` validation suite into the project using
the integration approach chosen during research (vendor / git submodule /
fetch-in-CI). Get the suite building and able to invoke `box` as `RUNTIME`.

Includes:

- Adding the suite to the repo via the chosen mechanism (and documenting how to
  update/pin it).
- Ensuring the `runtimetest` helper and validation test binaries build in our
  environment (Go toolchain, bats dependency).
- A reproducible way to point the suite at the locally-built `box` binary.

## Acceptance criteria

- [ ] runtime-tools validation suite is present/pinned in the repo via the chosen approach
- [ ] The suite and `runtimetest` helper build cleanly locally
- [ ] The suite can be pointed at the `box` binary and at least starts executing tests
- [ ] How to update the pinned suite version is documented
- [ ] `.gitignore` / build artifacts handled so the suite doesn't pollute the tree

## Parent / tracking issue

Part of {{issue:00-epic.md}}.

## Blocked by

- {{issue:01-research-suite.md}}

## Rough size

M — a day or two
