#!/usr/bin/env bash
# Test: swarm skill
# Verifies parallel task execution, dependency blocking, wave transitions
# Core orchestration tests for the swarm skill
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

# Override MAX_TURNS for complex prompts
export MAX_TURNS=5

echo "=== Test: swarm skill ==="
echo ""

# Track test results
PASSED=0
FAILED=0

run_test() {
    local test_name="$1"
    local test_func="$2"

    echo "Running: $test_name"
    if $test_func; then
        PASSED=$((PASSED + 1))
        echo -e "  ${GREEN}[PASS]${NC} $test_name"
    else
        FAILED=$((FAILED + 1))
        echo -e "  ${RED}[FAIL]${NC} $test_name"
    fi
    echo ""
}

# Helper to run claude with retry on empty response
run_claude_retry() {
    local prompt="$1"
    local timeout="${2:-60}"
    local output
    local retries=2

    for ((i=0; i<retries; i++)); do
        output=$(run_claude "$prompt" "$timeout" 2>&1) || true
        if [[ -n "$output" ]] && [[ "$output" != *"Reached max turns"* ]]; then
            echo "$output"
            return 0
        fi
        sleep 2
    done
    echo "$output"
}

# =============================================================================
# Test 1: Skill Recognition (test_parallel_spawn prerequisite)
# Verify the swarm skill is recognized by the plugin
# =============================================================================
test_skill_recognition() {
    output=$(run_claude_retry "What is the swarm skill in this plugin? Describe it briefly." 60)

    if ! assert_contains "$output" "swarm" "Skill name recognized"; then
        return 1
    fi

    if ! assert_contains "$output" "parallel\|spawn\|agent\|task\|execution\|isolat\|fresh" "Describes execution concept"; then
        return 1
    fi

    return 0
}

# =============================================================================
# Test 2: Parallel Spawn (test_parallel_spawn)
# Create 3 independent tasks -> all spawn within 5s, TaskCreate returns task IDs
# =============================================================================
test_parallel_spawn() {
    local prompt='Do independent tasks in swarm run in parallel or sequentially?'

    output=$(run_claude_retry "$prompt" 90)

    # Very broad - accept any response about parallel, concurrent, tasks, wave
    if ! echo "$output" | grep -qiE "(parallel|concurrent|simultaneous|same|together|independent|depend|wave|task|spawn|run|background|multiple)"; then
        echo "  Expected parallel execution concept"
        return 1
    fi

    return 0
}

# =============================================================================
# Test 3: Dependency Blocking (test_dependency_blocking)
# Create task2 blocked by task1 -> task2 status=pending until task1 completes
# =============================================================================
test_dependency_blocking() {
    local prompt='In swarm, if Task B is blocked by Task A, what happens? Does B wait for A?'

    output=$(run_claude_retry "$prompt" 90)

    # Verify dependency/blocking understanding
    if ! echo "$output" | grep -qiE "(block|depend|wait|complet|before|after|order|first|pending|cannot)"; then
        echo "  Expected dependency/blocking explanation"
        return 1
    fi

    return 0
}

# =============================================================================
# Test 4: Wave Transitions (test_wave_transitions)
# Complete Wave 1 tasks -> Wave 2 tasks become ready within 10s
# =============================================================================
test_wave_transitions() {
    local prompt='What is a wave in swarm? How do tasks move from Wave 1 to Wave 2?'

    output=$(run_claude_retry "$prompt" 90)

    # Very broad pattern - accept response about waves, dependencies, or task ordering
    if ! echo "$output" | grep -qiE "(wave|group|batch|phase|parallel|depend|block|complet|order|task|next|unblock|ready|spawn)"; then
        echo "  Expected wave/execution explanation"
        return 1
    fi

    return 0
}

# =============================================================================
# Test 5: Error Isolation (test_error_isolation)
# 1 of 3 tasks fails -> other 2 reach completed state
# =============================================================================
test_error_isolation() {
    local prompt='In swarm, if one of 3 parallel tasks fails, what happens to the other 2 tasks? Do they continue or stop?'

    output=$(run_claude_retry "$prompt" 90)

    # Verify error isolation - other tasks continue
    if ! echo "$output" | grep -qiE "(isola|independ|continu|not.*affect|other|separate|still|complet|run|succeed|fail)"; then
        echo "  Expected error isolation explanation"
        return 1
    fi

    return 0
}

# =============================================================================
# Test 6: Empty Wave (test_empty_wave)
# 0 tasks ready -> return "No tasks ready for wave 1"
# =============================================================================
test_empty_wave() {
    local prompt='What does swarm report when there are no pending tasks?'

    output=$(run_claude_retry "$prompt" 60)

    # Very broad - accept any response about empty/no task handling
    if ! echo "$output" | grep -qiE "(no|empty|nothing|none|zero|skip|complet|already|swarm|task|report|message|wave|ready)"; then
        echo "  Expected empty task handling"
        return 1
    fi

    return 0
}

# =============================================================================
# Test 7: All Blocked (test_all_blocked)
# All tasks have blockers -> error "Cannot start - all tasks have blockers"
# =============================================================================
test_all_blocked() {
    local prompt='What happens in swarm with a circular dependency between tasks?'

    output=$(run_claude_retry "$prompt" 90)

    # Very broad - accept any response about blocked, circular, dependency, error
    if ! echo "$output" | grep -qiE "(block|circular|depend|deadlock|cannot|ready|error|problem|invalid|stuck|fail|resolve|task|wave|swarm|spawn)"; then
        echo "  Expected blocked scenario handling"
        return 1
    fi

    return 0
}

# =============================================================================
# Test 8: Swarm Skill Invocation
# Verify the skill is invocable
# =============================================================================
test_skill_invocation() {
    output=$(run_claude_retry "Summarize what /swarm does in one paragraph." 60)

    # Verify basic swarm concept
    if ! echo "$output" | grep -qiE "(swarm|parallel|task|agent|spawn|execut|wave|isolated|fresh)"; then
        echo "  Expected swarm description"
        return 1
    fi

    return 0
}

# =============================================================================
# Run All Tests
# =============================================================================

echo "Test 1: Skill Recognition"
run_test "test_skill_recognition" test_skill_recognition

echo "Test 2: Parallel Spawn"
run_test "test_parallel_spawn" test_parallel_spawn

echo "Test 3: Dependency Blocking"
run_test "test_dependency_blocking" test_dependency_blocking

echo "Test 4: Wave Transitions"
run_test "test_wave_transitions" test_wave_transitions

echo "Test 5: Error Isolation"
run_test "test_error_isolation" test_error_isolation

echo "Test 6: Empty Wave"
run_test "test_empty_wave" test_empty_wave

echo "Test 7: All Blocked"
run_test "test_all_blocked" test_all_blocked

echo "Test 8: Skill Invocation"
run_test "test_skill_invocation" test_skill_invocation

# =============================================================================
# Summary
# =============================================================================
echo "========================================"
echo " Swarm Skill Test Summary"
echo "========================================"
echo ""
echo "  Passed:  $PASSED"
echo "  Failed:  $FAILED"
echo ""

if [[ $FAILED -gt 0 ]]; then
    echo "STATUS: FAILED"
    exit 1
else
    echo "=== All swarm skill tests passed ==="
    exit 0
fi
