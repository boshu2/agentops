#!/usr/bin/env bash
# Audit onboarding experience for new developers
# Version: 2.0.0 - Extracted from testing-onboarding-audit.md

set -euo pipefail

DOCS_DIR="${DOCS_DIR:-docs}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Load shared constraints
# shellcheck source=tools/scripts/load-constraints.sh
source "$REPO_ROOT/tools/scripts/load-constraints.sh"

violation_count=0

# Critical onboarding files (must exist)
CRITICAL_FILES=(
    "README.md"
    "CLAUDE.md"
    "Makefile"
)

# Important onboarding docs
ONBOARDING_DOCS=(
    "docs/how-to/developer-setup/BOOTSTRAP_UNIFIED_FLOW.md"
    "docs/AGENTOPS_MANIFESTO.md"
)

main() {
    echo "## Onboarding Experience Audit Report"
    echo "**Date:** $(date -Iseconds)"
    echo ""
    echo "**Purpose:** Validate Day 1 developer onboarding experience"
    echo ""

    # 1. Check critical files exist
    echo "### P0 Critical: Essential Files"
    echo ""

    missing_critical=0
    for file in "${CRITICAL_FILES[@]}"; do
        if [[ ! -f "$REPO_ROOT/$file" ]]; then
            report_violation "CRITICAL" "Missing essential file: $file"
            ((missing_critical++))
            ((violation_count++))
        else
            report_success "Found: $file"
        fi
    done
    echo ""

    # 2. Check bootstrap/onboarding documentation exists
    echo "### P1 High: Onboarding Documentation"
    echo ""

    missing_docs=0
    for doc in "${ONBOARDING_DOCS[@]}"; do
        if [[ ! -f "$REPO_ROOT/$doc" ]]; then
            report_violation "HIGH" "Missing onboarding doc: $doc"
            ((missing_docs++))
            ((violation_count++))
        else
            report_success "Found: $doc"
        fi
    done
    echo ""

    # 3. Check Makefile has essential targets
    echo "### P1 High: Essential Makefile Targets"
    echo ""

    if [[ -f "$REPO_ROOT/Makefile" ]]; then
        missing_targets=0
        essential_targets=("help" "quick" "validate")

        for target in "${essential_targets[@]}"; do
            if ! grep -q "^${target}:" "$REPO_ROOT/Makefile"; then
                report_violation "HIGH" "Missing Makefile target: $target"
                ((missing_targets++))
                ((violation_count++))
            else
                report_success "Found target: $target"
            fi
        done
    else
        report_violation "CRITICAL" "Makefile not found"
        ((violation_count++))
    fi
    echo ""

    # 4. Check for bootstrap script
    echo "### P1 High: Bootstrap Capability"
    echo ""

    bootstrap_found=false
    if [[ -f "$REPO_ROOT/Makefile" ]]; then
        if grep -q "^bootstrap:" "$REPO_ROOT/Makefile"; then
            report_success "Bootstrap target found in Makefile"
            bootstrap_found=true
        fi
    fi

    if [[ "$bootstrap_found" == "false" ]]; then
        report_violation "MEDIUM" "No bootstrap target found (makes onboarding harder)"
        ((violation_count++))
    fi
    echo ""

    # 5. Check README quality
    echo "### P2 Medium: README.md Quality"
    echo ""

    if [[ -f "$REPO_ROOT/README.md" ]]; then
        readme_size=$(wc -l < "$REPO_ROOT/README.md")
        if [[ $readme_size -lt 10 ]]; then
            report_violation "MEDIUM" "README.md is too short ($readme_size lines - should have setup instructions)"
            ((violation_count++))
        else
            report_success "README.md has content ($readme_size lines)"
        fi

        # Check for essential sections
        if ! grep -qi "getting started\|quick start\|setup\|installation" "$REPO_ROOT/README.md"; then
            report_violation "MEDIUM" "README.md missing 'Getting Started' or 'Setup' section"
            ((violation_count++))
        fi
    fi
    echo ""

    # Summary
    echo "### Summary"
    echo ""
    echo "**Onboarding Readiness:**"
    echo "- Critical files: $(( ${#CRITICAL_FILES[@]} - missing_critical ))/${#CRITICAL_FILES[@]}"
    echo "- Onboarding docs: $(( ${#ONBOARDING_DOCS[@]} - missing_docs ))/${#ONBOARDING_DOCS[@]}"
    echo "- Bootstrap capable: $bootstrap_found"
    echo ""

    if [[ $violation_count -eq 0 ]]; then
        report_success "All onboarding experience checks passed"
        echo ""
        echo "**Result:** ✅ No violations"
        echo ""
        echo "**Next Steps for New Developer:**"
        echo "1. Read README.md"
        echo "2. Run: make help"
        echo "3. Run: make bootstrap (if available)"
        echo "4. Run: make quick (validation)"
        echo "5. Read: CLAUDE.md (agent usage guide)"
        exit 0
    else
        echo "**Result:** ❌ $violation_count violation(s) found"
        echo ""
        echo "**Action Required:**"
        echo "1. Fix missing critical files (README.md, CLAUDE.md, Makefile)"
        echo "2. Add missing onboarding documentation"
        echo "3. Add essential Makefile targets (help, quick, validate)"
        echo "4. Improve README.md with Getting Started section"
        exit 1
    fi
}

main "$@"
