#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
  cat <<'EOF'
Usage: bash scripts/validate-next-work-contract-parity.sh [repo-root]
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ $# -gt 1 ]]; then
  usage >&2
  exit 2
fi

if [[ $# -eq 1 ]]; then
  ROOT="$1"
fi

if [[ "$ROOT" != /* ]]; then
  ROOT="$(cd "$ROOT" && pwd)"
fi

SCHEMA="$ROOT/docs/contracts/next-work.schema.md"
HARVEST_REF="$ROOT/skills/post-mortem/references/harvest-next-work.md"
POST_MORTEM_SKILL="$ROOT/skills/post-mortem/SKILL.md"
POST_MORTEM_CODEX_SKILL="$ROOT/skills-codex/post-mortem/SKILL.md"
PHASE_CONTRACT="$ROOT/skills/rpi/references/phase-data-contracts.md"
GATE4="$ROOT/skills/rpi/references/gate4-loop-and-spawn.md"
RUNTIME="$(mktemp -t nextwork-runtime.XXXXXX)"
trap 'rm -f "$RUNTIME"' EXIT
cat \
  "$ROOT/cli/internal/rpi/types.go" \
  "$ROOT/cli/internal/rpi/helpers.go" \
  "$ROOT/cli/cmd/ao/rpi_loop.go" \
  > "$RUNTIME" 2>/dev/null || true
SMOKE="$ROOT/tests/smoke-test.sh"

failures=0

fail() {
  echo "FAIL: $1" >&2
  failures=$((failures + 1))
}

have_rg() {
  command -v rg >/dev/null 2>&1
}

require_file() {
  local path="$1"
  [[ -f "$path" ]] || fail "missing file: ${path#$ROOT/}"
}

contains_fixed_file() {
  local needle="$1"
  local path="$2"

  if have_rg; then
    rg -Fq "$needle" "$path"
    return
  fi

  grep -Fq -- "$needle" "$path"
}

contains_fixed_stdin() {
  local needle="$1"

  if have_rg; then
    rg -Fq "$needle"
    return
  fi

  grep -Fq -- "$needle"
}

require_contains() {
  local path="$1"
  local needle="$2"
  local label="$3"
  if ! contains_fixed_file "$needle" "$path"; then
    fail "$label"
  fi
}

require_section_contains() {
  local path="$1"
  local start="$2"
  local end="$3"
  local needle="$4"
  local label="$5"
  local section

  section="$(awk -v start="$start" -v end="$end" '
    index($0, start) {in_section=1}
    in_section {print}
    in_section && index($0, end) {exit}
  ' "$path")"

  if [[ -z "$section" ]]; then
    fail "missing section $start in ${path#$ROOT/}"
    return
  fi

  if ! printf '%s\n' "$section" | contains_fixed_stdin "$needle"; then
    fail "$label"
  fi
}

require_section_not_contains() {
  local path="$1"
  local start="$2"
  local end="$3"
  local needle="$4"
  local label="$5"
  local section

  section="$(awk -v start="$start" -v end="$end" '
    index($0, start) {in_section=1}
    in_section {print}
    in_section && index($0, end) {exit}
  ' "$path")"

  if [[ -z "$section" ]]; then
    fail "missing section $start in ${path#$ROOT/}"
    return
  fi

  if printf '%s\n' "$section" | contains_fixed_stdin "$needle"; then
    fail "$label"
  fi
}

for path in \
  "$SCHEMA" \
  "$HARVEST_REF" \
  "$POST_MORTEM_SKILL" \
  "$POST_MORTEM_CODEX_SKILL" \
  "$PHASE_CONTRACT" \
  "$GATE4" \
  "$RUNTIME" \
  "$SMOKE"; do
  require_file "$path"
done

require_contains "$SCHEMA" "schema_version: 1.3" \
  "next-work schema is not at v1.3"
require_contains "$SCHEMA" 'Item lifecycle inside `items[]` is authoritative.' \
  "next-work schema must declare item lifecycle authority"
require_contains "$SCHEMA" "may be empty when a post-mortem finds nothing actionable" \
  "next-work schema must permit empty items arrays"
require_contains "$SCHEMA" "consumers may rewrite existing lines to claim, release, fail, or consume individual items" \
  "next-work schema must describe rewrite semantics"
require_contains "$SCHEMA" "Legacy Compatibility" \
  "next-work schema must document legacy flat rows"

entry_fields=(
  source_epic timestamp items consumed claim_status claimed_by claimed_at
  consumed_by consumed_at failed_at
)
item_fields=(
  title type severity source description evidence target_repo proof_ref consumed
  claim_status claimed_by claimed_at consumed_by consumed_at failed_at
)
proof_ref_fields=(
  kind target_id run_id path
)
proof_ref_kinds=(
  completed_run evidence_only_closure execution_packet
)
item_types=(
  tech-debt improvement pattern-fix process-improvement feature bug task
)
item_sources=(
  council-finding retro-learning retro-pattern evolve-generator
  feature-suggestion backlog-processing
)

for field in "${entry_fields[@]}"; do
  require_contains "$SCHEMA" "\`$field\`" \
    "next-work schema missing entry field \`$field\`"
  require_contains "$RUNTIME" "json:\"$field" \
    "runtime next-work structs missing json field $field"
done

for field in "${item_fields[@]}"; do
  require_contains "$SCHEMA" "\`$field\`" \
    "next-work schema missing item field \`$field\`"
done

for field in "${proof_ref_fields[@]}"; do
  require_contains "$SCHEMA" "\`$field\`" \
    "next-work schema missing proof_ref field \`$field\`"
done

for value in "${proof_ref_kinds[@]}"; do
  require_contains "$SCHEMA" "\`$value\`" \
    "next-work schema missing proof_ref kind $value"
  require_contains "$HARVEST_REF" "$value" \
    "harvest-next-work reference missing proof_ref kind $value"
done

require_contains "$RUNTIME" 'json:"proof_ref,omitempty"' \
  "runtime next-work structs missing json field proof_ref"
for field in kind target_id run_id path; do
  require_contains "$RUNTIME" "json:\"$field" \
    "runtime proof_ref struct missing json field $field"
done

for value in "${item_types[@]}"; do
  require_contains "$SCHEMA" "\`$value\`" \
    "next-work schema missing type enum value $value"
  require_contains "$HARVEST_REF" "$value" \
    "harvest-next-work reference missing type enum value $value"
done

for value in high medium low; do
  require_contains "$SCHEMA" "\`$value\`" \
    "next-work schema missing severity enum value $value"
done

for value in "${item_sources[@]}"; do
  require_contains "$SCHEMA" "\`$value\`" \
    "next-work schema missing source enum value $value"
  require_contains "$HARVEST_REF" "$value" \
    "harvest-next-work reference missing source enum value $value"
done

for value in available in_progress consumed; do
  require_contains "$SCHEMA" "\`$value\`" \
    "next-work schema missing claim_status value $value"
  require_contains "$RUNTIME" "\"$value\"" \
    "runtime next-work logic missing claim_status value $value"
done

require_contains "$HARVEST_REF" "docs/contracts/next-work.schema.md" \
  "harvest-next-work must reference the tracked next-work schema"
require_contains "$HARVEST_REF" "proof_ref" \
  "harvest-next-work must document proof_ref emission"
require_contains "$POST_MORTEM_SKILL" "docs/contracts/next-work.schema.md" \
  "post-mortem skill must reference the tracked next-work schema"
require_contains "$POST_MORTEM_CODEX_SKILL" "docs/contracts/next-work.schema.md" \
  "generated Codex post-mortem skill must reference the tracked next-work schema"
require_contains "$GATE4" "docs/contracts/next-work.schema.md" \
  "rpi gate4 reference must point at the tracked next-work schema"
require_contains "$PHASE_CONTRACT" "item lifecycle as authoritative" \
  "phase-data-contracts must describe item lifecycle authority"
require_contains "$PHASE_CONTRACT" 'entry aggregate flips to `consumed=true` only after every child item is consumed' \
  "phase-data-contracts must describe aggregate consumption rule"
require_contains "$GATE4" "Never mark an item consumed at pick-time" \
  "rpi gate4 must retain claim-before-consume rule"
require_contains "$SMOKE" "validate-next-work-contract-parity.sh" \
  "smoke-test must execute the next-work contract parity validator"

require_contains "$RUNTIME" "case \"feature\", \"improvement\", \"tech-debt\", \"pattern-fix\", \"bug\", \"task\":" \
  "RPI runtime is missing workTypeRank coverage for pattern-fix"
require_contains "$RUNTIME" 'omitted item `claim_status` semantically' \
  "runtime comments or docs should preserve omitted claim_status semantics"

# Proof-backed completion evidence — runtime must use proof, not heuristic suppression.
require_contains "$RUNTIME" 'CompletionEvidence' \
  "runtime must reference CompletionEvidence for proof-backed skip logic"
require_contains "$RUNTIME" 'completion_evidence' \
  "runtime next-work struct must have completion_evidence json tag"

# Harvest reference must show the batched v1.3 write shape.
require_contains "$HARVEST_REF" "source_epic" \
  "harvest-next-work reference must show batched source_epic field"
require_contains "$HARVEST_REF" "items" \
  "harvest-next-work reference must show batched items array"
require_contains "$HARVEST_REF" 'claim_status' \
  "harvest-next-work reference must show claim_status lifecycle"
require_contains "$HARVEST_REF" "Schema Validation" \
  "harvest-next-work reference must include schema validation section"

# Crank skill must not contain flat next-work append examples.
CRANK_SKILL="$ROOT/skills/crank/SKILL.md"
if [[ -f "$CRANK_SKILL" ]]; then
  if contains_fixed_file 'echo "{\"title\":' "$CRANK_SKILL" 2>/dev/null; then
    fail "crank skill contains legacy flat-row next-work append example"
  fi
fi

for skill in "$POST_MORTEM_SKILL" "$POST_MORTEM_CODEX_SKILL"; do
  require_section_contains "$skill" '#### Step ACT.3: Feed Next-Work' '#### Step ACT.4: Update Marker' \
    "docs/contracts/next-work.schema.md" \
    "${skill#$ROOT/} ACT.3 must reference the tracked next-work schema"
  require_section_contains "$skill" '#### Step ACT.3: Feed Next-Work' '#### Step ACT.4: Update Marker' \
    "source_epic:" \
    "${skill#$ROOT/} ACT.3 must show batched next-work entries"
  require_section_contains "$skill" '#### Step ACT.3: Feed Next-Work' '#### Step ACT.4: Update Marker' \
    "items:" \
    "${skill#$ROOT/} ACT.3 must show batched item arrays"
  require_section_contains "$skill" '#### Step ACT.3: Feed Next-Work' '#### Step ACT.4: Update Marker' \
    "\"proof_ref\"" \
    "${skill#$ROOT/} ACT.3 must show proof_ref emission guidance"
  require_section_contains "$skill" '#### Step ACT.3: Feed Next-Work' '#### Step ACT.4: Update Marker' \
    'claim_status: "available"' \
    "${skill#$ROOT/} ACT.3 must initialize entry claim_status"
  require_section_not_contains "$skill" '#### Step ACT.3: Feed Next-Work' '#### Step ACT.4: Update Marker' \
    'echo "{\"title\":' \
    "${skill#$ROOT/} ACT.3 still contains the legacy flat-row append example"
done

# Drift validation: if a live next-work.jsonl exists, verify all entries conform to v1.3.
LIVE_QUEUE="$ROOT/.agents/rpi/next-work.jsonl"
if [[ -f "$LIVE_QUEUE" ]] && command -v jq >/dev/null 2>&1; then
  drift_count=0
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    if ! echo "$line" | jq -e '.source_epic and .items and (.claim_status != null)' >/dev/null 2>&1; then
      drift_count=$((drift_count + 1))
    fi
  done < "$LIVE_QUEUE"
  if [[ "$drift_count" -gt 0 ]]; then
    fail "next-work.jsonl has $drift_count entries not conforming to v1.3 schema (missing source_epic, items, or claim_status)"
  fi

  aggregate_self_drift="$(
    jq -s -r '
      to_entries[] as $line |
      select(($line.value.items? | type) == "array") |
      select(
        (((($line.value.consumed // false) == true) and ($line.value.claim_status != "consumed")) or
        ((($line.value.consumed // false) != true) and ($line.value.claim_status == "consumed")))
      ) |
      "line \($line.key + 1) source_epic=\($line.value.source_epic // "<missing>") consumed=\($line.value.consumed // false) claim_status=\($line.value.claim_status // "<missing>")"
    ' "$LIVE_QUEUE" 2>/dev/null || true
  )"
  if [[ -n "$aggregate_self_drift" ]]; then
    fail "next-work.jsonl has aggregate lifecycle self drift: $(printf '%s\n' "$aggregate_self_drift" | head -1)"
  fi

  lifecycle_drift="$(
    jq -s -r '
      def item_status:
        if .claim_status == "in_progress" then "in_progress"
        elif ((.consumed // false) == true) or (.claim_status == "consumed") then "consumed"
        else "available"
        end;
      def explicit_lifecycle:
        has("claim_status") or
        ((.consumed // false) == true) or
        has("claimed_by") or
        has("claimed_at") or
        has("consumed_by") or
        has("consumed_at") or
        has("failed_at");
      def aggregate_status:
        if ((.consumed // false) == true) or (.claim_status == "consumed") then "consumed"
        elif .claim_status == "in_progress" then "in_progress"
        else "available"
        end;
      def expected_status($statuses):
        if all($statuses[]; . == "consumed") then "consumed"
        elif any($statuses[]; . == "in_progress") then "in_progress"
        else "available"
        end;
      to_entries[] as $line |
      select(($line.value.items? | type) == "array") |
      ($line.value.items | map(select(explicit_lifecycle))) as $explicit |
      select(($explicit | length) > 0) |
      ($line.value.items | map(item_status)) as $statuses |
      expected_status($statuses) as $want |
      ($line.value | aggregate_status) as $got |
      select($want != $got) |
      "line \($line.key + 1) source_epic=\($line.value.source_epic // "<missing>") aggregate=\($got) items=\($want)"
    ' "$LIVE_QUEUE" 2>/dev/null || true
  )"
  if [[ -n "$lifecycle_drift" ]]; then
    fail "next-work.jsonl has aggregate/item lifecycle drift: $(printf '%s\n' "$lifecycle_drift" | head -1)"
  fi
fi

if [[ "$failures" -gt 0 ]]; then
  echo "next-work contract parity validation FAILED ($failures finding(s))." >&2
  exit 1
fi

echo "next-work contract parity validation passed."
