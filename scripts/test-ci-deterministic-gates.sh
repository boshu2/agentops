#!/usr/bin/env bash
# scripts/test-ci-deterministic-gates.sh — local CI-equivalent dry-run.
#
# Runs the deterministic CI gates that have historically surprised PRs at push
# time (registry-check, skill-lint, heal --strict, codex artifact metadata)
# in a single non-fail-fast batch. Reports all failures together so you can
# fix them in one diagnostic round instead of N push iterations.
#
# Filed as soc-ws40 in the 2026-05-07 CI-push-gate-toil retrospective. See
# .agents/learnings/2026-05-07-ci-push-gate-toil-pattern.md for the data.
#
# Usage:
#   bash scripts/test-ci-deterministic-gates.sh             # full surface
#   bash scripts/test-ci-deterministic-gates.sh -q          # quiet (rc only)
#   bash scripts/test-ci-deterministic-gates.sh --skip-codex # skip codex parity
#
# Exit:
#   0  all gates pass
#   1  one or more gates fail (each failure prints diagnostic + final summary)
#   2  argument or environment error
#
# Pairs with scripts/pre-push-gate.sh: that script runs --fast, this one runs
# the deterministic-only surface. Run both before push when changes touch
# skills/, schemas/, registry-input paths, or codex-mirrored skills.

set -euo pipefail

QUIET=0
SKIP_CODEX=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    -q|--quiet) QUIET=1; shift ;;
    --skip-codex) SKIP_CODEX=1; shift ;;
    -h|--help)
      sed -n '2,21p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ANSI colors only when stdout is a TTY.
if [[ -t 1 ]]; then
  GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[0;33m'; RESET='\033[0m'
else
  GREEN=''; RED=''; YELLOW=''; RESET=''
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
run_gate "registry-check" bash scripts/generate-registry.sh --check

# Gate 2: skill-lint suite.
run_gate "skill-lint" bash tests/skills/lint-skills.sh

# Gate 3: heal-skill --strict (catches dead refs, unlinked refs, name mismatches).
run_gate "heal-skill --strict" bash skills/heal-skill/scripts/heal.sh --strict

# Gate 4: codex artifact metadata (skip with --skip-codex when iterating fast).
if [[ "$SKIP_CODEX" == 0 ]]; then
  if [[ -x scripts/refresh-codex-artifacts.sh ]]; then
    # --check-only path: validate without writing. The refresh script's
    # validation FAILS when source skills changed without matching codex
    # mirror updates. We invoke it with a workspace-preserving check.
    run_gate "codex artifact metadata" bash -c '
      bash scripts/refresh-codex-artifacts.sh --scope head 2>&1
      # If the refresh wrote any changes, the audit found drift — fail.
      if ! git diff --quiet -- skills-codex/; then
        echo "drift detected in skills-codex/ after refresh" >&2
        # Restore so we do not leave the workspace dirty when only running
        # gates. Operator should re-run the refresh and commit if real.
        git checkout -- skills-codex/ 2>/dev/null || true
        exit 1
      fi
    '
  else
    log "${YELLOW}WARN${RESET}  codex artifact metadata: refresh-codex-artifacts.sh not executable; skipped"
  fi
fi

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
