---
title: "[Task]: Integrate the runtime-tools validation suite into the repo"
labels: task
uploaded:
---

## What needs to be done

Bring the `opencontainers/runtime-tools` validation suite into the project as a
**git submodule** pinned at commit `e5b4542` — the approach chosen during
research ({{issue:01-research-suite.md}}, see
[`docs/oci-validation.md` §4](../../docs/oci-validation.md)). Get the suite
building and able to invoke `box` as `RUNTIME`.

Includes:

- Adding the submodule at `third_party/runtime-tools` pinned to commit
  `e5b454202754ff211f8dbeb98a398b5c3d346b79` (last commit before master moved to
  runtime-spec v1.3.0; aligns with our v1.2.0 pin). Document the update flow
  (bump the submodule ref, commit).
- Ensuring the `runtimetest` helper and validation test binaries build in our
  environment. **Dependencies are the Go toolchain + `prove` (Perl TAP::Harness)
  — NOT bats, and NOT modern node-tap.** The suite has no `.bats` files; tests
  compile to `.t` ELF binaries that emit TAP, and current node-tap (v15+) can't
  execute them (it imports test files as JS/TS). The arch rootfs tarballs
  (`rootfs-amd64.tar.gz`) are checked into the upstream repo, so no Docker/rootfs
  build is needed.
- Repointing `scripts/oci-validation.sh` from its current `.cache/` scratch
  clone to the submodule path so dev and CI share one source of truth.

> Note: `scripts/oci-validation.sh` already drives the suite against `box` from a
> gitignored `.cache/` clone (landed with the research task); this issue makes
> that source the pinned submodule instead.

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
