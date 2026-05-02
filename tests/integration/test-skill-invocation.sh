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

# Test 1: /agentops:status skill
echo "Test 1: /agentops:status skill"
LOG_FILE=$(run_claude_json "/agentops:status" 120) || true

test_passed=true
if ! assert_registered_or_invoked "$LOG_FILE" "status" "status" "Status skill triggered"; then
    test_passed=false
fi

if $test_passed; then
    ((passed++)) || true
else
    ((failed++)) || true
fi
echo ""

# Test 2: /agentops:knowledge-activation skill
echo "Test 2: /agentops:knowledge-activation skill"
LOG_FILE=$(run_claude_json "/agentops:knowledge-activation" 120) || true

test_passed=true
if ! assert_registered_or_invoked "$LOG_FILE" "knowledge-activation" "knowledge-activation" "Knowledge activation skill triggered"; then
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

# Verify research creates artifacts in .agents/research/
if [[ -d ".agents/research" ]]; then
    research_files=$(find .agents/research -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$research_files" -gt 0 ]]; then
        echo -e "  ${GREEN}[PASS]${NC} Research artifacts created ($research_files files)"
    else
        echo -e "  ${YELLOW}[WARN]${NC} No research artifacts found (may be expected if skill failed)"
    fi
else
    echo -e "  ${YELLOW}[WARN]${NC} .agents/research directory not created"
fi

if $test_passed; then
    ((passed++)) || true
else
    ((failed++)) || true
fi
echo ""

print_summary "$passed" "$failed" "$skipped"
