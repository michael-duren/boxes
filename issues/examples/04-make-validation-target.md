---
title: "[Task]: Add a local 'make validation' target to run the suite against box"
labels: task
uploaded:
---

## What needs to be done

Add a `make validation` (and/or `scripts/validation.sh`) target that builds
`box`, points the integrated runtime-tools suite at it, and runs the validation
tests locally with one command.

The target should:

- Build the `box` binary (reuse existing `make` build wiring).
- Run the runtime-tools validation suite with `RUNTIME` set to the built binary.
- Surface results as readable output (TAP / pass-fail summary).
- Allow running a single test or a subset for focused debugging.
- Not fail the developer's whole build when individual tests fail (since the
  suite is expected to be partially red during active development).

## Acceptance criteria

- [ ] `make validation` builds `box` and runs the suite in one command
- [ ] Results are printed as a clear pass/fail summary
- [ ] A way to run a single/subset of tests is documented (e.g. `make validation TEST=...`)
- [ ] The target is documented in the README "Development" section
- [ ] Running it produces a real (possibly partially-failing) result against current `box`

## Parent / tracking issue

Part of {{issue:00-epic.md}}.

## Blocked by

- {{issue:03-integrate-suite.md}}

## Rough size

S — a few hours
