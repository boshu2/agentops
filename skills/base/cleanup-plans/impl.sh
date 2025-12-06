#!/usr/bin/env bash
# Archive completed plans to archived/ directory
# Version: 2.0.0 - Production implementation

set -euo pipefail

DRY_RUN="${DRY_RUN:-false}"
PLANS_DIR="${PLANS_DIR:-docs/reference/plans}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Load shared constraints
# shellcheck source=tools/scripts/load-constraints.sh
source "$REPO_ROOT/tools/scripts/load-constraints.sh"

files_moved=0

main() {
    echo "## Plans Cleanup Report"
    echo "**Date:** $(date -Iseconds)"
    echo "**Mode:** ${DRY_RUN}"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "**DRY RUN MODE** - No files will be moved"
        echo ""
    fi

    # Check directories exist
    if [[ ! -d "$PLANS_DIR/active" ]]; then
        echo "No active plans directory found"
        exit 0
    fi

    # Create archived if needed
    [[ ! -d "$PLANS_DIR/archived" && "$DRY_RUN" == "false" ]] && mkdir -p "$PLANS_DIR/archived"

    # Find completed plans
    echo "### Archiving Completed Plans"
    echo ""

    for file in "$PLANS_DIR/active"/*.md; do
        [[ ! -f "$file" ]] && continue

        if grep -qi "^Status:.*completed\|^\*\*Status:\*\*.*completed" "$file"; then
            target="$PLANS_DIR/archived/$(basename "$file")"
            if [[ "$DRY_RUN" == "true" ]]; then
                echo "  [DRY RUN] Would archive: $(basename "$file")"
                files_moved=$((files_moved + 1))
            else
                echo "  Archiving: $(basename "$file")"
                git mv "$file" "$target" 2>/dev/null || mv "$file" "$target"
                files_moved=$((files_moved + 1))
            fi
        fi
    done

    [[ $files_moved -eq 0 ]] && report_success "No completed plans to archive"
    echo ""

    # Summary
    echo "### Summary"
    echo "**Plans Archived:** $files_moved"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        [[ $files_moved -gt 0 ]] && echo "**Result:** 🔍 Dry run - would archive $files_moved plan(s)" || echo "**Result:** ✅ No cleanup needed"
    else
        [[ $files_moved -gt 0 ]] && report_success "Archived $files_moved plan(s)" || report_success "No cleanup needed"
    fi
    exit 0
}

main "$@"
