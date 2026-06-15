---
title: "[Task]: Run the validation suite in CI as a non-blocking job"
labels: task
uploaded:
---

## What needs to be done

Run the validation suite in CI (GitHub Actions) as a **non-blocking** job so we
can watch conformance evolve without red-gating active development.

The job should:

- Run on a Linux runner with the kernel features the runtime needs (namespaces,
  cgroups v2). Note runner limitations (e.g. user-namespace / rootless constraints).
- Build `box` and run `make validation`.
- Be marked non-required / `continue-on-error` so a partially-red suite doesn't
  block merges during this epic.
- Upload the suite output (TAP / summary) as a build artifact for inspection.

## Acceptance criteria

- [ ] A GitHub Actions workflow runs the validation suite on push/PR
- [ ] The job is non-blocking (`continue-on-error` or not a required check)
- [ ] Suite output is uploaded as an artifact
- [ ] Any runner/kernel limitations affecting which tests can run are documented
- [ ] The workflow is green-or-neutral even while individual validation tests fail

## Parent / tracking issue

Part of {{issue:00-epic.md}}.

## Blocked by

- {{issue:04-make-validation-target.md}}

## Rough size

M — a day or two
