#!/usr/bin/env bash
# Test: research skill
# Verifies that the skill is recognized and describes correct workflow
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"
export MAX_TURNS=6

echo "=== Test: research skill ==="
echo ""

# Test 1: Verify skill is recognized
echo "Test 1: Skill recognition..."

output=$(run_claude "Answer concisely without running tools: what is the /agentops:research skill in this plugin?" 60)

if assert_contains "$output" "research" "Skill name recognized"; then
    : # pass
else
    exit 1
fi

if assert_contains "$output" "explore\|investigation\|codebase\|discovery" "Describes exploration"; then
    : # pass
else
    exit 1
fi

echo ""

# Test 2: Verify skill mentions Explore agents
echo "Test 2: Explore agent dispatch..."

output=$(run_claude "Answer concisely without running tools: does /agentops:research use Explore agents, and how?" 60)

if assert_contains "$output" "explore\|agent\|dispatch\|task" "Mentions agent dispatch"; then
    : # pass
else
    exit 1
fi

echo ""

# Test 3: Verify output artifacts
echo "Test 3: Output artifacts..."

output=$(run_claude "Answer concisely without running tools: where does /agentops:research write its output?" 60)

if assert_contains "$output" ".agents\|research" "Mentions .agents/research directory"; then
    : # pass
else
    exit 1
fi

echo ""

echo "=== All research skill tests passed ==="
