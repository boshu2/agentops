#!/usr/bin/env bash
# sweep-namespace-references.sh — Apply DeprecatedCommands substitutions to files.
#
# Reads the DeprecatedCommands map from cli/internal/quality/stale_refs.go and generates
# sed substitutions. Applies longest-match-first to avoid partial replacements.
#
# Usage:
#   bash scripts/sweep-namespace-references.sh [--apply] [files...]
#
# Default mode is dry-run (shows what would change).
# Pass --apply to modify files in place.
#
# If no files are specified, scans: hooks/*.sh skills/*/SKILL.md docs/*.md scripts/*.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DEPRECATED_COMMANDS_GO="$REPO_ROOT/cli/internal/quality/stale_refs.go"

APPLY=false
FILES=()

# Escape special sed characters in a string
escape_sed() {
    printf '%s\n' "$1" | sed -e 's/[\/&|]/\\&/g'
}

# Parse args
for arg in "$@"; do
    case "$arg" in
        --apply) APPLY=true ;;
        --dry-run) APPLY=false ;;
        --help|-h)
            echo "Usage: $0 [--apply] [--dry-run] [files...]"
            echo ""
            echo "Reads the canonical quality.DeprecatedCommands map and applies sed substitutions."
            echo "Default: dry-run mode (shows changes without modifying files)."
            echo ""
            echo "If no files specified, scans: hooks/*.sh skills/*/SKILL.md docs/*.md scripts/*.sh"
            exit 0
            ;;
        *) FILES+=("$arg") ;;
    esac
done

# Default file set if none specified
if [ ${#FILES[@]} -eq 0 ]; then
    # shellcheck disable=SC2206
    FILES=($REPO_ROOT/hooks/*.sh $REPO_ROOT/skills/*/SKILL.md $REPO_ROOT/docs/*.md $REPO_ROOT/scripts/*.sh)
fi

# Extract substitution pairs from stale_refs.go (longest-match-first via sort -rn)
# Format: old_cmd|new_cmd (one per line, sorted by old_cmd length descending)
PAIRS=$(sed -n '/var DeprecatedCommands/,/^}/p' "$DEPRECATED_COMMANDS_GO" \
    | grep '"ao ' \
    | sed 's/.*"\(ao [^"]*\)".*:.*"\(ao [^"]*\)".*/\1|\2/' \
    | awk -F'|' '{print length($1), $0}' \
    | sort -rn \
    | cut -d' ' -f2-)

if [ -z "$PAIRS" ]; then
    echo "ERROR: Could not extract DeprecatedCommands from $DEPRECATED_COMMANDS_GO"
    exit 1
fi

PAIR_COUNT=$(echo "$PAIRS" | wc -l | tr -d ' ')
echo "Loaded $PAIR_COUNT substitution pairs from stale_refs.go"
echo ""

TOTAL_REPLACEMENTS=0
FILES_CHANGED=0

for file in "${FILES[@]}"; do
    [ -f "$file" ] || continue

    file_replacements=0
    display_path="${file#"$REPO_ROOT"/}"

    while IFS='|' read -r old_cmd new_cmd; do
        [ -z "$old_cmd" ] && continue
        count=$(grep -c "$old_cmd" "$file" 2>/dev/null || true)
        if [ "$count" -gt 0 ]; then
            file_replacements=$((file_replacements + count))
            if $APPLY; then
                escaped_old=$(escape_sed "$old_cmd")
                escaped_new=$(escape_sed "$new_cmd")
                # Use | as sed delimiter since commands contain spaces but not pipes
                sed -i '' "s|${escaped_old}|${escaped_new}|g" "$file"
            else
                echo "  $display_path: '$old_cmd' → '$new_cmd' ($count occurrence(s))"
            fi
        fi
    done <<< "$PAIRS"

    if [ "$file_replacements" -gt 0 ]; then
        TOTAL_REPLACEMENTS=$((TOTAL_REPLACEMENTS + file_replacements))
        FILES_CHANGED=$((FILES_CHANGED + 1))
        if $APPLY; then
            echo "  $display_path: $file_replacements replacement(s) applied"
        fi
    fi
done

echo ""
if $APPLY; then
    echo "Applied: $TOTAL_REPLACEMENTS replacement(s) across $FILES_CHANGED file(s)"
else
    echo "Dry-run: $TOTAL_REPLACEMENTS replacement(s) would be made across $FILES_CHANGED file(s)"
    echo "Run with --apply to modify files."
fi
