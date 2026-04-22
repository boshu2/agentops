#!/usr/bin/env bash
# check-compile-health.sh — Gate: Compile defrag report is fresh and stale count is acceptable.
#
# Exit 0 = PASS, Exit 1 = FAIL
#
# Environment overrides:
#   COMPILE_OUTPUT_DIR   Directory where ao defrag writes (default: $AGENTS_DIR)
#   AGENTS_DIR          .agents base dir (default: .agents)
#   COMPILE_MAX_AGE_HOURS  Max age of latest defrag report in hours (default: 26)
#   COMPILE_MAX_STALE    Max allowed stale learning count (default: 5)
set -euo pipefail

AGENTS_DIR="${AGENTS_DIR:-.agents}"
MAX_AGE_HOURS="${COMPILE_MAX_AGE_HOURS:-26}"
MAX_STALE="${COMPILE_MAX_STALE:-5}"

# COMPILE_OUTPUT_DIR overrides AGENTS_DIR prefix (used in CI where defrag writes to /tmp/)
DEFRAG_LATEST="${COMPILE_OUTPUT_DIR:-$AGENTS_DIR}/defrag/latest.json"

# Gate 1: defrag report must exist.
# Fall back to the most recent Dream overnight preview when no canonical
# report is present and COMPILE_OUTPUT_DIR is unset — Dream writes a
# dry-run defrag at .agents/overnight/<run>/defrag/latest.json every night,
# and that report exposes .timestamp + .prune.stale_count in the same shape
# this gate consumes. Only apply the fallback for the canonical AGENTS_DIR
# path so CI overrides remain strict.
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
    echo "FAIL: $DEFRAG_LATEST not found — run 'ao defrag' first"
    exit 1
fi

# Gate 2: defrag report must be recent
ts=$(jq -r '.timestamp' "$DEFRAG_LATEST" 2>/dev/null || echo "")
if [[ -z "$ts" || "$ts" == "null" ]]; then
    echo "FAIL: could not read .timestamp from $DEFRAG_LATEST"
    exit 1
fi

# Parse timestamp — support both Linux (date -d) and macOS (date -j)
if date --version >/dev/null 2>&1; then
    # GNU date (Linux)
    ts_epoch=$(date -d "$ts" +%s 2>/dev/null) || { echo "FAIL: could not parse timestamp '$ts'"; exit 1; }
else
    # BSD date (macOS) — strip trailing Z, handle both RFC3339 and ISO8601
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

# Gate 3: stale learning count must be within budget
stale=$(jq -r '.prune.stale_count // 0' "$DEFRAG_LATEST" 2>/dev/null || echo "0")
if [[ "$stale" -gt "$MAX_STALE" ]]; then
    echo "FAIL: $stale stale learnings (max: $MAX_STALE) — run 'ao defrag --prune'"
    exit 1
fi

echo "PASS: Compile health OK (defrag ${age_hours}h ago, stale=${stale}/${MAX_STALE})"
exit 0
