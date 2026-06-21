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
# Requires: go (1.19+), git, tar, and a TAP consumer (`prove` from Perl, or
# node-tap's `tap`). Run on Linux; use `sudo -E` for tests that need root.

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

# 4. Pick a TAP consumer: explicit $RUNNER, else node-tap `tap`, else `prove`,
#    else run the .t binaries directly (they emit TAP themselves).
#    NB: probe that the tool actually RUNS, not just that it's on PATH — a
#    version-manager shim (e.g. mise) can put a `tap` on PATH that errors out
#    ("Cannot find package 'tap'") because the package isn't really installed.
runner="${RUNNER:-}"
if [[ -z "$runner" ]]; then
    if tap --version >/dev/null 2>&1; then runner="tap"
    elif prove --version >/dev/null 2>&1; then runner="prove"
    else runner="direct"; fi
fi

# 5. Run. Must run from the suite root so the .t binaries find rootfs-*.tar.gz.
echo ">> running tests against RUNTIME=$BOX_BIN via '$runner'"
echo "   (expected to fail on a rootless host — see docs/oci-validation.md §5)"
echo
test_paths=()
for t in $TESTS; do test_paths+=("validation/$t/$t.t"); done

cd "$SUITE_DIR"
export RUNTIME="$BOX_BIN"
# Test failures are expected during the conformance epic, so don't let a red
# suite fail this script — it has done its job once the tests have run.
case "$runner" in
    tap)    tap "${test_paths[@]}" || true ;;
    prove)  prove -v "${test_paths[@]}" || true ;;
    direct) for p in "${test_paths[@]}"; do echo "# $p"; "./$p" || true; done ;;
esac

echo
echo ">> done — the suite ran against box (see TAP above for pass/fail)"
