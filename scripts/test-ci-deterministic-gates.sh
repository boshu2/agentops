#!/usr/bin/env bash
# scripts/test-ci-deterministic-gates.sh — local CI-equivalent dry-run.
#
# Runs the deterministic CI gates that have historically surprised PRs at push
# time (registry-check, skill-lint, heal --strict) in a single non-fail-fast
# batch. Reports all failures together so you can
# fix them in one diagnostic round instead of N push iterations.
#
# Filed as soc-ws40 in the 2026-05-07 CI-push-gate-toil retrospective. See
# .agents/learnings/2026-05-07-ci-push-gate-toil-pattern.md for the data.
#
# Usage:
#   bash scripts/test-ci-deterministic-gates.sh             # full surface
#   bash scripts/test-ci-deterministic-gates.sh -q          # quiet (rc only)
#
# Exit:
#   0  all gates pass
#   1  one or more gates fail (each failure prints diagnostic + final summary)
#   2  argument or environment error
#
# Pairs with `ao gate check --fast`: that path runs changed-scope, this one runs
# the deterministic-only surface. Run both before push when changes touch
# skills/, schemas/ or registry-input paths.

set -euo pipefail

QUIET=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    -q|--quiet) QUIET=1; shift ;;
    -h|--help)
      sed -n '2,20p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ANSI colors only when stdout is a TTY.
if [[ -t 1 ]]; then
  GREEN='\033[0;32m'; RED='\033[0;31m'; RESET='\033[0m'
else
  GREEN=''; RED=''; RESET=''
fi

declare -a FAILURES=()
declare -a PASSES=()

log() {
  [[ "$QUIET" == 1 ]] && return 0
  printf '%b\n' "$*"
}

run_gate() {
  local name="$1"; shift
  local cmd=("$@")
  local out rc
  if out=$("${cmd[@]}" 2>&1); then
    rc=0
  else
    rc=$?
  fi
  if [[ "$rc" -eq 0 ]]; then
    PASSES+=("$name")
    log "${GREEN}  ok${RESET}  $name"
  else
    FAILURES+=("$name")
    log "${RED}FAIL${RESET}  $name (exit $rc)"
    if [[ "$QUIET" != 1 ]]; then
      printf '%s\n' "$out" | sed 's/^/    /'
    fi
  fi
}

log "=== CI-deterministic gates (local dry-run) ==="
log "REPO_ROOT=$REPO_ROOT"
log ""

# Gate 1: registry-check (post-soc-k47k: deterministic across local/CI).
run_gate "skill-mesh-check" python3 scripts/generate-skill-mesh.py --check

# Gate 2: skill-lint suite.
run_gate "skill-lint" bash tests/skills/lint-skills.sh

# Gate 3: heal.sh --strict (catches dead refs, unlinked refs, name mismatches).
run_gate "heal --strict" bash skills/skill-builder/scripts/heal.sh --strict

log ""
log "=== Summary ==="
log "Passed: ${#PASSES[@]}"
log "Failed: ${#FAILURES[@]}"
if [[ "${#FAILURES[@]}" -gt 0 ]]; then
  log ""
  log "${RED}FAIL gates:${RESET} ${FAILURES[*]}"
  log ""
  log "Fix locally before pushing — these gates run in CI and will block the PR."
  exit 1
fi
log "${GREEN}All deterministic CI gates pass.${RESET}"
exit 0
