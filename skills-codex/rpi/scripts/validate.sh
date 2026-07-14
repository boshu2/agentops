#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

phase_control_pattern='MAX_EPIC_WAVES|wave=0|wave=\$\(\(wave|\$wave -ge 50|global wave limit \(50\)|max budget per task: 2|retry once|max 2|max 3 total attempts|--max-cycles|3 validation failures|3\+ failures|after 3 failures|max 2 attempts|after 2 attempts|max 2 retries|after 2 retries|Retry \$RETRY_COUNT/2|Premortem failed 3x|retry limit|MAX_RETRIES|Attempts: 3/3|attempt: 1/3|Attempt counter: 2/3|--budget='

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: rpi" "grep -q '^name: rpi' '$SKILL_DIR/SKILL.md'"
check "references/ directory exists" "[ -d '$SKILL_DIR/references' ]"
check "references/ has at least 3 files" "[ \$(ls '$SKILL_DIR/references/' | wc -l) -ge 3 ]"
check "SKILL.md mentions research phase" "grep -qi 'research' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions plan phase" "grep -qiE '/plan|plan' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions Premortem phase" "grep -qi 'premortem' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions crank phase" "grep -qi '/crank' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions Learn phase" "grep -qiE '[/\$]learn' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions Validate phase" "grep -qi '/validate' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions immutable verdict handoff" "grep -qi 'immutable.*verdict' '$SKILL_DIR/SKILL.md'"
check "RPI docs mention next-work handoff metadata" "grep -q 'queue claim/finalize metadata' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "phase-data-contracts documents claim lifecycle" "grep -q 'claim_status' '$SKILL_DIR/references/phase-data-contracts.md' && grep -q 'release the claim back to available state' '$SKILL_DIR/references/phase-data-contracts.md'"
check "gate4-loop-and-spawn documents claim before consume" "grep -q 'claim the current cycle' '$SKILL_DIR/references/gate4-loop-and-spawn.md' && grep -q 'Never mark an item consumed at pick-time' '$SKILL_DIR/references/gate4-loop-and-spawn.md'"
check "RPI docs mention repo execution profile" "grep -qi 'repo execution profile' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "RPI docs mention execution packet" "grep -qi 'execution packet' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "RPI docs mention contract_surfaces" "grep -q 'contract_surfaces' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "RPI docs mention done_criteria" "grep -q 'done_criteria' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "phase-data-contracts documents execution packet" "grep -q 'execution_packet' '$SKILL_DIR/references/phase-data-contracts.md' && grep -qi 'repo execution profile' '$SKILL_DIR/references/phase-data-contracts.md'"
check "post-verdict order routes through Learn" "grep -Fq 'Validate -> Learn -> orchestrator' '$SKILL_DIR/SKILL.md'"
check "Learn is the only post-verdict handoff" "grep -Fq 'Learn is the only post-verdict handoff' '$SKILL_DIR/SKILL.md'"
check "only orchestrator invokes Premortem" "grep -Fq 'Only the orchestrator may invoke Premortem' '$SKILL_DIR/SKILL.md'"
check "re-plan contract handles all dispositions" "grep -Fq 'material_change' '$SKILL_DIR/references/agile-replan-loop.md' && grep -Fq 'no_change' '$SKILL_DIR/references/agile-replan-loop.md' && grep -Fq 'terminal' '$SKILL_DIR/references/agile-replan-loop.md'"
check "execution packet validator exists" "[ -x '$SKILL_DIR/scripts/validate-execution-packet.py' ]"
check "persistent run governor exists" "[ -x '$SKILL_DIR/scripts/run-governor.py' ]"
check "run governor schema is valid JSON" "jq empty '$SKILL_DIR/schemas/run-governor.schema.json'"
check "run governor checker compiles" "python3 -m py_compile '$SKILL_DIR/scripts/run-governor.py'"
check "RPI routes Crank and Validate through persistent governor" "grep -q 'Crank and Validate request admission' '$SKILL_DIR/SKILL.md' && grep -q 'authorized:true' '$SKILL_DIR/SKILL.md'"
check "RPI declares canonical disposition language" "grep -q 'NOTE.*REPAIR.*REPLAN.*HOLD.*ANDON' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/pull-flow-governor.md'"
check "RPI has no private three-attempt controller" "! grep -q '3 total attempts before' '$SKILL_DIR/SKILL.md'"
check "RPI authoritative references have no private phase controller" \
  "! rg -q -i '$phase_control_pattern' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references'"
check "RPI protects HOLD and ANDON behind explicit authority ports" \
  "grep -q 'can neither create nor clear.*HOLD' '$SKILL_DIR/references/pull-flow-governor.md' && grep -q 'break.*helper.*human' '$SKILL_DIR/references/pull-flow-governor.md' && grep -q 'subparsers.add_parser(\"human\")' '$SKILL_DIR/scripts/run-governor.py'"
check "critical constraints precede the core contract" "test \"\$(grep -n '^## Critical Constraints$' '$SKILL_DIR/SKILL.md' | head -1 | cut -d: -f1)\" -lt \"\$(grep -n '^## Core Contract$' '$SKILL_DIR/SKILL.md' | head -1 | cut -d: -f1)\""

packet_fixture="$(mktemp)"
invalid_packet_fixture="$(mktemp)"
future_packet_fixture="$(mktemp)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
artifact_dir="$REPO_ROOT/.agents/rpi"
discovery_artifact="$artifact_dir/validate-fixture-$$-discovery.md"
implementation_artifact="$artifact_dir/validate-fixture-$$-implementation.md"
validation_artifact="$artifact_dir/validate-fixture-$$-validation.md"
learn_artifact="$artifact_dir/validate-fixture-$$-learn.md"
mkdir -p "$artifact_dir"
printf 'discovery evidence\n' >"$discovery_artifact"
printf 'implementation evidence\n' >"$implementation_artifact"
printf 'validation evidence\n' >"$validation_artifact"
printf 'learn evidence\n' >"$learn_artifact"
trap 'rm -f "$packet_fixture" "$invalid_packet_fixture" "$future_packet_fixture" "$discovery_artifact" "$implementation_artifact" "$validation_artifact" "$learn_artifact"' EXIT

printf '%s\n' "{\"schema_version\":1,\"packet_state\":\"terminal\",\"objective\":\"prove four umbrellas\",\"skills_loaded\":[{\"name\":\"rpi\",\"reason\":\"orchestrator\"},{\"name\":\"discovery\",\"reason\":\"phase-1\"},{\"name\":\"crank\",\"reason\":\"phase-2\"},{\"name\":\"validate\",\"reason\":\"phase-3\"},{\"name\":\"learn\",\"reason\":\"phase-4\"}],\"phase_receipts\":[{\"phase\":\"discovery\",\"skill\":\"discovery\",\"status\":\"DONE\",\"artifact\":\".agents/rpi/$(basename "$discovery_artifact")\"},{\"phase\":\"crank\",\"skill\":\"crank\",\"status\":\"DONE\",\"artifact\":\".agents/rpi/$(basename "$implementation_artifact")\"},{\"phase\":\"validate\",\"skill\":\"validate\",\"status\":\"PASS\",\"artifact\":\".agents/rpi/$(basename "$validation_artifact")\"},{\"phase\":\"learn\",\"skill\":\"learn\",\"status\":\"DONE\",\"artifact\":\".agents/rpi/$(basename "$learn_artifact")\"}]}" >"$packet_fixture"
check "execution packet validator accepts core schema plus receipts" "python3 '$SKILL_DIR/scripts/validate-execution-packet.py' '$packet_fixture' >/dev/null"
jq --arg artifact ".agents/rpi/$(basename "$discovery_artifact")" '{
  schema_version: 3,
  packet_state: "prospective",
  objective: "honest Discovery handoff",
  skills_loaded: [.skills_loaded[] | select(.name == "rpi" or .name == "discovery")],
  phase_receipts: [
    {phase:"discovery", skill:"discovery", status:"DONE", artifact:$artifact},
    {phase:"crank", skill:"crank", status:"pending"},
    {phase:"validate", skill:"validate", status:"not_checked"},
    {phase:"learn", skill:"learn", status:"not_checked"}
  ]
}' "$packet_fixture" >"$invalid_packet_fixture"
check "execution packet validator accepts honest prospective receipts" \
  "python3 '$SKILL_DIR/scripts/validate-execution-packet.py' '$invalid_packet_fixture' | grep -q 'valid prospective execution packet'"
jq '.skills_loaded += [{name:"crank", reason:"future-phase"}]' \
  "$invalid_packet_fixture" >"$future_packet_fixture"
check "execution packet validator rejects unrun prospective skill loads" \
  "python3 '$SKILL_DIR/scripts/validate-execution-packet.py' '$future_packet_fixture' 2>&1 | grep -q 'prospective skills_loaded must omit unrun phase skill: crank'"
jq '.schema_version = 3
    | .pre_mortem_verdict = "PASS"
    | .premortem_verdict = "FAIL"' \
  "$packet_fixture" >"$invalid_packet_fixture"
if python3 "$SKILL_DIR/scripts/validate-execution-packet.py" "$invalid_packet_fixture" >/dev/null 2>&1; then
  echo "FAIL: execution packet validator rejects conflicting mortem verdict aliases"
  FAIL=$((FAIL + 1))
else
  echo "PASS: execution packet validator rejects conflicting mortem verdict aliases"
  PASS=$((PASS + 1))
fi
jq '.schema_version = 3
    | .pre_mortem_verdict = "PASS"
    | .premortem_verdict = "PASS"
    | .artifacts = {"pre_mortem_path":"legacy.md","premortem_path":"canonical.md"}' \
  "$packet_fixture" >"$invalid_packet_fixture"
if python3 "$SKILL_DIR/scripts/validate-execution-packet.py" "$invalid_packet_fixture" >/dev/null 2>&1; then
  echo "FAIL: execution packet validator rejects conflicting mortem artifact aliases"
  FAIL=$((FAIL + 1))
else
  echo "PASS: execution packet validator rejects conflicting mortem artifact aliases"
  PASS=$((PASS + 1))
fi
jq '.schema_version = 3
    | .pre_mortem_verdict = "PASS"
    | .premortem_verdict = "PASS"
    | del(.artifacts)' \
  "$packet_fixture" >"$invalid_packet_fixture"
check "execution packet validator accepts equal mortem verdict transition aliases" \
  "python3 '$SKILL_DIR/scripts/validate-execution-packet.py' '$invalid_packet_fixture' >/dev/null"
jq '.schema_version = 3
    | .pre_mortem_verdict = "PASS"
    | .premortem_verdict = "PASS"
    | .artifacts = {"pre_mortem_path":"same.md","premortem_path":"same.md"}' \
  "$packet_fixture" >"$invalid_packet_fixture"
if python3 "$SKILL_DIR/scripts/validate-execution-packet.py" "$invalid_packet_fixture" >/dev/null 2>&1; then
  echo "FAIL: execution packet validator rejects dual mortem artifact aliases even when equal"
  FAIL=$((FAIL + 1))
else
  echo "PASS: execution packet validator rejects dual mortem artifact aliases even when equal"
  PASS=$((PASS + 1))
fi
printf '%s\n' '{"schema_version":1,"objective":"prove fail closed","skills_loaded":[{"name":"rpi","reason":"orchestrator"}],"phase_receipts":[]}' >"$invalid_packet_fixture"
if python3 "$SKILL_DIR/scripts/validate-execution-packet.py" "$invalid_packet_fixture" >/dev/null 2>&1; then
  echo "FAIL: execution packet validator rejects missing receipts"
  FAIL=$((FAIL + 1))
else
  echo "PASS: execution packet validator rejects missing receipts"
  PASS=$((PASS + 1))
fi
jq 'del(.phase_receipts[] | select(.phase == "learn"))' "$packet_fixture" >"$invalid_packet_fixture"
if python3 "$SKILL_DIR/scripts/validate-execution-packet.py" "$invalid_packet_fixture" >/dev/null 2>&1; then
  echo "FAIL: execution packet validator rejects incomplete lifecycle receipts"
  FAIL=$((FAIL + 1))
else
  echo "PASS: execution packet validator rejects incomplete lifecycle receipts"
  PASS=$((PASS + 1))
fi
jq '.phase_receipts = [.phase_receipts[0], .phase_receipts[1], .phase_receipts[3], .phase_receipts[2]]' "$packet_fixture" >"$invalid_packet_fixture"
if python3 "$SKILL_DIR/scripts/validate-execution-packet.py" "$invalid_packet_fixture" >/dev/null 2>&1; then
  echo "FAIL: execution packet validator rejects out-of-order Learn receipt"
  FAIL=$((FAIL + 1))
else
  echo "PASS: execution packet validator rejects out-of-order Learn receipt"
  PASS=$((PASS + 1))
fi
jq '(.phase_receipts[] | select(.phase == "discovery") | .status) = "BLOCKED" | (.phase_receipts[] | select(.phase == "crank") | .status) = "PARTIAL" | (.phase_receipts[] | select(.phase == "validate") | .status) = "REFUTED" | (.phase_receipts[] | select(.phase == "learn") | .status) = "PARTIAL"' "$packet_fixture" >"$invalid_packet_fixture"
if python3 "$SKILL_DIR/scripts/validate-execution-packet.py" "$invalid_packet_fixture" >/dev/null 2>&1; then
  echo "FAIL: execution packet validator rejects unsuccessful final receipts"
  FAIL=$((FAIL + 1))
else
  echo "PASS: execution packet validator rejects unsuccessful final receipts"
  PASS=$((PASS + 1))
fi
sed "s#$(basename "$validation_artifact")#definitely-missing.md#" "$packet_fixture" >"$invalid_packet_fixture"
if python3 "$SKILL_DIR/scripts/validate-execution-packet.py" "$invalid_packet_fixture" >/dev/null 2>&1; then
  echo "FAIL: execution packet validator rejects missing artifact evidence"
  FAIL=$((FAIL + 1))
else
  echo "PASS: execution packet validator rejects missing artifact evidence"
  PASS=$((PASS + 1))
fi
jq '(.phase_receipts[] | select(.phase == "validate") | .artifact) = "../outside.md"' "$packet_fixture" >"$invalid_packet_fixture"
if python3 "$SKILL_DIR/scripts/validate-execution-packet.py" "$invalid_packet_fixture" >/dev/null 2>&1; then
  echo "FAIL: execution packet validator rejects artifact path escape"
  FAIL=$((FAIL + 1))
else
  echo "PASS: execution packet validator rejects artifact path escape"
  PASS=$((PASS + 1))
fi

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
