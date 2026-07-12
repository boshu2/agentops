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
    grep -Fq '**Checkpoint:**' "$skill_md" &&
    grep -Fq '**Artifact directory:**' "$skill_md" &&
    grep -Fq '**Filename convention:**' "$skill_md" &&
    grep -Fq '**Serialization/schema format:**' "$skill_md" &&
    grep -Fq '**Validator command:**' "$skill_md" &&
    grep -Fq '**Downstream handoff:**' "$skill_md" &&
    grep -Fq '## Quality Checklist' "$skill_md"
}

validate_dashboard_json() {
  jq -e '.schema_version==1 and (.generated_at|type)=="string" and (.current_work|type)=="object" and (.latest_gates|type)=="object" and (.next_action|type)=="object" and (.next_action.priority|type)=="number" and (.next_action.message|type)=="string" and (.coverage|type)=="array" and all(.coverage[]; (.source|type)=="string" and (.status=="available" or .status=="unavailable" or .status=="malformed"))' "$1" >/dev/null
}

validate_priority_order() {
  local negative_line resume_line
  negative_line="$(grep -n 'Recent WARN/FAIL/REFUTED verdict exists' "$SKILL_MD" | cut -d: -f1)"
  resume_line="$(grep -n 'Claimed/in-progress bead exists' "$SKILL_MD" | cut -d: -f1)"
  [[ -n "$negative_line" && -n "$resume_line" && "$negative_line" -lt "$resume_line" ]]
}

validate_feature() {
  local feature="$SKILL_DIR/references/status.feature"
  [[ -s "$feature" ]] &&
    grep -Fq 'Feature: Status renders resumable AgentOps truth' "$feature" &&
    grep -Fq 'unavailable is distinct from an empty result' "$feature" &&
    grep -Fq 'a negative verdict outranks ordinary continuation' "$feature" &&
    grep -Fq 'dashboard probes remain observational' "$feature"
}

record "SKILL.md exists" test -f "$SKILL_MD"
record "status contract is complete" validate_contract "$SKILL_MD"
record "dashboard contract reference exists" test -s "$SKILL_DIR/references/dashboard-contract.md"
record "status feature is current and packaged" validate_feature
record "Codex lifecycle mutation is bounded" grep -Fq 'The idempotent thread-start record written by `ao codex ensure-start` is the only permitted status-side mutation; all dashboard probes after it are read-only.' "$SKILL_MD"
record "unavailable is not empty" grep -Fq 'never render a missing source as healthy or empty' "$SKILL_MD"
record "negative verdict outranks ordinary resume" validate_priority_order
record "Codex execution profile exists" grep -Fq '## Codex Execution Profile' "$SKILL_MD"
record "Codex guardrails exist" grep -Fq '## Guardrails' "$SKILL_MD"
record "Codex startup guard is exact" grep -Fq 'In Codex hookless mode, run `ao codex ensure-start` before gathering dashboard state; the CLI records startup once per thread and skips duplicates automatically.' "$SKILL_MD"

pawl_fixture="$(mktemp)"
output_fixture="$(mktemp)"
valid_fixture="$(mktemp)"
invalid_fixture="$(mktemp)"
trap 'rm -f "$pawl_fixture" "$output_fixture" "$valid_fixture" "$invalid_fixture"' EXIT

sed 's/HELPER-UNSTUCK -> AUTO-REDO/HELPER-UNSTUCK -> MANUAL/' "$SKILL_MD" >"$pawl_fixture"
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

printf '%s\n' '{"schema_version":1,"generated_at":"2026-07-12T13:00:00Z","current_work":{"in_progress":[]},"latest_gates":{"verdicts":[]},"next_action":{"priority":8,"message":"Start research"},"coverage":[{"source":"git status","status":"available","detail":"exit 0"}]}' >"$valid_fixture"
printf '%s\n' '{"schema_version":1,"current_work":{},"latest_gates":{},"next_action":{"priority":"high","message":"guess"},"coverage":[{"source":"tracker","status":"healthy"}]}' >"$invalid_fixture"
record "JSON validator accepts complete dashboard" validate_dashboard_json "$valid_fixture"
if validate_dashboard_json "$invalid_fixture"; then
  echo "FAIL: JSON validator rejects incomplete or invented status"
  FAIL=$((FAIL + 1))
else
  echo "PASS: JSON validator rejects incomplete or invented status"
  PASS=$((PASS + 1))
fi

echo
echo "Results: $PASS passed, $FAIL failed"
(( FAIL == 0 ))
