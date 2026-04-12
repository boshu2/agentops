#!/usr/bin/env bash
# Test: crank skill
# Verifies the crank autonomous execution skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: crank skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the crank skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "crank" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "autonom\|execut\|epic\|parallel" "Describes autonomous execution"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify epic completion
echo "Test 2: Epic completion behavior..."

output=$(run_claude "When does the crank skill stop? What triggers completion?" 45)

if assert_contains "$output" "close\|complet\|finish\|children" "Mentions completion condition"; then
    :
else
    exit 1
fi

echo ""

echo "=== All crank skill tests passed ==="
