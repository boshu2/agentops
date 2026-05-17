#!/usr/bin/env bash
# check-compile-oscillation.sh — Gate: No evolve goals are oscillating in the
# most recent defrag report.
#
# Exit 0 = PASS, Exit 1 = FAIL, Exit 77 = SKIP
#
# Looks at .agents/defrag/latest.json first. When missing and COMPILE_OUTPUT_DIR
# is unset, falls back to the freshest Dream overnight preview at
# .agents/overnight/<run>/defrag/latest.json (same shape). This keeps the gate
# green on machines that have run Dream but not a manual `ao defrag`.
#
# Environment overrides:
#   COMPILE_OUTPUT_DIR   Directory where ao defrag writes (default: $AGENTS_DIR)
#   AGENTS_DIR           .agents base dir (default: .agents)
#   COMPILE_MAX_AGE_HOURS Max age of latest defrag report in hours (default: 26)
#   COMPILE_REQUIRE_ARTIFACT When 1, missing report is FAIL instead of SKIP
set -euo pipefail

AGENTS_DIR="${AGENTS_DIR:-.agents}"
MAX_AGE_HOURS="${COMPILE_MAX_AGE_HOURS:-26}"
DEFRAG_LATEST="${COMPILE_OUTPUT_DIR:-$AGENTS_DIR}/defrag/latest.json"

if [[ ! -f "$DEFRAG_LATEST" && -z "${COMPILE_OUTPUT_DIR:-}" ]]; then
    overnight_root="$AGENTS_DIR/overnight"
    if [[ -d "$overnight_root" ]]; then
        fallback="$(find "$overnight_root" -path '*/defrag/latest.json' -type f -printf '%T@ %p\n' 2>/dev/null \
            | sort -n | tail -n 1 | awk '{print $2}')"
        if [[ -n "$fallback" && -f "$fallback" ]]; then
            echo "INFO: $DEFRAG_LATEST not found; falling back to overnight preview $fallback"
            DEFRAG_LATEST="$fallback"
        fi
    fi
fi

if [[ ! -f "$DEFRAG_LATEST" ]]; then
    if [[ "${COMPILE_REQUIRE_ARTIFACT:-0}" == "1" ]]; then
        echo "FAIL: $DEFRAG_LATEST not found — run 'ao defrag' first"
        exit 1
    fi
    echo "SKIP: $DEFRAG_LATEST not found — run 'ao defrag' to evaluate compile oscillation"
    exit 77
fi

ts=$(jq -r '.timestamp' "$DEFRAG_LATEST" 2>/dev/null || echo "")
if [[ -z "$ts" || "$ts" == "null" ]]; then
    echo "FAIL: could not read .timestamp from $DEFRAG_LATEST"
    exit 1
fi

if date --version >/dev/null 2>&1; then
    ts_epoch=$(date -d "$ts" +%s 2>/dev/null) || { echo "FAIL: could not parse timestamp '$ts'"; exit 1; }
else
    ts_clean="${ts%Z}"
    ts_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S%z" "$ts_clean" +%s 2>/dev/null) \
        || ts_epoch=$(date -j -f "%Y-%m-%dT%H:%M:%S" "$ts_clean" +%s 2>/dev/null) \
        || { echo "FAIL: could not parse timestamp '$ts'"; exit 1; }
fi

now_epoch=$(date +%s)
age_seconds=$(( now_epoch - ts_epoch ))
age_hours=$(( age_seconds / 3600 ))

if [[ $age_hours -gt $MAX_AGE_HOURS ]]; then
    echo "FAIL: last defrag was ${age_hours}h ago (max: ${MAX_AGE_HOURS}h) — run 'ao defrag'"
    exit 1
fi

if ! jq -e "(.oscillation.oscillating_goals // []) | length == 0" "$DEFRAG_LATEST" >/dev/null 2>&1; then
    count=$(jq -r "(.oscillation.oscillating_goals // []) | length" "$DEFRAG_LATEST" 2>/dev/null || echo "?")
    echo "FAIL: $count oscillating goal(s) in $DEFRAG_LATEST"
    exit 1
fi

echo "PASS: no oscillating goals in $DEFRAG_LATEST"
exit 0
