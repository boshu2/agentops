#!/usr/bin/env bash
# Test: handoff skill
# Verifies the handoff skill for session continuation works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: handoff skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the handoff skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "handoff" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "session\|continu\|pause\|context" "Describes session continuation"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify handoff document creation
echo "Test 2: Handoff document creation..."

output=$(run_claude "Where does the handoff skill write its output? What directory?" 45)

if assert_contains "$output" ".agents\|handoff" "Mentions output directory"; then
    :
else
    exit 1
fi

echo ""

# Test 3: Verify context preservation
echo "Test 3: Context preservation..."

output=$(run_claude "What does the handoff skill capture for the next session?" 45)

if assert_contains "$output" "accomplish\|commit\|file\|change\|context\|state" "Captures session context"; then
    :
else
    exit 1
fi

echo ""

echo "=== All handoff skill tests passed ==="
