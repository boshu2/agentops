#!/usr/bin/env bash
set -euo pipefail
SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0; FAIL=0

check() { if bash -c "$2"; then echo "PASS: $1"; PASS=$((PASS + 1)); else echo "FAIL: $1"; FAIL=$((FAIL + 1)); fi; }

validate_pawl_contract() {
  local skill_md="$1"
  grep -Fq 'WARN|FAIL|REFUTED -> AUTO-REDO' "$skill_md" &&
    grep -Fq 'BREAKER -> HOLD -> ONE-HELPER' "$skill_md" &&
    grep -Fq 'HELPER-UNSTUCK -> AUTO-REDO' "$skill_md" &&
    grep -Fq 'HELPER-ESCALATE -> HUMAN' "$skill_md" &&
    grep -Fq 'REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN' "$skill_md" &&
    ! grep -Fq 'Discovery BLOCKED | Stop' "$skill_md"
}
export -f validate_pawl_contract

check "SKILL.md exists" "[ -f '$SKILL_DIR/SKILL.md' ]"
check "SKILL.md has YAML frontmatter" "head -1 '$SKILL_DIR/SKILL.md' | grep -q '^---$'"
check "SKILL.md has name: rpi" "grep -q '^name: rpi' '$SKILL_DIR/SKILL.md'"
check "references/ directory exists" "[ -d '$SKILL_DIR/references' ]"
check "references/ has at least 3 files" "[ \$(ls '$SKILL_DIR/references/' | wc -l) -ge 3 ]"
check "SKILL.md mentions research phase" "grep -qi 'research' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions plan phase" "grep -qiE '/plan|plan' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions pre-mortem phase" "grep -qi 'pre-mortem' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions crank phase" "grep -qiE '(/crank|\\\$crank)' '$SKILL_DIR/SKILL.md'"
check "SKILL.md mentions vibe phase" "grep -qiE '/vibe|vibe' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "SKILL.md mentions post-mortem phase" "grep -qi 'post-mortem' '$SKILL_DIR/SKILL.md'"
check "RPI docs mention next-work handoff metadata" "grep -q 'queue claim/finalize metadata' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "phase-data-contracts documents claim lifecycle" "grep -q 'claim_status' '$SKILL_DIR/references/phase-data-contracts.md' && grep -q 'release the claim back to available state' '$SKILL_DIR/references/phase-data-contracts.md'"
check "gate4-loop-and-spawn documents claim before consume" "grep -q 'claim the current cycle' '$SKILL_DIR/references/gate4-loop-and-spawn.md' && grep -q 'Never mark an item consumed at pick-time' '$SKILL_DIR/references/gate4-loop-and-spawn.md'"
check "RPI docs mention repo execution profile" "grep -qi 'repo execution profile' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "RPI docs mention execution packet" "grep -qi 'execution packet' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "RPI docs mention contract_surfaces" "grep -q 'contract_surfaces' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "RPI docs mention done_criteria" "grep -q 'done_criteria' '$SKILL_DIR/SKILL.md' '$SKILL_DIR/references/phase-data-contracts.md'"
check "phase-data-contracts documents execution packet" "grep -q 'execution_packet' '$SKILL_DIR/references/phase-data-contracts.md' && grep -qi 'repo execution profile' '$SKILL_DIR/references/phase-data-contracts.md'"
check "pawl recovery state machine is complete" "validate_pawl_contract '$SKILL_DIR/SKILL.md'"
check "execution packet validator exists" "[ -x '$SKILL_DIR/scripts/validate-execution-packet.py' ]"
check "constraints are the first H2" "[ \"\$(awk '/^---$/{n++;next} n==2 && /^## /{print;exit}' '$SKILL_DIR/SKILL.md')\" = '## Critical Constraints' ]"

deletion_fixture="$(mktemp)"
packet_fixture="$(mktemp)"
invalid_packet_fixture="$(mktemp)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
artifact_dir="$REPO_ROOT/.agents/rpi"
discovery_artifact="$artifact_dir/validate-fixture-$$-discovery.md"
implementation_artifact="$artifact_dir/validate-fixture-$$-implementation.md"
validation_artifact="$artifact_dir/validate-fixture-$$-validation.md"
mkdir -p "$artifact_dir"
printf 'discovery evidence\n' >"$discovery_artifact"
printf 'implementation evidence\n' >"$implementation_artifact"
printf 'validation evidence\n' >"$validation_artifact"
trap 'rm -f "$deletion_fixture" "$packet_fixture" "$invalid_packet_fixture" "$discovery_artifact" "$implementation_artifact" "$validation_artifact"' EXIT
awk '!/HELPER-UNSTUCK -> AUTO-REDO/' "$SKILL_DIR/SKILL.md" >"$deletion_fixture"
if validate_pawl_contract "$deletion_fixture"; then
  echo "FAIL: deletion fixture rejects a missing pawl transition"
  FAIL=$((FAIL + 1))
else
  echo "PASS: deletion fixture rejects a missing pawl transition"
  PASS=$((PASS + 1))
fi

printf '%s\n' "{\"schema_version\":1,\"objective\":\"prove pawl recovery\",\"skills_loaded\":[{\"name\":\"rpi\",\"reason\":\"orchestrator\"},{\"name\":\"discovery\",\"reason\":\"phase-1\"},{\"name\":\"crank\",\"reason\":\"phase-2\"},{\"name\":\"validate\",\"reason\":\"phase-3\"}],\"phase_receipts\":[{\"phase\":\"discovery\",\"skill\":\"discovery\",\"status\":\"DONE\",\"artifact\":\".agents/rpi/$(basename "$discovery_artifact")\"},{\"phase\":\"implementation\",\"skill\":\"crank\",\"status\":\"DONE\",\"artifact\":\".agents/rpi/$(basename "$implementation_artifact")\"},{\"phase\":\"validation\",\"skill\":\"validate\",\"status\":\"PASS\",\"artifact\":\".agents/rpi/$(basename "$validation_artifact")\"}]}" >"$packet_fixture"
check "execution packet validator accepts complete disk-backed receipts" "python3 '$SKILL_DIR/scripts/validate-execution-packet.py' '$packet_fixture' >/dev/null"
jq '(.phase_receipts[] | select(.phase == "discovery") | .status) = "BLOCKED" | (.phase_receipts[] | select(.phase == "implementation") | .status) = "PARTIAL" | (.phase_receipts[] | select(.phase == "validation") | .status) = "REFUTED"' "$packet_fixture" >"$invalid_packet_fixture"
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

echo ""; echo "Results: $PASS passed, $FAIL failed"
[ $FAIL -eq 0 ] && exit 0 || exit 1
