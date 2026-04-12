#!/usr/bin/env bash
# Test watch-codex-stream.sh behavioral correctness
# Test harness: intentionally -uo (not -euo) to accumulate PASS/FAIL
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
WATCHER="${REPO_ROOT}/lib/scripts/watch-codex-stream.sh"
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

echo "=== Test: watch-codex-stream.sh ==="

# Test 1: Complete JSONL stream → exit 0, status=completed
echo "Test 1: Complete JSONL stream"
cat "$FIXTURES/sample-events.jsonl" | bash "$WATCHER" "$TMPDIR/t1-status.json"
assert_eq "exit code 0" "0" "$?"
assert_eq "status completed" "completed" "$(jq -r '.status' "$TMPDIR/t1-status.json")"
assert_eq "events count 7" "7" "$(jq -r '.events_count' "$TMPDIR/t1-status.json")"
assert_eq "input tokens" "1500" "$(jq -r '.token_usage.input' "$TMPDIR/t1-status.json")"
assert_eq "output tokens" "800" "$(jq -r '.token_usage.output' "$TMPDIR/t1-status.json")"

# Test 2: Empty stream → exit 1, status=error
echo "Test 2: Empty stream"
echo "" | bash "$WATCHER" "$TMPDIR/t2-status.json"
T2_EXIT=$?
assert_eq "exit code 1" "1" "$T2_EXIT"
assert_eq "status error" "error" "$(jq -r '.status' "$TMPDIR/t2-status.json")"
assert_eq "events count 0" "0" "$(jq -r '.events_count' "$TMPDIR/t2-status.json")"

# Test 3: Idle timeout → exit 2, status=timeout
echo "Test 3: Idle timeout"
(echo '{"type":"turn.started"}'; sleep 3) | CODEX_IDLE_TIMEOUT=1 timeout 5 bash "$WATCHER" "$TMPDIR/t3-status.json"
T3_EXIT=$?
assert_eq "exit code 2" "2" "$T3_EXIT"
assert_eq "status timeout" "timeout" "$(jq -r '.status' "$TMPDIR/t3-status.json")"
assert_eq "events count 1" "1" "$(jq -r '.events_count' "$TMPDIR/t3-status.json")"

# Test 4: Malformed JSONL mixed with valid
echo "Test 4: Malformed JSONL handling"
(echo 'not json at all'; echo '{"type":"turn.completed","usage":{"input_tokens":5,"output_tokens":3}}') | bash "$WATCHER" "$TMPDIR/t4-status.json"
T4_EXIT=$?
assert_eq "exit code 0" "0" "$T4_EXIT"
assert_eq "status completed" "completed" "$(jq -r '.status' "$TMPDIR/t4-status.json")"

# Test 5: Single turn.completed only
echo "Test 5: Single event"
echo '{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}' | bash "$WATCHER" "$TMPDIR/t5-status.json"
assert_eq "exit code 0" "0" "$?"
assert_eq "input 100" "100" "$(jq -r '.token_usage.input' "$TMPDIR/t5-status.json")"

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
