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

# Test 2: Verify the three checks
echo "Test 2: Three checks..."

output=$(run_claude "Answer concisely without running tools: what three checks does the premortem skill run on a plan, and in what order?" 60)

if assert_contains "$output" "verif\|one-way\|irreversib\|construct" "Mentions the three checks"; then
    :
else
    exit 1
fi

echo ""

echo "=== All premortem skill tests passed ==="
