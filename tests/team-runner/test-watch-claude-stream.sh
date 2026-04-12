#!/usr/bin/env bash
# Test watch-claude-stream.sh behavioral correctness
# Test harness: intentionally -uo (not -euo) to accumulate PASS/FAIL
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WATCHER="${REPO_ROOT}/lib/scripts/watch-claude-stream.sh"
FIXTURES="${SCRIPT_DIR}/fixtures"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

PASS=0
FAIL=0

assert_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [[ "$expected" == "$actual" ]]; then
        echo "  PASS: $desc"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $desc (expected=$expected, actual=$actual)"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Test: watch-claude-stream.sh ==="

# Test 1: Complete JSONL stream → exit 0, status=completed, output written
echo "Test 1: Complete JSONL stream"
cat "$FIXTURES/sample-claude-events.jsonl" | bash "$WATCHER" "$TMPDIR/t1-status.json" "$TMPDIR/t1-output.json"
assert_eq "exit code 0" "0" "$?"
assert_eq "status completed" "completed" "$(jq -r '.status' "$TMPDIR/t1-status.json")"
assert_eq "events count 3" "3" "$(jq -r '.events_count' "$TMPDIR/t1-status.json")"
assert_eq "input tokens" "120" "$(jq -r '.token_usage.input' "$TMPDIR/t1-status.json")"
assert_eq "worker status" "done" "$(jq -r '.status' "$TMPDIR/t1-output.json")"

# Test 2: Empty stream → exit 1, status=error
echo "Test 2: Empty stream"
echo "" | bash "$WATCHER" "$TMPDIR/t2-status.json" "$TMPDIR/t2-output.json"
T2_EXIT=$?
assert_eq "exit code 1" "1" "$T2_EXIT"
assert_eq "status error" "error" "$(jq -r '.status' "$TMPDIR/t2-status.json")"

# Test 3: Idle timeout → exit 2, status=timeout
echo "Test 3: Idle timeout"
(echo '{"type":"system","subtype":"init"}'; sleep 3) | CLAUDE_IDLE_TIMEOUT=1 timeout 5 bash "$WATCHER" "$TMPDIR/t3-status.json" "$TMPDIR/t3-output.json"
T3_EXIT=$?
assert_eq "exit code 2" "2" "$T3_EXIT"
assert_eq "status timeout" "timeout" "$(jq -r '.status' "$TMPDIR/t3-status.json")"
assert_eq "events count 1" "1" "$(jq -r '.events_count' "$TMPDIR/t3-status.json")"

# Test 4: Success without structured output → exit 1, status=error
echo "Test 4: Missing structured output"
echo '{"type":"result","subtype":"success","is_error":false,"usage":{"input_tokens":5,"output_tokens":3}}' | bash "$WATCHER" "$TMPDIR/t4-status.json" "$TMPDIR/t4-output.json"
T4_EXIT=$?
assert_eq "exit code 1" "1" "$T4_EXIT"
assert_eq "status error" "error" "$(jq -r '.status' "$TMPDIR/t4-status.json")"
assert_eq "output absent" "false" "$(test -f "$TMPDIR/t4-output.json" && echo true || echo false)"

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
