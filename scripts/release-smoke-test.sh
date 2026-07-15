#!/usr/bin/env bash
# Release smoke for the retained AgentOps 4 CLI surface.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AO="$REPO_ROOT/cli/bin/ao"
SKIP_BUILD=false

usage() {
  cat <<'EOF'
Usage: bash scripts/release-smoke-test.sh [--skip-build]

Build the default ao binary, execute every generated leaf help path, exercise
the retained read-only surface, and prove removed commands are inert failures.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-build) SKIP_BUILD=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PASS=0
FAIL=0
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ao-release-smoke.XXXXXX")"
trap 'rm -rf "$TMP_ROOT"' EXIT

pass() { printf 'PASS: %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=$((FAIL + 1)); }

run_ok() {
  local label="$1"
  shift
  local output rc=0
  output=$("$@" 2>&1) || rc=$?
  if [[ "$rc" -eq 0 ]] && ! grep -Eq '(^panic:|runtime error:|^goroutine [0-9]+ \[)' <<<"$output"; then
    pass "$label"
  else
    fail "$label (exit $rc)"
    sed -n '1,8p' <<<"$output" >&2
  fi
}

run_json() {
  local label="$1"
  shift
  local output rc=0
  output=$("$@" 2>"$TMP_ROOT/stderr") || rc=$?
  if [[ "$rc" -eq 0 ]] && jq -e . >/dev/null 2>&1 <<<"$output"; then
    pass "$label"
  else
    fail "$label (exit $rc or invalid JSON)"
    sed -n '1,8p' "$TMP_ROOT/stderr" >&2
  fi
}

run_tombstone() {
  local label="$1"
  shift
  local output rc=0 lines
  output=$("$@" 2>&1) || rc=$?
  lines=$(awk 'NF { count++ } END { print count+0 }' <<<"$output")
  if [[ "$rc" -ne 0 && "$lines" -eq 1 ]] && grep -qiE 'removed|no longer' <<<"$output"; then
    pass "$label"
  else
    fail "$label (expected one-line nonzero tombstone, got exit $rc and $lines lines)"
    sed -n '1,8p' <<<"$output" >&2
  fi
}

if [[ "$SKIP_BUILD" == "false" ]]; then
  run_ok "build default ao binary" bash -c "cd '$REPO_ROOT/cli' && go build -o bin/ao ./cmd/ao"
fi
[[ -x "$AO" ]] || { echo "missing binary: $AO" >&2; exit 1; }

run_ok "root help" "$AO" --help
run_json "version JSON" "$AO" version --json
run_json "capabilities JSON" "$AO" capabilities --json
run_ok "ao quick-start dry-run" "$AO" quick-start --dry-run
run_ok "all generated leaf help paths" bash "$REPO_ROOT/tests/cli/test-all-leaf-help-smoke.sh" --skip-build --binary "$AO"

run_json "status JSON" "$AO" status --json
run_json "skills list JSON" "$AO" skills list --json
run_json "skills graph JSON" "$AO" skills graph --format json
run_json "flywheel status JSON" "$AO" flywheel status --json
run_json "goals validate JSON" "$AO" goals validate --json
run_json "provenance list JSON" bash -c "cd '$TMP_ROOT' && '$AO' provenance list --json"
run_json "source-link dry-run JSON" env HOME="$TMP_ROOT/home" "$AO" skills link --dest "$TMP_ROOT/skills" --dry-run --json

for command in \
  claim close constraint converge crank done governor land membrane next-work \
  pawl plan-pawl reconcile state validate worktree yield; do
  run_tombstone "ao $command tombstone" "$AO" "$command"
done
run_tombstone "ao goals trace tombstone" "$AO" goals trace
run_tombstone "ao session memory tombstone" "$AO" session memory
run_tombstone "ao skills edit tombstone" "$AO" skills edit

printf '\nRelease smoke: %d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
