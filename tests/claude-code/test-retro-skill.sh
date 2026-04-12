#!/usr/bin/env bash
# Test: retro skill
# Verifies the retro skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: retro skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the retro skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "retro" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "learn\|extract\|pattern\|retrospective" "Describes learning extraction"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify output artifacts
echo "Test 2: Output artifacts..."

output=$(run_claude "Where does the retro skill write learnings? What directory?" 45)

if assert_contains "$output" ".agents\|learning\|retro" "Mentions output directory"; then
    :
else
    exit 1
fi

echo ""

echo "=== All retro skill tests passed ==="
