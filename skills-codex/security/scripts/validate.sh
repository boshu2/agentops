#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

validate_contract() {
  local skill_md="$1"
  [[ "$(awk '/^---$/{n++;next} n==2 && /^## /{print;exit}' "$skill_md")" == "## Critical Constraints" ]] &&
    grep -Fq 'WARN|FAIL|REFUTED -> AUTO-REDO' "$skill_md" &&
    grep -Fq 'BREAKER -> HOLD -> ONE-HELPER' "$skill_md" &&
    grep -Fq 'HELPER-UNSTUCK -> AUTO-REDO' "$skill_md" &&
    grep -Fq 'HELPER-ESCALATE -> HUMAN' "$skill_md" &&
    grep -Fq 'REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN' "$skill_md" &&
    grep -Fq '**Artifact directory:**' "$skill_md" &&
    grep -Fq '**Filename convention:**' "$skill_md" &&
    grep -Fq '**Serialization/schema format:**' "$skill_md" &&
    grep -Fq '**Validator command:**' "$skill_md" &&
    grep -Fq '**Downstream handoff:**' "$skill_md" &&
    grep -Fq '## Quality Checklist' "$skill_md" &&
    grep -Fq '**Checkpoint:**' "$skill_md"
}
export -f validate_contract

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "security_suite.py exists" "[ -f '$SKILL_DIR/scripts/security_suite.py' ]"
check "security_suite.py compiles" "python3 -m py_compile '$SKILL_DIR/scripts/security_suite.py'"
check "prompt_redteam.py exists" "[ -f '$SKILL_DIR/scripts/prompt_redteam.py' ]"
check "prompt_redteam.py compiles" "python3 -m py_compile '$SKILL_DIR/scripts/prompt_redteam.py'"
check "suite runbook exists" "[ -s '$SKILL_DIR/references/security-suite-runbook.md' ]"
check "security contract is complete" "validate_contract '$SKILL_DIR/SKILL.md'"

pawl_fixture="$(mktemp)"
output_fixture="$(mktemp)"
trap 'rm -f "$pawl_fixture" "$output_fixture"' EXIT
sed 's/HELPER-UNSTUCK -> AUTO-REDO/HELPER-UNSTUCK -> MANUAL/' "$SKILL_DIR/SKILL.md" >"$pawl_fixture"
awk '!/\*\*Validator command:\*\*/' "$SKILL_DIR/SKILL.md" >"$output_fixture"
if validate_contract "$pawl_fixture"; then
  echo "FAIL: deletion fixture rejects missing pawl transition"
  FAIL=$((FAIL + 1))
else
  echo "PASS: deletion fixture rejects missing pawl transition"
  PASS=$((PASS + 1))
fi
if validate_contract "$output_fixture"; then
  echo "FAIL: deletion fixture rejects incomplete output handoff"
  FAIL=$((FAIL + 1))
else
  echo "PASS: deletion fixture rejects incomplete output handoff"
  PASS=$((PASS + 1))
fi

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
