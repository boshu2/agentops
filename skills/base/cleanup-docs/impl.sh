#!/usr/bin/env bash
# Move misplaced documentation to correct Diátaxis directories
# Version: 2.0.0 - Production implementation

set -euo pipefail

DRY_RUN="${DRY_RUN:-false}"
DOCS_DIR="${DOCS_DIR:-docs}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Load shared constraints
# shellcheck source=tools/scripts/load-constraints.sh
source "$REPO_ROOT/tools/scripts/load-constraints.sh"

files_moved=0

main() {
    echo "## Documentation Cleanup Report"
    echo "**Date:** $(date -Iseconds)"
    echo "**Mode:** ${DRY_RUN}"
    echo "**Documentation Directory:** $DOCS_DIR"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "**DRY RUN MODE** - No files will be moved"
        echo ""
    fi

    # Move misplaced docs from root to explanation/ (most common case for root docs)
    echo "### Moving Misplaced Root Documents"
    echo ""

    if [[ ! -d "$DOCS_DIR/explanation" ]]; then
        echo "Creating $DOCS_DIR/explanation/ directory..."
        if [[ "$DRY_RUN" == "false" ]]; then
            mkdir -p "$DOCS_DIR/explanation"
        fi
    fi

    moved_count=0
    while IFS= read -r -d '' file; do
        basename_file=$(basename "$file")
        # Skip README.md and index.md
        if [[ "$basename_file" != "README.md" && "$basename_file" != "index.md" ]]; then
            target="$DOCS_DIR/explanation/$basename_file"

            if [[ "$DRY_RUN" == "true" ]]; then
                echo "  [DRY RUN] Would move: $file → $target"
            else
                echo "  Moving: $file → $target"
                git mv "$file" "$target" 2>/dev/null || mv "$file" "$target"
            fi
            moved_count=$((moved_count + 1))
            files_moved=$((files_moved + 1))
        fi
    done < <(find "$DOCS_DIR" -maxdepth 1 -type f -name "*.md" -print0 2>/dev/null || true)

    if [[ $moved_count -eq 0 ]]; then
        report_success "No misplaced files in documentation root"
    else
        echo ""
        echo "  **Moved:** $moved_count file(s) to explanation/"
    fi
    echo ""

    # Summary
    echo "### Summary"
    echo ""
    echo "**Files Moved:** $files_moved"
    echo ""

    if [[ "$DRY_RUN" == "true" ]]; then
        if [[ $files_moved -gt 0 ]]; then
            echo "**Result:** 🔍 Dry run - would move $files_moved file(s)"
            echo ""
            echo "**To apply changes:** DRY_RUN=false make cleanup-docs"
        else
            echo "**Result:** ✅ No cleanup needed"
        fi
        exit 0
    else
        if [[ $files_moved -gt 0 ]]; then
            report_success "Cleanup complete - $files_moved file(s) organized"
            echo ""
            echo "**Next Steps:**"
            echo "1. Review moved files: git status"
            echo "2. Update any broken links"
            echo "3. Run: make audit-diataxis (verify compliance)"
            echo "4. Commit: git add . && git commit -m 'docs: Move misplaced files to correct Diátaxis directories'"
            exit 0
        else
            report_success "No cleanup needed - all docs properly organized"
            exit 0
        fi
    fi
}

main "$@"
