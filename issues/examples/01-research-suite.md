---
title: "[Task]: Research the OCI runtime-tools validation suite & CLI contract"
labels: task
uploaded:
---

## What needs to be done

Research how the OCI runtime validation suite works and write up what `box` must
satisfy to be driven by it. The output is a short design note (in the repo, e.g.
`docs/oci-validation.md`) plus a recommendation for how to integrate the suite.

Cover:

- The [OCI Runtime Command Line Interface](https://github.com/opencontainers/runtime-tools/blob/master/docs/command-line-interface.md)
  contract: the exact `create` / `start` / `state` / `kill` / `delete` semantics,
  exit codes, and stdout/stderr expectations the suite relies on.
- The structure of the [`opencontainers/runtime-tools`](https://github.com/opencontainers/runtime-tools)
  `validation/` directory — how the `runtimetest` helper and the per-feature test
  binaries are built and run (the `RUNTIME=<bin> make localvalidation` flow, bats, TAP output).
- How the suite locates and invokes the runtime under test, and what env/flags it passes.
- Which `runtime-spec` version aligns with our pinned `runtime-spec v1.2.0` dependency.
- Integration options (vendor vs. git submodule vs. fetch-in-CI) with a recommendation.

## Acceptance criteria

- [ ] A design note committed to the repo summarizing the CLI contract and suite mechanics
- [ ] A documented command (even if failing) that runs at least one validation test against `box`
- [ ] A recommendation on vendor vs. submodule vs. fetch-in-CI, with rationale
- [ ] Target runtime-spec version chosen and recorded
- [ ] Follow-up tasks updated/refined based on findings

## Rough size

M — a day or two
