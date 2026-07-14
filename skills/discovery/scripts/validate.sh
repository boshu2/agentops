#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
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

check "SKILL.md exists" "test -f '$SKILL_DIR/SKILL.md'"
check "frontmatter name" "grep -q '^name: discovery' '$SKILL_DIR/SKILL.md'"
check "research delegated" "grep -q '^3\. \*\*Research' '$SKILL_DIR/SKILL.md'"
check "Plan owns exact plan" "grep -q '^5\. \*\*Plan' '$SKILL_DIR/SKILL.md'"
check "Premortem owns exact-plan verdict" "grep -q '^6\. \*\*Premortem' '$SKILL_DIR/SKILL.md' && grep -q 'premortem-plan-verdict.v1' '$SKILL_DIR/references/dag.md'"
check "idea challenge is advisory" "grep -q 'advisory evidence' '$SKILL_DIR/SKILL.md' && grep -q 'Optional advisory idea challenge' '$SKILL_DIR/references/dag.md'"
check "legacy planning authorities absent" "! rg -q 'ApprovalEdge|Fable|duel_verdict_dir|duel_decision|ao plan-pawl|plan-pawl duel|phase-budgets\.md' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references'"
check "no private planning controller" "! rg -q 'attempts:|max-rounds|retry budget:|MAX_.*ATTEMPT|HELPER-UNSTUCK' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/dag.md'"
check "prospective handoff is honest" "grep -q 'packet_state.*prospective' '$SKILL_DIR/references/output-templates.md' && grep -q '\"validate\": \"not_checked\"' '$SKILL_DIR/references/output-templates.md'"
check "phase budget owner deleted" "test ! -e '$SKILL_DIR/references/phase-budgets.md'"

echo
echo "Results: $PASS passed, $FAIL failed"
test "$FAIL" -eq 0
