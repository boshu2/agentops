#!/usr/bin/env bash
# test-json-flag-consistency.sh — Verify --json flag produces valid JSON on ao commands
# Part of na-8ar.3
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
AO="$REPO_ROOT/cli/bin/ao"

ERRORS=0
PASSED=0
TMP_BASE="${TMPDIR:-/tmp}"
mkdir -p "$TMP_BASE"
WORK_DIR=$(mktemp -d "$TMP_BASE/ao-json-flag-consistency.XXXXXX")
STDERR_TMP="$WORK_DIR/stderr"

cleanup() {
  if [[ -n "${WORK_DIR:-}" && -d "${WORK_DIR:-}" ]]; then
    rm -rf "$WORK_DIR"
  fi
}

trap 'cleanup' EXIT

pass() { echo -e "\033[0;32m✓\033[0m $1"; PASSED=$((PASSED + 1)); }
fail() { echo -e "\033[0;31m✗\033[0m $1"; ERRORS=$((ERRORS + 1)); }

# ── Pre-flight: ensure ao binary exists ──────────────────────────────
if [[ ! -x "$AO" ]]; then
  echo "ao binary not found at $AO — attempting build..."
  if ! (cd "$REPO_ROOT/cli" && make build 2>&1); then
    echo "FATAL: could not build ao binary"
    exit 2
  fi
  if [[ ! -x "$AO" ]]; then
    echo "FATAL: ao binary still missing after build"
    exit 2
  fi
fi

echo "=== JSON Flag Consistency Tests ==="
echo "Binary: $AO"
echo ""

# ── Helper: test a command with --json ───────────────────────────────
# Usage: test_json_cmd <label> <ao-args...>
# Runs the command, checks:
#   1. --json flag is accepted (not "unknown flag")
#   2. stdout is valid JSON (via jq empty)
test_json_cmd() {
  local label="$1"; shift
  local stdout stderr rc

  # Capture stdout and stderr separately
  stdout=$("$AO" "$@" --json 2>"$STDERR_TMP") && rc=$? || rc=$?
  stderr=$(cat "$STDERR_TMP" 2>/dev/null || true)

  # Check if --json flag was rejected as unknown
  if echo "$stderr" | grep -qi "unknown flag.*--json"; then
    fail "$label — --json flag not recognized"
    return
  fi

  if [[ $rc -ne 0 ]]; then
    fail "$label — exited $rc with --json"
    return
  fi

  # Empty stdout: command ran but produced no output
  if [[ -z "$stdout" ]]; then
    fail "$label — exited 0 but produced no JSON output"
    return
  fi

  # Validate JSON
  if echo "$stdout" | jq empty 2>/dev/null; then
    pass "$label"
  else
    fail "$label — produced non-JSON output despite --json flag"
  fi
}

# ── Tests: commands that should produce structured output ────────────

echo "--- Core commands ---"
test_json_cmd "ao version"          version
test_json_cmd "ao config --show"    config --show
test_json_cmd "ao status"           status
test_json_cmd "ao doctor"           doctor
test_json_cmd "ao badge"            badge

echo ""
echo "--- Hook commands ---"
test_json_cmd "ao hooks show"       hooks show

echo ""
echo "--- Knowledge commands ---"
test_json_cmd "ao inject"           inject
test_json_cmd "ao search 'test'"    search "test"
test_json_cmd "ao anti-patterns"    anti-patterns
test_json_cmd "ao dedup"            dedup
test_json_cmd "ao contradict"       contradict
test_json_cmd "ao findings list"    findings list
test_json_cmd "ao findings stats"   findings stats

echo ""
echo "--- Pool / Flywheel commands ---"
test_json_cmd "ao pool list"        pool list
test_json_cmd "ao flywheel status"  flywheel status

echo ""
echo "--- Output flag equivalence (--json vs -o json) ---"
# Verify --json and -o json produce identical output on a stable command
json_flag=$("$AO" config --show --json 2>/dev/null) || true
o_flag=$("$AO" config --show -o json 2>/dev/null) || true

if [[ -n "$json_flag" && -n "$o_flag" ]]; then
  if [[ "$json_flag" == "$o_flag" ]]; then
    pass "--json and -o json produce identical output (config --show)"
  else
    fail "--json and -o json produce DIFFERENT output (config --show)"
  fi
else
  fail "--json/-o json equivalence test skipped (one or both produced no output)"
fi

# ── Summary ──────────────────────────────────────────────────────────
echo ""
echo "=== JSON Flag Consistency ==="
echo "Passed: $PASSED  Errors: $ERRORS"

exit $((ERRORS > 0 ? 1 : 0))
