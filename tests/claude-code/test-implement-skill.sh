#!/usr/bin/env bash
# Test: implement skill
# Verifies the implement skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: implement skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the implement skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "implement" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "task\|issue\|work\|lifecycle\|execut" "Describes task execution"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify lifecycle
echo "Test 2: Lifecycle handling..."

output=$(run_claude "What lifecycle does the implement skill follow? What steps?" 45)

if assert_contains "$output" "start\|progress\|complete\|close\|commit" "Mentions lifecycle steps"; then
    :
else
    exit 1
fi

echo ""

echo "=== All implement skill tests passed ==="
