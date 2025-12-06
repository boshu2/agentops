#!/usr/bin/env bash
# Audit documentation for Diátaxis framework compliance
# Version: 2.0.0 - Full implementation extracted from documentation-diataxis-auditor.md

set -euo pipefail

DOCS_DIR="${DOCS_DIR:-docs}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Load shared constraints
# shellcheck source=tools/scripts/load-constraints.sh
source "$REPO_ROOT/tools/scripts/load-constraints.sh"

violation_count=0

# Diátaxis directories and deprecated/special directories
VALID_DIRS=("how-to" "explanation" "reference" "tutorial" "tutorials")
SPECIAL_DIRS=("showcase" "metrics" "gitlab-official-docs" "releases" "archive")
DEPRECATED_DIRS=("architecture" "guides" "sessions" "testing")

main() {
    echo "## Diátaxis Compliance Audit Report"
    echo "**Date:** $(date -Iseconds)"
    echo "**Documentation Directory:** $DOCS_DIR"
    echo ""

    # 1. Check for docs in root (should be in Diátaxis subdirs)
    echo "### Misplaced Documentation Files"
    echo ""

    misplaced_count=0
    while IFS= read -r -d '' file; do
        basename_file=$(basename "$file")
        # Skip README.md, index.md, and special files
        if [[ "$basename_file" != "README.md" && "$basename_file" != "index.md" ]]; then
            report_violation "HIGH" "Document in root: $file (move to: how-to/, explanation/, reference/, or tutorial/)"
            ((misplaced_count++))
            ((violation_count++))
        fi
    done < <(find "$DOCS_DIR" -maxdepth 1 -type f -name "*.md" -print0 2>/dev/null || true)

    if [[ $misplaced_count -eq 0 ]]; then
        report_success "No misplaced files in documentation root"
    fi
    echo ""

    # 2. Check for deprecated/non-Diátaxis directories
    echo "### Deprecated/Non-Diátaxis Directories"
    echo ""

    deprecated_count=0
    for deprecated in "${DEPRECATED_DIRS[@]}"; do
        if [[ -d "$DOCS_DIR/$deprecated" ]]; then
            report_violation "MEDIUM" "Deprecated directory: $DOCS_DIR/$deprecated (migrate contents to Diátaxis structure)"
            ((deprecated_count++))
            ((violation_count++))
        fi
    done

    if [[ $deprecated_count -eq 0 ]]; then
        report_success "No deprecated documentation directories found"
    fi
    echo ""

    # 3. Check for invalid subdirectories (not Diátaxis or special)
    echo "### Invalid Documentation Directories"
    echo ""

    invalid_count=0
    while IFS= read -r -d '' dir; do
        dirname_base=$(basename "$dir")
        is_valid=false

        # Check if valid Diátaxis dir
        for valid in "${VALID_DIRS[@]}"; do
            [[ "$dirname_base" == "$valid" ]] && is_valid=true && break
        done

        # Check if special allowed dir
        for special in "${SPECIAL_DIRS[@]}"; do
            [[ "$dirname_base" == "$special" ]] && is_valid=true && break
        done

        # Check if deprecated (already reported above)
        for deprecated in "${DEPRECATED_DIRS[@]}"; do
            [[ "$dirname_base" == "$deprecated" ]] && is_valid=true && break
        done

        if [[ "$is_valid" == "false" ]]; then
            report_violation "LOW" "Unknown directory: $dir (verify if needed or categorize properly)"
            ((invalid_count++))
        fi
    done < <(find "$DOCS_DIR" -mindepth 1 -maxdepth 1 -type d -print0 2>/dev/null || true)

    if [[ $invalid_count -eq 0 ]]; then
        report_success "All documentation directories are recognized"
    fi
    echo ""

    # Summary
    echo "### Summary"
    echo ""
    echo "**Diátaxis Directories:** ${VALID_DIRS[*]}"
    echo "**Special Directories:** ${SPECIAL_DIRS[*]}"
    echo "**Deprecated Directories:** ${DEPRECATED_DIRS[*]} (should be migrated)"
    echo ""

    if [[ $violation_count -eq 0 ]]; then
        report_success "All Diátaxis compliance checks passed"
        echo ""
        echo "**Result:** ✅ No violations"
        exit 0
    else
        echo "**Result:** ❌ $violation_count violation(s) found"
        echo ""
        echo "**Action Required:**"
        echo "1. Move root-level docs to appropriate Diátaxis directories"
        echo "2. Migrate content from deprecated directories"
        echo "3. Categorize or remove unknown directories"
        exit 1
    fi
}

main "$@"
