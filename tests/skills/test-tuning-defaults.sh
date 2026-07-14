#!/usr/bin/env bash
# Planning-review defaults after the S2 direct cut.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
passed=0
failed=0

check() {
  local label="$1"
  shift
  if "$@"; then
    printf '  PASS: %s\n' "$label"
    passed=$((passed + 1))
  else
    printf '  FAIL: %s\n' "$label"
    failed=$((failed + 1))
  fi
}

contains() { grep -Fq -- "$2" "$1"; }
absent() { ! rg -q -- "$2" "${@:3}"; }

PREMORTEM="$REPO_ROOT/skills/premortem/SKILL.md"
DISCOVERY="$REPO_ROOT/skills/discovery"
GOAL_DESIGN="$REPO_ROOT/skills/goal-design/SKILL.md"
DUELING="$REPO_ROOT/skills/dueling-idea-genies/SKILL.md"

check "Premortem defaults to one fresh judge" contains "$PREMORTEM" 'Use one fresh-context judge'
check "Premortem verdict is binary" contains "$PREMORTEM" 'Emit exactly `PASS` or `FAIL`'
check "Premortem requires author/judge separation" contains "$PREMORTEM" 'author_id != judge_id'
check "model family is optional" contains "$PREMORTEM" 'Model and family'
check "Premortem owns no phase controller" absent 'unused' 'max-rounds|MAX_.*ATTEMPT|helper consultation|phase budget' "$PREMORTEM"
check "Discovery phase budget owner is deleted" test ! -e "$DISCOVERY/references/phase-budgets.md"
check "Discovery has no alternate readiness authority" absent 'unused' 'ApprovalEdge|Fable|ao plan-pawl|duel_verdict_dir' "$DISCOVERY/SKILL.md" "$DISCOVERY/references"
check "Goal Design is deterministic only" contains "$GOAL_DESIGN" 'Goal Design checks packet shape'
check "Dueling evidence routes to Plan" contains "$DUELING" 'advisory evidence for Plan'
check "Dueling emits no readiness" contains "$DUELING" 'Emit no readiness'

printf '\nPassed: %d\nFailed: %d\n' "$passed" "$failed"
if (( failed > 0 )); then
  echo 'OVERALL: FAIL'
  exit 1
fi
echo 'OVERALL: PASS'
