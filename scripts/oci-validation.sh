#!/usr/bin/env bash
#
# oci-validation.sh — drive the OCI runtime-tools validation suite against `box`.
#
# This is the research artifact for issue 01 (see docs/oci-validation.md): a
# single documented command that runs at least one upstream validation test
# against the locally-built `box` binary. It is EXPECTED TO FAIL today — the
# suite needs root + user namespaces, and `box`'s namespace/rootfs setup is
# still in progress. The point is that the upstream harness can drive `box`
# unmodified.
#
# Issue 03 will replace the clone-into-.cache step with a pinned git submodule
# at third_party/runtime-tools, and issue 04 will wrap this as `make validation`.
#
# Usage:
#   ./scripts/oci-validation.sh                  # runs validation/default
#   TEST=state ./scripts/oci-validation.sh       # run a single test
#   TEST="state kill delete" ./scripts/oci-validation.sh
#   RUNNER=prove ./scripts/oci-validation.sh     # force the TAP consumer
#
# Requires: go (1.19+), git, tar. No TAP consumer needed — the default 'direct'
# runner executes each compiled `.t` and tallies ok/not-ok itself. RUNNER=prove
# uses Perl's TAP::Harness instead. Run on Linux; use `sudo -E` for tests that
# need root. (Do NOT use node-tap: v15+ imports test files as JS/TS and can't
# execute the Go ELF `.t` binaries — see docs/oci-validation.md §5.)

set -euo pipefail

# Pinned runtime-tools commit: the last commit before master moved to
# runtime-spec v1.3.0. It depends on runtime-spec v1.1.0, which is
# API-compatible with our pinned v1.2.0. See docs/oci-validation.md §3.
RUNTIME_TOOLS_REPO="https://github.com/opencontainers/runtime-tools.git"
RUNTIME_TOOLS_COMMIT="e5b454202754ff211f8dbeb98a398b5c3d346b79"

REPO_ROOT="$(git rev-parse --show-toplevel)"
SUITE_DIR="$REPO_ROOT/.cache/runtime-tools"
BOX_BIN="$REPO_ROOT/bin/box"
TESTS="${TEST:-default}"

# 1. Build box.
echo ">> building box"
make -C "$REPO_ROOT" build >/dev/null

# 2. Fetch the suite at the pinned commit (idempotent).
if [[ ! -d "$SUITE_DIR/.git" ]]; then
    echo ">> cloning runtime-tools into .cache/runtime-tools"
    git clone --quiet "$RUNTIME_TOOLS_REPO" "$SUITE_DIR"
fi

echo ">> checking out runtime-tools @ ${RUNTIME_TOOLS_COMMIT:0:12}"
git -C "$SUITE_DIR" fetch --quiet origin "$RUNTIME_TOOLS_COMMIT" 2>/dev/null || true
git -C "$SUITE_DIR" checkout --quiet "$RUNTIME_TOOLS_COMMIT"

# 3. Build the in-container helper and the requested test binaries.
echo ">> building runtimetest (static) + test binaries: $TESTS"
( cd "$SUITE_DIR" && CGO_ENABLED=0 go build -o runtimetest ./cmd/runtimetest )
for t in $TESTS; do
    src="$SUITE_DIR/validation/$t/$t.go"
    if [[ ! -f "$src" ]]; then
        echo "!! no such validation test: $t (looked for validation/$t/$t.go)" >&2
        exit 2
    fi
    ( cd "$SUITE_DIR" && go build -o "validation/$t/$t.t" "validation/$t/$t.go" )
done

# 4. Choose how to consume the tests' TAP. Each `.t` is a compiled executable
#    that emits TAP itself, so the simplest, dependency-free path is to run them
#    directly and tally ok/not-ok from the output — that's the default
#    ('direct'). RUNNER=prove uses Perl's TAP::Harness instead (heads-up: its
#    strict YAMLish parser flags a spurious FAIL on tap-go's `{...}` diagnostic
#    blocks even when the assertions pass). node-tap is intentionally unsupported.
runner="${RUNNER:-direct}"
logdir="$REPO_ROOT/.cache/validation-logs"
mkdir -p "$logdir"

# 5. Run. Must run from the suite root so the .t binaries find rootfs-*.tar.gz.
echo ">> running tests against RUNTIME=$BOX_BIN via '$runner'"
echo "   (expected to fail on a rootless host — see docs/oci-validation.md §5)"
echo
cd "$SUITE_DIR"
export RUNTIME="$BOX_BIN"

if [[ "$runner" == "prove" ]]; then
    paths=(); for t in $TESTS; do paths+=("validation/$t/$t.t"); done
    # Failures are expected during the conformance epic; don't abort the script.
    prove -v "${paths[@]}" || true
    echo
    echo ">> done — the suite ran against box (see TAP above for pass/fail)"
    exit 0
fi

# 'direct': run each test, tee its TAP to a log, then summarise.
pass_total=0 fail_total=0 err_total=0
for t in $TESTS; do
    log="$logdir/$t.log"
    echo "# ── $t ──────────────────────────────"
    # A failing test exits non-zero; capture that via PIPESTATUS without letting
    # `set -e` abort the loop (drop errexit just around the run).
    set +e
    "./validation/$t/$t.t" 2>&1 | tee "$log"
    rc=${PIPESTATUS[0]}
    set -e

    pass=$(grep -cE '^[[:space:]]*ok ' "$log" || true)
    fail=$(grep -cE '^[[:space:]]*not ok ' "$log" || true)

    # A non-zero exit with no "not ok" line means the test crashed before (or
    # without) emitting TAP — e.g. `create` failing on a rootless host. Count it
    # as an error so a hard failure isn't silently read as "0 failures" (the bug
    # in a plain `grep "not ok" | wc -l` approach).
    status="ok"
    if [[ "$rc" -ne 0 && "$fail" -eq 0 ]]; then
        err_total=$((err_total + 1)); status="ERROR (exit $rc, no TAP emitted)"
    elif [[ "$fail" -gt 0 ]]; then
        status="FAIL"
    fi
    pass_total=$((pass_total + pass)); fail_total=$((fail_total + fail))
    echo "#   → $t: ${pass} passed, ${fail} failed  [${status}]"
    echo
done

echo "────────────────────────────────────────────"
echo ">> summary: ${pass_total} passed, ${fail_total} failed, ${err_total} errored"
echo "   logs in ${logdir#"$REPO_ROOT"/}/<test>.log"
# Non-fatal: the suite is expected to be partially red during the epic.
exit 0
