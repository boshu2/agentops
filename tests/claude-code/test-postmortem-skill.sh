#!/usr/bin/env bash
# Test: postmortem skill
# Verifies the optional retrospective causal-analysis skill is recognized
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"
export MAX_TURNS=6

echo "=== Test: postmortem skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "Answer concisely without running tools: what is the /agentops:postmortem skill in this plugin?" 60)

if assert_contains "$output" "postmortem" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "causal\|retrospect\|evidence\|counterfactual" "Describes retrospective causal analysis"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify the non-gating boundary
echo "Test 2: Non-gating boundary..."

output=$(run_claude "Answer concisely without running tools: what authority does /agentops:postmortem deliberately not have?" 60)

if assert_contains "$output" "gate\|proof\|plan\|tracker\|delivery\|promot" "Names its non-authority"; then
    :
else
    exit 1
fi

echo ""

echo "=== All postmortem skill tests passed ==="
