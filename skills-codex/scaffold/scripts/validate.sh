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
    grep -Fq '(["build","test","lint"]-[$r.validation[].kind])|length==0' "$skill_md" &&
    grep -Fq '($r.verdict != "PASS" or all($r.validation[]; .exit_code == 0))' "$skill_md" &&
    grep -Fq '**Downstream handoff:**' "$skill_md" &&
    grep -Fq '## Quality Checklist' "$skill_md"
}

record "SKILL.md exists" test -f "$SKILL_MD"
record "SKILL.md has name: scaffold" grep -q '^name: scaffold' "$SKILL_MD"
record "SKILL.md mentions boilerplate or starter" grep -qiE 'boilerplate|starter' "$SKILL_MD"
record "SKILL.md mentions component or project generation" grep -qiE 'component|project|generat' "$SKILL_MD"
record "scaffold contract is complete" validate_contract "$SKILL_MD"
record "overwrite requires explicit authorization" grep -Fq 'explicit authorization' "$SKILL_MD"
record "scaffold never pushes" grep -Fq 'Never push' "$SKILL_MD"

pawl_fixture="$(mktemp)"
output_fixture="$(mktemp)"
receipt_fixture="$(mktemp)"
invalid_receipt_fixture="$(mktemp)"
trap 'rm -f "$pawl_fixture" "$output_fixture" "$receipt_fixture" "$invalid_receipt_fixture"' EXIT
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

validate_receipt() {
  jq -e '. as $r | .schema_version==1 and (["domain","project","component","ci"]|index($r.mode))!=null and ($r.target_root|type=="string" and length>0) and ($r.files_created|type=="array" and all(.[]; type=="string")) and ($r.files_modified|type=="array" and all(.[]; type=="string")) and (($r.files_created|length)+($r.files_modified|length)>0) and ($r.validation|type=="array" and length>0 and all(.[]; (.kind as $kind | (["build","test","lint"]|index($kind))!=null) and (.command|type=="string" and length>0) and (.exit_code|type=="number"))) and ((["build","test","lint"]-[$r.validation[].kind])|length==0) and (($r.commit==null) or ($r.commit|type=="string")) and (["PASS","WARN","FAIL"]|index($r.verdict))!=null and ($r.verdict != "PASS" or all($r.validation[]; .exit_code == 0)) and ($r.next_action|type=="string" and length>0)' "$1" >/dev/null
}

printf '%s\n' '{"schema_version":1,"mode":"component","target_root":"internal/example","files_created":["internal/example/example.go"],"files_modified":[],"validation":[{"kind":"build","command":"go build ./internal/example","exit_code":0},{"kind":"test","command":"go test ./internal/example","exit_code":0},{"kind":"lint","command":"go vet ./internal/example","exit_code":0}],"commit":null,"verdict":"PASS","next_action":"review generated diff"}' >"$receipt_fixture"
printf '%s\n' '{"schema_version":1,"mode":"component","target_root":"","files_created":[],"files_modified":[],"validation":[],"commit":null,"verdict":"PASS","next_action":""}' >"$invalid_receipt_fixture"
record "receipt validator accepts complete handoff" validate_receipt "$receipt_fixture"
if validate_receipt "$invalid_receipt_fixture"; then
  echo "FAIL: receipt validator rejects incomplete handoff"
  FAIL=$((FAIL + 1))
else
  echo "PASS: receipt validator rejects incomplete handoff"
  PASS=$((PASS + 1))
fi
printf '%s\n' '{"schema_version":1,"mode":"component","target_root":"internal/example","files_created":["internal/example/example.go"],"files_modified":[],"validation":[{"kind":"test","command":"go test ./...","exit_code":0}],"commit":null,"verdict":"PASS","next_action":"review generated diff"}' >"$invalid_receipt_fixture"
if validate_receipt "$invalid_receipt_fixture"; then
  echo "FAIL: receipt validator rejects missing build/test/lint coverage"
  FAIL=$((FAIL + 1))
else
  echo "PASS: receipt validator rejects missing build/test/lint coverage"
  PASS=$((PASS + 1))
fi
printf '%s\n' '{"schema_version":1,"mode":"component","target_root":"internal/example","files_created":["internal/example/example.go"],"files_modified":[],"validation":[{"kind":"build","command":"go build ./...","exit_code":0},{"kind":"test","command":"go test ./...","exit_code":1},{"kind":"lint","command":"go vet ./...","exit_code":0}],"commit":null,"verdict":"PASS","next_action":"review generated diff"}' >"$invalid_receipt_fixture"
if validate_receipt "$invalid_receipt_fixture"; then
  echo "FAIL: receipt validator rejects PASS with failed validation command"
  FAIL=$((FAIL + 1))
else
  echo "PASS: receipt validator rejects PASS with failed validation command"
  PASS=$((PASS + 1))
fi

echo
echo "Results: $PASS passed, $FAIL failed"
(( FAIL == 0 ))
