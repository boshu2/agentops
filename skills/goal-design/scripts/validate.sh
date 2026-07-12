#!/usr/bin/env bash
# shellcheck disable=SC2016 # Exact Markdown fixtures intentionally keep literal backticks.
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
PASS=0
FAIL=0

check() {
  local name="$1" cmd="$2"
  if bash -c "$cmd"; then
    echo "PASS: $name"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $name"
    FAIL=$((FAIL + 1))
  fi
}

check_exact_line() {
  local name="$1" expected="$2"
  if grep -Fqx -- "$expected" "$SKILL_DIR/SKILL.md"; then
    echo "PASS: $name"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $name"
    FAIL=$((FAIL + 1))
  fi
}

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "frontmatter name" "grep -q '^name: goal-design' '$SKILL_DIR/SKILL.md'"
check "helper documented" "grep -q 'scripts/goal-design-packet.py new' '$SKILL_DIR/SKILL.md'"
check "checker documented" "grep -q 'check-goal-design-packet.sh' '$SKILL_DIR/SKILL.md'"
check "independent validation required" "grep -qi 'independent validation' '$SKILL_DIR/SKILL.md'"
check "helper tests exist" "[ -f '$REPO_ROOT/tests/scripts/goal-design-packet.bats' ]"
check "constraints make packet canonical" "grep -q '^## Constraints' '$SKILL_DIR/SKILL.md' && grep -qi 'contract of record' '$SKILL_DIR/SKILL.md'"
check "constraints require checker before validation" "grep -qi 'checker before independent validation' '$SKILL_DIR/SKILL.md' && grep -q 'mark-validated' '$SKILL_DIR/SKILL.md'"
check_exact_line "constraints distinguish refutation from breaker trip" '- Route every plain `REFUTED` verdict directly through repeated `AUTO-REDO`; do not consult a helper or raise an andon unless the circuit breaker trips. A breaker trip enters `HOLD` and gets exactly one bounded helper consultation: `UNSTUCK` resumes the automatic loop, while `ESCALATE` reaches a human. Refusal-lane work, explicit judgment, and exhausted budgets go directly to a human because routine blockers are not andons.'
check_exact_line "REFUTED returns to AUTO-REDO" '| `REFUTED` | `AUTO-REDO` | Repair from the evidence and rerun validation; do not consult a helper or human. |'
check_exact_line "AUTO-REDO may be refuted again" '| `AUTO-REDO` | `REFUTED` | Repeat the automatic repair loop while the circuit breaker remains closed. |'
check_exact_line "breaker trip enters HOLD" '| `CIRCUIT-BREAKER-TRIP` | `HOLD` | Freeze mutation and preserve the blocker evidence. |'
check_exact_line "HOLD permits one helper" '| `HOLD` | `HELPER` | Run exactly one bounded helper consultation for this blocker class. |'
check_exact_line "UNSTUCK resumes AUTO-REDO" '| `HELPER-UNSTUCK` | `AUTO-REDO` | Apply the concrete next action and resume the automatic repair loop. |'
check_exact_line "helper ESCALATE reaches human" '| `HELPER-ESCALATE` | `HUMAN` | Hand back the preserved evidence and helper verdict. |'
check_exact_line "human-only classes skip helper" '| `REFUSAL-LANE / EXPLICIT-JUDGMENT / BUDGET-EXHAUSTED` | `HUMAN` | Skip the helper and ask the operator. |'
check_exact_line "plain REFUTED never holds or consults" 'A plain `REFUTED` verdict never enters `HOLD` and never invokes a helper. Never'
check "output specification is complete" "grep -q '^## Output Specification' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Filename:\*\*' '$SKILL_DIR/SKILL.md' && grep -qi 'validation command' '$SKILL_DIR/SKILL.md' && grep -qi 'downstream handoff' '$SKILL_DIR/SKILL.md'"
check "quality checklist has three rules" "awk '/^## Quality Checklist/{f=1;next} f&&/^## /{exit} f&&/^- /{n++} END{exit !(n>=3)}' '$SKILL_DIR/SKILL.md'"
check "kernel stays within 250 lines" "[ \$(wc -l < '$SKILL_DIR/SKILL.md') -le 250 ]"

echo
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
