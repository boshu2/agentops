#!/usr/bin/env bash
# Test: plan skill
# Verifies the plan skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: plan skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the plan skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "plan" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "decompos\|break\|task\|issue\|epic" "Describes task decomposition"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify beads integration
echo "Test 2: Beads integration..."

output=$(run_claude "Does the plan skill create beads issues? How?" 45)

if assert_contains "$output" "bead\|issue\|bd\|track" "Mentions beads/issues"; then
    :
else
    exit 1
fi

echo ""

echo "=== All plan skill tests passed ==="
