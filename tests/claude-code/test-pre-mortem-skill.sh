#!/usr/bin/env bash
# Test: pre-mortem skill
# Verifies the pre-mortem simulation skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: pre-mortem skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the pre-mortem skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "pre-mortem\|premortem" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "simulat\|failure\|risk\|prevent" "Describes failure simulation"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify iteration simulation
echo "Test 2: Iteration simulation..."

output=$(run_claude "How does the pre-mortem skill simulate failures? What does it iterate?" 45)

if assert_contains "$output" "iterat\|simulat\|implementation\|mode" "Mentions simulation iterations"; then
    :
else
    exit 1
fi

echo ""

echo "=== All pre-mortem skill tests passed ==="
