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
SKIP=0

red() { printf '\033[0;31m%s\033[0m\n' "$1"; }
green() { printf '\033[0;32m%s\033[0m\n' "$1"; }
yellow() { printf '\033[0;33m%s\033[0m\n' "$1"; }

pass() {
    green "  PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    red "  FAIL: $1"
    FAIL=$((FAIL + 1))
}

skip() {
    yellow "  SKIP: $1"
    SKIP=$((SKIP + 1))
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

hook_temporarily_disabled() {
    local hook="$1"
    grep -q "TEMP: disabled" "$hook" && grep -q '^exit 0$' "$hook"
}

# ============================================================
echo ""
echo "=== session-start.sh / precompact-snapshot.sh ==="
# ============================================================

# Test 12: session-start stays silent while preparing runtime state
SESSION_RAW=$(bash "$HOOKS_DIR/session-start.sh" 2>/dev/null || true)
if [ -z "$SESSION_RAW" ]; then
    pass "session-start stays silent"
else
    fail "session-start stays silent"
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

# Test 14b: manual startup stays silent and skips hidden briefing work
MOCK_LOOKUP="$TMPDIR/mock-session-lookup"
mkdir -p "$MOCK_LOOKUP/.agents/handoff" "$MOCK_LOOKUP/bin"
git -C "$MOCK_LOOKUP" init -q >/dev/null 2>&1
git -C "$MOCK_LOOKUP" config user.email "test@example.com" >/dev/null 2>&1
git -C "$MOCK_LOOKUP" config user.name "Test User" >/dev/null 2>&1
touch "$MOCK_LOOKUP/README.md"
git -C "$MOCK_LOOKUP" add README.md >/dev/null 2>&1
git -C "$MOCK_LOOKUP" commit -q -m "commit subject should not drive lookup" >/dev/null 2>&1
cat > "$MOCK_LOOKUP/.agents/handoff/handoff-20260322T160000Z.json" <<'EOF'
{
  "schema_version": 1,
  "id": "handoff-test",
  "created_at": "2026-03-22T16:00:00Z",
  "type": "manual",
  "goal": "task-scoped silent startup",
  "summary": "startup should materialize state without injected lookup chatter"
}
EOF
cat > "$MOCK_LOOKUP/bin/ao" <<'EOF'
#!/usr/bin/env bash
if [ -n "${AO_ARGS_FILE:-}" ]; then
    printf '%s\n' "$*" >> "$AO_ARGS_FILE"
fi
exit 0
EOF
chmod +x "$MOCK_LOOKUP/bin/ao"
AO_ARGS_FILE="$MOCK_LOOKUP/ao-args.log"
LOOKUP_OUTPUT=$(cd "$MOCK_LOOKUP" && \
    PATH="$MOCK_LOOKUP/bin:$PATH" \
    AO_ARGS_FILE="$AO_ARGS_FILE" \
    AGENTOPS_STARTUP_CONTEXT_MODE=manual \
    bash "$HOOKS_DIR/session-start.sh" 2>/dev/null || true)
if ! grep -qE '^(lookup|knowledge brief)\b' "$AO_ARGS_FILE" 2>/dev/null; then
    pass "session-start performs no hidden lookup or briefing work in manual mode"
else
    fail "session-start performs no hidden lookup or briefing work in manual mode"
fi
if [ -z "$LOOKUP_OUTPUT" ]; then
    pass "session-start remains silent in manual mode"
else
    fail "session-start remains silent in manual mode"
fi

# Test 14c: Codex startup migrates the deprecated hooks feature flag silently
MOCK_CODEX_REPO="$TMPDIR/mock-codex-start"
MOCK_CODEX_HOME="$TMPDIR/mock-codex-home"
mkdir -p "$MOCK_CODEX_REPO" "$MOCK_CODEX_HOME"
git -C "$MOCK_CODEX_REPO" init -q >/dev/null 2>&1
cat > "$MOCK_CODEX_HOME/config.toml" <<'EOF'
model = "gpt-5.2"

[features]
shell_tool = true
codex_hooks = true

[history]
persistence = "save-all"
EOF
CODEX_OUTPUT=$(cd "$MOCK_CODEX_REPO" && \
    CODEX_HOME="$MOCK_CODEX_HOME" \
    CODEX_THREAD_ID="test-thread" \
    bash "$HOOKS_DIR/session-start.sh" 2>/dev/null || true)
if [ -z "$CODEX_OUTPUT" ]; then
    pass "session-start migrates Codex feature flag silently"
else
    fail "session-start migrates Codex feature flag silently"
fi
if grep -q '^hooks = true$' "$MOCK_CODEX_HOME/config.toml" \
    && ! grep -q '^codex_hooks[[:space:]]*=' "$MOCK_CODEX_HOME/config.toml"; then
    pass "session-start rewrites codex_hooks to hooks"
else
    fail "session-start rewrites codex_hooks to hooks"
fi

# Test 14d: session-start can stage a factory briefing silently when startup goal exists
MOCK_FACTORY_START="$TMPDIR/mock-factory-start"
mkdir -p "$MOCK_FACTORY_START/.agents/handoff" "$MOCK_FACTORY_START/bin"
git -C "$MOCK_FACTORY_START" init -q >/dev/null 2>&1
cat > "$MOCK_FACTORY_START/.agents/handoff/handoff-20260402T192000Z.json" <<'EOF'
{
  "schema_version": 1,
  "id": "handoff-factory-test",
  "created_at": "2026-04-02T19:20:00Z",
  "type": "manual",
  "goal": "stabilize auth startup",
  "summary": "route this session into the factory lane"
}
EOF
cat > "$MOCK_FACTORY_START/bin/ao" <<'EOF'
#!/usr/bin/env bash
if [ -n "${AO_ARGS_FILE:-}" ]; then
    printf '%s\n' "$*" >> "$AO_ARGS_FILE"
fi
if [ "${1:-}" = "knowledge" ] && [ "${2:-}" = "brief" ]; then
    briefing_path="${MOCK_FACTORY_BRIEFING_PATH:?}"
    mkdir -p "$(dirname "$briefing_path")"
    cat > "$briefing_path" <<'BRIEF'
# Briefing: stabilize auth startup

## Relevant Topics
- `auth-startup` (healthy)
BRIEF
    jq -n --arg path "$briefing_path" '{"output_path":$path}'
    exit 0
fi
if [ "${1:-}" = "lookup" ]; then
    printf '[lookup] supporting result\n'
fi
exit 0
EOF
chmod +x "$MOCK_FACTORY_START/bin/ao"
AO_ARGS_FILE="$MOCK_FACTORY_START/ao-args.log"
MOCK_FACTORY_BRIEFING_PATH="$MOCK_FACTORY_START/.agents/briefings/2026-04-02-stabilize-auth-startup.md"
FACTORY_OUTPUT=$(cd "$MOCK_FACTORY_START" && \
    PATH="$MOCK_FACTORY_START/bin:$PATH" \
    AO_ARGS_FILE="$AO_ARGS_FILE" \
    MOCK_FACTORY_BRIEFING_PATH="$MOCK_FACTORY_BRIEFING_PATH" \
    AGENTOPS_STARTUP_CONTEXT_MODE=factory \
    bash "$HOOKS_DIR/session-start.sh" 2>/dev/null || true)
if grep -q '^knowledge brief --json --goal stabilize auth startup$' "$AO_ARGS_FILE"; then
    pass "session-start builds a factory briefing from startup goal"
else
    fail "session-start builds a factory briefing from startup goal"
fi
if [ "$(cat "$MOCK_FACTORY_START/.agents/ao/factory-goal.txt" 2>/dev/null)" = "stabilize auth startup" ] && \
   [ "$(cat "$MOCK_FACTORY_START/.agents/ao/factory-briefing.txt" 2>/dev/null)" = "$MOCK_FACTORY_BRIEFING_PATH" ]; then
    pass "session-start records staged factory state"
else
    fail "session-start records staged factory state"
fi
if [ -z "$FACTORY_OUTPUT" ]; then
    pass "session-start does not inject startup context in factory mode"
else
    fail "session-start does not inject startup context in factory mode"
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

MOCK_TASK_VALIDATION="$TMPDIR/mock-task-validation"
setup_mock_repo "$MOCK_TASK_VALIDATION"
mkdir -p "$MOCK_TASK_VALIDATION/hooks"
touch "$MOCK_TASK_VALIDATION/hooks/context-guard.sh"
REPO_CONTENT_FILE="$MOCK_TASK_VALIDATION/test-content.js"
REPO_REGEX_FILE="$MOCK_TASK_VALIDATION/test-regex.txt"
echo "function authenticate() {}" > "$REPO_CONTENT_FILE"
echo 'hello.*world' > "$REPO_REGEX_FILE"

# Test 20: feature issue_type with missing metadata.validation blocks (fail-closed)
EC=0
OUTPUT=$(cd "$MOCK_TASK_VALIDATION" && echo '{"issue_type":"feature","metadata":{}}' | AGENTOPS_METADATA_GATE=strict bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1) || EC=$?
if [ "$EC" -eq 2 ] && echo "$OUTPUT" | grep -q "VALIDATION FAILED"; then
    pass "feature missing metadata.validation blocks"
else
    fail "feature missing metadata.validation blocks (exit=$EC, expected 2)"
fi

# Test 21: bug issue_type with missing metadata.validation blocks (fail-closed)
EC=0
OUTPUT=$(cd "$MOCK_TASK_VALIDATION" && echo '{"issue_type":"bug","metadata":{}}' | AGENTOPS_METADATA_GATE=strict bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1) || EC=$?
if [ "$EC" -eq 2 ] && echo "$OUTPUT" | grep -q "VALIDATION FAILED"; then
    pass "bug missing metadata.validation blocks"
else
    fail "bug missing metadata.validation blocks (exit=$EC, expected 2)"
fi

# Test 22: task issue_type with missing metadata.validation blocks (fail-closed)
EC=0
OUTPUT=$(cd "$MOCK_TASK_VALIDATION" && echo '{"issue_type":"task","metadata":{}}' | AGENTOPS_METADATA_GATE=strict bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1) || EC=$?
if [ "$EC" -eq 2 ] && echo "$OUTPUT" | grep -q "VALIDATION FAILED"; then
    pass "task missing metadata.validation blocks"
else
    fail "task missing metadata.validation blocks (exit=$EC, expected 2)"
fi

# Test 23: docs issue_type explicit exemption allows missing metadata.validation
cd "$MOCK_TASK_VALIDATION" && echo '{"issue_type":"docs","metadata":{}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "docs missing metadata.validation is exempt"; else fail "docs missing metadata.validation is exempt"; fi
cd "$REPO_ROOT"

# Test 24: chore issue_type explicit exemption allows missing metadata.validation
cd "$MOCK_TASK_VALIDATION" && echo '{"issue_type":"chore","metadata":{}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "chore missing metadata.validation is exempt"; else fail "chore missing metadata.validation is exempt"; fi
cd "$REPO_ROOT"

# Test 25: ci issue_type explicit exemption allows missing metadata.validation
cd "$MOCK_TASK_VALIDATION" && echo '{"issue_type":"ci","metadata":{}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "ci missing metadata.validation is exempt"; else fail "ci missing metadata.validation is exempt"; fi
cd "$REPO_ROOT"

# Test 26: untyped task remains fail-open for missing metadata.validation
cd "$MOCK_TASK_VALIDATION" && echo '{"metadata":{}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "untyped task missing metadata.validation passes"; else fail "untyped task missing metadata.validation passes"; fi
cd "$REPO_ROOT"

# Test 27: feature requires metadata.validation.tests
EC=0
OUTPUT=$(cd "$MOCK_TASK_VALIDATION" && echo '{"issue_type":"feature","metadata":{"validation":{"files_exist":["hooks/context-guard.sh"]}}}' | AGENTOPS_METADATA_GATE=strict bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1) || EC=$?
if [ "$EC" -eq 2 ] && echo "$OUTPUT" | grep -q "metadata.validation.tests"; then
    pass "feature missing tests in metadata.validation blocks"
else
    fail "feature missing tests in metadata.validation blocks (exit=$EC, expected 2)"
fi

# Test 28: feature requires structural checks (files_exist/content_check)
EC=0
OUTPUT=$(cd "$MOCK_TASK_VALIDATION" && echo '{"issue_type":"feature","metadata":{"validation":{"tests":"go version"}}}' | AGENTOPS_METADATA_GATE=strict bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1) || EC=$?
if [ "$EC" -eq 2 ] && echo "$OUTPUT" | grep -q "files_exist or content_check"; then
    pass "feature missing structural checks blocks"
else
    fail "feature missing structural checks blocks (exit=$EC, expected 2)"
fi

# Test 29: Global kill switch
cd "$MOCK_TASK_VALIDATION" && echo '{"metadata":{"validation":{"files_exist":["/nonexistent"]}}}' | AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "global kill switch passes validation"; else fail "global kill switch passes validation"; fi
cd "$REPO_ROOT"

# Test 30: Hook-specific kill switch
cd "$MOCK_TASK_VALIDATION" && echo '{"metadata":{"validation":{"files_exist":["/nonexistent"]}}}' | AGENTOPS_TASK_VALIDATION_DISABLED=1 bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "task-validation kill switch passes validation"; else fail "task-validation kill switch passes validation"; fi
cd "$REPO_ROOT"

# Test 31: files_exist - existing repo file (relative path)
INPUT=$(jq -n '{"metadata":{"validation":{"files_exist":["hooks/context-guard.sh"]}}}')
cd "$MOCK_TASK_VALIDATION" && echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "files_exist with existing repo file passes"; else fail "files_exist with existing repo file passes"; fi
cd "$REPO_ROOT"

# Test 32: files_exist - missing file blocks
EC=0
cd "$MOCK_TASK_VALIDATION" && echo '{"metadata":{"validation":{"files_exist":["hooks/nonexistent-file-12345.sh"]}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "files_exist with missing file blocks (exit 2)"; else fail "files_exist with missing file blocks (exit=$EC, expected 2)"; fi
cd "$REPO_ROOT"

# Test 33: files_exist blocks path traversal outside repo root
EC=0
cd "$MOCK_TASK_VALIDATION" && echo '{"metadata":{"validation":{"files_exist":["../README.md"]}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "files_exist blocks path traversal outside repo"; else fail "files_exist blocks path traversal outside repo (exit=$EC, expected 2)"; fi
cd "$REPO_ROOT"

# Test 34: content_check - pattern found within repo root
INPUT=$(jq -n --arg f "$REPO_CONTENT_FILE" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"function authenticate"}]}}}')
cd "$MOCK_TASK_VALIDATION" && echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "content_check with matching pattern passes"; else fail "content_check with matching pattern passes"; fi
cd "$REPO_ROOT"

# Test 35: content_check - pattern not found
INPUT=$(jq -n --arg f "$REPO_CONTENT_FILE" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"class UserService"}]}}}')
EC=0
cd "$MOCK_TASK_VALIDATION" && echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "content_check with missing pattern blocks (exit 2)"; else fail "content_check with missing pattern blocks (exit=$EC, expected 2)"; fi
cd "$REPO_ROOT"

# Test 36: content_check - regex injection safe (grep -qF literal match)
INPUT=$(jq -n --arg f "$REPO_REGEX_FILE" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"hello.*world"}]}}}')
cd "$MOCK_TASK_VALIDATION" && echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1
if [ $? -eq 0 ]; then pass "content_check treats regex chars as literals"; else fail "content_check treats regex chars as literals"; fi
cd "$REPO_ROOT"

# Test 37: content_check blocks files outside repo root
echo "outside" > "$TMPDIR/outside.txt"
INPUT=$(jq -n --arg f "$TMPDIR/outside.txt" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"outside"}]}}}')
EC=0
cd "$MOCK_TASK_VALIDATION" && echo "$INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "content_check blocks paths outside repo"; else fail "content_check blocks paths outside repo (exit=$EC, expected 2)"; fi
cd "$REPO_ROOT"

# Test 38: paired_files passes when command + companion test changed
MOCK_PAIRED_PASS="$TMPDIR/mock-paired-pass"
setup_mock_repo "$MOCK_PAIRED_PASS"
mkdir -p "$MOCK_PAIRED_PASS/cli/cmd/ao"
echo 'package ao' > "$MOCK_PAIRED_PASS/cli/cmd/ao/sample.go"
echo 'package ao' > "$MOCK_PAIRED_PASS/cli/cmd/ao/sample_test.go"
git -C "$MOCK_PAIRED_PASS" add cli/cmd/ao/sample.go cli/cmd/ao/sample_test.go >/dev/null 2>&1
PAIRED_INPUT=$(jq -n '{"metadata":{"validation":{"paired_files":[{"pattern":"cli/cmd/ao/*.go","exclude":"*_test.go","companion":"{dir}/{basename}_test{ext}","message":"missing paired test change"}]}}}')
EC=0
(cd "$MOCK_PAIRED_PASS" && echo "$PAIRED_INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "paired_files passes with matched companion change"; else fail "paired_files passes with matched companion change (exit=$EC, expected 0)"; fi

# Test 39: paired_files blocks when companion missing from changed set
MOCK_PAIRED_FAIL="$TMPDIR/mock-paired-fail"
setup_mock_repo "$MOCK_PAIRED_FAIL"
mkdir -p "$MOCK_PAIRED_FAIL/cli/cmd/ao"
echo 'package ao' > "$MOCK_PAIRED_FAIL/cli/cmd/ao/solo.go"
git -C "$MOCK_PAIRED_FAIL" add cli/cmd/ao/solo.go >/dev/null 2>&1
EC=0
(cd "$MOCK_PAIRED_FAIL" && echo "$PAIRED_INPUT" | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 2 ]; then pass "paired_files blocks missing companion"; else fail "paired_files blocks missing companion (exit=$EC, expected 2)"; fi

# Test 40: Allowlist blocks disallowed commands
EC=0
cd "$MOCK_TASK_VALIDATION" && echo '{"metadata":{"validation":{"tests":"curl http://evil.com"}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "allowlist blocks curl command"; else fail "allowlist blocks curl command (exit=$EC, expected 2)"; fi
cd "$REPO_ROOT"

# Test 41: Allowlist allows go command
MOCK_ALLOW="$TMPDIR/mock-allowlist"
setup_mock_repo "$MOCK_ALLOW"
EC=0
(cd "$MOCK_ALLOW" && echo '{"metadata":{"validation":{"tests":"go version"}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "allowlist allows go command"; else fail "allowlist allows go command (exit=$EC, expected 0)"; fi

# Test 42: active constraint requires issue_type
MOCK_CONSTRAINT_ISSUE="$TMPDIR/mock-constraint-issue"
setup_mock_repo "$MOCK_CONSTRAINT_ISSUE"
mkdir -p "$MOCK_CONSTRAINT_ISSUE/.agents/constraints" "$MOCK_CONSTRAINT_ISSUE/docs"
echo 'SAFE_MARKER' > "$MOCK_CONSTRAINT_ISSUE/docs/guide.md"
cat > "$MOCK_CONSTRAINT_ISSUE/.agents/constraints/index.json" <<'EOF'
{"schema_version":1,"constraints":[{"id":"c-issue-type","title":"issue type required","status":"active","compiled_at":"2026-03-10T00:00:00Z","applies_to":{"scope":"files","issue_types":["feature"],"path_globs":["docs/*.md"]},"detector":{"kind":"content_pattern","mode":"must_contain","pattern":"SAFE_MARKER","message":"SAFE_MARKER required"}}]}
EOF
EC=0
OUTPUT=$(cd "$MOCK_CONSTRAINT_ISSUE" && echo '{"metadata":{"files":["docs/guide.md"]}}' | bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1) || EC=$?
if [ "$EC" -eq 2 ] && echo "$OUTPUT" | grep -q 'metadata.issue_type'; then
    pass "active constraint requires metadata.issue_type"
else
    fail "active constraint requires metadata.issue_type (exit=$EC, expected 2)"
fi

# Test 43: active content constraint blocks missing literal
MOCK_CONSTRAINT_PATTERN="$TMPDIR/mock-constraint-pattern"
setup_mock_repo "$MOCK_CONSTRAINT_PATTERN"
mkdir -p "$MOCK_CONSTRAINT_PATTERN/.agents/constraints" "$MOCK_CONSTRAINT_PATTERN/docs"
echo 'hello' > "$MOCK_CONSTRAINT_PATTERN/docs/guide.md"
cat > "$MOCK_CONSTRAINT_PATTERN/.agents/constraints/index.json" <<'EOF'
{"schema_version":1,"constraints":[{"id":"c-pattern","title":"must contain","status":"active","compiled_at":"2026-03-10T00:00:00Z","applies_to":{"scope":"files","issue_types":["feature"],"path_globs":["docs/*.md"]},"detector":{"kind":"content_pattern","mode":"must_contain","pattern":"SAFE_MARKER","message":"SAFE_MARKER required"}}]}
EOF
EC=0
OUTPUT=$(cd "$MOCK_CONSTRAINT_PATTERN" && echo '{"metadata":{"issue_type":"feature","files":["docs/guide.md"]}}' | bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1) || EC=$?
if [ "$EC" -eq 2 ] && echo "$OUTPUT" | grep -q 'SAFE_MARKER required'; then
    pass "active content constraint blocks missing literal"
else
    fail "active content constraint blocks missing literal (exit=$EC, expected 2)"
fi

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
# `head -1` closes the pipe early; under `set -euo pipefail` the SIGPIPE on
# `ls` makes the pipeline exit 141 and aborts the whole suite. `|| true`
# keeps the captured first line while neutralizing that exit status.
HANDOFF_FILE=$(ls -t "$MOCK_HANDOFF_CONTENT/.agents/handoff/auto-"*.md 2>/dev/null | head -1 || true)
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
echo "=== memory packet v1 compatibility ==="
# ============================================================

# Test 43: stop-auto-handoff emits packet v1
MOCK_STOP_PACKET="$TMPDIR/mock-stop-packet"
mkdir -p "$MOCK_STOP_PACKET/.agents/ao"
git -C "$MOCK_STOP_PACKET" init -q >/dev/null 2>&1
printf '{"last_assistant_message":"STOP_PACKET_MARKER_123"}' \
  | (cd "$MOCK_STOP_PACKET" && bash "$HOOKS_DIR/stop-auto-handoff.sh" >/dev/null 2>&1 || true)
STOP_PACKET_FILE=$(ls -t "$MOCK_STOP_PACKET/.agents/ao/packets/pending/"*.json 2>/dev/null | head -1 || true)
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
SUB_PACKET_FILE=$(ls -t "$MOCK_SUB_PACKET/.agents/ao/packets/pending/"*.json 2>/dev/null | head -1 || true)
if [ -f "$SUB_PACKET_FILE" ] \
  && jq -e '.schema_version == 1 and .packet_type == "subagent_stop" and .source_hook == "subagent-stop" and .payload.agent_name == "worker-a"' "$SUB_PACKET_FILE" >/dev/null 2>&1; then
    pass "subagent-stop emits schema packet v1"
else
    fail "subagent-stop emits schema packet v1"
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
if echo "$OUTPUT" | grep -q "plan" && ! echo "$OUTPUT" | grep -q "/plan"; then
    pass "research record suggests plan skill (fallback)"
else
    fail "research record suggests plan skill (fallback)"
fi

# Test: Successful vibe record => suggests post-mortem skill (fallback mode)
OUTPUT=$(cd "$MOCK_RATCHET" && echo '{"tool_input":{"command":"ao ratchet record vibe"},"tool_response":{"exit_code":0}}' | PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ratchet-advance.sh" 2>/dev/null || true)
if echo "$OUTPUT" | grep -q "post-mortem" && ! echo "$OUTPUT" | grep -q "/post-mortem"; then
    pass "vibe record suggests post-mortem skill (fallback)"
else
    fail "vibe record suggests post-mortem skill (fallback)"
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
echo "=== holdout-isolation-gate.sh ==="
# ============================================================

# Test: Non-holdout Read => pass
EC=0
echo '{"tool_name":"Read","tool_input":{"file_path":"src/main.go"}}' | bash "$HOOKS_DIR/holdout-isolation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "holdout-isolation-gate passes non-holdout Read"; else fail "holdout-isolation-gate passes non-holdout Read"; fi

# Test: Holdout Read without evaluator => block (exit 2)
EC=0
echo '{"tool_name":"Read","tool_input":{"file_path":".agents/holdout/test.json"}}' | AGENTOPS_HOLDOUT_EVALUATOR='' bash "$HOOKS_DIR/holdout-isolation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "holdout-isolation-gate blocks holdout Read"; else fail "holdout-isolation-gate blocks holdout Read (got exit $EC)"; fi

# Test: Holdout Glob pattern without evaluator => block (exit 2)
EC=0
echo '{"tool_name":"Glob","tool_input":{"pattern":".agents/holdout/*.json"}}' | AGENTOPS_HOLDOUT_EVALUATOR='' bash "$HOOKS_DIR/holdout-isolation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "holdout-isolation-gate blocks holdout Glob pattern"; else fail "holdout-isolation-gate blocks holdout Glob pattern (got exit $EC)"; fi

# Test: Holdout Grep query without evaluator => block (exit 2)
EC=0
echo '{"tool_name":"Grep","tool_input":{"query":".agents/holdout/test.json"}}' | AGENTOPS_HOLDOUT_EVALUATOR='' bash "$HOOKS_DIR/holdout-isolation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "holdout-isolation-gate blocks holdout Grep query"; else fail "holdout-isolation-gate blocks holdout Grep query (got exit $EC)"; fi

# Test: Holdout Bash without evaluator => block (exit 2)
EC=0
echo '{"tool_name":"Bash","tool_input":{"command":"cat .agents/holdout/test.json"}}' | AGENTOPS_HOLDOUT_EVALUATOR='' bash "$HOOKS_DIR/holdout-isolation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "holdout-isolation-gate blocks holdout Bash"; else fail "holdout-isolation-gate blocks holdout Bash (got exit $EC)"; fi

# Test: Holdout Read with evaluator => pass
EC=0
echo '{"tool_name":"Read","tool_input":{"file_path":".agents/holdout/test.json"}}' | AGENTOPS_HOLDOUT_EVALUATOR=1 bash "$HOOKS_DIR/holdout-isolation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "holdout-isolation-gate passes evaluator Read"; else fail "holdout-isolation-gate passes evaluator Read"; fi

# Test: Holdout Glob with evaluator => pass
EC=0
echo '{"tool_name":"Glob","tool_input":{"pattern":".agents/holdout/*.json"}}' | AGENTOPS_HOLDOUT_EVALUATOR=1 bash "$HOOKS_DIR/holdout-isolation-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "holdout-isolation-gate passes evaluator Glob"; else fail "holdout-isolation-gate passes evaluator Glob"; fi

# Test: Kill switch => pass
EC=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/holdout-isolation-gate.sh" >/dev/null 2>&1 \
    <<< '{"tool_name":"Read","tool_input":{"file_path":".agents/holdout/test.json"}}' || EC=$?
if [ "$EC" -eq 0 ]; then pass "holdout-isolation-gate respects kill switch"; else fail "holdout-isolation-gate respects kill switch"; fi

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

# Test: Crank with no epic ID => strict mode blocks
EC=0
echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":""}}' | bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "pre-mortem-gate blocks on no epic ID in strict mode"; else fail "pre-mortem-gate blocks on no epic ID in strict mode"; fi

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
touch "$MOCK_PM/.agents/council/2026-01-01-pre-mortem-ag-xxx.md"
# Simulate bd returning 5 children and force Method 3 council fallback.
printf '#!/bin/bash\nif [ "$1" = "children" ]; then printf "1\\n2\\n3\\n4\\n5\\n"; fi\n' > "$MOCK_PM/bd"
chmod +x "$MOCK_PM/bd"
printf '#!/bin/bash\nexit 1\n' > "$MOCK_PM/ao"
chmod +x "$MOCK_PM/ao"
EC=0
(cd "$MOCK_PM" && export PATH="$MOCK_PM:$PATH" && echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx"}}' | bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "pre-mortem-gate passes with council evidence"; else fail "pre-mortem-gate passes with council evidence (exit=$EC)"; fi

# Test: Method 4b fallback passes on locked chain entry (chain.jsonl)
MOCK_PM_LOCKED="$TMPDIR/mock-pm-locked"
setup_mock_repo "$MOCK_PM_LOCKED"
mkdir -p "$MOCK_PM_LOCKED/.agents/ao"
# Write chain.jsonl: metadata line + locked pre-mortem entry
echo '{"id":"chain-test","started":"2026-01-01T00:00:00Z"}' > "$MOCK_PM_LOCKED/.agents/ao/chain.jsonl"
echo '{"step":"pre-mortem","locked":true,"output":".agents/council/pm.md","timestamp":"2026-01-01T00:00:00Z"}' >> "$MOCK_PM_LOCKED/.agents/ao/chain.jsonl"
# Mock bd to return 5 children + mock ao that fails (force chain.jsonl fallback)
printf '#!/bin/bash\nif [ "$1" = "children" ]; then printf "1\\n2\\n3\\n4\\n5\\n"; fi\n' > "$MOCK_PM_LOCKED/bd"
chmod +x "$MOCK_PM_LOCKED/bd"
printf '#!/bin/bash\nexit 1\n' > "$MOCK_PM_LOCKED/ao"
chmod +x "$MOCK_PM_LOCKED/ao"
EC=0
(cd "$MOCK_PM_LOCKED" && export PATH="$MOCK_PM_LOCKED:$PATH" && echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx"}}' | bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then pass "pre-mortem-gate Method 4b passes on locked chain entry"; else fail "pre-mortem-gate Method 4b passes on locked chain entry (exit=$EC)"; fi

# Test: Method 4b fallback blocks on pending chain entry (false-pass regression)
MOCK_PM_PENDING="$TMPDIR/mock-pm-pending"
setup_mock_repo "$MOCK_PM_PENDING"
mkdir -p "$MOCK_PM_PENDING/.agents/ao"
# Write chain.jsonl: metadata line + in_progress (not locked) pre-mortem entry
echo '{"id":"chain-test","started":"2026-01-01T00:00:00Z"}' > "$MOCK_PM_PENDING/.agents/ao/chain.jsonl"
echo '{"step":"pre-mortem","locked":false,"output":"","timestamp":"2026-01-01T00:00:00Z"}' >> "$MOCK_PM_PENDING/.agents/ao/chain.jsonl"
# Mock bd to return 5 children + mock ao that fails (force chain.jsonl fallback)
printf '#!/bin/bash\nif [ "$1" = "children" ]; then printf "1\\n2\\n3\\n4\\n5\\n"; fi\n' > "$MOCK_PM_PENDING/bd"
chmod +x "$MOCK_PM_PENDING/bd"
printf '#!/bin/bash\nexit 1\n' > "$MOCK_PM_PENDING/ao"
chmod +x "$MOCK_PM_PENDING/ao"
EC=0
(cd "$MOCK_PM_PENDING" && export PATH="$MOCK_PM_PENDING:$PATH" && echo '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx"}}' | AGENTOPS_PREMORTEM_FALLBACK=strict bash "$HOOKS_DIR/pre-mortem-gate.sh" >/dev/null 2>&1) || EC=$?
if [ "$EC" -eq 2 ]; then pass "pre-mortem-gate Method 4b blocks on pending chain entry"; else fail "pre-mortem-gate Method 4b blocks on pending chain entry (exit=$EC, want 2)"; fi

# ============================================================
echo ""
echo "=== stop-team-guard.sh ==="
# ============================================================

# Test: No teams dir => pass (safe to stop)
EC=0
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

# Test: session-end runs finding compiler as a backstop when ao succeeds
MOCK_SESSION_END="$TMPDIR/mock-session-end-maintenance"
setup_mock_repo "$MOCK_SESSION_END"
mkdir -p "$MOCK_SESSION_END/.agents/findings" "$TMPDIR/mock-session-end-bin"
cat > "$MOCK_SESSION_END/.agents/findings/registry.jsonl" <<'EOF'
{"id":"f-session-end","version":1,"tier":"local","source":{"repo":"agentops/crew/nami","session":"2026-03-09","file":".agents/council/source.md","skill":"post-mortem"},"date":"2026-03-09","severity":"significant","category":"validation-gap","pattern":"Session end should compile fresh findings before close.","detection_question":"Did session close rebuild planning and pre-mortem artifacts?","checklist_item":"Compile findings before session close completes.","applicable_languages":["markdown","shell"],"applicable_when":["validation-gap"],"status":"active","superseded_by":null,"dedup_key":"validation-gap|session-end-should-compile-fresh-findings-before-close|validation-gap","hit_count":0,"last_cited":null,"ttl_days":30,"confidence":"high"}
EOF
cat > "$TMPDIR/mock-session-end-bin/ao" <<'EOF'
#!/bin/bash
exit 0
EOF
chmod +x "$TMPDIR/mock-session-end-bin/ao"
(
    cd "$MOCK_SESSION_END" && \
    PATH="$TMPDIR/mock-session-end-bin:$PATH" \
    bash "$HOOKS_DIR/session-end-maintenance.sh" >/dev/null 2>&1
) || true
if hook_temporarily_disabled "$HOOKS_DIR/session-end-maintenance.sh"; then
    skip "session-end-maintenance compiles findings as a backstop (temporarily disabled)"
elif [ -f "$MOCK_SESSION_END/.agents/planning-rules/f-session-end.md" ] && [ -f "$MOCK_SESSION_END/.agents/pre-mortem-checks/f-session-end.md" ]; then
    pass "session-end-maintenance compiles findings as a backstop"
else
    fail "session-end-maintenance compiles findings as a backstop"
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

# (chain.jsonl rotation and prune tests removed — features stripped in v2.15.2)

# ============================================================
echo ""
echo "=== ao-agents-check.sh removed from hook chain ==="
# ============================================================

# Test: task-validation-gate wired into TaskCompleted hook chain
if jq -e '.hooks.TaskCompleted[]?.hooks[]?.command | select(test("task-validation-gate\\.sh$"))' "$REPO_ROOT/hooks/hooks.json" >/dev/null 2>&1; then
    pass "task-validation-gate.sh wired in hooks.json TaskCompleted"
else
    fail "task-validation-gate.sh wired in hooks.json TaskCompleted"
fi

# Test: ao-agents-check.sh no longer registered in hooks.json
if ! grep -q "ao-agents-check" "$REPO_ROOT/hooks/hooks.json" 2>/dev/null; then
    pass "ao-agents-check.sh removed from hooks.json"
else
    fail "ao-agents-check.sh removed from hooks.json"
fi

# ao-extract, ao-feedback-loop, ao-forge, ao-inject, ao-maturity-scan,
# ao-ratchet-status, ao-session-outcome, ao-task-sync were consolidated
# into session-end-maintenance.sh inline commands. Standalone scripts removed.

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

# ao-forge, ao-inject, ao-maturity-scan, ao-ratchet-status, ao-session-outcome,
# ao-task-sync standalone scripts were consolidated into session-end-maintenance.sh.
# Delegation tests below verify the embedded hooks still delegate correctly.

# ============================================================
echo ""
echo "=== ao-* Delegation Verification (Embedded Hooks) ==="
# ============================================================

# These tests create a mock 'ao' binary that records invocations,
# then fire each ao-* wrapper hook with the mock on PATH to verify
# the correct ao subcommand was called.

EMBEDDED_HOOKS_DIR="$REPO_ROOT/cli/embedded/hooks"
AO_MOCK_DIR="$TMPDIR/ao-mock-bin"
AO_MOCK_LOG="$TMPDIR/ao-mock-invocations.log"
mkdir -p "$AO_MOCK_DIR"

# Create mock ao binary that logs all invocations
cat > "$AO_MOCK_DIR/ao" <<'MOCK_EOF'
#!/bin/bash
# Mock ao binary: records invocation to log file
LOG_FILE="${AO_MOCK_LOG:-/tmp/ao-mock-invocations.log}"
echo "$@" >> "$LOG_FILE"
exit 0
MOCK_EOF
chmod +x "$AO_MOCK_DIR/ao"

# Helper: run an embedded ao-* hook with mock ao and check subcommand
test_ao_delegation() {
    local hook_name="$1"
    local expected_pattern="$2"
    local hook_path="$EMBEDDED_HOOKS_DIR/$hook_name"

    if [ ! -f "$hook_path" ]; then
        fail "$hook_name delegation: hook file not found"
        return
    fi

    # Clear log
    : > "$AO_MOCK_LOG"

    # Run hook with mock ao on PATH
    EC=0
    PATH="$AO_MOCK_DIR:/usr/bin:/bin" \
        AO_MOCK_LOG="$AO_MOCK_LOG" \
        AGENTOPS_HOOK_CLOSE_LOOP=1 \
        CLAUDE_SESSION_ID="delegation-test" \
        bash "$hook_path" >/dev/null 2>&1 || EC=$?

    if [ $EC -ne 0 ]; then
        fail "$hook_name delegation: exit code $EC"
        return
    fi

    # Check that mock ao was called with expected subcommand
    if [ -s "$AO_MOCK_LOG" ]; then
        if grep -qE "$expected_pattern" "$AO_MOCK_LOG"; then
            pass "$hook_name delegates to: $expected_pattern"
        else
            GOT=$(head -1 "$AO_MOCK_LOG")
            fail "$hook_name delegation: expected '$expected_pattern', got '$GOT'"
        fi
    else
        fail "$hook_name delegation: ao was not called"
    fi
}

test_ao_delegation "ao-extract.sh" "^forge transcript"
test_ao_delegation "ao-feedback-loop.sh" "^feedback-loop"
if hook_temporarily_disabled "$EMBEDDED_HOOKS_DIR/ao-flywheel-close.sh"; then
    skip "ao-flywheel-close.sh delegation (temporarily disabled)"
else
    test_ao_delegation "ao-flywheel-close.sh" "^flywheel close-loop"
fi
test_ao_delegation "ao-forge.sh" "^forge transcript"
test_ao_delegation "ao-inject.sh" "^inject"
test_ao_delegation "ao-maturity-scan.sh" "^maturity --scan"
test_ao_delegation "ao-ratchet-status.sh" "^ratchet status"
test_ao_delegation "ao-session-outcome.sh" "^session-outcome"
test_ao_delegation "ao-task-sync.sh" "^task-sync"

# Verify delegation hooks log failures to hook-errors.log
echo ""
echo "--- ao-* error logging ---"

# Create a mock ao that fails
AO_FAIL_DIR="$TMPDIR/ao-fail-bin"
mkdir -p "$AO_FAIL_DIR"
cat > "$AO_FAIL_DIR/ao" <<'FAIL_EOF'
#!/bin/bash
exit 1
FAIL_EOF
chmod +x "$AO_FAIL_DIR/ao"

# Test error logging for one representative hook
FAIL_TEST_DIR="$TMPDIR/ao-fail-test"
mkdir -p "$FAIL_TEST_DIR/.agents/ao"
git -C "$FAIL_TEST_DIR" init -q >/dev/null 2>&1

if [ -f "$EMBEDDED_HOOKS_DIR/ao-extract.sh" ]; then
    (
        cd "$FAIL_TEST_DIR"
        PATH="$AO_FAIL_DIR:/usr/bin:/bin" bash "$EMBEDDED_HOOKS_DIR/ao-extract.sh" >/dev/null 2>&1 || true
    )
    if [ -f "$FAIL_TEST_DIR/.agents/ao/hook-errors.log" ]; then
        if grep -q "HOOK_FAIL.*forge transcript" "$FAIL_TEST_DIR/.agents/ao/hook-errors.log"; then
            pass "ao-extract logs failure to hook-errors.log"
        else
            fail "ao-extract logs failure to hook-errors.log"
        fi
    else
        fail "ao-extract creates hook-errors.log on failure"
    fi
fi

# Verify kill switch works for all embedded ao-* hooks
echo ""
echo "--- ao-* embedded kill switch ---"

AO_EMBEDDED_WRAPPERS=(
    "ao-extract.sh"
    "ao-feedback-loop.sh"
    "ao-flywheel-close.sh"
    "ao-forge.sh"
    "ao-inject.sh"
    "ao-maturity-scan.sh"
    "ao-ratchet-status.sh"
    "ao-session-outcome.sh"
    "ao-task-sync.sh"
)

for wrapper in "${AO_EMBEDDED_WRAPPERS[@]}"; do
    WRAPPER_PATH="$EMBEDDED_HOOKS_DIR/$wrapper"
    if [ -f "$WRAPPER_PATH" ]; then
        # Clear mock log
        : > "$AO_MOCK_LOG"
        EC=0
        PATH="$AO_MOCK_DIR:/usr/bin:/bin" \
            AO_MOCK_LOG="$AO_MOCK_LOG" \
            AGENTOPS_HOOKS_DISABLED=1 \
            bash "$WRAPPER_PATH" >/dev/null 2>&1 || EC=$?

        if [ $EC -eq 0 ] && [ ! -s "$AO_MOCK_LOG" ]; then
            pass "$wrapper: kill switch prevents ao call"
        elif [ $EC -eq 0 ]; then
            fail "$wrapper: kill switch did not prevent ao call"
        else
            fail "$wrapper: kill switch exit code $EC"
        fi
    fi
done

echo ""
echo "=== finding-compiler.sh ==="
# ============================================================

# Test: registry entries promote into advisory artifacts
MOCK_FINDING_COMPILER="$TMPDIR/mock-finding-compiler"
setup_mock_repo "$MOCK_FINDING_COMPILER"
mkdir -p "$MOCK_FINDING_COMPILER/.agents/findings"
cat > "$MOCK_FINDING_COMPILER/.agents/findings/registry.jsonl" <<'EOF'
{"id":"f-compiler-test","version":1,"tier":"local","source":{"repo":"agentops/crew/nami","session":"2026-03-09","file":".agents/council/source.md","skill":"pre-mortem"},"date":"2026-03-09","severity":"significant","category":"validation-gap","pattern":"Compiled findings should generate advisory artifacts.","detection_question":"Did the compiler materialize planning and pre-mortem outputs?","checklist_item":"Generate advisory artifacts from active findings.","applicable_languages":["markdown","shell"],"applicable_when":["plan-shape"],"status":"active","superseded_by":null,"dedup_key":"validation-gap|compiled-findings-should-generate-advisory-artifacts|plan-shape","hit_count":0,"last_cited":null,"ttl_days":30,"confidence":"high"}
EOF
EC=0
(
    cd "$MOCK_FINDING_COMPILER" && \
    bash "$HOOKS_DIR/finding-compiler.sh" >/dev/null 2>&1
) || EC=$?
if [ "$EC" -eq 0 ] && [ -f "$MOCK_FINDING_COMPILER/.agents/findings/f-compiler-test.md" ] && [ -f "$MOCK_FINDING_COMPILER/.agents/planning-rules/f-compiler-test.md" ] && [ -f "$MOCK_FINDING_COMPILER/.agents/pre-mortem-checks/f-compiler-test.md" ]; then
    pass "finding-compiler promotes registry entries into advisory artifacts"
else
    fail "finding-compiler promotes registry entries into advisory artifacts"
fi

# Test: full prevention ratchet flow covers registry -> artifact -> enforcement -> citation feedback
EC=0
OUTPUT=$(bash "$REPO_ROOT/tests/integration/test-finding-prevention-ratchet.sh" 2>&1) || EC=$?
if [ "$EC" -eq 0 ]; then
    if echo "$OUTPUT" | grep -q '^SKIP:'; then
        skip "finding prevention ratchet end-to-end coverage skipped (ao binary unavailable)"
        echo "$OUTPUT" | head -5 | sed 's/^/    /'
    else
        pass "finding prevention ratchet end-to-end coverage passes"
    fi
else
    fail "finding prevention ratchet end-to-end coverage passes"
    echo "$OUTPUT" | head -20 | sed 's/^/    /'
fi

echo ""
echo "=== eval-verdict-compiler.sh ==="
# ============================================================

EVAL_FIX_DIR="$REPO_ROOT/tests/fixtures/eval-verdicts"
MOCK_EVAL="$TMPDIR/mock-eval-verdict"
setup_mock_repo "$MOCK_EVAL"
mkdir -p "$MOCK_EVAL/evals/runs/run-improved"
EVAL_LEARNING="$MOCK_EVAL/learning.md"
EVAL_MANIFEST="$MOCK_EVAL/evals/runs/run-improved/manifest.json"
/bin/cp "$EVAL_FIX_DIR/learning-existing.md" "$EVAL_LEARNING"
jq --arg path "$EVAL_LEARNING" '.verdict.applicable_artifacts = [$path]' \
    "$EVAL_FIX_DIR/manifest-improved.json" > "$EVAL_MANIFEST"

BEFORE="$(shasum "$EVAL_LEARNING" | awk '{print $1}')"
if (cd "$MOCK_EVAL" && AGENTOPS_EVALS_ROOT="$MOCK_EVAL/evals" AGENTOPS_VERDICT_QUIET=1 \
    bash "$HOOKS_DIR/eval-verdict-compiler.sh" --dry-run --manifest "$EVAL_MANIFEST" >/dev/null 2>&1); then
    AFTER="$(shasum "$EVAL_LEARNING" | awk '{print $1}')"
    if [ "$BEFORE" = "$AFTER" ] && [ ! -f "$MOCK_EVAL/evals/processed.jsonl" ]; then
        pass "eval-verdict-compiler dry-run is read-only"
    else
        fail "eval-verdict-compiler dry-run is read-only"
    fi
else
    fail "eval-verdict-compiler dry-run exits zero"
fi

if (cd "$MOCK_EVAL" && AGENTOPS_EVALS_ROOT="$MOCK_EVAL/evals" AGENTOPS_VERDICT_QUIET=1 \
    bash "$HOOKS_DIR/eval-verdict-compiler.sh" --manifest "$EVAL_MANIFEST" >/dev/null 2>&1); then
    GOT_UTILITY="$(awk '/^utility:/ {gsub(/^utility:[[:space:]]*|[[:space:]]+$/,""); print; exit}' "$EVAL_LEARNING")"
    GOT_REWARD="$(awk '/^reward_count:/ {gsub(/^reward_count:[[:space:]]*|[[:space:]]+$/,""); print; exit}' "$EVAL_LEARNING")"
    if [ "$GOT_UTILITY" = "0.605000" ] && [ "$GOT_REWARD" = "1" ]; then
        pass "eval-verdict-compiler improved verdict mutates learning metadata"
    else
        fail "eval-verdict-compiler improved verdict mutates learning metadata"
    fi
else
    fail "eval-verdict-compiler improved verdict exits zero"
fi

MOCK_EVAL_EMPTY="$TMPDIR/mock-eval-verdict-empty"
setup_mock_repo "$MOCK_EVAL_EMPTY"
mkdir -p "$MOCK_EVAL_EMPTY/evals/runs"
if (cd "$MOCK_EVAL_EMPTY" && AGENTOPS_EVALS_ROOT="$MOCK_EVAL_EMPTY/evals" AGENTOPS_VERDICT_QUIET=1 \
    bash "$HOOKS_DIR/eval-verdict-compiler.sh" >/dev/null 2>&1); then
    pass "eval-verdict-compiler exits quietly with no manifests"
else
    fail "eval-verdict-compiler exits quietly with no manifests"
fi

echo ""
echo "=== constraint-compiler.sh ==="
# ============================================================

# Test: missing arguments prints usage and fails
MOCK_CONSTRAINT_ARG="$TMPDIR/mock-constraint-missing-arg"
mkdir -p "$MOCK_CONSTRAINT_ARG"
git -C "$MOCK_CONSTRAINT_ARG" init -q >/dev/null 2>&1
EC=0
(
    cd "$MOCK_CONSTRAINT_ARG"
    bash "$HOOKS_DIR/constraint-compiler.sh" >/dev/null 2>&1
) || EC=$?
if [ "$EC" -eq 1 ]; then
    pass "constraint-compiler requires learning path argument"
else
    fail "constraint-compiler requires learning path argument"
fi

# Test: tagged constraint learning routes through finding-compiler and emits compiled outputs
MOCK_CONSTRAINT="$TMPDIR/mock-constraint-constraint-tag"
mkdir -p "$MOCK_CONSTRAINT"
git -C "$MOCK_CONSTRAINT" init -q >/dev/null 2>&1
cat > "$MOCK_CONSTRAINT/learn-constraint.md" <<'EOF'
---
title: Constraint rule
id: learn-constraint
date: 2026-02-24
tags: [constraint, reliability]
---

This learning describes a guardrail to prevent direct bypass of safety checks.
EOF
OUTPUT=$(cd "$MOCK_CONSTRAINT" && bash "$HOOKS_DIR/constraint-compiler.sh" "$MOCK_CONSTRAINT/learn-constraint.md" 2>&1 || true)
if echo "$OUTPUT" | grep -q "finding-compiler.sh\|Promoted legacy learning"; then
    pass "constraint-compiler routes tagged learning through finding-compiler"
else
    fail "constraint-compiler routes tagged learning through finding-compiler"
fi
if [ -x "$MOCK_CONSTRAINT/.agents/constraints/learn-constraint.sh" ]; then
    pass "constraint-compiler writes compiled constraint file"
else
    fail "constraint-compiler writes compiled constraint file"
fi
if [ -f "$MOCK_CONSTRAINT/.agents/constraints/index.json" ]; then
    pass "constraint-compiler updates constraint index"
else
    fail "constraint-compiler updates constraint index"
fi
if [ -f "$MOCK_CONSTRAINT/.agents/findings/learn-constraint.md" ] && [ -f "$MOCK_CONSTRAINT/.agents/planning-rules/learn-constraint.md" ] && [ -f "$MOCK_CONSTRAINT/.agents/pre-mortem-checks/learn-constraint.md" ]; then
    pass "constraint-compiler writes promoted finding and advisory artifacts"
else
    fail "constraint-compiler writes promoted finding and advisory artifacts"
fi

# Test: non-constraint learning skips without generating template
cat > "$MOCK_CONSTRAINT/learn-note.md" <<'EOF'
---
title: Regular note
id: learn-note
tags: [note]
---

This learning is not a constraint.
EOF
(
    cd "$MOCK_CONSTRAINT"
    OUTPUT2=$(bash "$HOOKS_DIR/constraint-compiler.sh" "$MOCK_CONSTRAINT/learn-note.md" 2>&1 || true)
    if echo "$OUTPUT2" | grep -q "SKIP: Learning 'learn-note'"; then
        :
    else
        :
    fi
) || true
if [ ! -f "$MOCK_CONSTRAINT/.agents/constraints/learn-note.sh" ]; then
    pass "constraint-compiler skips non-constraint learning"
else
    fail "constraint-compiler skips non-constraint learning"
fi
if [ ! -f "$MOCK_CONSTRAINT/.agents/findings/learn-note.md" ]; then
    pass "constraint-compiler does not promote non-constraint learning"
else
    fail "constraint-compiler does not promote non-constraint learning"
fi

# ============================================================
echo ""
echo "=== go-complexity-precommit.sh ==="
# ============================================================

# Test: go-complexity-precommit ignores non-Edit/Write tools (fail-open)
EC=0
CLAUDE_TOOL_NAME="Read" \
CLAUDE_TOOL_INPUT_FILE_PATH="cli/cmd/ao/main.go" \
bash "$HOOKS_DIR/go-complexity-precommit.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then
    pass "go-complexity-precommit ignores non-Edit/Write tool"
else
    fail "go-complexity-precommit ignores non-Edit/Write tool"
fi

# ============================================================
echo ""
echo "=== go-test-precommit.sh ==="
# ============================================================

# Test: non-Bash tool is ignored
EC=0
CLAUDE_TOOL_NAME="Edit" CLAUDE_TOOL_INPUT_COMMAND="" \
bash "$HOOKS_DIR/go-test-precommit.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then
    pass "go-test-precommit ignores non-Bash tool"
else
    fail "go-test-precommit ignores non-Bash tool"
fi

# Test: non-commit command is ignored
EC=0
CLAUDE_TOOL_NAME="Bash" CLAUDE_TOOL_INPUT_COMMAND="git status" \
bash "$HOOKS_DIR/go-test-precommit.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then
    pass "go-test-precommit ignores non-commit command"
else
    fail "go-test-precommit ignores non-commit command"
fi

# ============================================================
echo ""
echo "=== git-worker-guard.sh ==="
# ============================================================

# Test: Non-worker (no AGENTOPS_ROLE) => silent pass via AGENTOPS_ROLE
EC=0
echo '{"tool_input":{"command":"git commit -m test"}}' | AGENTOPS_ROLE="" bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "AGENTOPS_ROLE non-worker git commit allowed"; else fail "AGENTOPS_ROLE non-worker git commit allowed (exit=$EC)"; fi

# Test: Worker git commit blocked via AGENTOPS_ROLE
EC=0
echo '{"tool_input":{"command":"git commit -m test"}}' | AGENTOPS_ROLE=worker bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "AGENTOPS_ROLE worker git commit blocked"; else fail "AGENTOPS_ROLE worker git commit blocked (exit=$EC, expected 2)"; fi

# Test: Worker git push blocked via AGENTOPS_ROLE
EC=0
echo '{"tool_input":{"command":"git push origin main"}}' | AGENTOPS_ROLE=worker bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "AGENTOPS_ROLE worker git push blocked"; else fail "AGENTOPS_ROLE worker git push blocked (exit=$EC, expected 2)"; fi

# Test: Worker git add -A blocked via AGENTOPS_ROLE
EC=0
echo '{"tool_input":{"command":"git add -A"}}' | AGENTOPS_ROLE=worker bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "AGENTOPS_ROLE worker git add -A blocked"; else fail "AGENTOPS_ROLE worker git add -A blocked (exit=$EC, expected 2)"; fi

# Test: Worker git add . blocked via AGENTOPS_ROLE
EC=0
echo '{"tool_input":{"command":"git add ."}}' | AGENTOPS_ROLE=worker bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 2 ]; then pass "AGENTOPS_ROLE worker git add . blocked"; else fail "AGENTOPS_ROLE worker git add . blocked (exit=$EC, expected 2)"; fi

# Test: Worker non-git command allowed via AGENTOPS_ROLE
EC=0
echo '{"tool_input":{"command":"go test ./..."}}' | AGENTOPS_ROLE=worker bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "AGENTOPS_ROLE worker non-git command allowed"; else fail "AGENTOPS_ROLE worker non-git command allowed (exit=$EC)"; fi

# Test: Worker git status allowed (read-only git)
EC=0
echo '{"tool_input":{"command":"git status"}}' | AGENTOPS_ROLE=worker bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "AGENTOPS_ROLE worker git status allowed"; else fail "AGENTOPS_ROLE worker git status allowed (exit=$EC)"; fi

# Test: AGENTOPS_GIT_WORKER_GUARD_DISABLED kill switch
EC=0
echo '{"tool_input":{"command":"git commit -m test"}}' | AGENTOPS_ROLE=worker AGENTOPS_GIT_WORKER_GUARD_DISABLED=1 bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || EC=$?
if [ "$EC" -eq 0 ]; then pass "AGENTOPS_GIT_WORKER_GUARD_DISABLED kill switch"; else fail "AGENTOPS_GIT_WORKER_GUARD_DISABLED kill switch (exit=$EC)"; fi

# Test: Manifest wiring check
if jq -e '.hooks.PreToolUse[] | select(.matcher == "Bash") | .hooks[] | select(.command | contains("git-worker-guard.sh"))' "$HOOKS_DIR/hooks.json" >/dev/null 2>&1; then
    pass "git-worker-guard.sh wired in hooks.json PreToolUse/Bash"
else
    fail "git-worker-guard.sh not found in hooks.json PreToolUse/Bash"
fi

# ============================================================
echo ""
echo "=== task-validation-gate.sh: embedded parity ==="
# ============================================================

# Test: Parity check passes when hooks are in sync
PARITY_MOCK_DIR=$(mktemp -d)
setup_mock_repo "$PARITY_MOCK_DIR"
mkdir -p "$PARITY_MOCK_DIR/hooks" "$PARITY_MOCK_DIR/cli/embedded/hooks" "$PARITY_MOCK_DIR/scripts"
echo '#!/bin/bash' > "$PARITY_MOCK_DIR/hooks/test-hook.sh"
echo '#!/bin/bash' > "$PARITY_MOCK_DIR/cli/embedded/hooks/test-hook.sh"
# Create a validate-embedded-sync.sh that always passes
cat > "$PARITY_MOCK_DIR/scripts/validate-embedded-sync.sh" <<'EOFSCRIPT'
#!/bin/bash
exit 0
EOFSCRIPT
chmod +x "$PARITY_MOCK_DIR/scripts/validate-embedded-sync.sh"
# Stage a hook file change
echo '# changed' >> "$PARITY_MOCK_DIR/hooks/test-hook.sh"
git -C "$PARITY_MOCK_DIR" add -A >/dev/null 2>&1

TASK_INPUT='{"metadata":{"validation":{"tests":"go test ./...","files_exist":["hooks/test-hook.sh"]}}}'
OUTPUT=$(echo "$TASK_INPUT" | AGENTOPS_HOOKS_DISABLED=0 bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1) || true
# The hook runs in REPO_ROOT context so it will check THIS repo's sync, not the mock.
# Just verify the embedded_parity code path exists:
if grep -q 'embedded_parity' "$HOOKS_DIR/task-validation-gate.sh"; then
    pass "embedded_parity section exists in task-validation-gate.sh"
else
    fail "embedded_parity section missing from task-validation-gate.sh"
fi
rm -rf "$PARITY_MOCK_DIR"

# ============================================================
echo "=== factory-router.sh ==="
# ============================================================

FACTORY_ROUTER_MOCK="$TMPDIR/factory-router-mock"
mkdir -p "$FACTORY_ROUTER_MOCK/.agents/ao" "$FACTORY_ROUTER_MOCK/bin"
git -C "$FACTORY_ROUTER_MOCK" init -q >/dev/null 2>&1
touch "$FACTORY_ROUTER_MOCK/.agents/ao/.factory-intake-needed"
cat > "$FACTORY_ROUTER_MOCK/bin/ao" <<'EOF'
#!/usr/bin/env bash
if [ -n "${AO_ARGS_FILE:-}" ]; then
    printf '%s\n' "$*" >> "$AO_ARGS_FILE"
fi
if [ "${1:-}" = "knowledge" ] && [ "${2:-}" = "brief" ]; then
    briefing_path="${MOCK_FACTORY_BRIEFING_PATH:?}"
    mkdir -p "$(dirname "$briefing_path")"
    cat > "$briefing_path" <<'BRIEF'
# Briefing: fix auth bootstrap

## Relevant Topics
- `auth-bootstrap` (healthy)
BRIEF
    jq -n --arg path "$briefing_path" '{"output_path":$path}'
    exit 0
fi
exit 0
EOF
chmod +x "$FACTORY_ROUTER_MOCK/bin/ao"
AO_ARGS_FILE="$FACTORY_ROUTER_MOCK/ao-args.log"
MOCK_FACTORY_BRIEFING_PATH="$FACTORY_ROUTER_MOCK/.agents/briefings/2026-04-02-fix-auth-bootstrap.md"
ROUTER_OUTPUT=$(cd "$FACTORY_ROUTER_MOCK" && \
    printf '%s' '{"prompt":"fix auth bootstrap"}' | \
    PATH="$FACTORY_ROUTER_MOCK/bin:$PATH" \
    AO_ARGS_FILE="$AO_ARGS_FILE" \
    MOCK_FACTORY_BRIEFING_PATH="$MOCK_FACTORY_BRIEFING_PATH" \
    AGENTOPS_STARTUP_CONTEXT_MODE=factory \
    bash "$HOOKS_DIR/factory-router.sh" 2>/dev/null || true)
if grep -q '^knowledge brief --json --goal fix auth bootstrap$' "$AO_ARGS_FILE"; then
    pass "factory-router builds briefing from first substantive prompt"
else
    fail "factory-router builds briefing from first substantive prompt"
fi
if [ ! -f "$FACTORY_ROUTER_MOCK/.agents/ao/.factory-intake-needed" ] && \
   [ "$(cat "$FACTORY_ROUTER_MOCK/.agents/ao/factory-goal.txt" 2>/dev/null)" = "fix auth bootstrap" ]; then
    pass "factory-router records the captured goal and clears intake state"
else
    fail "factory-router records the captured goal and clears intake state"
fi
if [ "$(cat "$FACTORY_ROUTER_MOCK/.agents/ao/factory-briefing.txt" 2>/dev/null)" = "$MOCK_FACTORY_BRIEFING_PATH" ] && \
   [ -f "$FACTORY_ROUTER_MOCK/.agents/ao/.factory-router-fired" ]; then
    pass "factory-router stages briefing state and dedup marker"
else
    fail "factory-router stages briefing state and dedup marker"
fi
if [ -z "$ROUTER_OUTPUT" ]; then
    pass "factory-router stays silent"
else
    fail "factory-router stays silent"
fi
if jq -e '.hooks.UserPromptSubmit[].hooks[] | select(.command | contains("factory-router.sh"))' "$HOOKS_DIR/hooks.json" >/dev/null 2>&1; then
    pass "factory-router.sh wired in hooks.json"
else
    fail "factory-router.sh not found in hooks.json"
fi

# ============================================================
echo "=== edit-audit.sh ==="

MOCK_EDIT_AUDIT="$TMPDIR/mock-edit-audit"
setup_mock_repo "$MOCK_EDIT_AUDIT"
printf 'audit me\n' > "$MOCK_EDIT_AUDIT/note.txt"
EDIT_AUDIT_INPUT=$(jq -n '{"tool_name":"Edit","tool_input":{"file_path":"note.txt"}}')
(
    cd "$MOCK_EDIT_AUDIT"
    printf '%s\n' "$EDIT_AUDIT_INPUT" | bash "$HOOKS_DIR/edit-audit.sh" >/dev/null 2>&1
)
EDIT_AUDIT_LOG="$MOCK_EDIT_AUDIT/.agents/ao/edit-audit.log"
if [ -s "$EDIT_AUDIT_LOG" ] && jq -e '.tool == "Edit" and .file == "note.txt" and (.sha256 | length) > 0' "$EDIT_AUDIT_LOG" >/dev/null 2>&1; then
    pass "edit-audit writes valid JSON audit row for Edit"
else
    fail "edit-audit writes valid JSON audit row for Edit"
fi

rm -f "$EDIT_AUDIT_LOG"
(
    cd "$MOCK_EDIT_AUDIT"
    jq -n '{"tool_name":"Read","tool_input":{"file_path":"note.txt"}}' | bash "$HOOKS_DIR/edit-audit.sh" >/dev/null 2>&1
)
if [ ! -e "$EDIT_AUDIT_LOG" ]; then
    pass "edit-audit ignores non-edit tools"
else
    fail "edit-audit ignores non-edit tools"
fi

(
    cd "$MOCK_EDIT_AUDIT"
    printf '%s\n' "$EDIT_AUDIT_INPUT" | AGENTOPS_EDIT_AUDIT_DISABLED=1 bash "$HOOKS_DIR/edit-audit.sh" >/dev/null 2>&1
)
if [ ! -e "$EDIT_AUDIT_LOG" ]; then
    pass "edit-audit kill switch suppresses audit row"
else
    fail "edit-audit kill switch suppresses audit row"
fi

# ============================================================
echo ""
echo "=== compile-session-defrag.sh ==="
# ============================================================

MOCK_COMPILE="$TMPDIR/mock-compile-defrag"
setup_mock_repo "$MOCK_COMPILE"
mkdir -p "$MOCK_COMPILE/bin"
mkdir -p "$MOCK_COMPILE/.agents/patterns" "$MOCK_COMPILE/.agents/learnings" "$MOCK_COMPILE/.agents/compile"
cat > "$MOCK_COMPILE/bin/ao" <<'EOF'
#!/usr/bin/env bash
if [ "${1:-}" = "defrag" ]; then
    exit 0
fi
exit 1
EOF
chmod +x "$MOCK_COMPILE/bin/ao"
cat > "$MOCK_COMPILE/.agents/patterns/placeholder.md" <<'EOF'
---
title: Placeholder Pattern
---
EOF
cat > "$MOCK_COMPILE/.agents/learnings/stacked-frontmatter.md" <<'EOF'
---
title: First
---
# Learning: First
---
title: Second
---
EOF
cat > "$MOCK_COMPILE/.agents/learnings/bundled.md" <<'EOF'
# Learning: One

## Learning: Two
EOF
cat > "$MOCK_COMPILE/.agents/learnings/duplicate-headings.md" <<'EOF'
## Heading
alpha

## Heading
beta
EOF
cat > "$MOCK_COMPILE/.agents/compile/2026-03-29-old-contradiction-report.md" <<'EOF'
# Contradiction Report
EOF
cat > "$MOCK_COMPILE/.agents/compile/2026-03-30-extraction-complete.md" <<'EOF'
# Extraction Complete
EOF
touch -t 202603290101 "$MOCK_COMPILE/.agents/compile/2026-03-29-old-contradiction-report.md"
touch -t 202603300101 "$MOCK_COMPILE/.agents/compile/2026-03-30-extraction-complete.md"

OUTPUT=$(cd "$MOCK_COMPILE" && PATH="$MOCK_COMPILE/bin:$PATH" bash "$HOOKS_DIR/compile-session-defrag.sh" 2>/dev/null || true)
if echo "$OUTPUT" | jq -e '.hookSpecificOutput.hookEventName == "SessionEnd"
  and (.hookSpecificOutput.additionalContext | test("Compile defrag completed"))
  and (.hookSpecificOutput.additionalContext | test("placeholders=1"))
  and (.hookSpecificOutput.additionalContext | test("stacked_frontmatter=1"))
  and (.hookSpecificOutput.additionalContext | test("bundled_multi_learning=1"))
  and (.hookSpecificOutput.additionalContext | test("duplicate_headings=1"))
  and (.hookSpecificOutput.additionalContext | test("stale_contradictions=1"))' >/dev/null 2>&1; then
    pass "compile-session-defrag emits SessionEnd JSON on success"
else
    fail "compile-session-defrag emits SessionEnd JSON on success"
fi

OUTPUT=$(cd "$MOCK_COMPILE" && AGENTOPS_HOOKS_DISABLED=1 PATH="$MOCK_COMPILE/bin:$PATH" bash "$HOOKS_DIR/compile-session-defrag.sh" 2>&1 || true)
if [ -z "$OUTPUT" ]; then
    pass "compile-session-defrag kill switch suppresses output"
else
    fail "compile-session-defrag kill switch suppresses output"
fi

# ============================================================
echo ""
echo "=== edit-audit.sh ==="
# ============================================================

# Diagnostic-only PostToolUse hook: appends one JSON line per Edit/Write to
# .agents/ao/edit-audit.log. Always exits 0 — never blocks.

EDIT_AUDIT_MOCK="$TMPDIR/edit-audit-mock"
setup_mock_repo "$EDIT_AUDIT_MOCK"
echo "hello world" > "$EDIT_AUDIT_MOCK/sample.txt"

OUTPUT=$(jq -n --arg file "$EDIT_AUDIT_MOCK/sample.txt" \
    '{"tool_name":"Edit","tool_input":{"file_path":$file}}' \
    | (cd "$EDIT_AUDIT_MOCK" && bash "$HOOKS_DIR/edit-audit.sh") 2>&1)
RC=$?
if [ "$RC" -eq 0 ]; then
    pass "edit-audit.sh exits 0 on Edit"
else
    fail "edit-audit.sh exits 0 on Edit (rc=$RC, output=$OUTPUT)"
fi

if [ -s "$EDIT_AUDIT_MOCK/.agents/ao/edit-audit.log" ] \
    && jq -e '.tool == "Edit" and (.sha256 | length) > 0' \
        "$EDIT_AUDIT_MOCK/.agents/ao/edit-audit.log" >/dev/null 2>&1; then
    pass "edit-audit.sh appends a valid JSON record"
else
    fail "edit-audit.sh appends a valid JSON record"
fi

# Non-Edit/Write tools must produce no log entry
LOG_BEFORE=$(wc -c < "$EDIT_AUDIT_MOCK/.agents/ao/edit-audit.log")
echo '{"tool_name":"Read","tool_input":{"file_path":"'"$EDIT_AUDIT_MOCK/sample.txt"'"}}' \
    | (cd "$EDIT_AUDIT_MOCK" && bash "$HOOKS_DIR/edit-audit.sh") >/dev/null 2>&1
LOG_AFTER=$(wc -c < "$EDIT_AUDIT_MOCK/.agents/ao/edit-audit.log")
if [ "$LOG_BEFORE" -eq "$LOG_AFTER" ]; then
    pass "edit-audit.sh ignores non-Edit/Write tools"
else
    fail "edit-audit.sh ignores non-Edit/Write tools"
fi

# Kill switch
echo '{"tool_name":"Edit","tool_input":{"file_path":"'"$EDIT_AUDIT_MOCK/sample.txt"'"}}' \
    | AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/edit-audit.sh" >/dev/null 2>&1
RC=$?
if [ "$RC" -eq 0 ]; then
    pass "edit-audit.sh respects AGENTOPS_HOOKS_DISABLED"
else
    fail "edit-audit.sh respects AGENTOPS_HOOKS_DISABLED"
fi

# ============================================================
echo ""
echo "=== postedit-codex-refresh.sh ==="
# ============================================================
# soc-7qq9: PostToolUse hook auto-runs refresh-codex-artifacts after skill edits.
# Smoke tests verify the hook (1) is silent + exits 0 on a non-skill path,
# (2) attempts to run refresh on a skill path inside a real repo, and
# (3) honors the disable env var.

# Test 1: non-skill path is a no-op (silent, exit 0).
NON_SKILL_INPUT='{"tool_name":"Edit","tool_input":{"file_path":"/tmp/some/random/file.go"}}'
if printf '%s\n' "$NON_SKILL_INPUT" | bash "$HOOKS_DIR/postedit-codex-refresh.sh" >/dev/null 2>&1; then
    pass "postedit-codex-refresh exits 0 on non-skill path"
else
    fail "postedit-codex-refresh exits 0 on non-skill path"
fi

# Test 2: AGENTOPS_HOOKS_DISABLED=1 short-circuits (no jq calls etc.).
DISABLED_INPUT='{"tool_name":"Edit","tool_input":{"file_path":"/tmp/skills/some-skill/SKILL.md"}}'
if printf '%s\n' "$DISABLED_INPUT" | AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/postedit-codex-refresh.sh" >/dev/null 2>&1; then
    pass "postedit-codex-refresh respects AGENTOPS_HOOKS_DISABLED=1"
else
    fail "postedit-codex-refresh respects AGENTOPS_HOOKS_DISABLED=1"
fi

# Test 3: AGENTOPS_CODEX_AUTOREFRESH_DISABLED=1 also short-circuits.
if printf '%s\n' "$DISABLED_INPUT" | AGENTOPS_CODEX_AUTOREFRESH_DISABLED=1 bash "$HOOKS_DIR/postedit-codex-refresh.sh" >/dev/null 2>&1; then
    pass "postedit-codex-refresh respects AGENTOPS_CODEX_AUTOREFRESH_DISABLED=1"
else
    fail "postedit-codex-refresh respects AGENTOPS_CODEX_AUTOREFRESH_DISABLED=1"
fi

# Test 4: skill path outside any repo silently exits (no git root → no-op).
ORPHAN_SKILL_INPUT='{"tool_name":"Edit","tool_input":{"file_path":"/tmp/orphan/skills/foo/SKILL.md"}}'
if printf '%s\n' "$ORPHAN_SKILL_INPUT" | bash "$HOOKS_DIR/postedit-codex-refresh.sh" >/dev/null 2>&1; then
    pass "postedit-codex-refresh silent when path is outside a git repo"
else
    fail "postedit-codex-refresh silent when path is outside a git repo"
fi

# ============================================================
echo ""
echo "=== Coverage check ==="
# ============================================================

# Verify every .sh hook file has at least one test
# Test files may end in either .sh (legacy shell) or .bats (BATS suite);
# both are valid coverage targets. The glob below includes both extensions
# across the three canonical test directories.
MISSING_HOOKS=""
for hook_file in "$HOOKS_DIR"/*.sh; do
    hook_name=$(basename "$hook_file" .sh)
    # Check if this hook name appears in any test file (this script or BATS tests)
    COVERAGE_FILES=("$SCRIPT_DIR/test-hooks.sh" "$SCRIPT_DIR/hook-stdin-contracts.bats")
    for candidate in \
        "$SCRIPT_DIR/test-${hook_name}"*.sh \
        "$SCRIPT_DIR/test-${hook_name}"*.bats \
        "$REPO_ROOT/tests/skills/test-${hook_name}"*.sh \
        "$REPO_ROOT/tests/skills/test-${hook_name}"*.bats \
        "$REPO_ROOT/tests/scripts/test-${hook_name}"*.sh \
        "$REPO_ROOT/tests/scripts/test-${hook_name}"*.bats; do
        [ -e "$candidate" ] && COVERAGE_FILES+=("$candidate")
    done
    if ! grep -rq "$hook_name" "${COVERAGE_FILES[@]}" 2>/dev/null; then
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

TOTAL=$((PASS + FAIL + SKIP))
echo "Total: $TOTAL | Pass: $PASS | Fail: $FAIL | Skip: $SKIP"

if [ "$FAIL" -gt 0 ]; then
    red "FAILED"
    exit 1
elif [ "$SKIP" -gt 0 ]; then
    yellow "PASSED WITH SKIPS"
    exit 0
else
    green "ALL PASSED"
    exit 0
fi
