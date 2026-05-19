#!/usr/bin/env bash
set -euo pipefail

# Purge low-quality learnings from the global store (~/.agents/learnings/).
# Removes markdown files with body text < 50 chars (auto-extracted garbage).
# Usage: scripts/purge-global-garbage.sh [--dry-run]

GLOBAL_DIR="${HOME}/.agents/learnings"
DRY_RUN=false
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=true

if [[ ! -d "$GLOBAL_DIR" ]]; then
    echo "Global store not found at $GLOBAL_DIR"
    exit 0
fi

total=0
removed=0

while IFS= read -r -d '' file; do
    total=$((total + 1))
    # Extract body after YAML frontmatter (everything after second ---)
    body=$(awk '/^---$/{c++;next} c>=2{print}' "$file" 2>/dev/null || true)

    # Strip whitespace for length check
    stripped=$(echo "$body" | tr -d '[:space:]')
    strippedlen=${#stripped}

    if [[ $strippedlen -lt 50 ]]; then
        removed=$((removed + 1))
        if $DRY_RUN; then
            echo "[DRY-RUN] Would remove: $file (body=$strippedlen chars)"
        else
            rm "$file"
            echo "Removed: $file (body=$strippedlen chars)"
        fi
    fi
done < <(find "$GLOBAL_DIR" -name "*.md" -type f -print0 2>/dev/null)

echo ""
echo "Total scanned: $total"
echo "Removed: $removed"
echo "Remaining: $((total - removed))"
if $DRY_RUN; then
    echo "(dry-run mode — no files were deleted)"
fi
