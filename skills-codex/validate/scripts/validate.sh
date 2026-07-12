#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
SKILL_MD="$SKILL_DIR/SKILL.md"
SCHEMA="$REPO_ROOT/schemas/verdict.v1.schema.json"
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
    grep -Fq '**Mode-budget assertion:** 8 modes.' "$skill_md" &&
    grep -Fq 'vibe` → `--mode=post-impl`' "$skill_md" &&
    grep -Fq 'READ-ONLY except writing your single verdict file' "$skill_md" &&
    grep -Fq 'This skill validates work; `ao pawl` certifies landing' "$skill_md" &&
    grep -Fq '**Checkpoint:**' "$skill_md" &&
    grep -Fq '**Artifact directory:**' "$skill_md" &&
    grep -Fq '**Filename convention:**' "$skill_md" &&
    grep -Fq '**Serialization/schema format:**' "$skill_md" &&
    grep -Fq '**Validator command:**' "$skill_md" &&
    grep -Fq '**Downstream handoff:**' "$skill_md" &&
    grep -Fq '## Quality Checklist' "$skill_md"
}

validate_result() {
  local result="$1"
  local author_session="$2"
  python3 -c 'import json,re,sys; from datetime import datetime; from jsonschema import Draft202012Validator,FormatChecker; schema=json.load(open(sys.argv[1])); instance=json.load(open(sys.argv[2])); author=sys.argv[3]; Draft202012Validator.check_schema(schema); Draft202012Validator(schema,format_checker=FormatChecker()).validate(instance); ts=instance["validated_at"]; assert re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})",ts); assert datetime.fromisoformat(ts.replace("Z","+00:00")).tzinfo is not None; assert instance["verdict"] != "PASS" or (instance["validator_session"] and instance["validator_session"] != author and not any(f["severity"] in {"critical","significant"} for f in instance["findings"]) and not instance["not_checked"])' "$SCHEMA" "$result" "$author_session" >/dev/null 2>&1
}

validate_report() {
  local report="$1"
  local validator_session="$2"
  local author_session="$3"
  local council_verdict report_verdict
  council_verdict="$(sed -nE 's/^## Council Verdict: (PASS|WARN|FAIL)$/\1/p' "$report")"
  report_verdict="$(sed -nE 's/^VERDICT: (PASS|WARN|FAIL)$/\1/p' "$report")"
  [[ "$(grep -Ec '^## Council Verdict: (PASS|WARN|FAIL)$' "$report")" -eq 1 ]] &&
    [[ "$(grep -Ec '^VERDICT: (PASS|WARN|FAIL)$' "$report")" -eq 1 ]] &&
    [[ "$council_verdict" == "$report_verdict" ]] &&
    [[ -n "$validator_session" ]] &&
    [[ "$report_verdict" != "PASS" || "$validator_session" != "$author_session" ]] &&
    awk -v judge="$validator_session" 'BEGIN{state=0;commands=0;reasons=0;seen=0;sourced=0;invalid=0;prefix="judge=" judge " command="} /^COMMANDS RUN:$/{commands++;if(state!=0)invalid=1;state=1;next} /^REASONS:$/{reasons++;if(state!=1||!seen)invalid=1;state=2;next} state==1&&NF{seen=1;if(index($0,prefix)==1&&length($0)>length(prefix))sourced=1} END{exit !(commands==1&&reasons==1&&seen&&sourced&&state==2&&!invalid)}' "$report"
}

record "validate contract is complete" validate_contract "$SKILL_MD"
record "verdict schema exists" test -s "$SCHEMA"
record "protocol reference exists" test -s "$SKILL_DIR/references/canonical-validation-protocol.md"
record "kernel stays within 250 lines" test "$(wc -l < "$SKILL_MD")" -le 250

pawl_fixture="$(mktemp)"
output_fixture="$(mktemp)"
valid_result="$(mktemp)"
invalid_result="$(mktemp)"
invalid_date_result="$(mktemp)"
self_judge_result="$(mktemp)"
blocking_finding_result="$(mktemp)"
significant_finding_result="$(mktemp)"
unchecked_result="$(mktemp)"
valid_report="$(mktemp)"
invalid_report="$(mktemp)"
wrong_order_report="$(mktemp)"
unsupported_report="$(mktemp)"
trap 'rm -f "$pawl_fixture" "$output_fixture" "$valid_result" "$invalid_result" "$invalid_date_result" "$self_judge_result" "$blocking_finding_result" "$significant_finding_result" "$unchecked_result" "$valid_report" "$invalid_report" "$wrong_order_report" "$unsupported_report"' EXIT

sed 's/HELPER-UNSTUCK -> AUTO-REDO/HELPER-UNSTUCK -> MANUAL/' "$SKILL_MD" >"$pawl_fixture"
awk '!/\*\*Validator command:\*\*/' "$SKILL_MD" >"$output_fixture"
if validate_contract "$pawl_fixture"; then echo "FAIL: missing helper transition rejected"; FAIL=$((FAIL + 1));
else echo "PASS: missing helper transition rejected"; PASS=$((PASS + 1)); fi
if validate_contract "$output_fixture"; then echo "FAIL: incomplete output handoff rejected"; FAIL=$((FAIL + 1));
else echo "PASS: incomplete output handoff rejected"; PASS=$((PASS + 1)); fi

printf '%s\n' '{"verdict_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","bead_id":"01ARZ3NDEKTSV4RRFFQ69G5FAW","verdict":"PASS","confidence":"HIGH","briefing_learnings":[],"findings":[],"not_checked":[],"validated_at":"2026-07-12T13:00:00Z","validator_session":"judge-1","schema_version":1}' >"$valid_result"
printf '%s\n' '{"verdict_id":"short","bead_id":"bad","verdict":"MAYBE","confidence":"HIGH","briefing_learnings":[],"findings":[],"not_checked":[],"validated_at":"not-a-date","validator_session":"author","schema_version":1}' >"$invalid_result"
printf '%s\n' '{"verdict_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","bead_id":"01ARZ3NDEKTSV4RRFFQ69G5FAW","verdict":"PASS","confidence":"HIGH","briefing_learnings":[],"findings":[],"not_checked":[],"validated_at":"not-a-date","validator_session":"judge-1","schema_version":1}' >"$invalid_date_result"
printf '%s\n' '{"verdict_id":"01ARZ3NDEKTSV4RRFFQ69G5FAX","bead_id":"01ARZ3NDEKTSV4RRFFQ69G5FAW","verdict":"PASS","confidence":"HIGH","briefing_learnings":[],"findings":[],"not_checked":[],"validated_at":"2026-07-12T13:00:00Z","validator_session":"author","schema_version":1}' >"$self_judge_result"
printf '%s\n' '{"verdict_id":"01ARZ3NDEKTSV4RRFFQ69G5FAY","bead_id":"01ARZ3NDEKTSV4RRFFQ69G5FAW","verdict":"PASS","confidence":"HIGH","briefing_learnings":[],"findings":[{"severity":"critical","description":"release gate is bypassed","location":"scripts/release.sh:10"}],"not_checked":[],"validated_at":"2026-07-12T13:00:00Z","validator_session":"judge-1","schema_version":1}' >"$blocking_finding_result"
printf '%s\n' '{"verdict_id":"01ARZ3NDEKTSV4RRFFQ69G5FB0","bead_id":"01ARZ3NDEKTSV4RRFFQ69G5FAW","verdict":"PASS","confidence":"HIGH","briefing_learnings":[],"findings":[{"severity":"significant","description":"required regression is missing","location":"tests/release.bats"}],"not_checked":[],"validated_at":"2026-07-12T13:00:00Z","validator_session":"judge-1","schema_version":1}' >"$significant_finding_result"
printf '%s\n' '{"verdict_id":"01ARZ3NDEKTSV4RRFFQ69G5FAZ","bead_id":"01ARZ3NDEKTSV4RRFFQ69G5FAW","verdict":"PASS","confidence":"HIGH","briefing_learnings":[],"findings":[],"not_checked":["release gate"],"validated_at":"2026-07-12T13:00:00Z","validator_session":"judge-1","schema_version":1}' >"$unchecked_result"
record "result validator accepts complete verdict.v1" validate_result "$valid_result" "author"
if validate_result "$invalid_result" "author"; then echo "FAIL: result validator rejects malformed verdict"; FAIL=$((FAIL + 1));
else echo "PASS: result validator rejects malformed verdict"; PASS=$((PASS + 1)); fi
if validate_result "$invalid_date_result" "author"; then echo "FAIL: result validator rejects invalid RFC3339 timestamp"; FAIL=$((FAIL + 1));
else echo "PASS: result validator rejects invalid RFC3339 timestamp"; PASS=$((PASS + 1)); fi
if validate_result "$self_judge_result" "author"; then echo "FAIL: PASS result rejects author as validator"; FAIL=$((FAIL + 1));
else echo "PASS: PASS result rejects author as validator"; PASS=$((PASS + 1)); fi
if validate_result "$blocking_finding_result" "author"; then echo "FAIL: PASS result rejects critical or significant findings"; FAIL=$((FAIL + 1));
else echo "PASS: PASS result rejects critical or significant findings"; PASS=$((PASS + 1)); fi
if validate_result "$significant_finding_result" "author"; then echo "FAIL: PASS result rejects significant findings"; FAIL=$((FAIL + 1));
else echo "PASS: PASS result rejects significant findings"; PASS=$((PASS + 1)); fi
if validate_result "$unchecked_result" "author"; then echo "FAIL: PASS result rejects nonempty not_checked"; FAIL=$((FAIL + 1));
else echo "PASS: PASS result rejects nonempty not_checked"; PASS=$((PASS + 1)); fi

printf '%s\n' '## Council Verdict: PASS' 'VERDICT: PASS' 'COMMANDS RUN:' 'judge=judge-1 command=go test ./...' 'REASONS:' '- command exited zero' >"$valid_report"
printf '%s\n' '## Council Verdict: PASS' 'VERDICT: FAIL' 'COMMANDS RUN:' 'judge=judge-1 command=go test ./...' 'REASONS:' '- contradictory headings' >"$invalid_report"
printf '%s\n' '## Council Verdict: PASS' 'VERDICT: PASS' 'REASONS:' '- wrong order' 'COMMANDS RUN:' 'judge=judge-1 command=go test ./...' >"$wrong_order_report"
printf '%s\n' '## Council Verdict: PASS' 'VERDICT: PASS' 'COMMANDS RUN:' 'tests looked good' 'REASONS:' '- unsupported prose' >"$unsupported_report"
record "report validator accepts measured single verdict" validate_report "$valid_report" "judge-1" "author"
if validate_report "$invalid_report" "judge-1" "author"; then echo "FAIL: report validator rejects contradictory verdict headings"; FAIL=$((FAIL + 1));
else echo "PASS: report validator rejects contradictory verdict headings"; PASS=$((PASS + 1)); fi
if validate_report "$wrong_order_report" "judge-1" "author"; then echo "FAIL: report validator rejects REASONS before COMMANDS RUN"; FAIL=$((FAIL + 1));
else echo "PASS: report validator rejects REASONS before COMMANDS RUN"; PASS=$((PASS + 1)); fi
if validate_report "$unsupported_report" "judge-1" "author"; then echo "FAIL: report validator rejects commands without judge source"; FAIL=$((FAIL + 1));
else echo "PASS: report validator rejects commands without judge source"; PASS=$((PASS + 1)); fi

echo
echo "Results: $PASS passed, $FAIL failed"
(( FAIL == 0 ))
