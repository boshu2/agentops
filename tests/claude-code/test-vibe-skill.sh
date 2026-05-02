#!/usr/bin/env bash
# Test: vibe skill
# Verifies the vibe validation skill works correctly
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"
export MAX_TURNS=6

echo "=== Test: vibe skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "Answer concisely without running tools: what is the /agentops:vibe skill in this plugin?" 60)

if assert_contains "$output" "vibe\|validation" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "security\|quality\|code\|review" "Describes validation aspects"; then
    :
else
    exit 1
fi

echo ""

# Test 2: Verify multi-domain validation
echo "Test 2: Multi-domain validation..."

output=$(run_claude "Using the /agentops:vibe skill context if needed, list the readiness domains it validates. Answer briefly." 60)

if assert_contains "$output" "security\|architecture\|quality\|accessibility\|correctness\|completeness\|hygiene\|test\|build\|lint" "Mentions validation domains"; then
    :
else
    exit 1
fi

echo ""

echo "=== All vibe skill tests passed ==="
