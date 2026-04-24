#!/usr/bin/env bats
# test-hooks.bats — Bats conversion of tests/hooks/test-hooks.sh
# Preserves at least the original coverage across all hook categories.

setup() {
    load helpers/test_helper
    _helper_setup
    export CLAUDE_SESSION_ID="bats-hooks-$$"
}

teardown() {
    _helper_teardown
}

# ═══════════════════════════════════════════════════════════════════════
# prompt-nudge.sh
# ═══════════════════════════════════════════════════════════════════════

@test "prompt-nudge: JSON injection resistance — special characters escaped" {
    # Verifies jq -n safely escapes all dangerous payloads
    for PAYLOAD in '"' '\\' '$(whoami)' '`id`' '<script>' "'; DROP TABLE" '{"nested":"json"}'; do
        RESULT=$(jq -n --arg nudge "$PAYLOAD" '{"hookSpecificOutput":{"additionalContext":$nudge}}')
        echo "$RESULT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
    done
}

# ═══════════════════════════════════════════════════════════════════════
# session-start.sh / precompact-snapshot.sh
# ═══════════════════════════════════════════════════════════════════════

@test "session-start: stays silent while preparing runtime state" {
    SESSION_RAW=$(bash "$HOOKS_DIR/session-start.sh" 2>/dev/null || true)
    [ -z "$SESSION_RAW" ]
}

@test "session-start: kill switch suppresses output" {
    OUTPUT=$(AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/session-start.sh" 2>&1 || true)
    [ -z "$OUTPUT" ]
}

@test "session-start: roots .agents paths to git root from subdir" {
    local mock="$TMP_TEST_DIR/mock-session"
    mkdir -p "$mock/subdir"
    git -C "$mock" init -q >/dev/null 2>&1
    (cd "$mock/subdir" && bash "$HOOKS_DIR/session-start.sh" >/dev/null 2>&1 || true)
    [ -d "$mock/.agents/research" ] && [ ! -d "$mock/subdir/.agents/research" ]
}

@test "session-start: fresh repo stages one-time new-user welcome marker" {
    local mock="$TMP_TEST_DIR/mock-fresh-session"
    mkdir -p "$mock"
    git -C "$mock" init -q >/dev/null 2>&1
    run bash -c 'cd "$1" && bash "$2" 2>&1' -- "$mock" "$HOOKS_DIR/session-start.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
    [ -f "$mock/.agents/ao/.new-user-welcome-needed" ]
}

@test "session-start: factory mode stages a matched briefing without injecting it" {
    local mock="$TMP_TEST_DIR/mock-factory-start"
    mkdir -p "$mock/.agents/handoff" "$mock/bin"
    git -C "$mock" init -q >/dev/null 2>&1
    cat > "$mock/.agents/handoff/handoff-20260402T192000Z.json" <<'EOF'
{
  "schema_version": 1,
  "id": "handoff-factory-test",
  "created_at": "2026-04-02T19:20:00Z",
  "type": "manual",
  "goal": "stabilize auth startup",
  "summary": "route this session into the factory lane"
}
EOF
    cat > "$mock/bin/ao" <<'EOF'
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
    chmod +x "$mock/bin/ao"
    local args_file="$mock/ao-args.log"
    local briefing_path="$mock/.agents/briefings/2026-04-02-stabilize-auth-startup.md"
    run bash -c 'cd "$1" && PATH="$1/bin:$PATH" AO_ARGS_FILE="$2" MOCK_FACTORY_BRIEFING_PATH="$3" AGENTOPS_STARTUP_CONTEXT_MODE=factory bash "$4" 2>&1' \
        -- "$mock" "$args_file" "$briefing_path" "$HOOKS_DIR/session-start.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
    grep -q '^knowledge brief --json --goal stabilize auth startup$' "$args_file"
    [ "$(cat "$mock/.agents/ao/factory-goal.txt" 2>/dev/null)" = "stabilize auth startup" ]
    [ "$(cat "$mock/.agents/ao/factory-briefing.txt" 2>/dev/null)" = "$briefing_path" ]
}

@test "precompact-snapshot: emits additionalContext JSON" {
    local mock="$TMP_TEST_DIR/mock-precompact"
    mkdir -p "$mock/.agents"
    git -C "$mock" init -q >/dev/null 2>&1
    PRECOMPACT_JSON=$(cd "$mock" && bash "$HOOKS_DIR/precompact-snapshot.sh" 2>/dev/null || true)
    echo "$PRECOMPACT_JSON" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
}

@test "precompact-snapshot: kill switch suppresses output" {
    local mock="$TMP_TEST_DIR/mock-precompact-kill"
    mkdir -p "$mock/.agents"
    git -C "$mock" init -q >/dev/null 2>&1
    OUTPUT=$(cd "$mock" && AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/precompact-snapshot.sh" 2>&1 || true)
    [ -z "$OUTPUT" ]
}

# ═══════════════════════════════════════════════════════════════════════
# pending-cleaner.sh
# ═══════════════════════════════════════════════════════════════════════

@test "pending-cleaner: fail-open outside git repo" {
    local non_git="$TMP_TEST_DIR/non-git"
    mkdir -p "$non_git"
    run bash -c 'cd "$1" && bash "$2" 2>&1' -- "$non_git" "$HOOKS_DIR/pending-cleaner.sh"
    [ "$status" -eq 0 ]
}

@test "pending-cleaner: stale pending.jsonl auto-cleared and archived" {
    local mock="$TMP_TEST_DIR/mock-pending"
    mkdir -p "$mock/.agents/ao"
    git -C "$mock" init -q >/dev/null 2>&1
    local pfile="$mock/.agents/ao/pending.jsonl"
    printf '{"session":"1"}\n{"session":"2"}\n' > "$pfile"
    touch -t 202401010101 "$pfile"
    (cd "$mock" && AGENTOPS_PENDING_STALE_SECONDS=1 AGENTOPS_PENDING_ALERT_LINES=1 bash "$HOOKS_DIR/pending-cleaner.sh" >/dev/null 2>&1 || true)
    # File should be empty or gone
    [ ! -s "$pfile" ]
    # Archive should exist
    ls "$mock/.agents/ao/archive"/pending-*.jsonl >/dev/null 2>&1
}

@test "pending-cleaner: kill switch preserves queue" {
    local mock="$TMP_TEST_DIR/mock-pending-kill"
    mkdir -p "$mock/.agents/ao"
    git -C "$mock" init -q >/dev/null 2>&1
    local pfile="$mock/.agents/ao/pending.jsonl"
    printf '{"session":"keep"}\n' > "$pfile"
    touch -t 202401010101 "$pfile"
    (cd "$mock" && AGENTOPS_HOOKS_DISABLED=1 AGENTOPS_PENDING_STALE_SECONDS=1 bash "$HOOKS_DIR/pending-cleaner.sh" >/dev/null 2>&1 || true)
    [ -s "$pfile" ]
}

# ═══════════════════════════════════════════════════════════════════════
# task-validation-gate.sh — error recovery
# ═══════════════════════════════════════════════════════════════════════

@test "task-validation-gate: test failure writes last-failure.json with all 6 fields" {
    local mock="$TMP_TEST_DIR/mock-fail"
    setup_mock_repo "$mock"
    (cd "$mock" && printf '%s' '{"subject":"test task","metadata":{"validation":{"tests":"make nonexistent-target-xyz"}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || true)
    [ -f "$mock/.agents/ao/last-failure.json" ]
    jq -e '.ts and .type and .command and .exit_code and .task_subject and .details' "$mock/.agents/ao/last-failure.json" >/dev/null 2>&1
}

@test "task-validation-gate: last-failure.json type field matches failure type" {
    local mock="$TMP_TEST_DIR/mock-files-fail"
    setup_mock_repo "$mock"
    (cd "$mock" && printf '%s' '{"subject":"files task","metadata":{"validation":{"files_exist":["nonexistent-file.txt"]}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" >/dev/null 2>&1 || true)
    [ -f "$mock/.agents/ao/last-failure.json" ]
    FAILURE_TYPE=$(jq -r '.type' "$mock/.agents/ao/last-failure.json" 2>/dev/null)
    [ "$FAILURE_TYPE" = "files_exist" ]
}

@test "task-validation-gate: stderr includes bug-hunt for test failures" {
    local mock="$TMP_TEST_DIR/mock-test-fail"
    mkdir -p "$mock/.agents/ao"
    git -C "$mock" init -q >/dev/null 2>&1
    OUTPUT=$(cd "$mock" && printf '%s' '{"subject":"test task","metadata":{"validation":{"tests":"make nonexistent-target-xyz"}}}' | bash "$HOOKS_DIR/task-validation-gate.sh" 2>&1 || true)
    [[ "$OUTPUT" == *"bug-hunt"* ]]
}

@test "task-validation-gate: allowlist blocks curl command" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"metadata":{"validation":{"tests":"curl http://evil.com"}}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 2 ]
}

@test "task-validation-gate: allowlist allows go command" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"metadata":{"validation":{"tests":"go version"}}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: active constraint requires metadata.issue_type" {
    local mock="$TMP_TEST_DIR/mock-constraint-issue"
    setup_mock_repo "$mock"
    mkdir -p "$mock/.agents/constraints" "$mock/docs"
    echo 'SAFE_MARKER' > "$mock/docs/guide.md"
    cat > "$mock/.agents/constraints/index.json" <<'EOF'
{"schema_version":1,"constraints":[{"id":"c-issue-type","title":"issue type required","status":"active","compiled_at":"2026-03-10T00:00:00Z","applies_to":{"scope":"files","issue_types":["feature"],"path_globs":["docs/*.md"]},"detector":{"kind":"content_pattern","mode":"must_contain","pattern":"SAFE_MARKER","message":"SAFE_MARKER required"}}]}
EOF
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$mock" '{"metadata":{"files":["docs/guide.md"]}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 2 ]
    [[ "$output" == *"metadata.issue_type"* ]]
}

@test "task-validation-gate: active content constraint blocks missing literal" {
    local mock="$TMP_TEST_DIR/mock-constraint-pattern"
    setup_mock_repo "$mock"
    mkdir -p "$mock/.agents/constraints" "$mock/docs"
    echo 'hello' > "$mock/docs/guide.md"
    cat > "$mock/.agents/constraints/index.json" <<'EOF'
{"schema_version":1,"constraints":[{"id":"c-pattern","title":"must contain","status":"active","compiled_at":"2026-03-10T00:00:00Z","applies_to":{"scope":"files","issue_types":["feature"],"path_globs":["docs/*.md"]},"detector":{"kind":"content_pattern","mode":"must_contain","pattern":"SAFE_MARKER","message":"SAFE_MARKER required"}}]}
EOF
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$mock" '{"metadata":{"issue_type":"feature","files":["docs/guide.md"]}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 2 ]
    [[ "$output" == *"SAFE_MARKER required"* ]]
}

@test "task-validation-gate: content_check with matching pattern passes" {
    local mock="$TMP_TEST_DIR/mock-content-check-pass"
    setup_mock_repo "$mock"
    local fixture_dir="$mock/fixtures"
    mkdir -p "$fixture_dir"
    local content_file="$fixture_dir/test-content.js"
    echo "function authenticate() {}" > "$content_file"
    INPUT=$(jq -n --arg f "$content_file" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"function authenticate"}]}}}')
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' -- "$mock" "$INPUT" "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: content_check with missing pattern blocks" {
    local mock="$TMP_TEST_DIR/mock-content-check-fail"
    setup_mock_repo "$mock"
    local fixture_dir="$mock/fixtures"
    mkdir -p "$fixture_dir"
    local content_file="$fixture_dir/test-content.js"
    echo "function authenticate() {}" > "$content_file"
    INPUT=$(jq -n --arg f "$content_file" '{"metadata":{"validation":{"content_check":[{"file":$f,"pattern":"class UserService"}]}}}')
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' -- "$mock" "$INPUT" "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 2 ]
}

@test "task-validation-gate: paired_files passes with matched companion change" {
    local mock="$TMP_TEST_DIR/mock-paired"
    setup_mock_repo "$mock"
    mkdir -p "$mock/cli/cmd/ao"
    echo 'package ao' > "$mock/cli/cmd/ao/sample.go"
    echo 'package ao' > "$mock/cli/cmd/ao/sample_test.go"
    git -C "$mock" add cli/cmd/ao/sample.go cli/cmd/ao/sample_test.go >/dev/null 2>&1
    PAIRED_INPUT=$(jq -n '{"metadata":{"validation":{"paired_files":[{"pattern":"cli/cmd/ao/*.go","exclude":"*_test.go","companion":"{dir}/{basename}_test{ext}","message":"missing paired test change"}]}}}')
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$mock" "$PAIRED_INPUT" "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: paired_files blocks missing companion" {
    local mock="$TMP_TEST_DIR/mock-paired-fail"
    setup_mock_repo "$mock"
    mkdir -p "$mock/cli/cmd/ao"
    echo 'package ao' > "$mock/cli/cmd/ao/solo.go"
    git -C "$mock" add cli/cmd/ao/solo.go >/dev/null 2>&1
    PAIRED_INPUT=$(jq -n '{"metadata":{"validation":{"paired_files":[{"pattern":"cli/cmd/ao/*.go","exclude":"*_test.go","companion":"{dir}/{basename}_test{ext}","message":"missing paired test change"}]}}}')
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$mock" "$PAIRED_INPUT" "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 2 ]
}

# ═══════════════════════════════════════════════════════════════════════
# precompact-snapshot.sh auto-handoff
# ═══════════════════════════════════════════════════════════════════════

@test "precompact: auto-handoff document created in .agents/handoff/" {
    local mock="$TMP_TEST_DIR/mock-handoff"
    mkdir -p "$mock/.agents/ao"
    git -C "$mock" init -q >/dev/null 2>&1
    (cd "$mock" && bash "$HOOKS_DIR/precompact-snapshot.sh" >/dev/null 2>&1 || true)
    ls "$mock/.agents/handoff/auto-"*.md >/dev/null 2>&1
}

@test "precompact: handoff contains Ratchet State section" {
    local mock="$TMP_TEST_DIR/mock-handoff-content"
    mkdir -p "$mock/.agents/ao"
    git -C "$mock" init -q >/dev/null 2>&1
    (cd "$mock" && bash "$HOOKS_DIR/precompact-snapshot.sh" >/dev/null 2>&1 || true)
    HANDOFF_FILE=$(ls -t "$mock/.agents/handoff/auto-"*.md 2>/dev/null | head -1)
    [ -f "$HANDOFF_FILE" ]
    grep -q "Ratchet State" "$HANDOFF_FILE"
}

# ═══════════════════════════════════════════════════════════════════════
# memory packet v1 compatibility
# ═══════════════════════════════════════════════════════════════════════

@test "stop-auto-handoff: emits schema packet v1" {
    local mock="$TMP_TEST_DIR/mock-stop-packet"
    setup_mock_repo "$mock"
    (cd "$mock" && printf '%s' '{"last_assistant_message":"STOP_PACKET_MARKER_123"}' | bash "$HOOKS_DIR/stop-auto-handoff.sh" >/dev/null 2>&1 || true)
    PACKET_FILE=$(ls -t "$mock/.agents/ao/packets/pending/"*.json 2>/dev/null | head -1)
    [ -f "$PACKET_FILE" ]
    jq -e '.schema_version == 1 and .packet_type == "stop" and .source_hook == "stop-auto-handoff"' "$PACKET_FILE" >/dev/null 2>&1
    jq -e '.payload.last_assistant_message | contains("STOP_PACKET_MARKER_123")' "$PACKET_FILE" >/dev/null 2>&1
}

@test "subagent-stop: emits schema packet v1" {
    local mock="$TMP_TEST_DIR/mock-sub-packet"
    setup_mock_repo "$mock"
    (cd "$mock" && printf '%s' '{"last_assistant_message":"SUB_MARKER_999","agent_name":"worker-a"}' | bash "$HOOKS_DIR/subagent-stop.sh" >/dev/null 2>&1 || true)
    PACKET_FILE=$(ls -t "$mock/.agents/ao/packets/pending/"*.json 2>/dev/null | head -1)
    [ -f "$PACKET_FILE" ]
    jq -e '.schema_version == 1 and .packet_type == "subagent_stop" and .source_hook == "subagent-stop" and .payload.agent_name == "worker-a"' "$PACKET_FILE" >/dev/null 2>&1
}

# ═══════════════════════════════════════════════════════════════════════
# standards-injector.sh
# ═══════════════════════════════════════════════════════════════════════

@test "standards-injector: python file injects standards context (original)" {
    OUTPUT=$(printf '%s' '{"tool_input":{"file_path":"/some/path/main.py"}}' | bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)
    echo "$OUTPUT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
}

@test "standards-injector: unknown extension produces no output (original)" {
    OUTPUT=$(printf '%s' '{"tool_input":{"file_path":"/some/path/data.csv"}}' | bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)
    [ -z "$OUTPUT" ]
}

# ═══════════════════════════════════════════════════════════════════════
# git-worker-guard.sh — AGENTOPS_ROLE tests
# ═══════════════════════════════════════════════════════════════════════

@test "git-worker-guard: AGENTOPS_ROLE=worker blocks git commit" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_ROLE=worker bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 2 ]
}

@test "git-worker-guard: AGENTOPS_ROLE=worker allows git status (read-only)" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_ROLE=worker bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git status"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 0 ]
}

@test "git-worker-guard: AGENTOPS_ROLE=worker blocks git add ." {
    run bash -c 'printf "%s" "$1" | AGENTOPS_ROLE=worker bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git add ."}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 2 ]
}

@test "git-worker-guard: AGENTOPS_GIT_WORKER_GUARD_DISABLED kill switch" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_ROLE=worker AGENTOPS_GIT_WORKER_GUARD_DISABLED=1 bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 0 ]
}

@test "git-worker-guard: git add -A blocks (uppercase bulk)" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_ROLE=worker bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git add -A"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 2 ]
}

@test "git-worker-guard: git add --all blocks" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_ROLE=worker bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git add --all"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 2 ]
}

@test "git-worker-guard: git add -u allows (selective update)" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_ROLE=worker bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git add -u"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 0 ]
}

@test "git-worker-guard: git add filename containing -a allows" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_ROLE=worker bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git add path/file-a.txt"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# dangerous-git-guard.sh — safe alternatives
# ═══════════════════════════════════════════════════════════════════════

@test "dangerous-git-guard: hard reset suggests safe alternative" {
    OUTPUT=$(printf '%s' '{"tool_input":{"command":"git reset --hard HEAD"}}' | bash "$HOOKS_DIR/dangerous-git-guard.sh" 2>&1 || true)
    echo "$OUTPUT" | grep -qi "stash\|soft"
}

@test "dangerous-git-guard: git clean -f blocked" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git clean -f"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 2 ]
}

@test "dangerous-git-guard: normal git commit allowed" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git commit -m fix"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# ratchet-advance.sh — advanced cases
# ═══════════════════════════════════════════════════════════════════════

@test "ratchet-advance: writes dedup flag file" {
    (cd "$MOCK_REPO" && printf '%s' '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}' | PATH="/usr/bin:/bin" bash "$HOOKS_DIR/ratchet-advance.sh" >/dev/null 2>&1 || true)
    [ -f "$MOCK_REPO/.agents/ao/.ratchet-advance-fired" ]
}

# ═══════════════════════════════════════════════════════════════════════
# commit-review-gate.sh
# ═══════════════════════════════════════════════════════════════════════

@test "commit-review-gate: non-git command ignored" {
    OUTPUT=$(printf '%s' '{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}' | bash "$HOOKS_DIR/commit-review-gate.sh" 2>&1)
    [ -z "$OUTPUT" ]
}

# ═══════════════════════════════════════════════════════════════════════
# intent-echo.sh
# ═══════════════════════════════════════════════════════════════════════

@test "intent-echo: destructive keyword triggers context (original)" {
    rm -f "$REPO_ROOT/.agents/ao/.intent-echo-fired" 2>/dev/null
    OUTPUT=$(printf '%s' '{"prompt":"delete all the old files"}' | bash "$HOOKS_DIR/intent-echo.sh" 2>&1)
    echo "$OUTPUT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
    rm -f "$REPO_ROOT/.agents/ao/.intent-echo-fired" 2>/dev/null
}

# ═══════════════════════════════════════════════════════════════════════
# factory-router.sh
# ═══════════════════════════════════════════════════════════════════════

@test "factory-router: first substantive prompt builds briefing and clears intake" {
    local mock="$TMP_TEST_DIR/mock-factory-router"
    mkdir -p "$mock/.agents/ao" "$mock/bin"
    git -C "$mock" init -q >/dev/null 2>&1
    touch "$mock/.agents/ao/.factory-intake-needed"
    cat > "$mock/bin/ao" <<'EOF'
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
    chmod +x "$mock/bin/ao"
    local args_file="$mock/ao-args.log"
    local briefing_path="$mock/.agents/briefings/2026-04-02-fix-auth-bootstrap.md"
    run bash -c 'cd "$1" && printf "%s" "$4" | PATH="$1/bin:$PATH" AO_ARGS_FILE="$2" MOCK_FACTORY_BRIEFING_PATH="$3" AGENTOPS_STARTUP_CONTEXT_MODE=factory bash "$5" 2>&1' \
        -- "$mock" "$args_file" "$briefing_path" '{"prompt":"fix auth bootstrap"}' "$HOOKS_DIR/factory-router.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
    grep -q '^knowledge brief --json --goal fix auth bootstrap$' "$args_file"
    [ ! -f "$mock/.agents/ao/.factory-intake-needed" ]
    [ "$(cat "$mock/.agents/ao/factory-goal.txt" 2>/dev/null)" = "fix auth bootstrap" ]
    [ "$(cat "$mock/.agents/ao/factory-briefing.txt" 2>/dev/null)" = "$briefing_path" ]
    [ -f "$mock/.agents/ao/.factory-router-fired" ]
}

# ═══════════════════════════════════════════════════════════════════════
# research-loop-detector.sh
# ═══════════════════════════════════════════════════════════════════════

@test "research-loop-detector: threshold triggers warning (original)" {
    echo "7" > "$REPO_ROOT/.agents/ao/.read-streak"
    OUTPUT=$(printf '%s' '{"tool_name":"Read"}' | CLAUDE_TOOL_NAME=Read bash "$HOOKS_DIR/research-loop-detector.sh" 2>&1)
    echo "$OUTPUT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
}

# ═══════════════════════════════════════════════════════════════════════
# citation-tracker.sh
# ═══════════════════════════════════════════════════════════════════════

@test "citation-tracker: dedup prevents duplicate in same session" {
    mkdir -p "$MOCK_REPO/.agents/learnings"
    echo "test" > "$MOCK_REPO/.agents/learnings/item.md"
    local sid="bats-cite-dedup-$$-$(date +%s)"
    # First citation
    (cd "$MOCK_REPO" && printf '%s' '{"tool_input":{"file_path":".agents/learnings/item.md"}}' | CLAUDE_SESSION_ID="$sid" bash "$HOOKS_DIR/citation-tracker.sh" >/dev/null 2>&1)
    before_lines=$(wc -l < "$MOCK_REPO/.agents/ao/citations.jsonl" | tr -d ' ')
    # Second citation — should be deduped
    (cd "$MOCK_REPO" && printf '%s' '{"tool_input":{"file_path":".agents/learnings/item.md"}}' | CLAUDE_SESSION_ID="$sid" bash "$HOOKS_DIR/citation-tracker.sh" >/dev/null 2>&1)
    after_lines=$(wc -l < "$MOCK_REPO/.agents/ao/citations.jsonl" | tr -d ' ')
    [ "$before_lines" = "$after_lines" ]
}

# ═══════════════════════════════════════════════════════════════════════
# context-guard.sh
# ═══════════════════════════════════════════════════════════════════════

@test "context-guard: strict mode blocks on handoff_now action" {
    local ao_mock="$TMP_TEST_DIR/bin"
    mkdir -p "$ao_mock"
    cat > "$ao_mock/ao" <<'AOEOF'
#!/bin/bash
echo '{"session":{"action":"handoff_now"},"hook_message":"Context critical"}'
AOEOF
    chmod +x "$ao_mock/ao"
    run bash -c 'printf "%s" "$1" | PATH="'"$ao_mock"':$PATH" CLAUDE_SESSION_ID="bats-ctx-strict" AGENTOPS_CONTEXT_GUARD_STRICT=1 bash "$2" 2>&1' \
        -- '{"prompt":"continue"}' "$HOOKS_DIR/context-guard.sh"
    [ "$status" -eq 2 ]
}

# ═══════════════════════════════════════════════════════════════════════
# stop-team-guard.sh
# ═══════════════════════════════════════════════════════════════════════

@test "stop-team-guard: safe when no teams dir" {
    local mock_home="$TMP_TEST_DIR/mock-home"
    mkdir -p "$mock_home/.claude"
    run bash -c 'HOME="$1" bash "$2" 2>&1' -- "$mock_home" "$HOOKS_DIR/stop-team-guard.sh"
    [ "$status" -eq 0 ]
}

@test "stop-team-guard: kill switch allows stop" {
    local mock_home="$TMP_TEST_DIR/mock-home-kill"
    mkdir -p "$mock_home/.claude/teams/test-team"
    echo '{"members":[{"name":"w1","tmuxPaneId":"some-session"}]}' > "$mock_home/.claude/teams/test-team/config.json"
    run bash -c 'HOME="$1" AGENTOPS_HOOKS_DISABLED=1 bash "$2" 2>&1' -- "$mock_home" "$HOOKS_DIR/stop-team-guard.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# Manifest wiring checks (from original test-hooks.sh)
# ═══════════════════════════════════════════════════════════════════════

@test "hooks.json: task-validation-gate wired in TaskCompleted" {
    jq -e '.hooks.TaskCompleted[]?.hooks[]?.command | select(test("task-validation-gate\\.sh$"))' "$HOOKS_DIR/hooks.json" >/dev/null 2>&1
}

@test "hooks.json: git-worker-guard wired in PreToolUse/Bash" {
    jq -e '.hooks.PreToolUse[] | select(.matcher == "Bash") | .hooks[] | select(.command | contains("git-worker-guard.sh"))' "$HOOKS_DIR/hooks.json" >/dev/null 2>&1
}

@test "hooks.json: commit-review-gate wired in PreToolUse/Bash" {
    jq -e '.hooks.PreToolUse[] | select(.matcher == "Bash") | .hooks[] | select(.command | contains("commit-review-gate.sh"))' "$HOOKS_DIR/hooks.json" >/dev/null 2>&1
}

@test "hooks.json: intent-echo wired in UserPromptSubmit" {
    jq -e '.hooks.UserPromptSubmit[].hooks[] | select(.command | contains("intent-echo.sh"))' "$HOOKS_DIR/hooks.json" >/dev/null 2>&1
}

@test "hooks.json: factory-router wired in UserPromptSubmit" {
    jq -e '.hooks.UserPromptSubmit[].hooks[] | select(.command | contains("factory-router.sh"))' "$HOOKS_DIR/hooks.json" >/dev/null 2>&1
}

@test "hooks.json: research-loop-detector wired in PostToolUse" {
    jq -e '.hooks.PostToolUse[].hooks[] | select(.command | contains("research-loop-detector.sh"))' "$HOOKS_DIR/hooks.json" >/dev/null 2>&1
}

@test "hooks.json: ao-agents-check removed" {
    ! grep -q "ao-agents-check" "$HOOKS_DIR/hooks.json"
}

# ═══════════════════════════════════════════════════════════════════════
# ao-* Delegation Verification (Embedded Hooks)
# ═══════════════════════════════════════════════════════════════════════

@test "ao-extract: delegates to forge transcript" {
    local embedded="$REPO_ROOT/cli/embedded/hooks"
    [ -f "$embedded/ao-extract.sh" ] || skip "embedded hooks not found"
    local ao_mock="$TMP_TEST_DIR/bin"
    local ao_log="$TMP_TEST_DIR/ao-invocations.log"
    mkdir -p "$ao_mock"
    cat > "$ao_mock/ao" <<MOCK_EOF
#!/bin/bash
echo "\$@" >> "$ao_log"
exit 0
MOCK_EOF
    chmod +x "$ao_mock/ao"
    : > "$ao_log"
    PATH="$ao_mock:/usr/bin:/bin" CLAUDE_SESSION_ID="delegation-test" bash "$embedded/ao-extract.sh" >/dev/null 2>&1 || true
    grep -qE "^forge transcript" "$ao_log"
}

# ═══════════════════════════════════════════════════════════════════════
# ao-* embedded kill switch
# ═══════════════════════════════════════════════════════════════════════

@test "ao-inject: kill switch prevents ao call" {
    local embedded="$REPO_ROOT/cli/embedded/hooks"
    [ -f "$embedded/ao-inject.sh" ] || skip "embedded hooks not found"
    local ao_mock="$TMP_TEST_DIR/bin"
    local ao_log="$TMP_TEST_DIR/ao-invocations.log"
    mkdir -p "$ao_mock"
    cat > "$ao_mock/ao" <<MOCK_EOF
#!/bin/bash
echo "\$@" >> "$ao_log"
exit 0
MOCK_EOF
    chmod +x "$ao_mock/ao"
    : > "$ao_log"
    PATH="$ao_mock:/usr/bin:/bin" AGENTOPS_HOOKS_DISABLED=1 bash "$embedded/ao-inject.sh" >/dev/null 2>&1 || true
    [ ! -s "$ao_log" ]
}

# ═══════════════════════════════════════════════════════════════════════
# constraint-compiler.sh
# ═══════════════════════════════════════════════════════════════════════

@test "finding-compiler: registry entries promote into advisory artifacts" {
    local mock="$TMP_TEST_DIR/mock-finding-compiler"
    setup_mock_repo "$mock"
    mkdir -p "$mock/.agents/findings"
    cat > "$mock/.agents/findings/registry.jsonl" <<'EOF'
{"id":"f-bats-compiler","version":1,"tier":"local","source":{"repo":"agentops/crew/nami","session":"2026-03-09","file":".agents/council/source.md","skill":"pre-mortem"},"date":"2026-03-09","severity":"significant","category":"validation-gap","pattern":"Finding compiler should emit advisory artifacts.","detection_question":"Did the compiler emit plan and pre-mortem files?","checklist_item":"Compile advisory artifacts from active findings.","applicable_languages":["markdown","shell"],"applicable_when":["plan-shape"],"status":"active","superseded_by":null,"dedup_key":"validation-gap|finding-compiler-should-emit-advisory-artifacts|plan-shape","hit_count":0,"last_cited":null,"ttl_days":30,"confidence":"high"}
EOF
    run bash -c 'cd "$1" && bash "$2" 2>&1' -- "$mock" "$HOOKS_DIR/finding-compiler.sh"
    [ "$status" -eq 0 ]
    [ -f "$mock/.agents/findings/f-bats-compiler.md" ]
    [ -f "$mock/.agents/planning-rules/f-bats-compiler.md" ]
    [ -f "$mock/.agents/pre-mortem-checks/f-bats-compiler.md" ]
}

@test "constraint-compiler: requires learning path argument" {
    local mock="$TMP_TEST_DIR/mock-constraint-arg"
    mkdir -p "$mock"
    git -C "$mock" init -q >/dev/null 2>&1
    run bash -c 'cd "$1" && bash "$2" 2>&1' -- "$mock" "$HOOKS_DIR/constraint-compiler.sh"
    [ "$status" -eq 1 ]
}

@test "constraint-compiler: tagged constraint learning routes through finding compiler outputs" {
    local mock="$TMP_TEST_DIR/mock-constraint"
    mkdir -p "$mock"
    git -C "$mock" init -q >/dev/null 2>&1
    cat > "$mock/learn-constraint.md" <<'LEARN_EOF'
---
title: Constraint rule
id: learn-constraint
date: 2026-02-24
tags: [constraint, reliability]
---

This learning describes a guardrail to prevent direct bypass of safety checks.
LEARN_EOF
    run bash -c 'cd "$1" && bash "$2" "$3" 2>&1' -- "$mock" "$HOOKS_DIR/constraint-compiler.sh" "$mock/learn-constraint.md"
    [ "$status" -eq 0 ]
    [[ "$output" == *"finding-compiler.sh"* || "$output" == *"Promoted legacy learning"* ]]
    [ -f "$mock/.agents/findings/learn-constraint.md" ]
    [ -f "$mock/.agents/planning-rules/learn-constraint.md" ]
    [ -f "$mock/.agents/pre-mortem-checks/learn-constraint.md" ]
    [ -x "$mock/.agents/constraints/learn-constraint.sh" ]
    [ -f "$mock/.agents/constraints/index.json" ]
}

# ═══════════════════════════════════════════════════════════════════════
# go-complexity-precommit.sh / go-test-precommit.sh / go-vet-post-edit.sh
# ═══════════════════════════════════════════════════════════════════════

@test "go-complexity-precommit: ignores non-Edit/Write tools" {
    run bash -c 'CLAUDE_TOOL_NAME="Read" CLAUDE_TOOL_INPUT_FILE_PATH="cli/cmd/ao/main.go" bash "$1" 2>&1' \
        -- "$HOOKS_DIR/go-complexity-precommit.sh"
    [ "$status" -eq 0 ]
}

@test "go-test-precommit: ignores non-Bash tool" {
    run bash -c 'CLAUDE_TOOL_NAME="Edit" CLAUDE_TOOL_INPUT_COMMAND="" bash "$1" 2>&1' \
        -- "$HOOKS_DIR/go-test-precommit.sh"
    [ "$status" -eq 0 ]
}

@test "go-test-precommit: non-Go staged commit exits silently" {
    local mock="$TMP_TEST_DIR/mock-go-test-precommit"
    setup_mock_repo "$mock"
    mkdir -p "$mock/cli"
    echo "module example.com/agentops-test" > "$mock/cli/go.mod"
    git -C "$mock" add cli/go.mod
    git -C "$mock" -c user.name=test -c user.email=t@t commit -q -m "init go module" 2>/dev/null
    echo "docs" > "$mock/README.md"
    git -C "$mock" add README.md
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$mock" '{"tool_name":"Bash","tool_input":{"command":"git commit -m docs"}}' "$HOOKS_DIR/go-test-precommit.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "go-vet-post-edit: ignores non-.go files" {
    run bash -c 'CLAUDE_TOOL_NAME="Edit" CLAUDE_TOOL_INPUT_FILE_PATH="/tmp/test.py" bash "$1" 2>&1' \
        -- "$HOOKS_DIR/go-vet-post-edit.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# session-end-maintenance.sh / ao-flywheel-close.sh
# ═══════════════════════════════════════════════════════════════════════

@test "session-end-maintenance: kill switch short-circuits" {
    run bash -c 'AGENTOPS_HOOKS_DISABLED=1 bash "$1" 2>&1' \
        -- "$HOOKS_DIR/session-end-maintenance.sh"
    [ "$status" -eq 0 ]
}

@test "session-end-maintenance: finding compiler backstop refreshes compiled artifacts" {
    local mock="$TMP_TEST_DIR/mock-session-end"
    setup_mock_repo "$mock"
    mkdir -p "$mock/.agents/findings" "$TMP_TEST_DIR/bin"
    cat > "$mock/.agents/findings/registry.jsonl" <<'EOF'
{"id":"f-bats-session-end","version":1,"tier":"local","source":{"repo":"agentops/crew/nami","session":"2026-03-09","file":".agents/council/source.md","skill":"post-mortem"},"date":"2026-03-09","severity":"significant","category":"validation-gap","pattern":"Session end should compile findings before close.","detection_question":"Did session end rebuild compiled artifacts?","checklist_item":"Compile findings as part of session close.","applicable_languages":["markdown","shell"],"applicable_when":["validation-gap"],"status":"active","superseded_by":null,"dedup_key":"validation-gap|session-end-should-compile-findings-before-close|validation-gap","hit_count":0,"last_cited":null,"ttl_days":30,"confidence":"high"}
EOF
    cat > "$TMP_TEST_DIR/bin/ao" <<'EOF'
#!/bin/bash
exit 0
EOF
    chmod +x "$TMP_TEST_DIR/bin/ao"
    run bash -c 'cd "$1" && PATH="$2:$PATH" bash "$3" 2>&1' \
        -- "$mock" "$TMP_TEST_DIR/bin" "$HOOKS_DIR/session-end-maintenance.sh"
    [ "$status" -eq 0 ]
    [ -f "$mock/.agents/planning-rules/f-bats-session-end.md" ]
    [ -f "$mock/.agents/pre-mortem-checks/f-bats-session-end.md" ]
}

@test "ao-flywheel-close: kill switch exits cleanly" {
    run bash -c 'AGENTOPS_HOOKS_DISABLED=1 bash "$1" 2>&1' \
        -- "$HOOKS_DIR/ao-flywheel-close.sh"
    [ "$status" -eq 0 ]
}

@test "ao-flywheel-close: fail-open without ao" {
    run bash -c 'PATH="/usr/bin:/bin" bash "$1" 2>&1' \
        -- "$HOOKS_DIR/ao-flywheel-close.sh"
    [ "$status" -eq 0 ]
}

@test "ao-flywheel-close: suppresses quiet command stdout" {
    local mock_bin="$TMP_TEST_DIR/mock-ao-close-bin"
    mkdir -p "$mock_bin"
    cat > "$mock_bin/ao" <<'EOF'
#!/usr/bin/env bash
echo "unexpected stdout"
echo "unexpected stderr" >&2
exit 0
EOF
    chmod +x "$mock_bin/ao"
    run bash -c 'PATH="$1:$PATH" bash "$2" 2>&1' \
        -- "$mock_bin" "$HOOKS_DIR/ao-flywheel-close.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# quality-signals.sh
# ═══════════════════════════════════════════════════════════════════════

@test "quality-signals: repeated prompt writes advisory signal" {
    local mock="$TMP_TEST_DIR/mock-quality-signals"
    setup_mock_repo "$mock"

    run bash -c 'cd "$1" && printf "%s" "$2" | CLAUDE_SESSION_ID="bats-quality-1" bash "$3" 2>&1' \
        -- "$mock" '{"prompt":"repeat this"}' "$HOOKS_DIR/quality-signals.sh"
    [ "$status" -eq 0 ]

    run bash -c 'cd "$1" && printf "%s" "$2" | CLAUDE_SESSION_ID="bats-quality-1" bash "$3" 2>&1' \
        -- "$mock" '{"prompt":"repeat this"}' "$HOOKS_DIR/quality-signals.sh"
    [ "$status" -eq 0 ]
    grep -q '"signal_type":"repeated_prompt"' "$mock/.agents/signals/session-quality.jsonl"
}

@test "quality-signals: stores prompt fingerprint instead of raw prompt text" {
    local mock="$TMP_TEST_DIR/mock-quality-signals-fingerprint"
    setup_mock_repo "$mock"

    run bash -c 'cd "$1" && printf "%s" "$2" | CODEX_SESSION_ID="codex-quality-1" bash "$3" 2>&1' \
        -- "$mock" '{"prompt":"private prompt fixture"}' "$HOOKS_DIR/quality-signals.sh"
    [ "$status" -eq 0 ]
    [ -f "$mock/.agents/ao/.last-prompt" ]
    ! grep -q "private prompt fixture" "$mock/.agents/ao/.last-prompt"
}

# ═══════════════════════════════════════════════════════════════════════
# Coverage check
# ═══════════════════════════════════════════════════════════════════════

@test "coverage: all hook scripts referenced in tests" {
    MISSING_HOOKS=""
    for hook_file in "$HOOKS_DIR"/*.sh; do
        hook_name=$(basename "$hook_file" .sh)
        # Count any explicit coverage in the hooks test suite, including dedicated per-hook tests.
        if ! grep -F -q "$hook_name" "$BATS_TEST_DIRNAME"/*.bats "$BATS_TEST_DIRNAME"/*.sh 2>/dev/null; then
            MISSING_HOOKS="$MISSING_HOOKS $hook_name"
        fi
    done
    [ -z "$MISSING_HOOKS" ]
}

# ═══════════════════════════════════════════════════════════════════════
# validate-hook-preflight.sh
# ═══════════════════════════════════════════════════════════════════════

@test "validate-hook-preflight: passes" {
    run bash "$REPO_ROOT/scripts/validate-hook-preflight.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# worktree-setup.sh / worktree-cleanup.sh
# ═══════════════════════════════════════════════════════════════════════

@test "worktree-setup: no-path fail-open" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"event":"WorktreeCreate"}' "$HOOKS_DIR/worktree-setup.sh"
    [ "$status" -eq 0 ]
}

@test "worktree-cleanup: no-path fail-open" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"event":"WorktreeRemove"}' "$HOOKS_DIR/worktree-cleanup.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# stop-auto-handoff.sh / subagent-stop.sh
# ═══════════════════════════════════════════════════════════════════════

@test "stop-auto-handoff: writes pending handoff" {
    local mock="$TMP_TEST_DIR/mock-stop-handoff"
    setup_mock_repo "$mock"
    (cd "$mock" && printf '%s' '{"last_assistant_message":"worker summary"}' | CLAUDE_SESSION_ID="bats-stop-1" bash "$HOOKS_DIR/stop-auto-handoff.sh" >/dev/null 2>&1)
    ls "$mock/.agents/handoff"/stop-*.md >/dev/null 2>&1
}

@test "subagent-stop: writes output artifact" {
    local mock="$TMP_TEST_DIR/mock-subagent-stop"
    setup_mock_repo "$mock"
    (cd "$mock" && printf '%s' '{"agent_name":"worker-alpha","last_assistant_message":"final worker output"}' | CLAUDE_SESSION_ID="bats-sub-1" bash "$HOOKS_DIR/subagent-stop.sh" >/dev/null 2>&1)
    ls "$mock/.agents/ao/subagent-outputs"/*.md >/dev/null 2>&1
}

# ═══════════════════════════════════════════════════════════════════════
# config-change-monitor.sh
# ═══════════════════════════════════════════════════════════════════════

@test "config-change-monitor: logs config changes" {
    local mock="$TMP_TEST_DIR/mock-config"
    setup_mock_repo "$mock"
    (cd "$mock" && printf '%s' '{"config_key":"approval_policy","old_value":"never","new_value":"on-request"}' | CLAUDE_SESSION_ID="bats-config-1" bash "$HOOKS_DIR/config-change-monitor.sh" >/dev/null 2>&1)
    [ -f "$mock/.agents/ao/config-changes.jsonl" ]
    grep -q '"config_key":"approval_policy"' "$mock/.agents/ao/config-changes.jsonl"
}

@test "config-change-monitor: strict mode blocks critical changes" {
    local mock="$TMP_TEST_DIR/mock-config-strict"
    setup_mock_repo "$mock"
    run bash -c 'cd "$1" && printf "%s" "$2" | AGENTOPS_CONFIG_GUARD_STRICT=1 bash "$3" 2>&1' \
        -- "$mock" '{"config_key":"approval_policy","old_value":"never","new_value":"on-request"}' "$HOOKS_DIR/config-change-monitor.sh"
    [ "$status" -eq 2 ]
}
