#!/usr/bin/env bash
# Functional subprocess smoke for the retained AgentOps 3.3 CLI.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ao-cli-integration.XXXXXX")"
AO="$TMP_ROOT/ao"
PASS=0
FAIL=0
trap 'rm -rf "$TMP_ROOT"' EXIT

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1" >&2; FAIL=$((FAIL + 1)); }

if (cd "$REPO_ROOT/cli" && go build -o "$AO" ./cmd/ao); then
  pass "build"
else
  fail "build"
  exit 1
fi

check_ok() {
  local label="$1"
  shift
  local output rc=0
  output=$("$@" 2>&1) || rc=$?
  if [[ "$rc" -eq 0 && -n "$output" ]] && ! grep -Eq '(^panic:|runtime error:)' <<<"$output"; then
    pass "$label"
  else
    fail "$label (exit $rc)"
    sed -n '1,5p' <<<"$output" >&2
  fi
}

check_json() {
  local label="$1"
  shift
  local output rc=0
  output=$("$@" 2>&1) || rc=$?
  if [[ "$rc" -eq 0 ]] && jq -e . >/dev/null 2>&1 <<<"$output"; then
    pass "$label"
  else
    fail "$label (exit $rc or invalid JSON)"
  fi
}

check_tombstone() {
  local label="$1"
  shift
  local output rc=0
  output=$("$@" 2>&1) || rc=$?
  if [[ "$rc" -ne 0 ]] && grep -qiE 'removed|no longer' <<<"$output"; then
    pass "$label"
  else
    fail "$label (expected removed-command failure)"
  fi
}

check_ok "root help" "$AO" --help
check_json "version" "$AO" version --json
check_json "capabilities" "$AO" capabilities --json
check_json "status" bash -c "cd '$TMP_ROOT' && '$AO' status --json"
check_json "provenance list" bash -c "cd '$TMP_ROOT' && '$AO' provenance list --json"
check_json "skills list" bash -c "cd '$REPO_ROOT' && '$AO' skills list --json"
check_json "skills link dry-run" env HOME="$TMP_ROOT/home" "$AO" skills link --dest "$TMP_ROOT/linked" --dry-run --json
check_ok "gate help" "$AO" gate check --help
check_ok "goals help" "$AO" goals --help
check_ok "session help" "$AO" session --help
check_tombstone "semantic validate tombstone" "$AO" validate
check_tombstone "delivery tombstone" "$AO" land

echo "Integration smoke: $PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]]
