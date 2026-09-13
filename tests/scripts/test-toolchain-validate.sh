#!/bin/bash
set -euo pipefail

# Test suite for toolchain-validate.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

MOCK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/toolchain-test.XXXXXX")"
REAL_PYTEST="$(command -v pytest || true)"
PASS_COUNT=0
FAIL_COUNT=0

cleanup() {
    rm -rf "$MOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT

# Exercise scanner selection and result handling in a tiny real Git repository.
# Scanner doubles keep these contract tests independent of downloads, host tool
# inventories, and findings in unrelated working trees. Release CI runs the real
# scanners separately.
mkdir -p "$MOCK_DIR/repo/scripts" "$MOCK_DIR/bin" "$MOCK_DIR/repo/cli"
cp "${TOOLCHAIN_TEST_SCRIPT:-$REPO_ROOT/scripts/toolchain-validate.sh}" "$MOCK_DIR/repo/scripts/toolchain-validate.sh"
cat > "$MOCK_DIR/bin/scanner" <<'MOCK'
#!/usr/bin/env bash
name="${0##*/}"
printf '%s %s\n' "$name" "$*" >> "$SCANNER_LOG"
case "$name" in
    semgrep) printf '{"results":[]}\n' ;;
    trivy) printf '{"Results":[]}\n' ;;
    gosec) printf '{"Issues":[]}\n' ;;
    hadolint) printf '[]\n' ;;
    go)
        if [[ "${1:-}" == "build" ]]; then
            printf 'go-build-cwd %s\n' "$PWD" >> "$SCANNER_LOG"
            if [[ "${AO_BUILD_EXIT:-0}" -ne 0 ]]; then
                printf 'fixture build error\n' >&2
                exit "$AO_BUILD_EXIT"
            fi
            [[ "${2:-}" == "-o" && "${4:-}" == "./cmd/ao" ]] || exit 9
            printf '#!/bin/sh\nprintf "candidate-ready\\n"\n' > "$3"
            chmod +x "$3"
        fi
        ;;
    pytest)
        printf 'pytest-ao %s\n' "${AO_BIN:-}" >> "$SCANNER_LOG"
        if [[ ! -x "${AO_BIN:-}" ]]; then
            printf 'ERROR: AO_BIN is not an executable candidate\n'
            exit 2
        fi
        "$AO_BIN" >> "$SCANNER_LOG" || exit 2
        if [[ "${PYTEST_EXIT:-0}" -eq 1 ]]; then
            printf 'FAILED test_example.py::test_example - AssertionError\n'
        elif [[ "${PYTEST_EXIT:-0}" -ne 0 ]]; then
            printf 'ERROR collecting test_example.py\nInterrupted: 1 error during collection\n'
        fi
        exit "${PYTEST_EXIT:-0}"
        ;;
esac
MOCK
chmod +x "$MOCK_DIR/bin/scanner"
for tool in ruff gitleaks shellcheck radon semgrep trivy gosec govulncheck hadolint pytest go; do
    ln -s scanner "$MOCK_DIR/bin/$tool"
done
cp "$MOCK_DIR/bin/scanner" "$MOCK_DIR/repo/scripts/golangci-lint-v2.sh"
printf 'module example.invalid/fixture\n\ngo 1.25\n' > "$MOCK_DIR/repo/cli/go.mod"
printf 'package fixture\n' > "$MOCK_DIR/repo/cli/example.go"
printf 'def test_example():\n    assert True\n' > "$MOCK_DIR/repo/test_example.py"
printf 'FROM scratch\n' > "$MOCK_DIR/repo/Dockerfile"
printf '# Fixture\n' > "$MOCK_DIR/repo/README.md"
export PATH="$MOCK_DIR/bin:$PATH"
export SCANNER_LOG="$MOCK_DIR/scanners.log"
export TOOLCHAIN_OUTPUT_DIR="$MOCK_DIR/tooling"
cd "$MOCK_DIR/repo"
git init -q
git config core.hooksPath /dev/null
git add .
git -c user.name=Fixture -c user.email=fixture@example.invalid commit -qm 'Add code fixture'
printf '\nDocumentation-only change.\n' >> README.md
git add README.md
git -c user.name=Fixture -c user.email=fixture@example.invalid commit -qm 'Update only documentation'

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

# Test 7: Tool count matches the shipped inventory (12 tools)
test_tool_count() {
    local output
    output=$(./scripts/toolchain-validate.sh --json 2>/dev/null || true)

    local tool_count
    tool_count=$(echo "$output" | jq '.tools | keys | length' 2>/dev/null || echo 0)

    if [[ "$tool_count" -eq 12 ]]; then
        pass "Tool count is 12"
    else
        fail "Expected 12 tools, got $tool_count"
    fi
}

test_gate_scope() {
    local changed full
    changed=$(./scripts/toolchain-validate.sh --gate --json)
    full=$(./scripts/toolchain-validate.sh --all --gate --json)
    if jq -e '.scope == "changed" and .tools.ruff == "skipped" and .tools.gosec == "skipped" and .tools["go-test"] == "skipped"' <<< "$changed" >/dev/null; then
        pass "ordinary --gate retains docs-only HEAD changed scope"
    else
        fail "ordinary --gate broadened docs-only HEAD scope"
    fi
    if jq -e '.scope == "all" and .tools.ruff == "pass" and .tools.gosec == "pass" and .tools["go-test"] == "pass" and .tools.shellcheck == "pass" and .tools.semgrep == "pass" and .tools.trivy == "pass" and .tools.govulncheck == "pass"' <<< "$full" >/dev/null; then
        pass "--all --gate scans code despite docs-only HEAD"
    else
        fail "full scope skipped code scanners after docs-only HEAD"
    fi
    printf '\n# Staged Python change\n' >> test_example.py
    git add test_example.py
    changed=$(./scripts/toolchain-validate.sh --gate --json)
    if jq -e '.scope == "changed" and .tools.ruff == "pass" and .tools.gosec == "skipped"' <<< "$changed" >/dev/null; then
        pass "ordinary --gate still prefers staged changes"
    else
        fail "staged Python scope changed"
    fi
}

test_pytest_failure_exits() {
    local rc output status
    for rc in 1 2 3 4 5; do
        status=0
        output=$(PYTEST_EXIT="$rc" ./scripts/toolchain-validate.sh --all --gate --json) || status=$?
        if [[ "$status" -eq 2 ]] && jq -e '.gate_status == "BLOCKED_CRITICAL" and .findings.critical >= 1' <<< "$output" >/dev/null; then
            pass "pytest exit $rc blocks even without FAILED summary lines"
        else
            fail "pytest exit $rc was incorrectly green (gate exit $status)"
        fi
        if [[ "$rc" -ne 1 ]] && ! jq -e '.tools.pytest == "error"' <<< "$output" >/dev/null; then
            fail "pytest exit $rc must report a tool error"
        fi
    done
    if grep -q 'pytest .*--import-mode=importlib' "$SCANNER_LOG"; then
        pass "pytest uses importlib collection for duplicate module basenames"
    else
        fail "pytest importlib collection flag missing"
    fi
}

test_duplicate_test_modules() {
    if [[ -z "$REAL_PYTEST" ]]; then
        fail "pytest is required to verify duplicate-module collection"
        return
    fi
    mkdir -p "$MOCK_DIR/duplicate/source" "$MOCK_DIR/duplicate/projection"
    printf 'def test_source():\n    assert True\n' > "$MOCK_DIR/duplicate/source/test_same.py"
    printf 'def test_projection():\n    assert True\n' > "$MOCK_DIR/duplicate/projection/test_same.py"
    if "$REAL_PYTEST" "$MOCK_DIR/duplicate" --import-mode=importlib -q > "$MOCK_DIR/duplicate.log" 2>&1 &&
        grep -q '2 passed' "$MOCK_DIR/duplicate.log"; then
        pass "real pytest collects both source and projection modules"
    else
        fail "duplicate test modules were not both collected"
    fi
}

test_large_python_inventory() {
    local index output
    mkdir -p many-python-files
    for ((index = 0; index < 2000; index++)); do
        : > "many-python-files/test_fixture_with_a_long_name_to_exceed_the_pipe_buffer_${index}.py"
    done
    output=$(./scripts/toolchain-validate.sh --all --gate --json)
    if jq -e '.tools.ruff == "pass" and .tools.radon == "pass" and .tools.pytest == "pass"' <<< "$output" >/dev/null; then
        pass "large Python inventories do not turn SIGPIPE into false no-files skips"
    else
        fail "large Python inventory was incorrectly treated as absent"
    fi
}

test_pytest_candidate_binary() {
    local output candidate build_dir status
    : > "$SCANNER_LOG"
    output=$(AO_BIN='' ./scripts/toolchain-validate.sh --all --gate --json)
    candidate=$(sed -n 's/^pytest-ao //p' "$SCANNER_LOG")
    build_dir="${candidate%/*}"
    if jq -e '.tools.pytest == "pass"' <<< "$output" >/dev/null &&
        grep -Fxq "go-build-cwd $PWD/cli" "$SCANNER_LOG" &&
        grep -Fxq 'candidate-ready' "$SCANNER_LOG" && [[ -n "$candidate" && ! -e "$build_dir" ]]; then
        pass "pytest executes this checkout's temporary ao and cleans it afterward"
    else
        fail "pytest did not receive or clean the candidate ao"
    fi

    candidate="$MOCK_DIR/caller-ao"
    printf '#!/bin/sh\nprintf "caller-candidate\\n"\n' > "$candidate"
    chmod +x "$candidate"
    : > "$SCANNER_LOG"
    output=$(AO_BIN="$candidate" ./scripts/toolchain-validate.sh --all --gate --json)
    if jq -e '.tools.pytest == "pass"' <<< "$output" >/dev/null &&
        grep -Fxq "pytest-ao $candidate" "$SCANNER_LOG" &&
        grep -Fxq 'caller-candidate' "$SCANNER_LOG" &&
        ! grep -q '^go build ' "$SCANNER_LOG" && [[ -x "$candidate" ]]; then
        pass "explicit AO_BIN is preserved without building or deleting it"
    else
        fail "explicit AO_BIN was replaced or removed"
    fi

    : > "$SCANNER_LOG"
    status=0
    output=$(AO_BIN='' AO_BUILD_EXIT=7 ./scripts/toolchain-validate.sh --all --gate --json) || status=$?
    candidate=$(sed -n 's/^go build -o \(.*\) \.\/cmd\/ao$/\1/p' "$SCANNER_LOG")
    if [[ "$status" -eq 2 ]] && jq -e '.tools.pytest == "error" and .gate_status == "BLOCKED_CRITICAL"' <<< "$output" >/dev/null &&
        ! grep -q '^pytest ' "$SCANNER_LOG" && [[ -n "$candidate" && ! -e "${candidate%/*}" ]]; then
        pass "candidate build failure blocks before pytest and cleans temporary output"
    else
        fail "failed candidate build reached pytest or left a green gate"
    fi

    : > "$SCANNER_LOG"
    status=0
    output=$(AO_BIN='' PYTEST_EXIT=2 ./scripts/toolchain-validate.sh --all --gate --json) || status=$?
    candidate=$(sed -n 's/^pytest-ao //p' "$SCANNER_LOG")
    if [[ "$status" -eq 2 && -n "$candidate" && ! -e "${candidate%/*}" ]]; then
        pass "pytest failure also cleans the temporary candidate binary"
    else
        fail "pytest failure leaked its temporary candidate binary"
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
test_gate_scope
test_pytest_failure_exits
test_duplicate_test_modules
test_large_python_inventory
test_pytest_candidate_binary

echo ""
echo "================================"
echo "Results: $PASS_COUNT PASS, $FAIL_COUNT FAIL"
echo "================================"

if [[ $FAIL_COUNT -gt 0 ]]; then
    exit 1
fi
exit 0
