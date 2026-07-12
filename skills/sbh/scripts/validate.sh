#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL_MD="$SKILL_DIR/SKILL.md"
PASS=0
FAIL=0

record() {
  local label="$1"
  shift
  if "$@"; then
    echo "PASS: $label"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $label"
    FAIL=$((FAIL + 1))
  fi
}

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
    grep -Fq '## Quality Checklist' "$skill_md"
}

record "SBH contract is complete" validate_contract "$SKILL_MD"
record "destructive commands require explicit authorization" grep -Fq 'explicit authorization' "$SKILL_MD"
record "same-mount ballast invariant is preserved" grep -Fq 'same mount' "$SKILL_MD"

pawl_fixture="$(mktemp)"
output_fixture="$(mktemp)"
trap 'rm -f "$pawl_fixture" "$output_fixture"' EXIT
awk '!/HELPER-UNSTUCK -> AUTO-REDO/' "$SKILL_MD" >"$pawl_fixture"
awk '!/\*\*Validator command:\*\*/' "$SKILL_MD" >"$output_fixture"

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

echo
echo "Results: $PASS passed, $FAIL failed"
(( FAIL == 0 ))
