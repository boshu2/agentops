#!/usr/bin/env bash
# Test: knowledge skill
# Verifies the knowledge query skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: knowledge skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the knowledge skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "knowledge" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "search\|query\|find\|learning\|pattern" "Describes knowledge querying"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify artifact types
echo "Test 2: Artifact types..."

output=$(run_claude "What types of artifacts can the knowledge skill search?" 45)

if assert_contains "$output" "learning\|pattern\|retro\|research" "Mentions artifact types"; then
    :
else
    exit 1
fi

echo ""

echo "=== All knowledge skill tests passed ==="
