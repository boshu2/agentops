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
check "SKILL.md has name: gc-membrane" "grep -q '^name: gc-membrane' '$SKILL_DIR/SKILL.md'"
check "constraints preserve canonical source" "grep -q '^## Constraints' '$SKILL_DIR/SKILL.md' && grep -q 'packs/agentops-membrane/' '$SKILL_DIR/SKILL.md'"
check "constraints fail closed" "grep -qi 'Fail closed.*quest stays open' '$SKILL_DIR/SKILL.md'"
check "constraints forbid automatic land" "grep -qi 'Never auto-merge or auto-push' '$SKILL_DIR/SKILL.md'"
check "output specification is complete" "grep -q '^## Output Specification' '$SKILL_DIR/SKILL.md' && grep -q '\*\*Filename:\*\*' '$SKILL_DIR/SKILL.md' && grep -qi 'validation command' '$SKILL_DIR/SKILL.md' && grep -qi 'downstream handoff' '$SKILL_DIR/SKILL.md'"
check "output separates terminal and degraded schemas" "grep -q 'pawl-verdict.v1' '$SKILL_DIR/SKILL.md' && grep -q 'gc-review-attempt.v1' '$SKILL_DIR/SKILL.md'"
check "terminal example passes canonical JSON Schema" "bash '$SKILL_DIR/scripts/validate-terminal-example.sh'"
check "nonce guidance uses lane input field" "grep -q \"input lane's.*agentops_nonce\" '$SKILL_DIR/SKILL.md' && grep -q 'Terminal refuters do not invent.*nonce_echo' '$SKILL_DIR/SKILL.md'"
check "filenames distinguish raw and copied evidence" "grep -q 'raw inputs use.*lane-<family>-round-<N>.json' '$SKILL_DIR/SKILL.md' && grep -q 'evidence-round-<N>/lane-<index>.json' '$SKILL_DIR/SKILL.md' && grep -q 'review-attempt-round-<N>.json' '$SKILL_DIR/SKILL.md'"
check "quality checklist has three rules" "awk '/^## Quality Checklist/{f=1;next} f&&/^## /{exit} f&&/^- /{n++} END{exit !(n>=3)}' '$SKILL_DIR/SKILL.md'"
check "kernel stays within 250 lines" "[ \$(wc -l < '$SKILL_DIR/SKILL.md') -le 250 ]"

echo
echo "Results: $PASS passed, $FAIL failed"
(( FAIL == 0 ))
