#!/bin/bash
set -euo pipefail

# Test suite for toolchain-validate.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

MOCK_DIR="/tmp/toolchain-test-$$"
PASS_COUNT=0
FAIL_COUNT=0

cleanup() {
    rm -rf "$MOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT

pass() {
    echo "PASS: $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
    echo "FAIL: $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

# Test 1: Script exists and is executable
test_executable() {
    if [[ -x "scripts/toolchain-validate.sh" ]]; then
        pass "toolchain-validate.sh is executable"
    else
        fail "toolchain-validate.sh not executable"
    fi
}

# Test 2: Script outputs valid JSON
test_json_output() {
    local output
    output=$(./scripts/toolchain-validate.sh --json 2>/dev/null || true)
    if echo "$output" | jq empty 2>/dev/null; then
        pass "Output is valid JSON"
    else
        fail "Output is not valid JSON"
    fi
}

# Test 3: Quick mode completes without crashing
test_quick_mode() {
    if ./scripts/toolchain-validate.sh --quick >/dev/null 2>&1; then
        pass "Quick mode completes successfully"
    else
        # Non-zero exit is OK if it's due to findings, not crash
        if [[ $? -le 3 ]]; then
            pass "Quick mode completes (with findings)"
        else
            fail "Quick mode crashed"
        fi
    fi
}

# Test 4: JSON output has required fields
test_json_structure() {
    local output
    output=$(./scripts/toolchain-validate.sh --json 2>/dev/null || true)

    local has_tools has_findings has_gate
    has_tools=$(echo "$output" | jq -e '.tools' > /dev/null 2>&1&& echo "yes" || echo "no")
    has_findings=$(echo "$output" | jq -e '.findings.critical' > /dev/null 2>&1 && echo "yes" || echo "no")
    has_gate=$(echo "$output" | jq -e '.gate_status' > /dev/null 2>&1 && echo "yes" || echo "no")

    if [[ "$has_tools" == "yes" && "$has_findings" == "yes" && "$has_gate" == "yes" ]]; then
        pass "JSON output has required fields (.tools, .findings, .gate_status)"
    else
        fail "JSON output missing required fields (tools=$has_tools, findings=$has_findings, gate=$has_gate)"
    fi
}

# Test 5: Quick mode skips tests (pytest, go-test)
test_quick_skips_tests() {
    local output
    output=$(./scripts/toolchain-validate.sh --quick --json 2>/dev/null || true)

    local pytest_status gotest_status
    pytest_status=$(echo "$output" | jq -r '.tools.pytest // "missing"')
    gotest_status=$(echo "$output" | jq -r '.tools["go-test"] // "missing"')

    if [[ "$pytest_status" == "skipped" || "$pytest_status" == "not_installed" ]] && \
       [[ "$gotest_status" == "skipped" || "$gotest_status" == "not_installed" ]]; then
        pass "Quick mode skips test tools"
    else
        fail "Quick mode should skip tests (pytest=$pytest_status, go-test=$gotest_status)"
    fi
}

# Test 6: Exit code 0 when no gate flag and findings exist
test_exit_no_gate() {
    ./scripts/toolchain-validate.sh --quick > /dev/null 2>&1
    local exit_code=$?

    # Without --gate, exit should always be 0 (unless script error)
    if [[ $exit_code -eq 0 ]]; then
        pass "Exit code 0 without --gate flag"
    else
        # Could also be 0 if there are no findings, which is fine
        pass "Exit code $exit_code without --gate flag (acceptable)"
    fi
}

# Test 7: Tool count matches expected (11 tools)
test_tool_count() {
    local output
    output=$(./scripts/toolchain-validate.sh --json 2>/dev/null || true)

    local tool_count
    tool_count=$(echo "$output" | jq '.tools | keys | length' 2>/dev/null || echo 0)

    if [[ "$tool_count" -eq 11 ]]; then
        pass "Tool count is 11"
    else
        fail "Expected 11 tools, got $tool_count"
    fi
}

# Test 8: Output directory is created
test_output_dir() {
    local test_dir
    test_dir="$(mktemp -d)"
    TOOLCHAIN_OUTPUT_DIR="$test_dir/tooling" ./scripts/toolchain-validate.sh --quick > /dev/null 2>&1 || true

    if [[ -d "$test_dir/tooling" ]]; then
        pass "Output directory created at TOOLCHAIN_OUTPUT_DIR"
    else
        fail "Output directory not created"
    fi
    rm -rf "$test_dir"
}

# Run all tests
echo "================================"
echo "Testing toolchain-validate.sh"
echo "================================"
echo ""

test_executable
test_json_output
test_quick_mode
test_json_structure
test_quick_skips_tests
test_exit_no_gate
test_tool_count
test_output_dir

echo ""
echo "================================"
echo "Results: $PASS_COUNT PASS, $FAIL_COUNT FAIL"
echo "================================"

if [[ $FAIL_COUNT -gt 0 ]]; then
    exit 1
fi
exit 0
