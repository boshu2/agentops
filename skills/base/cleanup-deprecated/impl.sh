#!/usr/bin/env bash
# Archive files marked DEPRECATED
# Version: 2.0.0 - Production implementation

set -euo pipefail

DRY_RUN="${DRY_RUN:-false}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Load shared constraints
# shellcheck source=tools/scripts/load-constraints.sh
source "$REPO_ROOT/tools/scripts/load-constraints.sh"

files_moved=0

main() {
    echo "## Deprecated Files Cleanup Report"
    echo "**Date:** $(date -Iseconds)"
    echo "**Mode:** ${DRY_RUN}"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "**DRY RUN MODE** - No files will be moved"
        echo ""
    fi

    # Create deprecated archive if needed
    [[ ! -d "deprecated" && "$DRY_RUN" == "false" ]] && mkdir -p deprecated

    # Find files with DEPRECATED marker
    echo "### Archiving Deprecated Files"
    echo ""

    # Search for files containing DEPRECATED marker
    while IFS= read -r file; do
        [[ ! -f "$file" ]] && continue

        target="deprecated/$(basename "$file")"
        if [[ "$DRY_RUN" == "true" ]]; then
            echo "  [DRY RUN] Would archive: $file"
            files_moved=$((files_moved + 1))
        else
            echo "  Archiving: $file"
            git mv "$file" "$target" 2>/dev/null || mv "$file" "$target"
            files_moved=$((files_moved + 1))
        fi
    done < <(grep -rl "DEPRECATED\|deprecated" .claude/ docs/ 2>/dev/null | grep -v "\.git\|deprecated/" | head -20 || true)

    [[ $files_moved -eq 0 ]] && report_success "No deprecated files to archive"
    echo ""

    # Summary
    echo "### Summary"
    echo "**Files Archived:** $files_moved"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        [[ $files_moved -gt 0 ]] && echo "**Result:** 🔍 Dry run - would archive $files_moved file(s)" || echo "**Result:** ✅ No cleanup needed"
    else
        [[ $files_moved -gt 0 ]] && report_success "Archived $files_moved file(s)" || report_success "No cleanup needed"
    fi
    exit 0
}

main "$@"
