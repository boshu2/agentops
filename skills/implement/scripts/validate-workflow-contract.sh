#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROFILE="${1:-source}"
case "$PROFILE" in
  source) CONTRACT="$SKILL_DIR/references/workflow.md"; PREFIX='##' ;;
  codex) CONTRACT="$SKILL_DIR/SKILL.md"; PREFIX='###' ;;
  *) echo "usage: $0 source|codex" >&2; exit 2 ;;
esac
SCHEMA="$SKILL_DIR/schemas/implementation-receipt.schema.json"

HEADING_SUFFIXES=(
  'Step 6: Commit the Change'
  'Step 7: Persist the Implementation Receipt'
  'Step 8: Independent Validation and Pawl Routing'
  'Step 9: Close the Issue with Confirmed Evidence'
  'Step 10: Record Implementation in Ratchet Chain'
  'Step 11: Report to User'
)
# Exact Markdown literals: backticks are data, never shell substitutions.
# shellcheck disable=SC2016
MARKERS=(
  '**Receipt path:** `.agents/evidence/implement/<issue-id>/<full-sha>/<issue-id>-<full-sha>-receipt.json`'
  '**Receipt schema:** `schemas/implementation-receipt.schema.json`'
  '**Closure verifier:** `scripts/verify-implementation-receipt.sh --issue <issue-id> --receipt "$RECEIPT"`'
  '**Close wrapper:** `scripts/close-with-implementation-receipt.sh --issue <issue-id> --receipt "$RECEIPT"`'
  '**Canonical-first invariant:** run the pinned pawl on canonical verdict/evidence before any archive copy or receipt rewrite.'
  '**Head-root archive invariant:** receipt evidence paths resolve only beneath `.agents/evidence/implement/<issue-id>/<head_sha>/`.'
  '**Test-contract rule:** after the first RED receipt, changing a test contract requires a new slice and a new RED receipt; GREEN-mode contracts are always immutable.'
  '| `CONFIRMED` | `CLOSE` | Independent evidence authorizes Step 9. |'
  '| `REFUTED` | `AUTO-REDO` | Repair from findings, write new GREEN evidence, update the receipt, and rerun Step 8; do not close or consult a helper. |'
  '| `CIRCUIT-BREAKER-TRIP` | `HOLD` | Freeze mutation and preserve the receipt plus blocker evidence. |'
  '| `HOLD` | `HELPER` | Run exactly one bounded helper consultation for this blocker class. |'
  '| `HELPER-UNSTUCK` | `AUTO-REDO` | Apply the concrete next action, reset the breaker for the new approach, and re-earn `CONFIRMED`. |'
  '| `HELPER-ESCALATE` | `HUMAN` | Hand back the receipt, blocker evidence, and helper verdict. |'
  '| `REFUSAL-LANE / EXPLICIT-JUDGMENT / BUDGET-EXHAUSTED` | `HUMAN` | Skip the helper and ask the operator. |'
)
RECEIPT_FIELDS=(schema_version issue_id base_sha head_sha work_class acceptance_ids changed_files red green independent_validation)

validate_contract() {
  local file="$1" prefix="$2" suffix heading line previous=0 marker
  [[ -s "$file" ]] || return 1
  for suffix in "${HEADING_SUFFIXES[@]}"; do
    heading="$prefix $suffix"
    line="$(grep -Fnx -- "$heading" "$file" | head -1 | cut -d: -f1)"
    [[ -n "$line" && "$line" -gt "$previous" ]] || return 1
    previous="$line"
  done
  for marker in "${MARKERS[@]}"; do
    grep -Fqx -- "$marker" "$file" || return 1
  done
}

validate_schema() {
  local file="$1" field
  jq -e 'type=="object" and .additionalProperties==false and (.["$defs"].sha.pattern=="^[0-9a-f]{40}$") and (.["$defs"].captured_red.required|index("observed_sha")!=null) and (.["$defs"].captured_red.required|index("test_files")!=null) and (.["$defs"].red_waiver.properties.reason.enum==["docs-only","pure-refactor"]) and (.["$defs"].captured_red.properties.exit_code.maximum==125) and (.["$defs"].command_evidence.properties.exit_code.const==0) and (.properties.independent_validation.oneOf[1].required|index("copied_verdict")!=null) and (.properties.independent_validation.oneOf[1].required|index("review_evidence")!=null)' "$file" >/dev/null || return 1
  for field in "${RECEIPT_FIELDS[@]}"; do
    jq -e --arg field "$field" '.required | index($field) != null' "$file" >/dev/null || return 1
  done
}

validate_close_order() {
  local file="$SKILL_DIR/scripts/close-with-implementation-receipt.bash" pawl copy verify manifest close
  # shellcheck disable=SC2016
  pawl="$(grep -n 'canonical pawl rejected before archive mutation' "$file" | cut -d: -f1)"
  # shellcheck disable=SC2016
  copy="$(grep -n 'cp "\$SOURCE" "\$STAGE/evidence/pawl-verdict.json"' "$file" | cut -d: -f1)"
  # shellcheck disable=SC2016
  verify="$(grep -n '^"\$VERIFY" --issue' "$file" | cut -d: -f1)"
  manifest="$(grep -n 'close manifest changed' "$file" | cut -d: -f1)"
  close="$(grep -n 'beads exec close' "$file" | cut -d: -f1)"
  [[ "$pawl" -lt "$copy" && "$copy" -lt "$verify" && "$verify" -lt "$manifest" && "$manifest" -lt "$close" ]]
}

validate_pinned_security_shells() {
  local file
  for file in "$SKILL_DIR/scripts/close-with-implementation-receipt.bash" "$SKILL_DIR/scripts/verify-implementation-receipt.bash"; do
    ! grep -Eq '(^|[;&|()]|[[:space:]])bash([[:space:]]|$)' "$file" || return 1
  done
  # shellcheck disable=SC2016
  grep -Fq 'PAWL_AUTOBIND=0 /bin/bash "$PAWL" check' "$SKILL_DIR/scripts/close-with-implementation-receipt.bash"
  # shellcheck disable=SC2016
  grep -Fq 'PAWL_AUTOBIND=0 /bin/bash "$pawl_script" check' "$SKILL_DIR/scripts/verify-implementation-receipt.bash"
}

self_test_contract() {
  local tmp complete variant item suffix
  tmp="$(mktemp -d)"; complete="$tmp/complete"; variant="$tmp/variant"
  for suffix in "${HEADING_SUFFIXES[@]}"; do printf '%s %s\n' "$PREFIX" "$suffix"; done >"$complete"
  printf '%s\n' "${MARKERS[@]}" >>"$complete"
  validate_contract "$complete" "$PREFIX" || { rm -rf "$tmp"; return 1; }
  for suffix in "${HEADING_SUFFIXES[@]}"; do
    item="$PREFIX $suffix"
    grep -Fvx -- "$item" "$complete" >"$variant"
    if validate_contract "$variant" "$PREFIX"; then rm -rf "$tmp"; return 1; fi
  done
  for item in "${MARKERS[@]}"; do
    grep -Fvx -- "$item" "$complete" >"$variant"
    if validate_contract "$variant" "$PREFIX"; then rm -rf "$tmp"; return 1; fi
  done
  {
    printf '%s %s\n' "$PREFIX" "${HEADING_SUFFIXES[1]}"
    printf '%s %s\n' "$PREFIX" "${HEADING_SUFFIXES[0]}"
    for suffix in "${HEADING_SUFFIXES[@]:2}"; do printf '%s %s\n' "$PREFIX" "$suffix"; done
    printf '%s\n' "${MARKERS[@]}"
  } >"$variant"
  if validate_contract "$variant" "$PREFIX"; then rm -rf "$tmp"; return 1; fi
  rm -rf "$tmp"
}

self_test_schema() {
  local tmp variant field
  tmp="$(mktemp -d)"; variant="$tmp/schema.json"
  validate_schema "$SCHEMA" || { rm -rf "$tmp"; return 1; }
  for field in "${RECEIPT_FIELDS[@]}"; do
    jq --arg field "$field" '.required -= [$field]' "$SCHEMA" >"$variant"
    if validate_schema "$variant"; then rm -rf "$tmp"; return 1; fi
  done
  rm -rf "$tmp"
}

validate_contract "$CONTRACT" "$PREFIX"
validate_schema "$SCHEMA"
self_test_contract
self_test_schema
validate_close_order
validate_pinned_security_shells
"$SKILL_DIR/scripts/test-implementation-receipt.sh" >/dev/null
echo "implement workflow contract ($PROFILE): PASS"
