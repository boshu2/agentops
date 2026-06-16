# Testing Skills

Comprehensive guide for writing and running skill tests in AgentOps.

---

## Overview

The AgentOps skill test framework provides utilities for validating Claude Code skills through automated integration testing. Tests verify skill recognition, behavior, and output quality by invoking Claude with specific prompts and asserting on the responses.

**Key Principles:**
- Tests run Claude Code with the plugin loaded
- Assertions validate output content and tool behavior
- JSON logging enables inspection of tool calls
- Timeouts prevent hung tests
- Retry logic handles transient failures

---

## Test Framework Location

```
tests/claude-code/
├── test-helpers.sh          # Core test utilities (source this)
├── logs/                    # JSON logs from test runs
├── test-<skill>-skill.sh    # Individual skill tests
└── ...
```

---

## test-helpers.sh Reference

### Configuration Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MAX_TURNS` | `3` | Maximum conversation turns per test |
| `DEFAULT_TIMEOUT` | `120` | Default timeout in seconds |
| `LOG_DIR` | `$SCRIPT_DIR/logs` | Directory for JSON logs |
| `REPO_ROOT` | Auto-detected | Plugin repository root |

Override in your test script:
```bash
export MAX_TURNS=5         # For complex prompts
export DEFAULT_TIMEOUT=90  # For longer operations
```

### Core Functions

#### `run_claude`

Run Claude Code with a prompt and capture plain text output.

```bash
run_claude "prompt text" [timeout_seconds]
```

**Parameters:**
- `prompt` (required): The prompt to send to Claude
- `timeout` (optional): Timeout in seconds (default: `$DEFAULT_TIMEOUT`)

**Returns:**
- Stdout: Claude's response text
- Exit code: 0 on success, non-zero on failure/timeout

**Example:**
```bash
output=$(run_claude "What is the research skill?" 45)
```

**Behavior:**
- Loads plugin from `$REPO_ROOT`
- Skips permission prompts (`--dangerously-skip-permissions`)
- Limits conversation turns (`--max-turns`)
- Returns response via stdout, errors via stderr

---

#### `run_claude_json`

Run Claude Code with JSON stream output for tool call analysis.

```bash
run_claude_json "prompt text" [timeout_seconds]
```

**Parameters:**
- `prompt` (required): The prompt to send to Claude
- `timeout` (optional): Timeout in seconds (default: `$DEFAULT_TIMEOUT`)

**Returns:**
- Stdout: Path to the JSONL log file
- Exit code: 0 on success, non-zero on failure/timeout

**Example:**
```bash
log_file=$(run_claude_json "Invoke /swarm to list tasks")
assert_skill_triggered "$log_file" "swarm" "Swarm skill invoked"
```

**Behavior:**
- Uses `--output-format stream-json`
- Saves output to timestamped file in `$LOG_DIR`
- File persists after test for debugging

---

### Assertion Functions

#### `assert_contains`

Check if output contains a pattern (case-insensitive).

```bash
assert_contains "output" "pattern" "test name"
```

**Parameters:**
- `output`: Text to search in
- `pattern`: Grep pattern to find (supports `\|` for OR)
- `test_name`: Label for test output

**Example:**
```bash
assert_contains "$output" "research\|explore\|investigate" "Describes research"
```

---

#### `assert_not_contains`

Check if output does NOT contain a pattern.

```bash
assert_not_contains "output" "pattern" "test name"
```

**Example:**
```bash
assert_not_contains "$output" "error\|fail" "No errors in output"
```

---

#### `assert_order`

Check if pattern A appears before pattern B.

```bash
assert_order "output" "pattern_a" "pattern_b" "test name"
```

**Example:**
```bash
assert_order "$output" "Research" "Implement" "Research before implement"
```

---

#### `assert_skill_triggered`

Check if a skill was invoked (requires JSON log).

```bash
assert_skill_triggered "log_file" "skill-name" "test name"
```

**Parameters:**
- `log_file`: Path to JSONL log from `run_claude_json`
- `skill_name`: Name of skill (without namespace prefix)
- `test_name`: Label for test output

**Example:**
```bash
log_file=$(run_claude_json "Use /research to explore this codebase")
assert_skill_triggered "$log_file" "research" "Research skill triggered"
```

**Note:** Handles namespaced skills (e.g., `agentops:research` matches `research`).

---

#### `assert_no_premature_tools`

Check that no tools (Bash, Read, Write, Edit, Glob, Grep) were called before the Skill invocation.

```bash
assert_no_premature_tools "log_file" "test name"
```

**Example:**
```bash
assert_no_premature_tools "$log_file" "No tools before skill invocation"
```

---

#### `assert_tool_called`

Check if a specific tool was called.

```bash
assert_tool_called "log_file" "ToolName" "test name"
```

**Example:**
```bash
assert_tool_called "$log_file" "Read" "Read tool was used"
```

---

#### `assert_tool_not_called`

Check if a specific tool was NOT called.

```bash
assert_tool_not_called "log_file" "ToolName" "test name"
```

**Example:**
```bash
assert_tool_not_called "$log_file" "Write" "No writes occurred"
```

---

### Utility Functions

#### `create_test_project`

Create a temporary test directory with standard AgentOps structure.

```bash
test_dir=$(create_test_project)
```

**Returns:** Path to temp directory containing:
- `.agents/learnings/`
- `.agents/research/`
- `.beads/`

---

#### `cleanup_test_project`

Remove a test project directory.

```bash
cleanup_test_project "$test_dir"
```

---

#### `cleanup_logs`

Clean up old log files, keeping the most recent 50.

```bash
cleanup_logs
```

---

#### `print_summary`

Print a formatted test summary.

```bash
print_summary passed failed skipped
```

**Example:**
```bash
print_summary 5 1 0  # 5 passed, 1 failed, 0 skipped
```

---

## Writing Skill Tests

### Basic Test Structure

```bash
#!/usr/bin/env bash
# Test: <skill-name> skill
# Verifies <description>
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/test-helpers.sh"

echo "=== Test: <skill-name> skill ==="
echo ""

# Test 1: Skill recognition
echo "Test 1: Skill recognition..."

output=$(run_claude "What is the <skill-name> skill in this plugin? Describe it briefly." 45)

if assert_contains "$output" "<skill-name>" "Skill name recognized"; then
    :
else
    exit 1
fi

if assert_contains "$output" "<key-concept>\|<alternative>" "Describes <purpose>"; then
    :
else
    exit 1
fi

echo ""

# Additional tests...

echo "=== All <skill-name> skill tests passed ==="
```

### Test Patterns

#### Pattern 1: Recognition Test
Verify the skill is recognized by Claude.

```bash
output=$(run_claude "What is the handoff skill in this plugin? Describe it briefly." 45)
assert_contains "$output" "handoff" "Skill name recognized"
assert_contains "$output" "session\|continu\|pause\|context" "Describes session continuation"
```

#### Pattern 2: Feature Test
Verify specific features are understood.

```bash
output=$(run_claude "What message categories does the inbox skill handle? List them." 45)
assert_contains "$output" "HELP_REQUEST\|pending\|completion\|done" "Mentions message categories"
```

#### Pattern 3: Behavior Test
Verify behavioral understanding.

```bash
output=$(run_claude "In swarm, if one of 3 parallel tasks fails, what happens to the other 2?" 90)
assert_contains "$output" "isola\|independ\|continu\|not.*affect" "Error isolation explained"
```

#### Pattern 4: Retry Logic (for complex/flaky tests)

```bash
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
```

#### Pattern 5: Structured Test Runner

```bash
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

# Define test functions
test_skill_recognition() {
    output=$(run_claude "What is the swarm skill?" 60)
    assert_contains "$output" "swarm" "Skill recognized"
}

# Run tests
run_test "Skill Recognition" test_skill_recognition

# Summary
if [[ $FAILED -gt 0 ]]; then
    exit 1
fi
```

---

## Coverage Requirements

### Minimum Coverage Per Skill

| Test Type | Required | Description |
|-----------|----------|-------------|
| Recognition | Yes | Skill is recognized by name |
| Purpose | Yes | Skill's purpose is understood |
| Key Features | 1+ | At least one feature-specific test |
| Edge Cases | Recommended | Error handling, empty inputs, etc. |

### Coverage by Skill Complexity

| Skill Type | Min Tests | Example |
|------------|-----------|---------|
| Simple (library) | 2-3 | standards, handoff |
| Standard | 3-4 | inbox, implement, vibe |
| Complex (orchestration) | 6-8 | swarm, crank |

### Current Test Coverage

| Skill | Tests | Coverage |
|-------|-------|----------|
| swarm | 8 | Full (recognition, spawn, blocking, waves, errors) |
| inbox | 3 | Standard (recognition, categories, threads) |
| handoff | 3 | Standard (recognition, output, context) |
| standards | 3 | Standard (recognition, languages, library) |
| vibe | 2 | Minimal (recognition, domains) |
| implement | 2 | Minimal (recognition, lifecycle) |
| research | - | None |
| plan | - | None |
| crank | - | None |
| retro | - | None |

### Quarantine Policy

Tests in `tests/_quarantine/` are legacy integration tests that require a running Claude Code instance to execute. They were moved to quarantine because they cannot run in CI without external services (a live Claude Code process, API access, etc.).

**Promotion path:** Tests move out of quarantine when they can run headlessly — either through mock-based execution (stubbing Claude responses) or API-stubbed harnesses that don't require a live Claude Code session.

---

## Running Tests

### Single Test

```bash
cd <repo-root>
./tests/claude-code/test-swarm-skill.sh
```

### All Tests

```bash
for test in tests/claude-code/test-*-skill.sh; do
    echo "Running $test..."
    bash "$test" || echo "FAILED: $test"
done
```

### With Custom Settings

```bash
MAX_TURNS=10 DEFAULT_TIMEOUT=180 ./tests/claude-code/test-swarm-skill.sh
```

---

## Troubleshooting

### Test Hangs

**Symptom:** Test never completes.

**Causes:**
1. Timeout too short for prompt complexity
2. Claude stuck in a loop
3. Permission prompt (should be skipped)

**Solutions:**
- Increase timeout: `run_claude "prompt" 180`
- Increase `MAX_TURNS` for complex prompts
- Check log files in `tests/claude-code/logs/`

---

### Empty Response

**Symptom:** `run_claude` returns empty output.

**Causes:**
1. Claude exceeded max turns
2. Prompt triggered rate limit
3. Plugin not loaded correctly

**Solutions:**
- Use retry pattern (see Pattern 4 above)
- Check for "Reached max turns" in output
- Verify `REPO_ROOT` is correct

---

### Skill Not Triggered

**Symptom:** `assert_skill_triggered` fails.

**Causes:**
1. Prompt doesn't mention skill clearly
2. Skill name mismatch (namespace issue)
3. Claude chose different approach

**Solutions:**
- Make prompt explicit: "Use /research to..."
- Check skill namespace in log file
- Review tool calls in JSON log:
  ```bash
  grep '"name":' tests/claude-code/logs/claude-*.jsonl | head -10
  ```

---

### Flaky Tests

**Symptom:** Test passes sometimes, fails other times.

**Causes:**
1. Claude response variation
2. Timeout boundary
3. External dependencies

**Solutions:**
- Use broader patterns: `"parallel\|concurrent\|simultaneous"`
- Add retry logic
- Increase timeout with margin
- Test core concepts, not exact wording

---

### Log File Not Found

**Symptom:** Assertion complains about missing log file.

**Causes:**
1. `run_claude_json` failed before writing
2. Wrong variable passed to assertion
3. Log directory permissions

**Solutions:**
- Check if `$LOG_DIR` exists and is writable
- Capture return value: `log_file=$(run_claude_json ...)`
- Check for timeout/error in stderr

---

### Debugging Failed Tests

1. **Check the log file:**
   ```bash
   ls -la tests/claude-code/logs/
   cat tests/claude-code/logs/claude-<timestamp>.jsonl | head -50
   ```

2. **Extract tool calls:**
   ```bash
   grep '"name":' tests/claude-code/logs/claude-*.jsonl | tail -20
   ```

3. **Check skill invocations:**
   ```bash
   grep -E '"skill":"' tests/claude-code/logs/claude-*.jsonl
   ```

4. **Run interactively in the harness under test:**
   ```text
   Open an interactive session with the plugin installed and ask:
   "What is the swarm skill?"

   Do not use Claude print mode.
   ```

---

## Best Practices

### Do

- Use broad patterns with alternatives: `"parallel\|concurrent\|spawn"`
- Set appropriate timeouts (45s for simple, 90s+ for complex)
- Include retry logic for integration tests
- Test concepts, not exact wording
- Clean up test projects in trap handlers
- Keep tests independent (no shared state)

### Don't

- Don't test for exact output strings
- Don't set `MAX_TURNS` too low for complex prompts
- Don't skip recognition tests (they verify plugin loading)
- Don't create tests that depend on external services
- Don't use `-uall` flag with git commands (memory issues)

---

## Example: Complete Test File

See `<repo-root>/tests/claude-code/test-swarm-skill.sh` for a comprehensive example with:
- 8 tests covering full skill behavior
- Retry logic for transient failures
- Structured test runner with pass/fail tracking
- Summary output with exit code
