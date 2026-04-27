#!/usr/bin/env bash
# check-competitive-freshness.sh — Goal gate script
# Verifies that competitive analysis docs have been updated within the last 45 days.
# Exit 0 = pass, exit 1 = fail (stale comparisons found)
set -euo pipefail

ROOT="${COMPETITIVE_FRESHNESS_ROOT:-}"
if [[ -z "$ROOT" ]]; then
    ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "Not in a git repo"; exit 1; }
fi
COMPARISONS_DIR="${COMPETITIVE_FRESHNESS_COMPARISONS_DIR:-$ROOT/docs/comparisons}"

if [[ ! -d "$COMPARISONS_DIR" ]]; then
    echo "FAIL: docs/comparisons/ directory not found"
    exit 1
fi

STALE_DAYS="${COMPETITIVE_FRESHNESS_STALE_DAYS:-45}"
NOW=$(date +%s)
STALE_COUNT=0
TOTAL=0

file_mtime() {
    stat -c %Y "$1" 2>/dev/null || stat -f %m "$1" 2>/dev/null || echo 0
}

comparison_files=()
while IFS= read -r f; do
    comparison_files+=("$f")
done < <(find "$COMPARISONS_DIR" -maxdepth 1 -type f \( -name 'vs-*.md' -o -name 'competitive-radar.md' \) | sort)

required_docs=("$COMPARISONS_DIR/competitive-radar.md")
for required_doc in "${required_docs[@]}"; do
    if [[ ! -f "$required_doc" ]]; then
        echo "FAIL: missing required competitive analysis: $(basename "$required_doc")"
        STALE_COUNT=$((STALE_COUNT + 1))
    fi
done

for f in "${comparison_files[@]}"; do
    TOTAL=$((TOTAL + 1))

    # Use git log for last-modified date (more reliable than filesystem mtime)
    REL_PATH="${f#"$ROOT"/}"
    LAST_COMMIT=$(git -C "$ROOT" log -1 --format="%ct" -- "$REL_PATH" 2>/dev/null || true)
    if [[ -z "$LAST_COMMIT" ]]; then
        LAST_COMMIT="$(file_mtime "$f")"
    fi
    if [[ "$LAST_COMMIT" -eq 0 ]]; then
        echo "STALE: $(basename "$f") — no commit or working-tree timestamp"
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
    echo "FAIL: No competitive analysis docs found"
    exit 1
fi

if [[ "$STALE_COUNT" -gt 0 ]]; then
    echo "FAIL: $STALE_COUNT competitive analysis issue(s); $TOTAL doc(s) checked (missing or >${STALE_DAYS} days old)"
    exit 1
fi

echo "PASS: All $TOTAL competitive analyses are fresh (<=${STALE_DAYS} days)"
exit 0
