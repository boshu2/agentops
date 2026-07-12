#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Backticks and jq fragments are exact Markdown markers, never shell execution.
# shellcheck disable=SC2016
MARKERS=(
  '## Constraints'
  '- **Observe before mutation.** Capture the `[RCH]` summary, `rch doctor --json`, and the first failing triage stage before changing daemon, hook, worker, or disk state because an unobserved repair destroys the evidence needed to prove causality.'
  '- **Fail-open is not success.** Treat every `[RCH] local (<reason>)` result as unresolved until the named reason is repaired or deliberately accepted because a green local build does not prove remote offload worked.'
  '- **Consult the pawl before raising the andon.** WARN, FAIL, or REFUTED contract results repair and rerun automatically because ordinary rejection is diagnostic evidence; only a breaker may enter HOLD or consume the single helper lane.'
  '## Breaker State Machine'
  '- **Ordinary rejection — `WARN|FAIL|REFUTED -> AUTO-REDO`:** repair the named defect, recapture evidence, and rerun the failed check; plain rejection never enters HOLD and never consumes the helper lane.'
  '- **Breaker — `BREAKER -> HOLD -> ONE-HELPER`:** when recovery cannot proceed safely with available permissions or observability, freeze mutations and route exactly one bounded helper consultation with the evidence packet.'
  '- **Recovered — `HELPER-UNSTUCK -> AUTO-REDO`:** leave HOLD, apply the bounded recovery, and re-earn the deterministic check plus pawl verdict before declaring the pipeline healthy.'
  '- **Helper escalation — `HELPER-ESCALATE -> HUMAN`:** stop automation and send the helper-provided concrete escalation packet to the operator.'
  '- **Direct human lane — `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`:** skip the helper and route directly to the operator; these are the only direct-human states.'
  '**Checkpoint:** before the first mutation, record the failing stage, exact command, exit code, and relevant `[RCH]` summary in the run evidence; after repair, rerun that same probe so the before/after claim is falsifiable.'
  '## Output Specification'
  '- **Path:** `.agents/evidence/remote-compilation/<run-id>/evidence.json` in the active repository, with raw command output stored beside it when needed.'
  '- **Filename convention:** the machine handoff is always `evidence.json`; `<run-id>` is a filesystem-safe timestamp or task identifier unique to the recovery attempt.'
  '- **Serialization/schema format:** JSON object `rch-evidence.v1` with nonempty `run_id`, `summary_line`, and `next_action`; enum `status` (`healthy|recovered|breaker`); enum `stage` (`availability|config|hook|classification|remote-compile|worker-pressure|complete`); nullable string `worker`; and a nonempty `commands` array of `{command:string,exit_code:number}` objects.'
  '- **Downstream handoff:** a `healthy|recovered` packet returns the verified worker and probe to the build lane; a `breaker` packet enters HOLD and accompanies the single helper, and only the explicit human states above reach the operator.'
  '## Quality Checklist'
  '- Ordinary rejection remains in AUTO-REDO; HOLD has exactly one helper, and operator escalation is limited to the declared human states.'
)

# `$in` is an exact jq/Markdown marker, not a shell variable.
# shellcheck disable=SC2016
SUBSTRINGS=(
  '- **Validator command:**'
  '. as $in | .schema_version=="rch-evidence.v1"'
)

validate_contract() {
  local file="$1" marker
  [[ -s "$file" ]] || return 1
  for marker in "${MARKERS[@]}"; do
    grep -Fqx -- "$marker" "$file" || return 1
  done
  for marker in "${SUBSTRINGS[@]}"; do
    grep -Fq -- "$marker" "$file" || return 1
  done
}

validate_evidence() {
  jq -e '
    . as $in
    | .schema_version=="rch-evidence.v1"
    and (.run_id|type=="string" and length>0)
    and (["healthy","recovered","breaker"]|index($in.status))!=null
    and (["availability","config","hook","classification","remote-compile","worker-pressure","complete"]|index($in.stage))!=null
    and ($in.summary_line|type=="string" and length>0)
    and (($in.worker==null) or ($in.worker|type=="string"))
    and ($in.next_action|type=="string" and length>0)
    and ($in.commands|type=="array" and length>0)
    and all($in.commands[]; (.command|type=="string" and length>0) and (.exit_code|type=="number"))
  ' "$1" >/dev/null
}

delete_one_marker_fixture() {
  local marker variant="$TMP/missing-marker.md"
  for marker in "${MARKERS[@]}"; do
    grep -Fvx -- "$marker" "$SKILL" >"$variant"
    if validate_contract "$variant"; then
      echo "rch contract validator accepted a missing marker: $marker" >&2
      return 1
    fi
  done
  for marker in "${SUBSTRINGS[@]}"; do
    grep -Fv -- "$marker" "$SKILL" >"$variant"
    if validate_contract "$variant"; then
      echo "rch contract validator accepted a missing substring: $marker" >&2
      return 1
    fi
  done
}

evidence_fixtures() {
  local valid="$TMP/valid.json" invalid="$TMP/invalid.json" field
  jq -n '{schema_version:"rch-evidence.v1",run_id:"age-test",status:"recovered",stage:"complete",summary_line:"[RCH] remote worker-a (1.2s)",worker:"worker-a",next_action:"resume build",commands:[{command:"rch self-test --all",exit_code:0}]}' >"$valid"
  validate_evidence "$valid"
  for field in schema_version run_id status stage summary_line next_action commands; do
    jq --arg field "$field" 'del(.[$field])' "$valid" >"$invalid"
    if validate_evidence "$invalid"; then
      echo "rch evidence validator accepted missing field: $field" >&2
      return 1
    fi
  done
  jq '.status="unknown"' "$valid" >"$invalid"
  if validate_evidence "$invalid"; then
    echo "rch evidence validator accepted an unknown status" >&2
    return 1
  fi
  jq '.commands[0].exit_code="zero"' "$valid" >"$invalid"
  if validate_evidence "$invalid"; then
    echo "rch evidence validator accepted a nonnumeric exit code" >&2
    return 1
  fi
}

[[ -f "$SKILL" ]] || { echo "FAIL: missing SKILL.md" >&2; exit 1; }
head -1 "$SKILL" | grep -q '^---$' || { echo "FAIL: missing frontmatter" >&2; exit 1; }
validate_contract "$SKILL"
delete_one_marker_fixture
evidence_fixtures
echo "OK: rch"
