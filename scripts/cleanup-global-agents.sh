#!/usr/bin/env bash
# cleanup-global-agents.sh — Audit and archive leaked repo-specific dirs from ~/.agents/
#
# Only learnings/ and patterns/ belong at the global level.
# Everything else (brainstorm, council, crank, etc.) was likely leaked from
# a session that wrote to ~/.agents/ instead of <repo>/.agents/.
#
# Usage:
#   scripts/cleanup-global-agents.sh              # Dry-run (default)
#   scripts/cleanup-global-agents.sh --force      # Actually move dirs to archive
#   scripts/cleanup-global-agents.sh --prune-old  # Remove archives older than 30 days

set -euo pipefail

GLOBAL_DIR="${HOME}/.agents"
ARCHIVE_DIR="${GLOBAL_DIR}/.archive"

# Directories that SHOULD exist at global level
ALLOWED_DIRS=("learnings" "patterns")

# Known repo-specific dirs that should NOT be global. Kept as documentation
# only; the loop below uses ALLOWED_DIRS as a positive whitelist (anything
# outside it is reported as leaked), so an explicit LEAKED_DIRS list is dead.
#   leaked examples: brainstorm council crank doc handoff knowledge plans
#                    products research retros rpi vibecheck ao

MODE="dry-run"

for arg in "$@"; do
    case "$arg" in
        --force)   MODE="force" ;;
        --prune-old) MODE="prune" ;;
        --help|-h)
            echo "Usage: $0 [--force | --prune-old]"
            echo ""
            echo "  (default)     Dry-run: report leaked dirs without moving"
            echo "  --force       Archive leaked dirs to ~/.agents/.archive/"
            echo "  --prune-old   Remove archives older than 30 days"
            exit 0
            ;;
    esac
done

if [[ "$MODE" == "prune" ]]; then
    if [[ ! -d "$ARCHIVE_DIR" ]]; then
        echo "No archive directory found at $ARCHIVE_DIR"
        exit 0
    fi
    echo "Pruning archives older than 30 days from $ARCHIVE_DIR..."
    count=0
    while IFS= read -r -d '' dir; do
        echo "  Removing: $(basename "$dir")"
        rm -rf "$dir"
        count=$((count + 1))
    done < <(find "$ARCHIVE_DIR" -mindepth 1 -maxdepth 1 -type d -mtime +30 -print0 2>/dev/null)
    echo "Pruned $count old archive(s)."
    exit 0
fi

if [[ ! -d "$GLOBAL_DIR" ]]; then
    echo "No ~/.agents/ directory found. Nothing to clean up."
    exit 0
fi

echo "Scanning $GLOBAL_DIR for leaked repo-specific directories..."
echo ""

found=0
total_files=0

for dir in "$GLOBAL_DIR"/*/; do
    [[ ! -d "$dir" ]] && continue
    dirname="$(basename "$dir")"

    # Skip allowed dirs
    skip=false
    for allowed in "${ALLOWED_DIRS[@]}"; do
        if [[ "$dirname" == "$allowed" ]]; then
            skip=true
            break
        fi
    done
    # Skip hidden dirs (like .archive, .gitignore)
    [[ "$dirname" == .* ]] && skip=true
    $skip && continue

    # Count files in leaked dir
    file_count=$(find "$dir" -type f 2>/dev/null | wc -l | tr -d ' ' || echo "0")
    total_files=$((total_files + file_count))
    found=$((found + 1))

    if [[ "$MODE" == "dry-run" ]]; then
        echo "  [LEAKED] $dirname/ ($file_count files)"
    elif [[ "$MODE" == "force" ]]; then
        timestamp=$(date +%Y%m%d-%H%M%S)
        archive_target="${ARCHIVE_DIR}/${timestamp}-${dirname}"
        mkdir -p "$ARCHIVE_DIR"
        mv "$dir" "$archive_target"
        echo "  [ARCHIVED] $dirname/ -> .archive/${timestamp}-${dirname}/ ($file_count files)"
    fi
done

echo ""
if [[ $found -eq 0 ]]; then
    echo "No leaked directories found. ~/.agents/ is clean."
else
    echo "Found $found leaked director(ies) with $total_files total file(s)."
    if [[ "$MODE" == "dry-run" ]]; then
        echo ""
        echo "Run with --force to archive these directories."
        echo "Run with --prune-old to remove archives older than 30 days."
    fi
fi
