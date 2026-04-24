#!/usr/bin/env bash
# check-skill-size.sh — warn on oversized SKILL.md files.
#
# Skills with very long SKILL.md bodies are harder to load into a fork context,
# and long frontmatter descriptions bloat the always-loaded skill catalog. This
# script reports every skill whose SKILL.md exceeds WARN_LINES (default 500), or
# whose description exceeds DESC_WARN_CHARS (default 120), and hard-fails above
# FAIL_LINES (default 800) or DESC_FAIL_CHARS (default 180) unless --warn-only is
# passed.
#
# Usage:
#   scripts/check-skill-size.sh              # warn and fail above threshold
#   scripts/check-skill-size.sh --warn-only  # report only (exit 0)
#   WARN_LINES=400 DESC_FAIL_CHARS=160 scripts/check-skill-size.sh
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "Not in a git repo"; exit 1; }
cd "$ROOT"

WARN_LINES=${WARN_LINES:-500}
FAIL_LINES=${FAIL_LINES:-800}
DESC_WARN_CHARS=${DESC_WARN_CHARS:-120}
DESC_FAIL_CHARS=${DESC_FAIL_CHARS:-180}
WARN_ONLY=0
for arg in "$@"; do
    case "$arg" in
        --warn-only) WARN_ONLY=1 ;;
    esac
done

WARN_COUNT=0
FAIL_COUNT=0
printf "Checking SKILL.md sizes (lines warn>%d fail>%d, description chars warn>%d fail>%d)\n" \
    "$WARN_LINES" "$FAIL_LINES" "$DESC_WARN_CHARS" "$DESC_FAIL_CHARS"
printf -- "---\n"

description_length() {
    local skill_md="$1"
    awk '
        /^---$/ {
            if (!in_fm) {
                in_fm = 1
                next
            }
            print length(desc)
            found = 1
            exit
        }
        in_fm && /^description:[[:space:]]*/ {
            desc = $0
            sub(/^description:[[:space:]]*/, "", desc)
            collecting = 1
            next
        }
        in_fm && collecting {
            if ($0 ~ /^[A-Za-z0-9_-]+:/) {
                collecting = 0
                next
            }
            line = $0
            sub(/^[[:space:]]+/, "", line)
            desc = desc " " line
        }
        END {
            if (!found) {
                print length(desc)
            }
        }
    ' "$skill_md"
}

while IFS= read -r skill_md; do
    lines=$(wc -l < "$skill_md")
    desc_chars=$(description_length "$skill_md")
    if [[ "$lines" -gt "$FAIL_LINES" ]]; then
        printf "FAIL  %4d lines  %s  (move reference material to %s/references/)\n" \
            "$lines" "$skill_md" "$(dirname "$skill_md")"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    elif [[ "$lines" -gt "$WARN_LINES" ]]; then
        printf "WARN  %4d lines  %s\n" "$lines" "$skill_md"
        WARN_COUNT=$((WARN_COUNT + 1))
    fi
    if [[ "$desc_chars" -gt "$DESC_FAIL_CHARS" ]]; then
        printf "FAIL  %4d chars  %s description  (keep frontmatter catalog concise)\n" \
            "$desc_chars" "$skill_md"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    elif [[ "$desc_chars" -gt "$DESC_WARN_CHARS" ]]; then
        printf "WARN  %4d chars  %s description\n" "$desc_chars" "$skill_md"
        WARN_COUNT=$((WARN_COUNT + 1))
    fi
done < <(find skills skills-codex -maxdepth 2 -name SKILL.md -type f | sort)

printf -- "---\n"
printf "Summary: %d warn, %d fail\n" "$WARN_COUNT" "$FAIL_COUNT"

if [[ "$FAIL_COUNT" -gt 0 && "$WARN_ONLY" -eq 0 ]]; then
    echo "Fix: split flagged SKILL.md bodies into references/*.md and keep descriptions below the catalog budget."
    exit 1
fi
exit 0
