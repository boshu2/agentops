#!/usr/bin/env bash
# Audit multi-agent workflow outputs for constraint violations
# Version: 1.0.0

set -euo pipefail

WORKFLOW_DIR="${WORKFLOW_DIR:-tmp/agent-phases}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Load shared constraints
# shellcheck source=tools/scripts/load-constraints.sh
source "$REPO_ROOT/tools/scripts/load-constraints.sh"

violation_count=0

main() {
    echo "## Workflow Audit Report"
    echo "**Date:** $(date -Iseconds)"
    echo "**Workflow Directory:** $WORKFLOW_DIR"
    echo ""

    # Check for kubic-cm modifications (CRITICAL)
    echo "### Critical Constraint Checks"
    echo ""

    if get_modified_files | grep -q "^kubic-cm/"; then
        report_violation "CRITICAL" "kubic-cm/ directory modified (read-only upstream)"
        get_modified_files | grep "^kubic-cm/" | while read -r file; do
            echo "  - $file"
        done
        ((violation_count++))
    else
        report_success "No kubic-cm/ modifications detected"
    fi
    echo ""

    # Check for generated file edits (CRITICAL)
    echo "### Generated File Checks"
    echo ""

    if get_modified_files | grep -q "^apps/.*values\.yaml$"; then
        report_violation "CRITICAL" "Generated values.yaml files modified (use config.env)"
        get_modified_files | grep "^apps/.*values\.yaml$" | while read -r file; do
            echo "  - $file (should edit config.env instead)"
        done
        ((violation_count++))
    else
        report_success "No generated file modifications detected"
    fi
    echo ""

    # Summary
    echo "### Summary"
    echo ""
    if [[ $violation_count -eq 0 ]]; then
        report_success "All workflow constraint checks passed"
        echo ""
        echo "**Result:** ✅ No violations"
        exit 0
    else
        echo "**Result:** ❌ $violation_count violation(s) found"
        echo ""
        echo "**Action Required:** Review and fix violations before merging"
        exit 1
    fi
}

main "$@"
