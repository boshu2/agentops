#!/usr/bin/env bash
# Smoke tests for skills/vibe/scripts/ol-validate.sh
# Uses fixture data — no real ol binary needed.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OL_VALIDATE="$REPO_ROOT/skills/vibe/scripts/ol-validate.sh"
PASS_FIXTURE="$REPO_ROOT/tests/fixtures/stage1-result-pass.json"
FAIL_FIXTURE="$REPO_ROOT/tests/fixtures/stage1-result-fail.json"

failures=0
_cleanup_dirs=()

cleanup() {
    for d in "${_cleanup_dirs[@]}"; do
        rm -rf "$d"
    done
}
trap cleanup EXIT

# Helper: create a temp dir that will be cleaned up at exit
make_tmpdir() {
    local d
    d="$(mktemp -d)"
    _cleanup_dirs+=("$d")
    echo "$d"
}

# Helper: run a named test, track pass/fail
run_test() {
    local name="$1"
    shift
    if "$@"; then
        echo "PASS: $name"
    else
        echo "FAIL: $name"
        ((failures++)) || true
    fi
}

# ── Test 1: ol returns pass fixture → exits 0, output contains PASSED ──

test_pass_fixture() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create .ol/config.yaml so the script detects OL
    mkdir -p "$tmpdir/.ol"
    echo "project: test" > "$tmpdir/.ol/config.yaml"

    # Stub ol: return pass fixture for "validate stage1"
    cat > "$tmpdir/ol" <<'STUB'
#!/usr/bin/env bash
cat "$OL_FIXTURE"
STUB
    chmod +x "$tmpdir/ol"

    # Run ol-validate.sh from the temp dir with our stub on PATH
    local output rc=0
    output="$(cd "$tmpdir" && OL_FIXTURE="$PASS_FIXTURE" PATH="$tmpdir:$PATH" bash "$OL_VALIDATE" 2>&1)" || rc=$?

    if [[ "$rc" -ne 0 ]]; then
        echo "  Expected exit 0, got $rc"
        echo "  Output: $output"
        return 1
    fi

    if ! echo "$output" | grep -q "PASSED"; then
        echo "  Output missing 'PASSED'"
        echo "  Output: $output"
        return 1
    fi
}

# ── Test 2: ol returns fail fixture → exits 1, output contains FAILED ──

test_fail_fixture() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    mkdir -p "$tmpdir/.ol"
    echo "project: test" > "$tmpdir/.ol/config.yaml"

    cat > "$tmpdir/ol" <<'STUB'
#!/usr/bin/env bash
cat "$OL_FIXTURE"
STUB
    chmod +x "$tmpdir/ol"

    local output rc=0
    output="$(cd "$tmpdir" && OL_FIXTURE="$FAIL_FIXTURE" PATH="$tmpdir:$PATH" bash "$OL_VALIDATE" 2>&1)" || rc=$?

    if [[ "$rc" -ne 1 ]]; then
        echo "  Expected exit 1, got $rc"
        echo "  Output: $output"
        return 1
    fi

    if ! echo "$output" | grep -q "FAILED"; then
        echo "  Output missing 'FAILED'"
        echo "  Output: $output"
        return 1
    fi
}

# ── Test 3: ol not found → exits 2 (skip) ──

test_ol_not_found() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # No .ol/config.yaml, no ol binary on PATH
    # Keep essential paths so bash/jq remain available, but exclude any real ol
    local safe_path="/usr/bin:/bin:/usr/sbin:/sbin"
    local output rc=0
    output="$(cd "$tmpdir" && PATH="$safe_path" bash "$OL_VALIDATE" 2>&1)" || rc=$?

    if [[ "$rc" -ne 2 ]]; then
        echo "  Expected exit 2, got $rc"
        echo "  Output: $output"
        return 1
    fi
}

# ── Run tests ──

echo "=== vibe ol-validate.sh smoke tests ==="
run_test "pass fixture -> exit 0, PASSED" test_pass_fixture
run_test "fail fixture -> exit 1, FAILED" test_fail_fixture
run_test "ol not found -> exit 2 (skip)"  test_ol_not_found

echo ""
if [[ "$failures" -gt 0 ]]; then
    echo "$failures test(s) FAILED"
    exit 1
else
    echo "All tests passed"
    exit 0
fi
