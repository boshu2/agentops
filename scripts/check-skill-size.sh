#!/usr/bin/env bash
# check-skill-size.sh — warn on oversized SKILL.md files.
#
# Skills with very long SKILL.md bodies are harder to load into a fork context
# and often bundle reference material that belongs in references/*.md. This
# script reports every skill whose SKILL.md exceeds WARN_LINES (default 500)
# and hard-fails above FAIL_LINES (default 800) unless --warn-only is passed.
#
# Usage:
#   scripts/check-skill-size.sh              # warn and fail above threshold
#   scripts/check-skill-size.sh --warn-only  # report only (exit 0)
#   WARN_LINES=400 scripts/check-skill-size.sh
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "Not in a git repo"; exit 1; }
cd "$ROOT"

WARN_LINES=${WARN_LINES:-500}
FAIL_LINES=${FAIL_LINES:-800}
WARN_ONLY=0
for arg in "$@"; do
    case "$arg" in
        --warn-only) WARN_ONLY=1 ;;
    esac
done

WARN_COUNT=0
FAIL_COUNT=0
printf "Checking SKILL.md sizes (warn>%d, fail>%d)\n" "$WARN_LINES" "$FAIL_LINES"
printf -- "---\n"

while IFS= read -r skill_md; do
    lines=$(wc -l < "$skill_md")
    name=$(basename "$(dirname "$skill_md")")
    if [[ "$lines" -gt "$FAIL_LINES" ]]; then
        printf "FAIL  %4d lines  %s  (move reference material to %s/references/)\n" \
            "$lines" "$skill_md" "$(dirname "$skill_md")"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    elif [[ "$lines" -gt "$WARN_LINES" ]]; then
        printf "WARN  %4d lines  %s\n" "$lines" "$skill_md"
        WARN_COUNT=$((WARN_COUNT + 1))
    fi
done < <(find skills -maxdepth 2 -name SKILL.md -type f | sort)

printf -- "---\n"
printf "Summary: %d warn, %d fail\n" "$WARN_COUNT" "$FAIL_COUNT"

if [[ "$FAIL_COUNT" -gt 0 && "$WARN_ONLY" -eq 0 ]]; then
    echo "Fix: split the flagged SKILL.md(s) into references/*.md linked from the main body."
    exit 1
fi
exit 0
