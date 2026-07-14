#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
PASS=0
FAIL=0

check() {
  if bash -c "$2"; then
    echo "PASS: $1"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $1"
    FAIL=$((FAIL + 1))
  fi
}

check "SKILL.md exists" "test -f '$SKILL'"
check "frontmatter name" "grep -q '^name: goal-design' '$SKILL'"
check "helper documented" "grep -q 'scripts/goal-design-packet.py new' '$SKILL'"
check "checker documented" "grep -q 'check-goal-design-packet.sh' '$SKILL'"
check "packet is canonical" "grep -q '^## Constraints' '$SKILL' && grep -qi 'contract of record' '$SKILL'"
check "deterministic-only boundary" "grep -q 'Goal Design checks packet shape' '$SKILL' && grep -q 'Premortem owns the' '$SKILL'"
check "semantic transition removed" "! rg -q '/validate|mark-validated|Last validation verdict|cross-family|council helper|AUTO-REDO|HELPER-UNSTUCK' '$SKILL' '$REPO_ROOT/scripts/goal-design-packet.py' '$REPO_ROOT/scripts/check-goal-design-packet.sh'"
check "packet tests exist" "test -f '$REPO_ROOT/tests/scripts/goal-design-packet.bats' && test -f '$REPO_ROOT/tests/scripts/check-goal-design-packet.bats'"
check "output specification complete" "grep -q '^## Output Specification' '$SKILL' && grep -q '\*\*Filename:\*\*' '$SKILL' && grep -q '\*\*Validation command:\*\*' '$SKILL' && grep -q '\*\*Downstream handoff:\*\*' '$SKILL'"
check "kernel stays within 250 lines" "test \$(wc -l < '$SKILL') -le 250"

echo
echo "Results: $PASS passed, $FAIL failed"
test "$FAIL" -eq 0
