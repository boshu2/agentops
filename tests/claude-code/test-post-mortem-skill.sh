#!/usr/bin/env bash
# Test: post-mortem skill
# Verifies the post-mortem validation skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: post-mortem skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the post-mortem skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "post-mortem\|postmortem" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "validat\|learn\|retro\|wrap" "Describes post-implementation review"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify combined workflow
echo "Test 2: Combined workflow..."

output=$(run_claude "What does the post-mortem skill combine? What does it run?" 45)

if assert_contains "$output" "retro\|vibe\|security\|extract" "Mentions combined workflows"; then
    :
else
    exit 1
fi

echo ""

echo "=== All post-mortem skill tests passed ==="
