#!/usr/bin/env bash
# Organize repository structure - basic cleanup
# Version: 2.0.0 - Simplified implementation

set -euo pipefail

DRY_RUN="${DRY_RUN:-false}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Load shared constraints
# shellcheck source=tools/scripts/load-constraints.sh
source "$REPO_ROOT/tools/scripts/load-constraints.sh"

actions_taken=0

main() {
    echo "## Repository Cleanup Report"
    echo "**Date:** $(date -Iseconds)"
    echo "**Mode:** ${DRY_RUN}"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "**DRY RUN MODE** - No changes will be made"
        echo ""
    fi

    # 1. Remove common temporary/cache files
    echo "### Removing Temporary Files"
    echo ""

    temp_patterns=("*.pyc" "__pycache__" ".DS_Store" "*.swp" "*.swo" "*~")
    for pattern in "${temp_patterns[@]}"; do
        while IFS= read -r file; do
            if [[ "$DRY_RUN" == "true" ]]; then
                echo "  [DRY RUN] Would remove: $file"
            else
                echo "  Removing: $file"
                rm -f "$file"
            fi
            actions_taken=$((actions_taken + 1))
        done < <(find . -name "$pattern" -type f 2>/dev/null | grep -v "\.git" | head -20 || true)
    done

    [[ $actions_taken -eq 0 ]] && report_success "No temporary files to clean"
    echo ""

    # 2. Check for large files that shouldn't be committed
    echo "### Checking for Large Files"
    echo ""

    large_count=0
    while IFS= read -r file size; do
        echo "  ⚠️  Large file: $file ($size bytes)"
        large_count=$((large_count + 1))
    done < <(find . -type f -size +1M 2>/dev/null | grep -v "\.git" | head -10 | xargs -I {} du -b {} 2>/dev/null || true)

    [[ $large_count -eq 0 ]] && report_success "No large files found"
    echo ""

    # Summary
    echo "### Summary"
    echo "**Actions Taken:** $actions_taken"
    echo "**Large Files:** $large_count"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        [[ $actions_taken -gt 0 ]] && echo "**Result:** 🔍 Dry run - would clean $actions_taken item(s)" || echo "**Result:** ✅ No cleanup needed"
    else
        [[ $actions_taken -gt 0 ]] && report_success "Cleaned $actions_taken item(s)" || report_success "Repository is clean"
    fi
    exit 0
}

main "$@"
