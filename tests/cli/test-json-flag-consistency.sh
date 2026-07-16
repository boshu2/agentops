#!/usr/bin/env bash
# Verify the retained machine-readable CLI subset emits valid JSON.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
AO="$REPO_ROOT/cli/bin/ao"
TMP_BASE="${TMPDIR:-/tmp}"
mkdir -p "$TMP_BASE"
WORK_DIR="$(mktemp -d "$TMP_BASE/ao-json-flag-consistency.XXXXXX")"
STDERR_TMP="$WORK_DIR/stderr"
PASS=0
ERRORS=0
trap 'rm -rf "$WORK_DIR"' EXIT

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1" >&2; ERRORS=$((ERRORS + 1)); }

if [[ ! -x "$AO" ]]; then
  (cd "$REPO_ROOT/cli" && go build -o bin/ao ./cmd/ao)
fi

test_json_cmd() {
  local label="$1"
  shift
  local stdout rc=0
  stdout=$("$AO" "$@" --json 2>"$STDERR_TMP") || rc=$?
  if [[ "$rc" -eq 0 && -n "$stdout" ]] && jq -e . >/dev/null 2>&1 <<<"$stdout"; then
    pass "$label"
  else
    fail "$label (exit $rc, expected valid JSON)"
    sed -n '1,5p' "$STDERR_TMP" >&2
  fi
}

echo "=== JSON Flag Consistency Tests ==="
test_json_cmd "ao version" version
test_json_cmd "ao capabilities" capabilities
test_json_cmd "ao config --show" config --show
test_json_cmd "ao status" status
test_json_cmd "ao skills list" skills list
test_json_cmd "ao skills graph" skills graph --format json
test_json_cmd "ao goals validate" goals validate
test_json_cmd "ao flywheel status" flywheel status
test_json_cmd "ao provenance list" provenance list

json_flag=$("$AO" config --show --json 2>/dev/null)
output_flag=$("$AO" config --show -o json 2>/dev/null)
if [[ "$json_flag" == "$output_flag" ]]; then
  pass "--json equals -o json"
else
  fail "--json differs from -o json"
fi

echo "=== JSON Flag Consistency ==="
echo "Passed: $PASS  Errors: $ERRORS"
[[ "$ERRORS" -eq 0 ]]
