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

validate_status() {
  jq -e '.schema_version==1 and (.frozen_dirs|type=="array" and all(.[]; type=="string" and length>0)) and (.acquired_at|type=="string" and length>0) and (.acquired_by|type=="string")' >/dev/null
}

roundtrip_status() {
  AO_SCOPE_LOCK="$1" "$2" scope status --json | validate_status
}

roundtrip_clear() {
  AO_SCOPE_LOCK="$1" "$2" scope status --json | jq -e '.schema_version==1 and .frozen_dirs==[]' >/dev/null
}

record "scope contract is complete" validate_contract "$SKILL_MD"
record "scope expansion requires explicit judgment" grep -Fq 'explicit scope-expansion judgment' "$SKILL_MD"
record "scope uses current agent and local shell" grep -Fq 'current agent and local shell' "$SKILL_MD"

pawl_fixture="$(mktemp)"
output_fixture="$(mktemp)"
lock_fixture="$(mktemp)"
rm -f "$lock_fixture"
trap 'rm -f "$pawl_fixture" "$output_fixture" "$lock_fixture"' EXIT
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

AO_BIN="${AO_BIN:-$(command -v ao || true)}"
if [[ -z "$AO_BIN" ]]; then
  echo "FAIL: ao binary is available for scope round-trip"
  FAIL=$((FAIL + 1))
else
  AO_SCOPE_LOCK="$lock_fixture" "$AO_BIN" scope freeze skills/scope >/dev/null
  record "ao scope freeze/status round-trip is valid" roundtrip_status "$lock_fixture" "$AO_BIN"
  AO_SCOPE_LOCK="$lock_fixture" "$AO_BIN" scope unfreeze >/dev/null
  record "ao scope unfreeze round-trip clears scope" roundtrip_clear "$lock_fixture" "$AO_BIN"
fi

echo
echo "Results: $PASS passed, $FAIL failed"
(( FAIL == 0 ))
