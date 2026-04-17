#!/usr/bin/env bash
# bootstrap-maturity.sh — One-time migration: add maturity: provisional to
# existing .md learnings that lack a maturity field.
#
# Usage: bash scripts/bootstrap-maturity.sh [learnings-dir]
# Default learnings dir: .agents/learnings
#
# Safe to run multiple times — skips files that already have maturity field.

set -euo pipefail

LEARNINGS_DIR="${1:-.agents/learnings}"

if [[ ! -d "$LEARNINGS_DIR" ]]; then
    echo "Directory not found: $LEARNINGS_DIR"
    exit 1
fi

total=0
updated=0
skipped=0

for file in "$LEARNINGS_DIR"/*.md; do
    [[ -f "$file" ]] || continue
    total=$((total + 1))

    # Check if file has YAML frontmatter
    head_line=$(head -1 "$file")
    if [[ "$head_line" != "---" ]]; then
        skipped=$((skipped + 1))
        continue
    fi

    # Check if maturity field already exists in frontmatter
    if grep -q "^maturity:" "$file"; then
        skipped=$((skipped + 1))
        continue
    fi

    # Add maturity: provisional after the opening ---
    # Use a temp file to avoid in-place edit portability issues
    tmpfile=$(mktemp)
    {
        echo "---"
        echo "maturity: provisional"
        tail -n +2 "$file"
    } > "$tmpfile"
    mv "$tmpfile" "$file"
    updated=$((updated + 1))
done

# Process .jsonl files: add "maturity":"provisional" if missing
jsonl_total=0
jsonl_updated=0
jsonl_skipped=0

if command -v jq >/dev/null 2>&1; then
    for file in "$LEARNINGS_DIR"/*.jsonl; do
        [[ -f "$file" ]] || continue
        jsonl_total=$((jsonl_total + 1))

        # Count lines missing the maturity field. JSONL is one object per
        # line, so we must treat each line independently — a single -e check
        # only inspects the first object.
        missing=$(jq -c 'select(has("maturity") | not)' "$file" 2>/dev/null | wc -l | tr -d ' ')
        if [[ "${missing:-0}" -eq 0 ]]; then
            jsonl_skipped=$((jsonl_skipped + 1))
            continue
        fi

        # Transform each object: add maturity only when missing, preserving
        # existing values. Use -c so output stays valid JSONL (compact, one
        # object per line) instead of jq's default pretty-printed stream.
        tmpfile=$(mktemp)
        if jq -c 'if has("maturity") then . else . + {"maturity": "provisional"} end' "$file" > "$tmpfile" 2>/dev/null; then
            mv "$tmpfile" "$file"
            jsonl_updated=$((jsonl_updated + 1))
        else
            rm -f "$tmpfile"
            jsonl_skipped=$((jsonl_skipped + 1))
        fi
    done
else
    echo "Warning: jq not found — skipping .jsonl files" >&2
fi

echo "Bootstrap maturity complete:"
echo "  .md files: $total total, $updated updated, $skipped skipped"
echo "  .jsonl files: $jsonl_total total, $jsonl_updated updated, $jsonl_skipped skipped"
