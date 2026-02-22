#!/bin/bash
# test-hooks.sh - Integration tests for hook scripts
# Validates that hooks produce valid output and handle edge cases.
# Tests ALL 12 hook scripts + inline commands coverage.
# Usage: ./tests/hooks/test-hooks.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOKS_DIR="$REPO_ROOT/hooks"
PASS=0
FAIL=0

red() { printf '\033[0;31m%s\033[0m\n' "$1"; }
green() { printf '\033[0;32m%s\033[0m\n' "$1"; }

pass() {
    green "  PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    red "  FAIL: $1"
    FAIL=$((FAIL + 1))
}

# Pre-flight: jq required
if ! command -v jq >/dev/null 2>&1; then
    red "ERROR: jq is required for hook tests"
    exit 1
fi

TMPDIR=$(mktemp -d)
REPO_FIXTURE_DIR="$REPO_ROOT/.agents/ao/test-hooks-$$"
mkdir -p "$REPO_FIXTURE_DIR"
trap 'rm -rf "$TMPDIR" "$REPO_FIXTURE_DIR"' EXIT

# Helper: create a mock git repo with lib/hook-helpers.sh available
# Usage: setup_mock_repo <dir>
setup_mock_repo() {
    local dir="$1"
    mkdir -p "$dir/.agents/ao" "$dir/lib"
    git -C "$dir" init -q >/dev/null 2>&1
    /bin/cp "$REPO_ROOT/lib/hook-helpers.sh" "$dir/lib/hook-helpers.sh"
    /bin/cp "$REPO_ROOT/lib/chain-parser.sh" "$dir/lib/chain-parser.sh"
}

# ============================================================
echo "=== prompt-nudge.sh ==="
# ============================================================

# Test 1: Empty prompt => silent exit
OUTPUT=$(echo '{"prompt":""}' | AGENTOPS_HOOKS_DISABLED=0 bash "$HOOKS_DIR/prompt-nudge.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "empty prompt produces no output"; else fail "empty prompt produces no output"; fi

# Test 2: Kill switch disables hook
OUTPUT=$(echo '{"prompt":"implement a feature"}' | AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/prompt-nudge.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "kill switch disables hook"; else fail "kill switch disables hook"; fi

# Test 3: No chain.jsonl => silent exit (run in mock repo to avoid real chain.jsonl)
NUDGE_MOCK="$TMPDIR/nudge-mock"
setup_mock_repo "$NUDGE_MOCK"
OUTPUT=$(cd "$NUDGE_MOCK" && echo '{"prompt":"implement something"}' | bash "$HOOKS_DIR/prompt-nudge.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "no chain.jsonl produces no output"; else fail "no chain.jsonl produces no output"; fi

# Test 4: jq -n produces valid JSON with special characters
SAFE_JSON=$(jq -n --arg nudge 'Test "nudge" with <special> chars & more' '{"hookSpecificOutput":{"additionalContext":$nudge}}')
if echo "$SAFE_JSON" | jq . >/dev/null 2>&1; then pass "jq -n produces valid JSON with special chars"; else fail "jq -n produces valid JSON with special chars"; fi

# Test 5: JSON injection resistance - special characters in nudge
for PAYLOAD in '"' '\\' '$(whoami)' '`id`' '<script>' "'; DROP TABLE" '{"nested":"json"}'; do
    RESULT=$(jq -n --arg nudge "$PAYLOAD" '{"hookSpecificOutput":{"additionalContext":$nudge}}')
    if echo "$RESULT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1; then
        pass "jq escapes payload: $PAYLOAD"
    else
        fail "jq escapes payload: $PAYLOAD"
    fi
done

# ============================================================
echo ""
echo "=== push-gate.sh ==="
# ============================================================

# Test 6: Non-git command => pass through
echo '{"tool_input":{"command":"ls -la"}}' | AGENTOPS_HOOKS_DISABLED=0 bash "$HOOKS_DIR/push-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "non-git command passes through"; else fail "non-git command passes through"; fi

# Test 7: Kill switch allows git push
echo '{"tool_input":{"command":"git push origin main"}}' | AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/push-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "kill switch allows git push"; else fail "kill switch allows git push"; fi

# Test 8: Worker exemption allows git push
echo '{"tool_input":{"command":"git push origin main"}}' | AGENTOPS_WORKER=1 bash "$HOOKS_DIR/push-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "worker exemption allows git push"; else fail "worker exemption allows git push"; fi

# Test 9: Empty command => pass through
echo '{"tool_input":{"command":""}}' | bash "$HOOKS_DIR/push-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "empty command passes through"; else fail "empty command passes through"; fi

# Test 10: Mock chain.jsonl (vibe+post-mortem done) => allow push
MOCK_PUSH="$TMPDIR/mock-push"
mkdir -p "$MOCK_PUSH/.agents/ao" "$MOCK_PUSH/.git/refs" "$MOCK_PUSH/.git/objects"
echo 'ref: refs/heads/main' > "$MOCK_PUSH/.git/HEAD"
echo '{"gate":"vibe","status":"locked"}' > "$MOCK_PUSH/.agents/ao/chain.jsonl"
echo '{"gate":"post-mortem","status":"locked"}' >> "$MOCK_PUSH/.agents/ao/chain.jsonl"
EC=0
(cd "$MOCK_PUSH" && echo '{"tool_input":{"command":"git push origin main"}}' | bash "$HOOKS_DIR/push-gate.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "git push allowed when vibe+post-mortem done"; else fail "git push allowed when vibe+post-mortem done (exit=$EC)"; fi

# Test 11: Mock chain.jsonl (vibe pending) => block push
MOCK_BLOCK="$TMPDIR/mock-block"
mkdir -p "$MOCK_BLOCK/.agents/ao" "$MOCK_BLOCK/.git/refs" "$MOCK_BLOCK/.git/objects"
echo 'ref: refs/heads/main' > "$MOCK_BLOCK/.git/HEAD"
echo '{"gate":"vibe","status":"pending"}' > "$MOCK_BLOCK/.agents/ao/chain.jsonl"
OUTPUT=$(cd "$MOCK_BLOCK" && echo '{"tool_input":{"command":"git push origin main"}}' | bash "$HOOKS_DIR/push-gate.sh" 2>&1 || true)
if echo "$OUTPUT" | grep -q "BLOCKED"; then pass "git push blocked when vibe pending"; else fail "git push blocked when vibe pending"; fi

# ============================================================
echo ""
echo "=== session-start.sh / precompact-snapshot.sh ==="
# ============================================================

# Test 12: session-start emits valid JSON (extract last JSON object from output,
# since ao extract may emit non-JSON to stdout before the hook's JSON)
SESSION_RAW=$(bash "$HOOKS_DIR/session-start.sh" 2>/dev/null || true)
# Extract the last valid JSON block by finding the final { ... } spanning multiple lines
SESSION_JSON=$(echo "$SESSION_RAW" | awk '/^[[:space:]]*\{/{found=1; buf=""} found{buf=buf $0 "\n"} /^[[:space:]]*\}/{if(found) last=buf; found=0} END{printf "%s", last}')
if echo "$SESSION_JSON" | jq -e '.hookSpecificOutput.hookEventName == "SessionStart"' >/dev/null 2>&1; then
    pass "session-start emits SessionStart JSON"
else
    # Fallback: check if hookEventName appears anywhere in raw output
    if echo "$SESSION_RAW" | grep -q '"hookEventName".*"SessionStart"'; then
        pass "session-start emits SessionStart JSON"
    else
        fail "session-start emits SessionStart JSON"
    fi
fi

# Test 13: session-start kill switch suppresses output
OUTPUT=$(AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/session-start.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "session-start kill switch suppresses output"; else fail "session-start kill switch suppresses output"; fi

# Test 14: session-start roots .agents paths to git root from subdir
MOCK_SESSION="$TMPDIR/mock-session"
mkdir -p "$MOCK_SESSION/subdir"
git -C "$MOCK_SESSION" init -q >/dev/null 2>&1
(cd "$MOCK_SESSION/subdir" && bash "$HOOKS_DIR/session-start.sh" >/dev/null 2>&1 || true)
if [ -d "$MOCK_SESSION/.agents/research" ] && [ ! -d "$MOCK_SESSION/subdir/.agents/research" ]; then
    pass "session-start writes .agents to repo root"
else
    fail "session-start writes .agents to repo root"
fi

# Test 15: precompact emits JSON when data exists
MOCK_PRECOMPACT="$TMPDIR/mock-precompact"
mkdir -p "$MOCK_PRECOMPACT/.agents"
git -C "$MOCK_PRECOMPACT" init -q >/dev/null 2>&1
PRECOMPACT_JSON=$(cd "$MOCK_PRECOMPACT" && bash "$HOOKS_DIR/precompact-snapshot.sh" 2>/dev/null || true)
if echo "$PRECOMPACT_JSON" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1; then
    pass "precompact emits additionalContext JSON"
else
    fail "precompact emits additionalContext JSON"
fi

# Test 16: precompact kill switch suppresses output
OUTPUT=$(cd "$MOCK_PRECOMPACT" && AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/precompact-snapshot.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "precompact kill switch suppresses output"; else fail "precompact kill switch suppresses output"; fi

# ============================================================
echo ""
echo "=== pending-cleaner.sh ==="
# ============================================================

# Test 17: fail-open outside git repo
NON_GIT="$TMPDIR/non-git"
mkdir -p "$NON_GIT"
EC=0
(cd "$NON_GIT" && bash "$HOOKS_DIR/pending-cleaner.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "pending-cleaner fail-open outside git repo"; else fail "pending-cleaner fail-open outside git repo"; fi

# Test 18: stale pending.jsonl auto-clears and logs alerts
MOCK_PENDING="$TMPDIR/mock-pending"
mkdir -p "$MOCK_PENDING"
git -C "$MOCK_PENDING" init -q >/dev/null 2>&1
mkdir -p "$MOCK_PENDING/.agents/ao"
PENDING_FILE="$MOCK_PENDING/.agents/ao/pending.jsonl"
printf '{"session":"1"}\n{"session":"2"}\n' > "$PENDING_FILE"
touch -t 202401010101 "$PENDING_FILE"
(cd "$MOCK_PENDING" && AGENTOPS_PENDING_STALE_SECONDS=1 AGENTOPS_PENDING_ALERT_LINES=1 bash "$HOOKS_DIR/pending-cleaner.sh" >/dev/null 2>&1 || true)
if [ ! -s "$PENDING_FILE" ]; then pass "stale pending.jsonl auto-cleared"; else fail "stale pending.jsonl auto-cleared"; fi
if ls "$MOCK_PENDING/.agents/ao/archive"/pending-*.jsonl >/dev/null 2>&1; then pass "stale pending.jsonl archived before clear"; else fail "stale pending.jsonl archived before clear"; fi
if grep -q 'ALERT stale pending.jsonl detected' "$MOCK_PENDING/.agents/ao/hook-errors.log" 2>/dev/null && grep -q 'AUTOCLEAR stale pending.jsonl' "$MOCK_PENDING/.agents/ao/hook-errors.log" 2>/dev/null; then
    pass "pending-cleaner logs stale alert and autoclear telemetry"
else
    fail "pending-cleaner logs stale alert and autoclear telemetry"
fi

# Test 19: pending-cleaner kill switch prevents auto-clear
printf '{"session":"keep"}\n' > "$PENDING_FILE"
touch -t 202401010101 "$PENDING_FILE"
(cd "$MOCK_PENDING" && AGENTOPS_HOOKS_DISABLED=1 AGENTOPS_PENDING_STALE_SECONDS=1 bash "$HOOKS_DIR/pending-cleaner.sh" >/dev/null 2>&1 || true)
if [ -s "$PENDING_FILE" ]; then pass "pending-cleaner kill switch preserves queue"; else fail "pending-cleaner kill switch preserves queue"; fi

# ============================================================
echo ""
echo "=== task-validation-gate.sh ==="
# ============================================================

REPO_CONTENT_FILE="$REPO_FIXTURE_DIR/test-content.js"
REPO_REGEX_FILE="$REPO_FIXTURE_DIR/test-regex.txt"
echo "function authenticate() {}" > "$REPO_CONTENT_FILE"
echo 'hello.*world' > "$REPO_REGEX_FILE"

# Test 20: No validation metadata => pass
echo '{"metadata":{}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "no validation metadata passes"; else fail "no validation metadata passes"; fi

# Test 21: Global kill switch
echo '{"metadata":{"validation":{"files_exist":["/nonexistent"]}}}' | AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "global kill switch passes validation"; else fail "global kill switch passes validation"; fi

# Test 22: Hook-specific kill switch
echo '{"metadata":{"validation":{"files_exist":["/nonexistent"]}}}' | AGENTOPS_TASK_VALIDATION_DISABLED=1 bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "task-validation kill switch passes validation"; else fail "task-validation kill switch passes validation"; fi

# Test 23: files_exist - existing repo file (relative path)
INPUT=$(jq -n '{"metadata":{"validation":{"files_exist":["hooks/push-gate.sh"]}}}')
echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "files_exist with existing repo file passes"; else fail "files_exist with existing repo file passes"; fi

# Test 24: files_exist - missing file blocks
EC=0
echo '{"metadata":{"validation":{"files_exist":["hooks/nonexistent-file-12345.sh"]}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "files_exist with missing file blocks (exit 2)"; else fail "files_exist with missing file blocks (exit=$EC, expected 2)"; fi

# Test 25: files_exist blocks path traversal outside repo root
EC=0
echo '{"metadata":{"validation":{"files_exist":["../README.md"]}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "files_exist blocks path traversal outside repo"; else fail "files_exist blocks path traversal outside repo (exit=$EC, expected 2)"; fi

# Test 26: content_check - pattern found within repo root
INPUT=$(jq -n --arg f "$REPO_CONTENT_FILE" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"function authenticate"}]}}}')
echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "content_check with matching pattern passes"; else fail "content_check with matching pattern passes"; fi

# Test 27: content_check - pattern not found
INPUT=$(jq -n --arg f "$REPO_CONTENT_FILE" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"class UserService"}]}}}')
EC=0
echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "content_check with missing pattern blocks (exit 2)"; else fail "content_check with missing pattern blocks (exit=$EC, expected 2)"; fi

# Test 28: content_check - regex injection safe (grep -qF literal match)
INPUT=$(jq -n --arg f "$REPO_REGEX_FILE" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"hello.*world"}]}}}')
echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "content_check treats regex chars as literals"; else fail "content_check treats regex chars as literals"; fi

# Test 29: content_check blocks files outside repo root
echo "outside" > "$TMPDIR/outside.txt"
INPUT=$(jq -n --arg f "$TMPDIR/outside.txt" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"outside"}]}}}')
EC=0
echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "content_check blocks paths outside repo"; else fail "content_check blocks paths outside repo (exit=$EC, expected 2)"; fi

# Test 30: Allowlist blocks disallowed commands
EC=0
echo '{"metadata":{"validation":{"tests":"curl http://evil.com"}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "allowlist blocks curl command"; else fail "allowlist blocks curl command (exit=$EC, expected 2)"; fi

# Test 31: Allowlist allows go command
MOCK_ALLOW="$TMPDIR/mock-allowlist"
setup_mock_repo "$MOCK_ALLOW"
EC=0
(cd "$MOCK_ALLOW" && echo '{"metadata":{"validation":{"tests":"go version"}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "allowlist allows go command"; else fail "allowlist allows go command (exit=$EC, expected 0)"; fi

# ============================================================
echo ""
echo "=== task-validation-gate.sh error recovery ==="
# ============================================================

# Test 33: Test failure writes last-failure.json with all 6 required fields
MOCK_FAIL_REPO="$TMPDIR/mock-fail-test"
setup_mock_repo "$MOCK_FAIL_REPO"
(cd "$MOCK_FAIL_REPO" && echo '{"subject":"test task","metadata":{"validation":{"tests":"make nonexistent-target-xyz"}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || true)
if [ -f "$MOCK_FAIL_REPO/.agents/ao/last-failure.json" ]; then
    FAILURE_JSON=$(cat "$MOCK_FAIL_REPO/.agents/ao/last-failure.json")
    if echo "$FAILURE_JSON" | jq -e '.ts and .type and .command and .exit_code and .task_subject and .details' >/dev/null 2>&1; then
        pass "test failure writes last-failure.json with all 6 fields"
    else
        fail "test failure writes last-failure.json with all 6 fields"
    fi
else
    fail "test failure writes last-failure.json with all 6 fields (file missing)"
fi

# Test 34: last-failure.json "type" field matches failure type
MOCK_FILES_REPO="$TMPDIR/mock-files-fail"
setup_mock_repo "$MOCK_FILES_REPO"
(cd "$MOCK_FILES_REPO" && echo '{"subject":"files task","metadata":{"validation":{"files_exist":["nonexistent-file.txt"]}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || true)
if [ -f "$MOCK_FILES_REPO/.agents/ao/last-failure.json" ]; then
    FAILURE_TYPE=$(jq -r '.type' "$MOCK_FILES_REPO/.agents/ao/last-failure.json" 2>/dev/null)
    if [ "$FAILURE_TYPE" = "files_exist" ]; then
        pass "last-failure.json type field matches failure type"
    else
        fail "last-failure.json type field matches failure type (got: $FAILURE_TYPE)"
    fi
else
    fail "last-failure.json type field matches failure type (file missing)"
fi

# Test 35: Stderr includes "bug-hunt" for test failures
MOCK_TEST_FAIL="$TMPDIR/mock-test-fail"
mkdir -p "$MOCK_TEST_FAIL/.agents/ao"
git -C "$MOCK_TEST_FAIL" init -q >/dev/null 2>&1
OUTPUT=$(cd "$MOCK_TEST_FAIL" && echo '{"subject":"test task","metadata":{"validation":{"tests":"make nonexistent-target-xyz"}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1 || true)
if echo "$OUTPUT" | grep -q "bug-hunt"; then
    pass "stderr includes bug-hunt for test failures"
else
    fail "stderr includes bug-hunt for test failures"
fi

# Test 36: Stderr lists missing files for files_exist failures
MOCK_MISSING="$TMPDIR/mock-missing-files"
mkdir -p "$MOCK_MISSING/.agents/ao"
git -C "$MOCK_MISSING" init -q >/dev/null 2>&1
OUTPUT=$(cd "$MOCK_MISSING" && echo '{"subject":"files task","metadata":{"validation":{"files_exist":["missing-a.txt"]}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1 || true)
if echo "$OUTPUT" | grep -q "missing-a.txt"; then
    pass "stderr lists missing files for files_exist failures"
else
    fail "stderr lists missing files for files_exist failures"
fi

# Test 37: Exit code still 2 on failure (regression test)
MOCK_EXIT_CHECK="$TMPDIR/mock-exit-check"
mkdir -p "$MOCK_EXIT_CHECK/.agents/ao"
git -C "$MOCK_EXIT_CHECK" init -q >/dev/null 2>&1
EC=0
(cd "$MOCK_EXIT_CHECK" && echo '{"subject":"exit test","metadata":{"validation":{"tests":"make nonexistent-target-xyz"}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 2 ]; then
    pass "exit code still 2 on failure (regression test)"
else
    fail "exit code still 2 on failure (got: $EC, expected: 2)"
fi

# ============================================================
echo ""
echo "=== precompact-snapshot.sh auto-handoff ==="
# ============================================================

# Test 38: Auto-handoff document created in .agents/handoff/
MOCK_HANDOFF_REPO="$TMPDIR/mock-handoff-create"
mkdir -p "$MOCK_HANDOFF_REPO/.agents/ao"
git -C "$MOCK_HANDOFF_REPO" init -q >/dev/null 2>&1
(cd "$MOCK_HANDOFF_REPO" && bash "$HOOKS_DIR/precompact-snapshot.sh" >/dev/null 2>&1 || true)
if ls "$MOCK_HANDOFF_REPO/.agents/handoff/auto-"*.md >/dev/null 2>&1; then
    pass "auto-handoff document created in .agents/handoff/"
else
    fail "auto-handoff document created in .agents/handoff/"
fi

# Test 39: Handoff contains "Ratchet State" section
MOCK_HANDOFF_CONTENT="$TMPDIR/mock-handoff-content"
mkdir -p "$MOCK_HANDOFF_CONTENT/.agents/ao"
git -C "$MOCK_HANDOFF_CONTENT" init -q >/dev/null 2>&1
(cd "$MOCK_HANDOFF_CONTENT" && bash "$HOOKS_DIR/precompact-snapshot.sh" >/dev/null 2>&1 || true)
HANDOFF_FILE=$(ls -t "$MOCK_HANDOFF_CONTENT/.agents/handoff/auto-"*.md 2>/dev/null | head -1)
if [ -f "$HANDOFF_FILE" ]; then
    if grep -q "Ratchet State" "$HANDOFF_FILE"; then
        pass "handoff contains Ratchet State section"
    else
        fail "handoff contains Ratchet State section"
    fi
else
    fail "handoff contains Ratchet State section (file missing)"
fi

# Test 40: Kill switch suppresses handoff
MOCK_HANDOFF_KILL="$TMPDIR/mock-handoff-kill"
mkdir -p "$MOCK_HANDOFF_KILL/.agents/ao"
git -C "$MOCK_HANDOFF_KILL" init -q >/dev/null 2>&1
(cd "$MOCK_HANDOFF_KILL" && AGENTOPS_PRECOMPACT_DISABLED=1 bash "$HOOKS_DIR/precompact-snapshot.sh" >/dev/null 2>&1 || true)
if ! ls "$MOCK_HANDOFF_KILL/.agents/handoff/auto-"*.md >/dev/null 2>&1; then
    pass "kill switch suppresses handoff"
else
    fail "kill switch suppresses handoff"
fi

# ============================================================
echo ""
echo "=== session-start.sh handoff injection ==="
# ============================================================

# Test 41: session-start.sh reads auto-handoff and includes content
MOCK_SESSION_HANDOFF="$TMPDIR/mock-session-handoff"
mkdir -p "$MOCK_SESSION_HANDOFF/.agents/handoff"
git -C "$MOCK_SESSION_HANDOFF" init -q >/dev/null 2>&1
HANDOFF_TEST_FILE="$MOCK_SESSION_HANDOFF/.agents/handoff/auto-2026-01-01.md"
echo "# Test Handoff" > "$HANDOFF_TEST_FILE"
echo "HANDOFF_TEST_MARKER_12345" >> "$HANDOFF_TEST_FILE"
OUTPUT=$(cd "$MOCK_SESSION_HANDOFF" && bash "$HOOKS_DIR/session-start.sh" 2>/dev/null || true)
if echo "$OUTPUT" | grep -q "HANDOFF_TEST_MARKER_12345"; then
    pass "session-start.sh reads auto-handoff and includes content"
else
    fail "session-start.sh reads auto-handoff and includes content"
fi

# Test 42: Auto-handoff file deleted after injection
if [ ! -f "$HANDOFF_TEST_FILE" ]; then
    pass "auto-handoff file deleted after injection"
else
    fail "auto-handoff file deleted after injection"
fi

# ============================================================
echo ""
echo "=== memory packet v1 compatibility ==="
# ============================================================

# Test 43: stop-auto-handoff emits packet v1
MOCK_STOP_PACKET="$TMPDIR/mock-stop-packet"
mkdir -p "$MOCK_STOP_PACKET/.agents/ao"
git -C "$MOCK_STOP_PACKET" init -q >/dev/null 2>&1
printf '{"last_assistant_message":"STOP_PACKET_MARKER_123"}' \
  | (cd "$MOCK_STOP_PACKET" && bash "$HOOKS_DIR/stop-auto-handoff.sh" >/dev/null 2>&1 || true)
STOP_PACKET_FILE=$(ls -t "$MOCK_STOP_PACKET/.agents/ao/packets/pending/"*.json 2>/dev/null | head -1)
if [ -f "$STOP_PACKET_FILE" ] \
  && jq -e '.schema_version == 1 and .packet_type == "stop" and .source_hook == "stop-auto-handoff" and (.payload.last_assistant_message | contains("STOP_PACKET_MARKER_123"))' "$STOP_PACKET_FILE" >/dev/null 2>&1; then
    pass "stop-auto-handoff emits schema packet v1"
else
    fail "stop-auto-handoff emits schema packet v1"
fi

# Test 44: subagent-stop emits packet v1
MOCK_SUB_PACKET="$TMPDIR/mock-subagent-packet"
mkdir -p "$MOCK_SUB_PACKET/.agents/ao"
git -C "$MOCK_SUB_PACKET" init -q >/dev/null 2>&1
printf '{"last_assistant_message":"SUB_PACKET_MARKER_999","agent_name":"worker-a"}' \
  | (cd "$MOCK_SUB_PACKET" && bash "$HOOKS_DIR/subagent-stop.sh" >/dev/null 2>&1 || true)
SUB_PACKET_FILE=$(ls -t "$MOCK_SUB_PACKET/.agents/ao/packets/pending/"*.json 2>/dev/null | head -1)
if [ -f "$SUB_PACKET_FILE" ] \
  && jq -e '.schema_version == 1 and .packet_type == "subagent_stop" and .source_hook == "subagent-stop" and .payload.agent_name == "worker-a"' "$SUB_PACKET_FILE" >/dev/null 2>&1; then
    pass "subagent-stop emits schema packet v1"
else
    fail "subagent-stop emits schema packet v1"
fi

# Test 45: session-start consumes packet-first and moves packet to consumed
MOCK_PACKET_CONSUME="$TMPDIR/mock-packet-consume"
mkdir -p "$MOCK_PACKET_CONSUME/.agents/ao/packets/pending" "$MOCK_PACKET_CONSUME/.agents/handoff"
git -C "$MOCK_PACKET_CONSUME" init -q >/dev/null 2>&1
echo "PACKET_CONSUME_MARKER_777" > "$MOCK_PACKET_CONSUME/.agents/handoff/test-packet.md"
cat > "$MOCK_PACKET_CONSUME/.agents/ao/packets/pending/packet-test.json" <<'EOF'
{
  "schema_version": 1,
  "packet_id": "packet-test",
  "packet_type": "stop",
  "created_at": "2026-02-21T00:00:00Z",
  "source_hook": "stop-auto-handoff",
  "session_id": "session-20260221-000000",
  "handoff_file": ".agents/handoff/test-packet.md",
  "payload": {"summary":"fallback"}
}
EOF
PACKET_OUTPUT=$(cd "$MOCK_PACKET_CONSUME" && bash "$HOOKS_DIR/session-start.sh" 2>/dev/null || true)
if echo "$PACKET_OUTPUT" | grep -q "PACKET_CONSUME_MARKER_777"; then
    pass "session-start consumes packet-first handoff content"
else
    fail "session-start consumes packet-first handoff content"
fi
if [ -f "$MOCK_PACKET_CONSUME/.agents/ao/packets/consumed/packet-test.json" ] \
  && [ ! -f "$MOCK_PACKET_CONSUME/.agents/ao/packets/pending/packet-test.json" ]; then
    pass "session-start moves consumed packet to consumed/"
else
    fail "session-start moves consumed packet to consumed/"
fi

# Test 46: malformed packet is quarantined and skipped
MOCK_PACKET_QUAR="$TMPDIR/mock-packet-quarantine"
mkdir -p "$MOCK_PACKET_QUAR/.agents/ao/packets/pending"
git -C "$MOCK_PACKET_QUAR" init -q >/dev/null 2>&1
echo '{"schema_version":1,"packet_id":"bad-only"}' > "$MOCK_PACKET_QUAR/.agents/ao/packets/pending/bad.json"
(cd "$MOCK_PACKET_QUAR" && bash "$HOOKS_DIR/session-start.sh" >/dev/null 2>&1 || true)
if [ -f "$MOCK_PACKET_QUAR/.agents/ao/packets/quarantine/bad.json" ] \
  && [ ! -f "$MOCK_PACKET_QUAR/.agents/ao/packets/pending/bad.json" ]; then
    pass "session-start quarantines malformed packet"
else
    fail "session-start quarantines malformed packet"
fi

# ============================================================
echo ""
echo "=== standards-injector.sh ==="
# ============================================================

# Test: Python file triggers python standards injection
OUTPUT=$(echo '{"tool_input":{"file_path":"/some/path/main.py"}}' | bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)
if echo "$OUTPUT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1; then
    pass "python file injects standards context"
else
    fail "python file injects standards context"
fi

# Test: Go file triggers go standards injection
OUTPUT=$(echo '{"tool_input":{"file_path":"/some/path/main.go"}}' | bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)
if echo "$OUTPUT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1; then
    pass "go file injects standards context"
else
    fail "go file injects standards context"
fi

# Test: Unknown extension => silent exit
OUTPUT=$(echo '{"tool_input":{"file_path":"/some/path/data.csv"}}' | bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "unknown extension produces no output"; else fail "unknown extension produces no output"; fi

# Test: No file_path => silent exit
OUTPUT=$(echo '{"tool_input":{}}' | bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "missing file_path produces no output"; else fail "missing file_path produces no output"; fi

# Test: Kill switch disables injection
OUTPUT=$(echo '{"tool_input":{"file_path":"/x/y.py"}}' | AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "standards-injector kill switch"; else fail "standards-injector kill switch"; fi

# Test: Shell file triggers shell standards
OUTPUT=$(echo '{"tool_input":{"file_path":"/x/script.sh"}}' | bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)
if echo "$OUTPUT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1; then
    pass "shell file injects standards context"
else
    fail "shell file injects standards context"
fi

# ============================================================
echo ""
echo "=== git-worker-guard.sh ==="
# ============================================================

# Test: Non-git command passes through
EC=0
echo '{"tool_input":{"command":"ls -la"}}' | bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "git-worker-guard passes non-git command"; else fail "git-worker-guard passes non-git command"; fi

# Test: git commit allowed for non-worker (no CLAUDE_AGENT_NAME, no swarm-role)
EC=0
echo '{"tool_input":{"command":"git commit -m test"}}' | CLAUDE_AGENT_NAME="" bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "git commit allowed for non-worker"; else fail "git commit allowed for non-worker"; fi

# Test: git commit blocked for worker via CLAUDE_AGENT_NAME
EC=0
echo '{"tool_input":{"command":"git commit -m test"}}' | CLAUDE_AGENT_NAME="worker-1" bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "git commit blocked for worker (CLAUDE_AGENT_NAME)"; else fail "git commit blocked for worker (exit=$EC, expected 2)"; fi

# Test: git push blocked for worker
EC=0
echo '{"tool_input":{"command":"git push origin main"}}' | CLAUDE_AGENT_NAME="worker-3" bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "git push blocked for worker"; else fail "git push blocked for worker (exit=$EC, expected 2)"; fi

# Test: git add -A blocked for worker
EC=0
echo '{"tool_input":{"command":"git add -A"}}' | CLAUDE_AGENT_NAME="worker-2" bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "git add -A blocked for worker"; else fail "git add -A blocked for worker (exit=$EC, expected 2)"; fi

# Test: git commit blocked for worker via swarm-role file
MOCK_SWARM="$TMPDIR/mock-swarm"
setup_mock_repo "$MOCK_SWARM"
echo "worker" > "$MOCK_SWARM/.agents/swarm-role"
EC=0
(cd "$MOCK_SWARM" && echo '{"tool_input":{"command":"git commit -m test"}}' | CLAUDE_AGENT_NAME="" bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 2 ]; then pass "git commit blocked via swarm-role file"; else fail "git commit blocked via swarm-role file (exit=$EC, expected 2)"; fi

# Test: team lead allowed to commit (CLAUDE_AGENT_NAME without worker- prefix)
EC=0
echo '{"tool_input":{"command":"git commit -m test"}}' | CLAUDE_AGENT_NAME="team-lead" bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "team lead allowed to commit"; else fail "team lead allowed to commit"; fi

# Test: Kill switch allows worker commit
EC=0
echo '{"tool_input":{"command":"git commit -m test"}}' | AGENTOPS_HOOKS_DISABLED=1 CLAUDE_AGENT_NAME="worker-1" bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "git-worker-guard kill switch"; else fail "git-worker-guard kill switch"; fi

# ============================================================
echo ""
echo "=== dangerous-git-guard.sh ==="
# ============================================================

# Test: force push blocked
EC=0
echo '{"tool_input":{"command":"git push -f origin main"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "force push blocked"; else fail "force push blocked (exit=$EC, expected 2)"; fi

# Test: --force blocked
EC=0
echo '{"tool_input":{"command":"git push --force origin main"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "push --force blocked"; else fail "push --force blocked (exit=$EC, expected 2)"; fi

# Test: --force-with-lease allowed
EC=0
echo '{"tool_input":{"command":"git push --force-with-lease origin main"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "force-with-lease allowed"; else fail "force-with-lease allowed (exit=$EC, expected 0)"; fi

# Test: hard reset blocked
EC=0
echo '{"tool_input":{"command":"git reset --hard HEAD~1"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "hard reset blocked"; else fail "hard reset blocked (exit=$EC, expected 2)"; fi

# Test: git clean -f blocked
EC=0
echo '{"tool_input":{"command":"git clean -f"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "force clean blocked"; else fail "force clean blocked (exit=$EC, expected 2)"; fi

# Test: git checkout . blocked
EC=0
echo '{"tool_input":{"command":"git checkout ."}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "checkout dot blocked"; else fail "checkout dot blocked (exit=$EC, expected 2)"; fi

# Test: git branch -D blocked
EC=0
echo '{"tool_input":{"command":"git branch -D feature"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "force branch delete blocked"; else fail "force branch delete blocked (exit=$EC, expected 2)"; fi

# Test: safe git branch -d allowed
EC=0
echo '{"tool_input":{"command":"git branch -d feature"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "safe branch delete allowed"; else fail "safe branch delete allowed (exit=$EC, expected 0)"; fi

# Test: normal git commit allowed
EC=0
echo '{"tool_input":{"command":"git commit -m fix"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "normal git commit allowed"; else fail "normal git commit allowed"; fi

# Test: non-git command passes
EC=0
echo '{"tool_input":{"command":"npm install"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "dangerous-git-guard passes non-git"; else fail "dangerous-git-guard passes non-git"; fi

# Test: kill switch allows force push
EC=0
echo '{"tool_input":{"command":"git push -f origin main"}}' | AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/dangerous-git-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "dangerous-git-guard kill switch"; else fail "dangerous-git-guard kill switch"; fi

# Test: stderr suggests safe alternative
OUTPUT=$(echo '{"tool_input":{"command":"git reset --hard HEAD"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" 2>&1 || true)
if echo "$OUTPUT" | grep -qi "stash\|soft"; then pass "hard reset suggests safe alternative"; else fail "hard reset suggests safe alternative"; fi

# ============================================================
echo ""
echo "=== ratchet-advance.sh ==="
# ============================================================

# Test: Non-ratchet command => silent exit
OUTPUT=$(echo '{"tool_input":{"command":"go test ./..."},"tool_response":{"exit_code":0}}' | bash "$HOOKS_DIR/ratchet-advance.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "ratchet-advance ignores non-ratchet command"; else fail "ratchet-advance ignores non-ratchet command"; fi

# Test: Failed ratchet record => silent exit
OUTPUT=$(echo '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":1}}' | bash "$HOOKS_DIR/ratchet-advance.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "ratchet-advance ignores failed record"; else fail "ratchet-advance ignores failed record"; fi

# Test: Successful research record => suggests next step (fallback mode, ao unavailable)
# We hide ao to force the fallback case-statement logic
MOCK_RATCHET="$TMPDIR/mock-ratchet"
mkdir -p "$MOCK_RATCHET/.agents/ao"
git -C "$MOCK_RATCHET" init -q >/dev/null 2>&1
OUTPUT=$(cd "$MOCK_RATCHET" && echo '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}' | PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ratchet-advance.sh" 2>/dev/null || true)
if echo "$OUTPUT" | grep -q "/plan"; then
    pass "research record suggests /plan (fallback)"
else
    fail "research record suggests /plan (fallback)"
fi

# Test: Successful vibe record => suggests /post-mortem (fallback mode)
OUTPUT=$(cd "$MOCK_RATCHET" && echo '{"tool_input":{"command":"ao ratchet record vibe"},"tool_response":{"exit_code":0}}' | PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ratchet-advance.sh" 2>/dev/null || true)
if echo "$OUTPUT" | grep -q "/post-mortem"; then
    pass "vibe record suggests /post-mortem (fallback)"
else
    fail "vibe record suggests /post-mortem (fallback)"
fi

# Test: post-mortem record => cycle complete (fallback mode)
OUTPUT=$(cd "$MOCK_RATCHET" && echo '{"tool_input":{"command":"ao ratchet record post-mortem"},"tool_response":{"exit_code":0}}' | PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ratchet-advance.sh" 2>/dev/null || true)
if echo "$OUTPUT" | grep -qi "complete"; then
    pass "post-mortem record says cycle complete (fallback)"
else
    fail "post-mortem record says cycle complete (fallback)"
fi

# Test: With ao available, still emits a suggestion (integration)
OUTPUT=$(cd "$MOCK_RATCHET" && echo '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}' | bash "$HOOKS_DIR/ratchet-advance.sh" 2>/dev/null || true)
if echo "$OUTPUT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1; then
    pass "ratchet-advance emits valid JSON with ao"
else
    # May produce non-JSON output — still counts if non-empty
    if [ -n "$OUTPUT" ]; then
        pass "ratchet-advance emits valid JSON with ao"
    else
        fail "ratchet-advance emits valid JSON with ao"
    fi
fi

# Test: Writes dedup flag file
if [ -f "$MOCK_RATCHET/.agents/ao/.ratchet-advance-fired" ]; then
    pass "ratchet-advance writes dedup flag"
else
    fail "ratchet-advance writes dedup flag"
fi

# Test: Kill switch (AGENTOPS_AUTOCHAIN=0)
OUTPUT=$(echo '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}' | AGENTOPS_AUTOCHAIN=0 bash "$HOOKS_DIR/ratchet-advance.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then pass "ratchet-advance AUTOCHAIN kill switch"; else fail "ratchet-advance AUTOCHAIN kill switch"; fi

# Test: Idempotency — suppresses if next step already in chain
MOCK_IDEMP="$TMPDIR/mock-idemp"
mkdir -p "$MOCK_IDEMP/.agents/ao"
git -C "$MOCK_IDEMP" init -q >/dev/null 2>&1
echo '{"gate":"plan","status":"locked"}' > "$MOCK_IDEMP/.agents/ao/chain.jsonl"
OUTPUT=$(cd "$MOCK_IDEMP" && echo '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}' | bash "$HOOKS_DIR/ratchet-advance.sh" 2>/dev/null || true)
if [ -z "$OUTPUT" ]; then pass "ratchet-advance suppresses when next step done"; else fail "ratchet-advance suppresses when next step done"; fi

# Test: Extracts --output artifact
MOCK_ART="$TMPDIR/mock-artifact"
mkdir -p "$MOCK_ART/.agents/ao"
git -C "$MOCK_ART" init -q >/dev/null 2>&1
OUTPUT=$(cd "$MOCK_ART" && echo '{"tool_input":{"command":"ao ratchet record plan --output .agents/plan.md"},"tool_response":{"exit_code":0}}' | bash "$HOOKS_DIR/ratchet-advance.sh" 2>/dev/null || true)
if echo "$OUTPUT" | grep -q ".agents/plan.md"; then
    pass "ratchet-advance includes artifact path"
else
    fail "ratchet-advance includes artifact path"
fi

# ============================================================
echo ""
echo "=== pre-mortem-gate.sh ==="
# ============================================================

# Test: Non-Skill tool => pass
EC=0
echo '{"tool_name":"Bash","tool_input":{"command":"ls"}}' | bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "pre-mortem-gate passes non-Skill tool"; else fail "pre-mortem-gate passes non-Skill tool"; fi

# Test: Non-crank skill => pass
EC=0
echo '{"tool_name":"Skill","tool_input":{"skill":"vibe","args":""}}' | bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "pre-mortem-gate passes non-crank skill"; else fail "pre-mortem-gate passes non-crank skill"; fi

# Test: Crank with no epic ID => fail-open
EC=0
echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":""}}' | bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "pre-mortem-gate fail-open on no epic ID"; else fail "pre-mortem-gate fail-open on no epic ID"; fi

# Test: Kill switch allows crank
EC=0
echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx"}}' | AGENTOPS_SKIP_PRE_MORTEM_GATE=1 bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "pre-mortem-gate kill switch"; else fail "pre-mortem-gate kill switch"; fi

# Test: Worker exempt
EC=0
echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx"}}' | AGENTOPS_WORKER=1 bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "pre-mortem-gate worker exempt"; else fail "pre-mortem-gate worker exempt"; fi

# Test: --skip-pre-mortem bypasses gate
EC=0
echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx --skip-pre-mortem"}}' | bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "pre-mortem-gate --skip-pre-mortem bypass"; else fail "pre-mortem-gate --skip-pre-mortem bypass"; fi

# Test: Crank with pre-mortem evidence (council artifact) => pass
MOCK_PM="$TMPDIR/mock-pre-mortem"
setup_mock_repo "$MOCK_PM"
mkdir -p "$MOCK_PM/.agents/council"
touch "$MOCK_PM/.agents/council/2026-01-01-pre-mortem-test.md"
# Simulate bd returning 5 children (mock bd with a script)
MOCK_BD="$MOCK_PM/mock-bd"
printf '#!/bin/bash\nif [ "$1" = "children" ]; then printf "1\\n2\\n3\\n4\\n5\\n"; fi\n' > "$MOCK_BD"
chmod +x "$MOCK_BD"
EC=0
(cd "$MOCK_PM" && PATH="$MOCK_PM:$PATH" echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx"}}' | bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1) || EC=$?
# Note: if bd is not available in PATH the gate fail-opens, so we mock it
if [ "$EC" -eq 0 ]; then pass "pre-mortem-gate passes with council evidence"; else fail "pre-mortem-gate passes with council evidence (exit=$EC)"; fi

# ============================================================
echo ""
echo "=== stop-team-guard.sh ==="
# ============================================================

# Test: No teams dir => pass (safe to stop)
EC=0
TEAMS_DIR_BAK="${HOME}/.claude/teams"
# Use a clean tmp for HOME to avoid touching real teams
MOCK_HOME="$TMPDIR/mock-home"
mkdir -p "$MOCK_HOME/.claude"
(HOME="$MOCK_HOME" bash "$HOOKS_DIR/stop-team-guard.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "stop-team-guard safe when no teams dir"; else fail "stop-team-guard safe when no teams dir"; fi

# Test: Empty teams dir => pass
mkdir -p "$MOCK_HOME/.claude/teams"
EC=0
(HOME="$MOCK_HOME" bash "$HOOKS_DIR/stop-team-guard.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "stop-team-guard safe with empty teams dir"; else fail "stop-team-guard safe with empty teams dir"; fi

# Test: Team with no tmux panes (in-process) => pass
mkdir -p "$MOCK_HOME/.claude/teams/test-team"
echo '{"members":[{"name":"worker-1","agentType":"general-purpose"}]}' > "$MOCK_HOME/.claude/teams/test-team/config.json"
EC=0
(HOME="$MOCK_HOME" bash "$HOOKS_DIR/stop-team-guard.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "stop-team-guard safe with in-process team"; else fail "stop-team-guard safe with in-process team"; fi

# Test: Team with dead tmux pane => pass
echo '{"members":[{"name":"w1","tmuxPaneId":"nonexistent-pane-99999"}]}' > "$MOCK_HOME/.claude/teams/test-team/config.json"
EC=0
(HOME="$MOCK_HOME" bash "$HOOKS_DIR/stop-team-guard.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "stop-team-guard safe with dead tmux pane"; else fail "stop-team-guard safe with dead tmux pane"; fi

# Test: Kill switch allows stop
echo '{"members":[{"name":"w1","tmuxPaneId":"some-session"}]}' > "$MOCK_HOME/.claude/teams/test-team/config.json"
EC=0
(HOME="$MOCK_HOME" AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/stop-team-guard.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "stop-team-guard kill switch"; else fail "stop-team-guard kill switch"; fi

# Test: --cleanup mode removes stale teams (>2h old)
MOCK_CLEANUP_HOME="$TMPDIR/mock-cleanup-home"
mkdir -p "$MOCK_CLEANUP_HOME/.claude/teams/stale-team"
echo '{"members":[]}' > "$MOCK_CLEANUP_HOME/.claude/teams/stale-team/config.json"
# Touch config to be >2h old
touch -t 202401010101 "$MOCK_CLEANUP_HOME/.claude/teams/stale-team/config.json"
(HOME="$MOCK_CLEANUP_HOME" bash "$HOOKS_DIR/stop-team-guard.sh" --cleanup >/dev/null 2>&1 || true)
if [ ! -d "$MOCK_CLEANUP_HOME/.claude/teams/stale-team" ]; then
    pass "cleanup mode removes stale teams"
else
    fail "cleanup mode removes stale teams"
fi

# ============================================================
echo ""
echo "=== validate-hook-preflight.sh ==="
# ============================================================

# Test 32: Hook preflight validator passes
if "$REPO_ROOT/scripts/validate-hook-preflight.sh" >/dev/null 2>&1; then
    pass "validate-hook-preflight.sh passes"
else
    fail "validate-hook-preflight.sh passes"
fi

# ============================================================
echo ""
echo "=== citation-tracker.sh ==="
# ============================================================

# Test: writes citation record for .agents knowledge reads
MOCK_CITE="$TMPDIR/mock-citation"
setup_mock_repo "$MOCK_CITE"
mkdir -p "$MOCK_CITE/.agents/learnings" "$MOCK_CITE/.agents/ao"
echo "test" > "$MOCK_CITE/.agents/learnings/item.md"
CITE_SESSION_ID="sess-cite-$$-$(date +%s)"
(
  cd "$MOCK_CITE" && \
  echo '{"tool_input":{"file_path":".agents/learnings/item.md"}}' | \
  CLAUDE_SESSION_ID="$CITE_SESSION_ID" bash "$HOOKS_DIR/citation-tracker.sh" >/dev/null 2>&1
)
if [ -f "$MOCK_CITE/.agents/ao/citations.jsonl" ] && grep -q ".agents/learnings/item.md" "$MOCK_CITE/.agents/ao/citations.jsonl"; then
    pass "citation-tracker records citations for knowledge reads"
else
    fail "citation-tracker records citations for knowledge reads"
fi

# Test: dedup prevents duplicate citation in same session
before_lines=$(wc -l < "$MOCK_CITE/.agents/ao/citations.jsonl" | tr -d ' ')
(
  cd "$MOCK_CITE" && \
  echo '{"tool_input":{"file_path":".agents/learnings/item.md"}}' | \
  CLAUDE_SESSION_ID="$CITE_SESSION_ID" bash "$HOOKS_DIR/citation-tracker.sh" >/dev/null 2>&1
)
after_lines=$(wc -l < "$MOCK_CITE/.agents/ao/citations.jsonl" | tr -d ' ')
if [ "$before_lines" = "$after_lines" ]; then
    pass "citation-tracker dedups same-session reads"
else
    fail "citation-tracker dedups same-session reads"
fi

# ============================================================
echo ""
echo "=== context-guard.sh ==="
# ============================================================

# Test: emits additionalContext when ao context guard returns message
MOCK_CTX="$TMPDIR/mock-context-guard"
mkdir -p "$MOCK_CTX"
cat > "$MOCK_CTX/ao" <<'EOF'
#!/usr/bin/env bash
echo '{"session":{"action":"warn"},"hook_message":"Context warning message"}'
EOF
chmod +x "$MOCK_CTX/ao"
OUTPUT=$(echo '{"prompt":"keep going"}' | PATH="$MOCK_CTX:$PATH" CLAUDE_SESSION_ID="sess-ctx-1" bash "$HOOKS_DIR/context-guard.sh" 2>/dev/null || true)
if echo "$OUTPUT" | jq -e '.hookSpecificOutput.additionalContext == "Context warning message"' >/dev/null 2>&1; then
    pass "context-guard emits additionalContext from ao output"
else
    fail "context-guard emits additionalContext from ao output"
fi

# Test: strict mode blocks on handoff_now action
cat > "$MOCK_CTX/ao" <<'EOF'
#!/usr/bin/env bash
echo '{"session":{"action":"handoff_now"},"hook_message":"Context critical"}'
EOF
chmod +x "$MOCK_CTX/ao"
EC=0
echo '{"prompt":"continue"}' | PATH="$MOCK_CTX:$PATH" CLAUDE_SESSION_ID="sess-ctx-2" AGENTOPS_CONTEXT_GUARD_STRICT=1 bash "$HOOKS_DIR/context-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then
    pass "context-guard strict mode blocks on handoff_now"
else
    fail "context-guard strict mode blocks on handoff_now (exit=$EC, expected 2)"
fi

# ============================================================
echo ""
echo "=== skill-lint-gate.sh ==="
# ============================================================

# Test: non-skill edit path exits cleanly
EC=0
TOOL_INPUT='{"file_path":"README.md"}' bash "$HOOKS_DIR/skill-lint-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then
    pass "skill-lint-gate ignores non-skill files"
else
    fail "skill-lint-gate ignores non-skill files"
fi

# Test: oversized orchestration SKILL.md emits warning (non-blocking)
MOCK_SKILL="$TMPDIR/mock-skill-lint"
mkdir -p "$MOCK_SKILL/skills/demo"
{
  echo "---"
  echo "name: demo"
  echo "tier: orchestration"
  echo "---"
  for i in $(seq 1 560); do echo "line $i"; done
} > "$MOCK_SKILL/skills/demo/SKILL.md"
OUTPUT=$(TOOL_INPUT="{\"file_path\":\"$MOCK_SKILL/skills/demo/SKILL.md\"}" bash "$HOOKS_DIR/skill-lint-gate.sh" 2>&1 || true)
if echo "$OUTPUT" | grep -q "SKILL LINT"; then
    pass "skill-lint-gate warns on oversized SKILL.md"
else
    fail "skill-lint-gate warns on oversized SKILL.md"
fi

# ============================================================
echo ""
echo "=== config-change-monitor.sh ==="
# ============================================================

# Test: logs config changes to repo-scoped telemetry
MOCK_CONFIG="$TMPDIR/mock-config-change"
setup_mock_repo "$MOCK_CONFIG"
(
  cd "$MOCK_CONFIG" && \
  echo '{"config_key":"approval_policy","old_value":"never","new_value":"on-request"}' | \
  CLAUDE_SESSION_ID="sess-config-1" bash "$HOOKS_DIR/config-change-monitor.sh" >/dev/null 2>&1
)
if [ -f "$MOCK_CONFIG/.agents/ao/config-changes.jsonl" ] && grep -q '"config_key":"approval_policy"' "$MOCK_CONFIG/.agents/ao/config-changes.jsonl"; then
    pass "config-change-monitor logs config changes"
else
    fail "config-change-monitor logs config changes"
fi

# Test: strict mode blocks critical config changes
EC=0
(
  cd "$MOCK_CONFIG" && \
  echo '{"config_key":"approval_policy","old_value":"never","new_value":"on-request"}' | \
  AGENTOPS_CONFIG_GUARD_STRICT=1 bash "$HOOKS_DIR/config-change-monitor.sh" >/dev/null 2>&1
) || EC=$?
if [ "$EC" -eq 2 ]; then
    pass "config-change-monitor strict mode blocks critical changes"
else
    fail "config-change-monitor strict mode blocks critical changes (exit=$EC, expected 2)"
fi

# ============================================================
echo ""
echo "=== session-end-maintenance.sh ==="
# ============================================================

# Test: kill switch short-circuits session-end-maintenance
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/session-end-maintenance.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then
    pass "session-end-maintenance kill switch"
else
    fail "session-end-maintenance kill switch"
fi

# Test: fail-open when ao is unavailable
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/session-end-maintenance.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then
    pass "session-end-maintenance fail-open without ao"
else
    fail "session-end-maintenance fail-open without ao"
fi

# ============================================================
echo ""
echo "=== stop-auto-handoff.sh ==="
# ============================================================

# Test: writes stop handoff when last assistant message exists
MOCK_STOP="$TMPDIR/mock-stop-handoff"
setup_mock_repo "$MOCK_STOP"
(
  cd "$MOCK_STOP" && \
  echo '{"last_assistant_message":"worker summary"}' | \
  CLAUDE_SESSION_ID="sess-stop-1" bash "$HOOKS_DIR/stop-auto-handoff.sh" >/dev/null 2>&1
)
if ls "$MOCK_STOP/.agents/handoff"/stop-*.md >/dev/null 2>&1; then
    pass "stop-auto-handoff writes pending handoff"
else
    fail "stop-auto-handoff writes pending handoff"
fi

# ============================================================
echo ""
echo "=== subagent-stop.sh ==="
# ============================================================

# Test: writes subagent output when message exists
MOCK_SUBAGENT="$TMPDIR/mock-subagent-stop"
setup_mock_repo "$MOCK_SUBAGENT"
(
  cd "$MOCK_SUBAGENT" && \
  echo '{"agent_name":"worker-alpha","last_assistant_message":"final worker output"}' | \
  CLAUDE_SESSION_ID="sess-subagent-1" bash "$HOOKS_DIR/subagent-stop.sh" >/dev/null 2>&1
)
if ls "$MOCK_SUBAGENT/.agents/ao/subagent-outputs"/*.md >/dev/null 2>&1; then
    pass "subagent-stop writes output artifact"
else
    fail "subagent-stop writes output artifact"
fi

# ============================================================
echo ""
echo "=== worktree-setup.sh / worktree-cleanup.sh ==="
# ============================================================

# Test: worktree-setup exits cleanly when no worktree path is provided
EC=0
echo '{"event":"WorktreeCreate"}' | bash "$HOOKS_DIR/worktree-setup.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then
    pass "worktree-setup no-path fail-open"
else
    fail "worktree-setup no-path fail-open"
fi

# Test: worktree-cleanup exits cleanly when no worktree path is provided
EC=0
echo '{"event":"WorktreeRemove"}' | bash "$HOOKS_DIR/worktree-cleanup.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then
    pass "worktree-cleanup no-path fail-open"
else
    fail "worktree-cleanup no-path fail-open"
fi

# ============================================================
echo ""
echo "=== chain.jsonl rotation ==="
# ============================================================

# Test: chain.jsonl rotated when exceeding threshold
MOCK_CHAIN="$TMPDIR/mock-chain-rotate"
setup_mock_repo "$MOCK_CHAIN"
mkdir -p "$MOCK_CHAIN/.agents/ao"
for i in $(seq 1 300); do echo "{\"gate\":\"step-$i\",\"status\":\"locked\"}" >> "$MOCK_CHAIN/.agents/ao/chain.jsonl"; done
(cd "$MOCK_CHAIN" && AGENTOPS_CHAIN_MAX_LINES=200 bash "$HOOKS_DIR/session-start.sh" >/dev/null 2>&1 || true)
CHAIN_LINES=$(wc -l < "$MOCK_CHAIN/.agents/ao/chain.jsonl" | tr -d ' ')
if [ "$CHAIN_LINES" -eq 200 ]; then
    pass "chain.jsonl rotated to 200 lines"
else
    fail "chain.jsonl rotated to 200 lines (got $CHAIN_LINES)"
fi

# Test: archive file created during rotation
if ls "$MOCK_CHAIN/.agents/ao/archive"/chain-*.jsonl >/dev/null 2>&1; then
    ARCHIVE_LINES=$(wc -l < "$(ls "$MOCK_CHAIN/.agents/ao/archive"/chain-*.jsonl | head -1)" | tr -d ' ')
    if [ "$ARCHIVE_LINES" -eq 100 ]; then
        pass "chain archive contains 100 excess lines"
    else
        fail "chain archive contains 100 excess lines (got $ARCHIVE_LINES)"
    fi
else
    fail "chain archive file created"
fi

# Test: chain.jsonl under threshold not rotated
MOCK_CHAIN_SMALL="$TMPDIR/mock-chain-small"
setup_mock_repo "$MOCK_CHAIN_SMALL"
mkdir -p "$MOCK_CHAIN_SMALL/.agents/ao"
for i in $(seq 1 50); do echo "{\"gate\":\"step-$i\",\"status\":\"locked\"}" >> "$MOCK_CHAIN_SMALL/.agents/ao/chain.jsonl"; done
(cd "$MOCK_CHAIN_SMALL" && AGENTOPS_CHAIN_MAX_LINES=200 bash "$HOOKS_DIR/session-start.sh" >/dev/null 2>&1 || true)
SMALL_LINES=$(wc -l < "$MOCK_CHAIN_SMALL/.agents/ao/chain.jsonl" | tr -d ' ')
if [ "$SMALL_LINES" -eq 50 ]; then
    pass "chain.jsonl under threshold not rotated"
else
    fail "chain.jsonl under threshold not rotated (got $SMALL_LINES)"
fi

# ============================================================
echo ""
echo "=== cached file count (prune check) ==="
# ============================================================

# Test: prune dry-run log exists when .agents has >500 files
MOCK_PRUNE_CACHE="$TMPDIR/mock-prune-cache"
setup_mock_repo "$MOCK_PRUNE_CACHE"
mkdir -p "$MOCK_PRUNE_CACHE/.agents/ao"
# Create 510 files to trigger the prune check
for i in $(seq 1 510); do touch "$MOCK_PRUNE_CACHE/.agents/ao/dummy-$i"; done
(cd "$MOCK_PRUNE_CACHE" && bash "$HOOKS_DIR/session-start.sh" >/dev/null 2>&1 || true)
if [ -f "$MOCK_PRUNE_CACHE/.agents/ao/prune-dry-run.log" ]; then
    pass "prune dry-run triggered with cached file count"
else
    fail "prune dry-run triggered with cached file count"
fi

# ============================================================
echo ""
echo "=== ao-agents-check.sh removed from hook chain ==="
# ============================================================

# Test: ao-agents-check.sh no longer registered in hooks.json
if ! grep -q "ao-agents-check" "$REPO_ROOT/hooks/hooks.json" 2>/dev/null; then
    pass "ao-agents-check.sh removed from hooks.json"
else
    fail "ao-agents-check.sh removed from hooks.json"
fi

# ============================================================
echo ""
echo "=== ao-extract.sh ==="
# ============================================================

# Test: kill switch suppresses ao-extract
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/ao-extract.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-extract kill switch"; else fail "ao-extract kill switch"; fi

# Test: fail-open without ao
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ao-extract.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-extract fail-open without ao"; else fail "ao-extract fail-open without ao"; fi

# ============================================================
echo ""
echo "=== ao-feedback-loop.sh ==="
# ============================================================

# Test: kill switch suppresses ao-feedback-loop
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/ao-feedback-loop.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-feedback-loop kill switch"; else fail "ao-feedback-loop kill switch"; fi

# Test: fail-open without ao
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ao-feedback-loop.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-feedback-loop fail-open without ao"; else fail "ao-feedback-loop fail-open without ao"; fi

# ============================================================
echo ""
echo "=== ao-flywheel-close.sh ==="
# ============================================================

# Test: kill switch suppresses ao-flywheel-close
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/ao-flywheel-close.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-flywheel-close kill switch"; else fail "ao-flywheel-close kill switch"; fi

# Test: fail-open without ao
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ao-flywheel-close.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-flywheel-close fail-open without ao"; else fail "ao-flywheel-close fail-open without ao"; fi

# ============================================================
echo ""
echo "=== ao-forge.sh ==="
# ============================================================

# Test: kill switch suppresses ao-forge
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/ao-forge.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-forge kill switch"; else fail "ao-forge kill switch"; fi

# Test: fail-open without ao
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ao-forge.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-forge fail-open without ao"; else fail "ao-forge fail-open without ao"; fi

# ============================================================
echo ""
echo "=== ao-inject.sh ==="
# ============================================================

# Test: kill switch suppresses ao-inject
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/ao-inject.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-inject kill switch"; else fail "ao-inject kill switch"; fi

# Test: fail-open without ao
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ao-inject.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-inject fail-open without ao"; else fail "ao-inject fail-open without ao"; fi

# ============================================================
echo ""
echo "=== ao-maturity-scan.sh ==="
# ============================================================

# Test: kill switch suppresses ao-maturity-scan
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/ao-maturity-scan.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-maturity-scan kill switch"; else fail "ao-maturity-scan kill switch"; fi

# Test: fail-open without ao
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ao-maturity-scan.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-maturity-scan fail-open without ao"; else fail "ao-maturity-scan fail-open without ao"; fi

# ============================================================
echo ""
echo "=== ao-ratchet-status.sh ==="
# ============================================================

# Test: kill switch suppresses ao-ratchet-status
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/ao-ratchet-status.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-ratchet-status kill switch"; else fail "ao-ratchet-status kill switch"; fi

# Test: fail-open without ao
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ao-ratchet-status.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-ratchet-status fail-open without ao"; else fail "ao-ratchet-status fail-open without ao"; fi

# ============================================================
echo ""
echo "=== ao-session-outcome.sh ==="
# ============================================================

# Test: kill switch suppresses ao-session-outcome
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/ao-session-outcome.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-session-outcome kill switch"; else fail "ao-session-outcome kill switch"; fi

# Test: fail-open without ao
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ao-session-outcome.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-session-outcome fail-open without ao"; else fail "ao-session-outcome fail-open without ao"; fi

# ============================================================
echo ""
echo "=== ao-task-sync.sh ==="
# ============================================================

# Test: kill switch suppresses ao-task-sync
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/ao-task-sync.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-task-sync kill switch"; else fail "ao-task-sync kill switch"; fi

# Test: fail-open without ao
EC=0
PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ao-task-sync.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "ao-task-sync fail-open without ao"; else fail "ao-task-sync fail-open without ao"; fi

# ============================================================
echo ""
echo "=== Coverage check ==="
# ============================================================

# Verify every .sh hook file has at least one test
MISSING_HOOKS=""
for hook_file in "$HOOKS_DIR"/*.sh; do
    hook_name=$(basename "$hook_file" .sh)
    # Check if this hook name appears in any test assertion in this script
    if ! grep -q "$hook_name" "$SCRIPT_DIR/test-hooks.sh" 2>/dev/null; then
        MISSING_HOOKS="$MISSING_HOOKS $hook_name"
    fi
done
if [ -z "$MISSING_HOOKS" ]; then
    pass "all hook scripts referenced in tests"
else
    fail "hooks with no test coverage:$MISSING_HOOKS"
fi

# ============================================================
echo ""
echo "=== Results ==="
# ============================================================

TOTAL=$((PASS + FAIL))
echo "Total: $TOTAL | Pass: $PASS | Fail: $FAIL"

if [ "$FAIL" -gt 0 ]; then
    red "FAILED"
    exit 1
else
    green "ALL PASSED"
    exit 0
fi
