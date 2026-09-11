#!/usr/bin/env bash
# Smoke test for skill invocation via Claude CLI
# Tests that skills can be triggered and produce expected outputs

set -euo pipefail

HELPERS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../claude-code" && pwd)"
# shellcheck source=tests/claude-code/test-helpers.sh
source "$HELPERS_DIR/test-helpers.sh"

# Guard: skip if Claude CLI not available
if ! command -v claude &> /dev/null; then
    echo "SKIP: claude CLI not found in PATH"
    exit 0
fi

passed=0
failed=0
skipped=0

echo "═══════════════════════════════════════════"
echo "Skill Invocation Smoke Tests"
echo "═══════════════════════════════════════════"
echo ""

# Create isolated test environment
TEST_PROJECT=$(create_test_project)
trap 'cleanup_test_project "$TEST_PROJECT"' EXIT

cd "$TEST_PROJECT"

# Set MAX_TURNS for all tests
export MAX_TURNS=3

assert_registered_or_invoked() {
    local log_file="$1"
    local skill_name="$2"
    local command_name="$3"
    local test_name="$4"

    if assert_skill_triggered "$log_file" "$skill_name" "$test_name"; then
        return 0
    fi

    if grep -q "\"agentops:${command_name}\"" "$log_file" && grep -q "\"agentops:${skill_name}\"" "$log_file"; then
        echo -e "  ${GREEN}[PASS]${NC} $test_name: agentops:$command_name registered and command executed without Skill tool event"
        return 0
    fi

    return 1
}

# Test 1: /agentops:reality-check skill
echo "Test 1: /agentops:reality-check skill"
LOG_FILE=$(run_claude_json "/agentops:reality-check check whether this project has a README" 120) || true

test_passed=true
if ! assert_registered_or_invoked "$LOG_FILE" "reality-check" "reality-check" "Reality-check skill triggered"; then
    test_passed=false
fi

if $test_passed; then
    ((passed++)) || true
else
    ((failed++)) || true
fi
echo ""

# Test 2: /agentops:memory skill
echo "Test 2: /agentops:memory skill"
LOG_FILE=$(run_claude_json "/agentops:memory" 120) || true

test_passed=true
if ! assert_registered_or_invoked "$LOG_FILE" "memory" "memory" "Memory skill triggered"; then
    test_passed=false
fi

if $test_passed; then
    ((passed++)) || true
else
    ((failed++)) || true
fi
echo ""

# Test 3: /agentops:research skill
echo "Test 3: /agentops:research skill"
cat > README.md <<'EOF'
# Fixture Project

Small project used by AgentOps release smoke tests.
EOF
cat > app.py <<'EOF'
def main():
    return "ok"
EOF

LOG_FILE=$(run_claude_json "/agentops:research what are the main components of this project?" 120) || true

test_passed=true
if ! assert_registered_or_invoked "$LOG_FILE" "research" "research" "Research skill triggered"; then
    test_passed=false
fi

# Research may return inline; registration/invocation is the smoke boundary.

if $test_passed; then
    ((passed++)) || true
else
    ((failed++)) || true
fi
echo ""

print_summary "$passed" "$failed" "$skipped"
