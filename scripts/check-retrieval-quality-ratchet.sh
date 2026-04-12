#!/usr/bin/env bash
# check-retrieval-quality-ratchet.sh - warn-then-fail retrieval eval ratchet

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

MANIFEST="${AGENTOPS_RETRIEVAL_RATCHET_MANIFEST:-.agents/rpi/ao-sessions-eval-queries-v1.json}"
SEARCH_ROOT="${AGENTOPS_RETRIEVAL_RATCHET_SEARCH_ROOT:-$REPO_ROOT}"
TURNS_DIR="${AGENTOPS_RETRIEVAL_RATCHET_TURNS_DIR:-.agents/ao/sessions/turns}"
THRESHOLD="${AGENTOPS_RETRIEVAL_RATCHET_MIN_ANY_RELEVANT:-0.60}"
STRICT_TURNS="${AGENTOPS_RETRIEVAL_RATCHET_STRICT_TURNS:-500}"

if ! command -v jq >/dev/null 2>&1; then
    echo "FAIL retrieval quality ratchet: jq is required" >&2
    exit 1
fi

turns_path="$TURNS_DIR"
if [[ "$turns_path" != /* ]]; then
    turns_path="$REPO_ROOT/$turns_path"
fi

turn_count=0
if [[ -d "$turns_path" ]]; then
    turn_count="$(find "$turns_path" -type f -name '*.md' 2>/dev/null | wc -l | tr -d ' ')"
fi

report_file="$(mktemp "${TMPDIR:-/tmp}/ao-retrieval-ratchet.XXXXXX.json")"
trap 'rm -f "$report_file"' EXIT

if ! (
    cd "$REPO_ROOT/cli"
    env -u AGENTOPS_RPI_RUNTIME go run ./cmd/ao retrieval-bench \
        --search-eval "$MANIFEST" \
        --search-root "$SEARCH_ROOT" \
        --json
) >"$report_file"; then
    echo "FAIL retrieval quality ratchet: eval command failed" >&2
    exit 1
fi

metric="$(jq -r '.any_relevant_at_k // empty' "$report_file")"
avg_precision="$(jq -r '.avg_precision_at_k // 0' "$report_file")"
queries="$(jq -r '.queries // 0' "$report_file")"
hits="$(jq -r '.hits // 0' "$report_file")"
missing="$(jq -r '.missing_ground_truth // 0' "$report_file")"

if [[ -z "$metric" ]]; then
    echo "FAIL retrieval quality ratchet: report missing any_relevant_at_k" >&2
    exit 1
fi

meets_threshold="$(awk -v got="$metric" -v want="$THRESHOLD" 'BEGIN { print (got + 0 >= want + 0) ? 1 : 0 }')"
strict_active="$(awk -v got="$turn_count" -v want="$STRICT_TURNS" 'BEGIN { print (got + 0 >= want + 0) ? 1 : 0 }')"

summary="any_relevant_at_k=$metric threshold=$THRESHOLD hits=$hits/$queries avg_precision_at_k=$avg_precision missing_ground_truth=$missing indexed_turns=$turn_count strict_after=$STRICT_TURNS"

if [[ "$meets_threshold" -eq 1 ]]; then
    echo "PASS retrieval quality ratchet: $summary"
    exit 0
fi

if [[ "$strict_active" -eq 1 ]]; then
    echo "FAIL retrieval quality ratchet: $summary" >&2
    exit 1
fi

echo "WARN retrieval quality ratchet: $summary"
exit 0
