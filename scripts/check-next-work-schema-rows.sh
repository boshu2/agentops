#!/usr/bin/env bash
# check-next-work-schema-rows.sh — Runtime gate: every row in
# .agents/rpi/next-work.jsonl conforms to the v1.3 schema enums.
#
# The existing scripts/validate-next-work-contract-parity.sh checks that
# the schema doc, runtime types, and skill docs agree. This complements it
# by validating actual queue file rows so legacy or hand-edited rows that
# slip in (e.g. severity=critical, source=post-mortem, type=docs) are
# caught at push time before producers and consumers diverge.
#
# Reports each drift with batch line number, item index, field, and
# offending value. Exits 1 on any violation.
set -euo pipefail

ROOT="${ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
QUEUE="${QUEUE:-$ROOT/.agents/rpi/next-work.jsonl}"

if [[ ! -f "$QUEUE" ]]; then
    echo "PASS: $QUEUE not present (no queue to lint)"
    exit 0
fi

command -v jq >/dev/null 2>&1 || { echo "FAIL: jq is required"; exit 1; }

# Schema enums mirror docs/contracts/next-work.schema.md §Enums and
# cli/internal/overnight/findings_router_test.go.
VALID_TYPES='tech-debt improvement pattern-fix process-improvement feature bug task'
VALID_SEVERITIES='low medium high'
VALID_SOURCES='council-finding retro-learning retro-pattern evolve-generator feature-suggestion backlog-processing'
VALID_CLAIM_STATUSES='available in_progress consumed'

in_set() {
    local needle="$1"
    shift
    for v in "$@"; do
        if [[ "$needle" == "$v" ]]; then return 0; fi
    done
    return 1
}

violations=0
report() {
    echo "FAIL: $*" >&2
    violations=$((violations + 1))
}

line_no=0
while IFS= read -r line; do
    line_no=$((line_no + 1))
    [[ -z "$line" ]] && continue
    if ! printf '%s' "$line" | jq -e . >/dev/null 2>&1; then
        report "line $line_no: malformed JSON"
        continue
    fi

    # Batch-level claim_status (optional but constrained when present).
    cs=$(printf '%s' "$line" | jq -r '.claim_status // empty')
    if [[ -n "$cs" ]] && ! in_set "$cs" $VALID_CLAIM_STATUSES; then
        report "line $line_no: batch claim_status=$cs not in {$VALID_CLAIM_STATUSES}"
    fi

    # Items-bearing row required for v1.3.
    if ! printf '%s' "$line" | jq -e '.items' >/dev/null 2>&1; then
        report "line $line_no: legacy flat row (no .items array) — v1.3 requires batch entries"
        continue
    fi

    item_count=$(printf '%s' "$line" | jq '.items | length')
    for ((j=0; j<item_count; j++)); do
        item=$(printf '%s' "$line" | jq -c ".items[$j]")
        ty=$(printf '%s' "$item" | jq -r '.type // empty')
        sev=$(printf '%s' "$item" | jq -r '.severity // empty')
        src=$(printf '%s' "$item" | jq -r '.source // empty')
        ics=$(printf '%s' "$item" | jq -r '.claim_status // empty')
        title=$(printf '%s' "$item" | jq -r '.title // ""' | head -c 60)

        if [[ -n "$ty" ]] && ! in_set "$ty" $VALID_TYPES; then
            report "line $line_no item $j ($title): type=$ty not in {$VALID_TYPES}"
        fi
        if [[ -n "$sev" ]] && ! in_set "$sev" $VALID_SEVERITIES; then
            report "line $line_no item $j ($title): severity=$sev not in {$VALID_SEVERITIES}"
        fi
        if [[ -n "$src" ]] && ! in_set "$src" $VALID_SOURCES; then
            report "line $line_no item $j ($title): source=$src not in {$VALID_SOURCES}"
        fi
        if [[ -n "$ics" ]] && ! in_set "$ics" $VALID_CLAIM_STATUSES; then
            report "line $line_no item $j ($title): claim_status=$ics not in {$VALID_CLAIM_STATUSES}"
        fi
    done
done < "$QUEUE"

if (( violations > 0 )); then
    echo "FAIL: $violations schema violation(s) in $QUEUE" >&2
    exit 1
fi

echo "PASS: $line_no row(s) in $QUEUE conform to v1.3 schema enums"
exit 0
