---
title: "[Epic]: Setting up the OCI Runtime Spec test suite"
labels: epic, roadmap
uploaded:
---

## Summary

The [Open Container Initiative](https://opencontainers.org/) (OCI) provides a
collection of tools for working with the
[OCI Runtime Specification](https://github.com/opencontainers/runtime-spec/blob/main/spec.md).
There exists a runtime validation suite, which can be used to validate container
runtimes according to the
[OCI Runtime Command Line Interface](https://github.com/opencontainers/runtime-tools/blob/master/docs/command-line-interface.md).

The goal for this epic is to research, implement and integrate the testing suite
mentioned here so we have an objective, upstream-defined measure of how close
`box` is to being a spec-compliant runtime.

> **NOTE:** During active development the test suite might **NOT** pass since
> features are still being developed. This epic is to be completed in parallel —
> it lands the harness and lets the pass/fail set grow as the runtime matures.

## Goals

- We can run the upstream `opencontainers/runtime-tools` validation suite against
  the `box` binary with a single command.
- `box` implements the OCI Runtime Command Line Interface (`create`, `start`,
  `state`, `kill`, `delete`) closely enough for the suite's harness to drive it.
- The suite runs in CI as a **non-blocking** job so we can watch conformance
  improve over time without red-gating active development.
- The set of passing vs. failing validation tests is visible and tracked, so
  progress against the runtime-spec is measurable.
- How to run the suite locally is documented for contributors.

## Tasks

- [ ] {{issue:01-research-suite.md}} Research the OCI runtime-tools validation suite & CLI contract
- [ ] {{issue:02-cli-conformance.md}} Make `box` conform to the OCI Runtime Command Line Interface
- [ ] {{issue:03-integrate-suite.md}} Integrate the runtime-tools validation suite into the repo
- [ ] {{issue:04-make-validation-target.md}} Add a local `make validation` target to run the suite against `box`
- [ ] {{issue:05-ci-integration.md}} Run the validation suite in CI as a non-blocking job
- [ ] {{issue:06-conformance-reporting.md}} Track and report conformance (passing vs. failing tests)

## Non-goals

- Passing 100% of the validation suite by the end of this epic — features are
  still being built; the harness lands now, green checks accrue later.
- Forking or re-implementing the upstream validation tests; we consume them.
- Conformance for the OCI **image** or **distribution** specs — runtime spec only.

## Open questions

- Vendor the runtime-tools suite, pull it as a git submodule, or fetch it at
  test time in CI? (trade-off: reproducibility vs. repo size/churn)
- Which runtime-spec version do we target first (`runtime-spec v1.2.0`, already a
  dependency in `go.mod`)?
- Do rootless-only constraints on the dev machine limit which tests can run?

## Target milestone

`v0.1 — runtime-spec conformance harness`
