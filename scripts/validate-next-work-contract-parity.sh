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
POSTMORTEM_SKILL="$ROOT/skills/postmortem/SKILL.md"
POSTMORTEM_CODEX_SKILL="$ROOT/skills-codex/postmortem/SKILL.md"
LEARN_SKILL="$ROOT/skills/learn/SKILL.md"
LEARN_CODEX_SKILL="$ROOT/skills-codex/learn/SKILL.md"
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

for path in \
  "$SCHEMA" \
  "$POSTMORTEM_SKILL" \
  "$POSTMORTEM_CODEX_SKILL" \
  "$LEARN_SKILL" \
  "$LEARN_CODEX_SKILL" \
  "$PHASE_CONTRACT" \
  "$GATE4" \
  "$RUNTIME" \
  "$SMOKE"; do
  require_file "$path"
done

require_contains "$SCHEMA" "schema_version: 1.4" \
  "next-work schema is not at v1.4"
require_contains "$SCHEMA" 'Item lifecycle inside `items[]` is authoritative.' \
  "next-work schema must declare item lifecycle authority"
require_contains "$SCHEMA" "may be empty when a producer finds nothing actionable" \
  "next-work schema must permit empty items arrays"
require_contains "$SCHEMA" "consumers may rewrite existing lines to claim, release, fail, or consume individual items" \
  "next-work schema must describe rewrite semantics"
require_contains "$SCHEMA" "Legacy Compatibility" \
  "next-work schema must document legacy flat rows"

entry_fields=(
  source_epic timestamp items consumed claim_status claimed_by claimed_at
  consumed_by consumed_at consumed_note consumed_ref failed_at
)
item_fields=(
  title type severity source description evidence target_repo proof_ref consumed
  consumed_note consumed_ref claim_status claimed_by claimed_at consumed_by consumed_at failed_at
)
proof_ref_fields=(
  kind target_id run_id path
)
proof_ref_kinds=(
  completed_run evidence_only_closure execution_packet
)
item_types=(
  tech-debt improvement pattern-fix process-improvement feature bug task docs chore
)
item_sources=(
  council-finding retro-learning retro-pattern evolve-generator
  feature-suggestion backlog-processing postmortem-finding post-mortem-finding manifest-classification
  dream-degraded
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
done

for value in high medium low; do
  require_contains "$SCHEMA" "\`$value\`" \
    "next-work schema missing severity enum value $value"
done

for value in "${item_sources[@]}"; do
  require_contains "$SCHEMA" "\`$value\`" \
    "next-work schema missing source enum value $value"
done

for value in available in_progress consumed; do
  require_contains "$SCHEMA" "\`$value\`" \
    "next-work schema missing claim_status value $value"
  require_contains "$RUNTIME" "\"$value\"" \
    "runtime next-work logic missing claim_status value $value"
done

require_contains "$POSTMORTEM_SKILL" "not the general learning" \
  "postmortem must remain a causal-analysis specialization"
require_contains "$POSTMORTEM_CODEX_SKILL" "not the general learning" \
  "generated Codex postmortem must remain a causal-analysis specialization"
require_contains "$LEARN_SKILL" 'plan_impact' \
  "Learn must own the post-verdict plan-impact handoff"
require_contains "$LEARN_CODEX_SKILL" 'plan_impact' \
  "generated Codex Learn must own the post-verdict plan-impact handoff"
for skill in "$POSTMORTEM_SKILL" "$POSTMORTEM_CODEX_SKILL"; do
  if contains_fixed_file "docs/contracts/next-work.schema.md" "$skill"; then
    fail "${skill#$ROOT/} must not own general next-work bookkeeping"
  fi
done
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
require_contains "$RUNTIME" "case \"process-improvement\", \"docs\", \"chore\":" \
  "RPI runtime is missing workTypeRank coverage for docs/chore"
require_contains "$RUNTIME" 'omitted item `claim_status` semantically' \
  "runtime comments or docs should preserve omitted claim_status semantics"

# Proof-backed completion evidence — runtime must use proof, not heuristic suppression.
require_contains "$RUNTIME" 'CompletionEvidence' \
  "runtime must reference CompletionEvidence for proof-backed skip logic"
require_contains "$RUNTIME" 'completion_evidence' \
  "runtime next-work struct must have completion_evidence json tag"

# Crank skill must not contain flat next-work append examples.
CRANK_SKILL="$ROOT/skills/crank/SKILL.md"
if [[ -f "$CRANK_SKILL" ]]; then
  if contains_fixed_file 'echo "{\"title\":' "$CRANK_SKILL" 2>/dev/null; then
    fail "crank skill contains legacy flat-row next-work append example"
  fi
fi

# Drift validation: if a live next-work.jsonl exists, verify all entries conform to v1.4.
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
    fail "next-work.jsonl has $drift_count entries not conforming to v1.4 schema (missing source_epic, items, or claim_status)"
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

  enum_drift="$(
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
      def active_item($entry):
        if explicit_lifecycle then item_status != "consumed"
        else ($entry | aggregate_status) != "consumed"
        end;
      def valid_type:
        . == "tech-debt" or . == "improvement" or . == "pattern-fix" or
        . == "process-improvement" or . == "feature" or . == "bug" or . == "task" or
        . == "docs" or . == "chore";
      def valid_severity:
        . == "high" or . == "medium" or . == "low";
      def valid_source:
        . == "council-finding" or . == "retro-learning" or . == "retro-pattern" or
        . == "evolve-generator" or . == "feature-suggestion" or . == "backlog-processing" or
        . == "postmortem-finding" or . == "post-mortem-finding" or . == "manifest-classification" or
        . == "dream-degraded";
      to_entries[] as $line |
      select(($line.value.items? | type) == "array") |
      ($line.value.items | to_entries[]) as $item |
      select($item.value | active_item($line.value)) |
      select(
        (($item.value.type | valid_type | not) or
        ($item.value.severity | valid_severity | not) or
        ($item.value.source | valid_source | not))
      ) |
      "line \($line.key + 1) source_epic=\($line.value.source_epic // "<missing>") item=\($item.key + 1) type=\($item.value.type // "<missing>") severity=\($item.value.severity // "<missing>") source=\($item.value.source // "<missing>")"
    ' "$LIVE_QUEUE" 2>/dev/null || true
  )"
  if [[ -n "$enum_drift" ]]; then
    fail "next-work.jsonl has active item enum drift: $(printf '%s\n' "$enum_drift" | head -1)"
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

  # Consumed-marker type drift (age-tkxq): the first-class per-item and
  # batch-level markers must be well-typed even when hand-edited — consumed is a
  # boolean, consumed_note / consumed_ref are strings. A wrong-typed marker on the
  # shared-mutable queue fails here (mirrors scripts/validate-next-work.sh).
  marker_type_drift="$(
    jq -s -r '
      def bad_bool($o; $k): ($o | has($k)) and (($o[$k] | type) != "boolean");
      def bad_str($o; $k):  ($o | has($k)) and (($o[$k] | type) != "string");
      to_entries[] as $line |
      select(($line.value.items? | type) == "array") |
      (
        (if bad_bool($line.value; "consumed") then "line \($line.key + 1) batch consumed must be boolean" else empty end),
        (if bad_str($line.value; "consumed_note") then "line \($line.key + 1) batch consumed_note must be string" else empty end),
        (if bad_str($line.value; "consumed_ref") then "line \($line.key + 1) batch consumed_ref must be string" else empty end),
        ($line.value.items | to_entries[] as $item |
          (if bad_bool($item.value; "consumed") then "line \($line.key + 1) item \($item.key + 1) consumed must be boolean" else empty end),
          (if bad_str($item.value; "consumed_note") then "line \($line.key + 1) item \($item.key + 1) consumed_note must be string" else empty end),
          (if bad_str($item.value; "consumed_ref") then "line \($line.key + 1) item \($item.key + 1) consumed_ref must be string" else empty end))
      )
    ' "$LIVE_QUEUE" 2>/dev/null || true
  )"
  if [[ -n "$marker_type_drift" ]]; then
    fail "next-work.jsonl has consumed-marker type drift: $(printf '%s\n' "$marker_type_drift" | head -1)"
  fi
fi

if [[ "$failures" -gt 0 ]]; then
  echo "next-work contract parity validation FAILED ($failures finding(s))." >&2
  exit 1
fi

echo "next-work contract parity validation passed."
