#!/usr/bin/env bash
# check-competitive-freshness.sh — Goal gate script
# Verifies that competitive analysis docs have been updated within the last 45 days.
# Exit 0 = pass, exit 1 = fail (stale comparisons found)
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "Not in a git repo"; exit 1; }
COMPARISONS_DIR="$ROOT/docs/comparisons"

if [[ ! -d "$COMPARISONS_DIR" ]]; then
    echo "FAIL: docs/comparisons/ directory not found"
    exit 1
fi

STALE_DAYS=45
NOW=$(date +%s)
STALE_COUNT=0
TOTAL=0

for f in "$COMPARISONS_DIR"/vs-*.md; do
    [[ -f "$f" ]] || continue
    TOTAL=$((TOTAL + 1))

    # Use git log for last-modified date (more reliable than filesystem mtime)
    LAST_COMMIT=$(git log -1 --format="%ct" -- "$f" 2>/dev/null) || LAST_COMMIT=0
    if [[ "$LAST_COMMIT" -eq 0 ]]; then
        echo "STALE: $(basename "$f") — never committed"
        STALE_COUNT=$((STALE_COUNT + 1))
        continue
    fi

    AGE_DAYS=$(( (NOW - LAST_COMMIT) / 86400 ))
    if [[ "$AGE_DAYS" -gt "$STALE_DAYS" ]]; then
        echo "STALE: $(basename "$f") — last updated ${AGE_DAYS}d ago (limit: ${STALE_DAYS}d)"
        STALE_COUNT=$((STALE_COUNT + 1))
    fi
done

if [[ "$TOTAL" -eq 0 ]]; then
    echo "FAIL: No vs-*.md comparison docs found"
    exit 1
fi

if [[ "$STALE_COUNT" -gt 0 ]]; then
    echo "FAIL: $STALE_COUNT/$TOTAL competitive analyses are stale (>${STALE_DAYS} days)"
    exit 1
fi

echo "PASS: All $TOTAL competitive analyses are fresh (<=${STALE_DAYS} days)"
exit 0
