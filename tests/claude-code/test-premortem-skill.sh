#!/usr/bin/env bash
# Test: premortem skill
# Verifies the premortem simulation skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"
export MAX_TURNS=6

echo "=== Test: premortem skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "Answer concisely without running tools: what is the premortem skill in this plugin?" 60)

if assert_contains "$output" "premortem" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "simulat\|failure\|risk\|prevent\|stress\|validat\|judge\|spec\|plan" "Describes failure simulation"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify iteration simulation
echo "Test 2: Iteration simulation..."

output=$(run_claude "Answer concisely without running tools: how does the premortem skill simulate failures, and what does it iterate?" 60)

if assert_contains "$output" "iterat\|simulat\|implementation\|mode\|judge\|council\|review\|scenario" "Mentions simulation iterations"; then
    :
else
    exit 1
fi

echo ""

echo "=== All premortem skill tests passed ==="
