#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
SKILL_MD="$SKILL_DIR/SKILL.md"
SCHEMA="$REPO_ROOT/schemas/pawl-verdict.v1.schema.json"
PASS=0
FAIL=0

record() {
  local label="$1"
  shift
  if "$@"; then echo "PASS: $label"; PASS=$((PASS + 1));
  else echo "FAIL: $label"; FAIL=$((FAIL + 1)); fi
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

validate_terminal() {
  python3 -m jsonschema -i "$1" "$SCHEMA" >/dev/null 2>&1 &&
    jq -e 'if .disposition=="CONFIRMED" then ([.refuters[].family | if .=="fable" or .=="anthropic" then "claude" elif .=="codex" or .=="openai" then "gpt" elif .=="agy" or .=="google" then "gemini" else . end] | unique | length) >= 2 else true end' "$1" >/dev/null
}

record "using-gc contract is complete" validate_contract "$SKILL_MD"
record "GC remains operator-selected" grep -Fq 'City-shaped multi-quest work is an' "$SKILL_MD"
record "GC uses its native surface" grep -Fq "No gc subcommand lives under" "$SKILL_MD"
record "LAW 0 doctor guard is required" grep -Fq 'law0-print-args' "$SKILL_MD"
record "terminal schema exists" test -s "$SCHEMA"

pawl_fixture="$(mktemp)"
output_fixture="$(mktemp)"
valid_fixture="$(mktemp)"
invalid_fixture="$(mktemp)"
trap 'rm -f "$pawl_fixture" "$output_fixture" "$valid_fixture" "$invalid_fixture"' EXIT

sed 's/HELPER-UNSTUCK -> AUTO-REDO/HELPER-UNSTUCK -> MANUAL/' "$SKILL_MD" >"$pawl_fixture"
awk '!/\*\*Validator command:\*\*/' "$SKILL_MD" >"$output_fixture"
if validate_contract "$pawl_fixture"; then echo "FAIL: missing helper transition rejected"; FAIL=$((FAIL + 1));
else echo "PASS: missing helper transition rejected"; PASS=$((PASS + 1)); fi
if validate_contract "$output_fixture"; then echo "FAIL: incomplete output handoff rejected"; FAIL=$((FAIL + 1));
else echo "PASS: incomplete output handoff rejected"; PASS=$((PASS + 1)); fi

printf '%s\n' '{"schema_version":"pawl-verdict.v1","bead_id":"q-1","pr":0,"head_sha":"abcdef0","disposition":"CONFIRMED","generated_at":"2026-07-12T13:00:00Z","mode":"multi-model","author_context_id":"builder","refuters":[{"family":"gpt","verdict":"CONFIRMED","context_id":"lane-1"},{"family":"gemini","verdict":"CONFIRMED","context_id":"lane-2"}]}' >"$valid_fixture"
printf '%s\n' '{"schema_version":"pawl-verdict.v1","bead_id":"q-1","pr":0,"head_sha":"abcdef0","disposition":"CONFIRMED","generated_at":"2026-07-12T13:00:00Z","mode":"multi-model","author_context_id":"builder","refuters":[{"family":"gpt","verdict":"CONFIRMED","context_id":"lane-1"},{"family":"codex","verdict":"CONFIRMED","context_id":"lane-2"}]}' >"$invalid_fixture"
record "terminal validator accepts cross-family CONFIRMED" validate_terminal "$valid_fixture"
if validate_terminal "$invalid_fixture"; then echo "FAIL: terminal validator rejects alias-only diversity"; FAIL=$((FAIL + 1));
else echo "PASS: terminal validator rejects alias-only diversity"; PASS=$((PASS + 1)); fi

echo
echo "Results: $PASS passed, $FAIL failed"
(( FAIL == 0 ))
