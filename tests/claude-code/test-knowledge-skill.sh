#!/usr/bin/env bash
# Test: knowledge skill
# Verifies the knowledge query skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"
export MAX_TURNS=6

echo "=== Test: knowledge skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "Answer concisely without running tools: what is the knowledge-activation skill in this plugin?" 60)

if assert_contains "$output" "knowledge" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "activat\|operationaliz\|learning\|pattern\|playbook\|briefing\|knowledge" "Describes knowledge activation"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify artifact types
echo "Test 2: Artifact types..."

output=$(run_claude "Answer concisely without running tools: what types of artifacts can the knowledge-activation skill use or produce?" 60)

if assert_contains "$output" "learning\|pattern\|retro\|research\|playbook\|belief\|briefing" "Mentions artifact types"; then
    :
else
    exit 1
fi

echo ""

echo "=== All knowledge skill tests passed ==="
