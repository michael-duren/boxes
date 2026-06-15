---
title: "[Task]: Track and report conformance (passing vs. failing tests)"
labels: task
uploaded:
---

## What needs to be done

Make conformance progress visible: track which validation tests pass vs. fail
over time so improvement against the runtime-spec is measurable.

Options to evaluate and pick from:

- Parse the suite's TAP output into a pass/fail summary (counts + per-test status).
- Publish the summary somewhere durable: a generated `docs/conformance.md`, a
  CI job summary, or a badge in the README.
- Optionally snapshot a baseline so regressions (a previously-passing test going
  red) are detectable.

Keep it lightweight — the point is a trend line, not a dashboard.

## Acceptance criteria

- [ ] Suite output is parsed into a pass/fail summary (total passing / failing / skipped)
- [ ] The summary is published somewhere contributors can see it (README badge, docs page, or CI summary)
- [ ] A baseline snapshot exists so regressions can be spotted
- [ ] How to regenerate the report is documented

## Parent / tracking issue

Part of {{issue:00-epic.md}}.

## Blocked by

- {{issue:05-ci-integration.md}}

## Rough size

S — a few hours
