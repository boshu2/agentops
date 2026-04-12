#!/usr/bin/env bash
# Smoke tests for skills/swarm/scripts/ol-wave-loader.sh and ol-ratchet.sh
# Uses temp fixtures — no real ol binary needed.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OL_WAVE_LOADER="$REPO_ROOT/skills/swarm/scripts/ol-wave-loader.sh"
OL_RATCHET="$REPO_ROOT/skills/swarm/scripts/ol-ratchet.sh"

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

# ── Test 1: ol-wave-loader.sh with valid wave JSON → sorted tab-delimited output ──

test_wave_loader_valid() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a wave fixture with entries in non-sorted order
    cat > "$tmpdir/wave.json" <<'EOF'
{
  "wave": [
    {"id": "ol-10.1", "title": "Third task", "spec_path": "specs/third.md", "priority": 3},
    {"id": "ol-10.2", "title": "First task", "spec_path": "specs/first.md", "priority": 1},
    {"id": "ol-10.3", "title": "Second task", "spec_path": "specs/second.md", "priority": 2}
  ]
}
EOF

    local output rc=0
    output="$(bash "$OL_WAVE_LOADER" "$tmpdir/wave.json" 2>&1)" || rc=$?

    if [[ "$rc" -ne 0 ]]; then
        echo "  Expected exit 0, got $rc"
        echo "  Output: $output"
        return 1
    fi

    # Verify output is tab-delimited and sorted by priority
    local line1 line2 line3
    line1="$(echo "$output" | sed -n '1p')"
    line2="$(echo "$output" | sed -n '2p')"
    line3="$(echo "$output" | sed -n '3p')"

    # Expected order: priority 1, 2, 3 -> id ol-10.2, ol-10.3, ol-10.1
    # Output format: id\ttitle\tspec_path\tpriority
    local expected1=$'ol-10.2\tFirst task\tspecs/first.md\t1'
    local expected2=$'ol-10.3\tSecond task\tspecs/second.md\t2'
    local expected3=$'ol-10.1\tThird task\tspecs/third.md\t3'

    if [[ "$line1" != "$expected1" ]]; then
        echo "  Line 1 mismatch"
        echo "  Expected: $expected1"
        echo "  Got:      $line1"
        return 1
    fi
    if [[ "$line2" != "$expected2" ]]; then
        echo "  Line 2 mismatch"
        echo "  Expected: $expected2"
        echo "  Got:      $line2"
        return 1
    fi
    if [[ "$line3" != "$expected3" ]]; then
        echo "  Line 3 mismatch"
        echo "  Expected: $expected3"
        echo "  Got:      $line3"
        return 1
    fi
}

# ── Test 2: ol-wave-loader.sh with missing file → exits non-zero ──

test_wave_loader_missing_file() {
    local rc=0
    bash "$OL_WAVE_LOADER" "/tmp/nonexistent-wave-$$-$RANDOM.json" >/dev/null 2>&1 || rc=$?

    if [[ "$rc" -eq 0 ]]; then
        echo "  Expected non-zero exit, got 0"
        return 1
    fi
}

# ── Test 3: ol-ratchet.sh with stub ol → exits 0 ──

test_ratchet_stub() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Stub ol that succeeds for "hero ratchet"
    cat > "$tmpdir/ol" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
    chmod +x "$tmpdir/ol"

    local output rc=0
    output="$(PATH="$tmpdir:$PATH" bash "$OL_RATCHET" "ol-527.1" 2>&1)" || rc=$?

    if [[ "$rc" -ne 0 ]]; then
        echo "  Expected exit 0, got $rc"
        echo "  Output: $output"
        return 1
    fi

    if ! echo "$output" | grep -q "success"; then
        echo "  Output missing 'success'"
        echo "  Output: $output"
        return 1
    fi
}

# ── Run tests ──

echo "=== swarm ol-wave-loader.sh / ol-ratchet.sh smoke tests ==="
run_test "wave loader valid JSON -> sorted tab-delimited" test_wave_loader_valid
run_test "wave loader missing file -> non-zero exit"      test_wave_loader_missing_file
run_test "ol-ratchet with stub ol -> exit 0"              test_ratchet_stub

echo ""
if [[ "$failures" -gt 0 ]]; then
    echo "$failures test(s) FAILED"
    exit 1
else
    echo "All tests passed"
    exit 0
fi
