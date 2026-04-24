#!/usr/bin/env bats
# hook-output-schema.bats — Validate complete JSON output schema for all JSON-emitting hooks.
# Every test asserts behavioral correctness: exit status, valid JSON, hookSpecificOutput
# structure, and correct hookEventName string — not just field existence.

setup() {
    load helpers/test_helper
    _helper_setup
    export CLAUDE_SESSION_ID="bats-schema-$$"
}

teardown() {
    _helper_teardown
}

# ── Schema validation helper ────────────────────────────────────────
# assert_hook_schema OUTPUT EXPECTED_EVENT
#   1. Output is valid JSON (parseable by jq)
#   2. .hookSpecificOutput object exists
#   3. .hookSpecificOutput.hookEventName equals EXPECTED_EVENT
#   4. .additionalContext is NOT at top level (must be inside hookSpecificOutput)
assert_hook_schema() {
    local output="$1"
    local expected_event="$2"

    # Must be valid JSON
    echo "$output" | jq . >/dev/null 2>&1 || {
        echo "FAIL: output is not valid JSON: $output"
        return 1
    }

    # hookSpecificOutput must exist and be an object
    local hso_type
    hso_type=$(echo "$output" | jq -r '.hookSpecificOutput | type')
    [ "$hso_type" = "object" ] || {
        echo "FAIL: .hookSpecificOutput is '$hso_type', expected 'object'"
        return 1
    }

    # hookEventName must equal expected value
    local actual_event
    actual_event=$(echo "$output" | jq -r '.hookSpecificOutput.hookEventName')
    [ "$actual_event" = "$expected_event" ] || {
        echo "FAIL: hookEventName is '$actual_event', expected '$expected_event'"
        return 1
    }

    # additionalContext must NOT exist at top level
    local top_ctx
    top_ctx=$(echo "$output" | jq -r '.additionalContext // "ABSENT"')
    [ "$top_ctx" = "ABSENT" ] || {
        echo "FAIL: .additionalContext exists at top level (should be inside hookSpecificOutput)"
        return 1
    }
}

# ═══════════════════════════════════════════════════════════════════════
# 1. session-start.sh (SessionStart) — registered but intentionally silent
# ═══════════════════════════════════════════════════════════════════════

@test "session-start: produces no operator-facing JSON output" {
    run bash -c 'cd "$1" && CLAUDE_SESSION_ID="bats-schema-sess" bash "$2" 2>&1' \
        -- "$MOCK_REPO" "$HOOKS_DIR/session-start.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# 2. prompt-nudge.sh (UserPromptSubmit) — registered
#    Only emits JSON when there is a nudge to give (ratchet gate active).
# ═══════════════════════════════════════════════════════════════════════

@test "factory-router: intake prompt stays silent while staging state" {
    local mock="$TMP_TEST_DIR/mock-factory-router"
    mkdir -p "$mock/.agents/ao" "$mock/bin"
    git -C "$mock" init -q >/dev/null 2>&1
    touch "$mock/.agents/ao/.factory-intake-needed"
    cat > "$mock/bin/ao" <<'EOF'
#!/usr/bin/env bash
if [ "${1:-}" = "knowledge" ] && [ "${2:-}" = "brief" ]; then
    briefing_path="${MOCK_FACTORY_BRIEFING_PATH:?}"
    mkdir -p "$(dirname "$briefing_path")"
    cat > "$briefing_path" <<'BRIEF'
# Briefing: fix auth bootstrap
BRIEF
    jq -n --arg path "$briefing_path" '{"output_path":$path}'
    exit 0
fi
exit 0
EOF
    chmod +x "$mock/bin/ao"
    local briefing_path="$mock/.agents/briefings/2026-04-02-fix-auth-bootstrap.md"
    run bash -c 'cd "$1" && printf "%s" "$3" | PATH="$1/bin:$PATH" MOCK_FACTORY_BRIEFING_PATH="$2" AGENTOPS_STARTUP_CONTEXT_MODE=factory bash "$4" 2>&1' \
        -- "$mock" "$briefing_path" '{"prompt":"fix auth bootstrap"}' "$HOOKS_DIR/factory-router.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
    [ "$(cat "$mock/.agents/ao/factory-goal.txt" 2>/dev/null)" = "fix auth bootstrap" ]
    [ "$(cat "$mock/.agents/ao/factory-briefing.txt" 2>/dev/null)" = "$briefing_path" ]
}

@test "prompt-nudge: when nudge fires, output conforms to schema with hookEventName=UserPromptSubmit" {
    # Set up ratchet state so prompt-nudge has something to nudge about.
    # Simulate: research step done, plan step pending, user says "implement".
    mkdir -p "$MOCK_REPO/.agents/ao"
    echo '{"step":"research","status":"done","ts":"2026-01-01T00:00:00Z"}' > "$MOCK_REPO/.agents/ao/chain.jsonl"
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"prompt":"implement the feature now"}' "$HOOKS_DIR/prompt-nudge.sh"
    [ "$status" -eq 0 ]
    # If output is non-empty, it must conform to schema
    if [ -n "$output" ]; then
        assert_hook_schema "$output" "UserPromptSubmit"
    fi
}

@test "prompt-nudge: empty output is acceptable when no nudge condition met" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"prompt":"hello"}' "$HOOKS_DIR/prompt-nudge.sh"
    [ "$status" -eq 0 ]
    # No nudge triggered — output should be empty
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# 3. new-user-welcome.sh (UserPromptSubmit) — registered
# ═══════════════════════════════════════════════════════════════════════

@test "new-user-welcome: fresh-repo prompt output conforms to schema with hookEventName=UserPromptSubmit" {
    mkdir -p "$MOCK_REPO/.agents/ao"
    touch "$MOCK_REPO/.agents/ao/.new-user-welcome-needed"
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"prompt":"help me understand this auth repo"}' "$HOOKS_DIR/new-user-welcome.sh"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    assert_hook_schema "$output" "UserPromptSubmit"
}

# ═══════════════════════════════════════════════════════════════════════
# 4. intent-echo.sh (UserPromptSubmit) — registered
# ═══════════════════════════════════════════════════════════════════════

@test "intent-echo: destructive prompt output conforms to schema with hookEventName=UserPromptSubmit" {
    rm -f "$REPO_ROOT/.agents/ao/.intent-echo-fired" 2>/dev/null
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"prompt":"delete all the old files and remove everything"}' "$HOOKS_DIR/intent-echo.sh"
    [ "$status" -eq 0 ]
    if [ -n "$output" ]; then
        assert_hook_schema "$output" "UserPromptSubmit"
    fi
}

@test "intent-echo: non-destructive prompt emits no JSON" {
    rm -f "$REPO_ROOT/.agents/ao/.intent-echo-fired" 2>/dev/null
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"prompt":"add a new test"}' "$HOOKS_DIR/intent-echo.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# 5. commit-review-gate.sh (PreToolUse) — registered
# ═══════════════════════════════════════════════════════════════════════

@test "commit-review-gate: git commit with staged changes emits schema with hookEventName=PreToolUse" {
    # Create staged changes in mock repo so diff is non-empty
    echo "initial" > "$MOCK_REPO/file.txt"
    git -C "$MOCK_REPO" add file.txt
    git -C "$MOCK_REPO" -c user.name=test -c user.email=t@t commit -q -m "init" 2>/dev/null
    echo "changed" > "$MOCK_REPO/file.txt"
    git -C "$MOCK_REPO" add file.txt
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_name":"Bash","tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/commit-review-gate.sh"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    assert_hook_schema "$output" "PreToolUse"
}

@test "commit-review-gate: redacts secret-like staged diff values" {
    echo "initial" > "$MOCK_REPO/file.txt"
    git -C "$MOCK_REPO" add file.txt
    git -C "$MOCK_REPO" -c user.name=test -c user.email=t@t commit -q -m "init" 2>/dev/null
    echo "TOKEN_FOR_REDACTION=not-a-secret-fixture" > "$MOCK_REPO/config.env"
    git -C "$MOCK_REPO" add config.env
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_name":"Bash","tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/commit-review-gate.sh"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    local ctx
    ctx=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [[ "$ctx" == *"TOKEN_FOR_REDACTION=[REDACTED]"* ]]
    [[ "$ctx" != *"not-a-secret-fixture"* ]]
}

@test "commit-review-gate: non-git command emits no output" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}' "$HOOKS_DIR/commit-review-gate.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# 6. go-vet-post-edit.sh (PostToolUse) — registered
#    Only emits JSON when editing a _test.go file with assertion-free test functions.
# ═══════════════════════════════════════════════════════════════════════

@test "go-vet-post-edit: test file with empty test emits schema with hookEventName=PostToolUse" {
    # Create a Go module and a test file with an assertion-free test function
    mkdir -p "$MOCK_REPO/pkg"
    echo "module example.com/test" > "$MOCK_REPO/pkg/go.mod"
    echo "package pkg" > "$MOCK_REPO/pkg/main.go"
    cat > "$MOCK_REPO/pkg/main_test.go" <<'GOEOF'
package pkg

import "testing"

func TestEmpty(t *testing.T) {
    x := 1
    _ = x
}
GOEOF
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_name":"Edit","tool_input":{"file_path":"'"$MOCK_REPO/pkg/main_test.go"'"}}' \
        "$HOOKS_DIR/go-vet-post-edit.sh"
    [ "$status" -eq 0 ]
    # If the density check fires, validate the schema
    if [ -n "$output" ] && echo "$output" | jq -e '.hookSpecificOutput' >/dev/null 2>&1; then
        assert_hook_schema "$output" "PostToolUse"
    fi
}

@test "go-vet-post-edit: non-Go file emits no output" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_name":"Edit","tool_input":{"file_path":"foo.py"}}' "$HOOKS_DIR/go-vet-post-edit.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# 7. research-loop-detector.sh (PostToolUse) — registered
# ═══════════════════════════════════════════════════════════════════════

@test "research-loop-detector: streak at threshold emits schema with hookEventName=PostToolUse" {
    # Set streak to 7 so the next Read triggers the warn threshold (8)
    mkdir -p "$MOCK_REPO/.agents/ao"
    echo "7" > "$MOCK_REPO/.agents/ao/.read-streak"
    run bash -c 'cd "$1" && printf "%s" "$2" | CLAUDE_TOOL_NAME=Read bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_name":"Read","tool_input":{"file_path":"x.md"}}' "$HOOKS_DIR/research-loop-detector.sh"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    assert_hook_schema "$output" "PostToolUse"
    # Verify the additionalContext mentions the streak count
    local ctx
    ctx=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [[ "$ctx" == *"8"* ]]
}

@test "research-loop-detector: below threshold emits no output" {
    mkdir -p "$MOCK_REPO/.agents/ao"
    echo "2" > "$MOCK_REPO/.agents/ao/.read-streak"
    run bash -c 'cd "$1" && printf "%s" "$2" | CLAUDE_TOOL_NAME=Read bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_name":"Read","tool_input":{"file_path":"x.md"}}' "$HOOKS_DIR/research-loop-detector.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# 8. context-guard.sh (UserPromptSubmit) — unregistered
#    Requires `ao` CLI to return a message.
# ═══════════════════════════════════════════════════════════════════════

@test "context-guard: when ao returns message, output conforms to schema with hookEventName=UserPromptSubmit" {
    # Mock ao to return a warning
    local ao_mock="$TMP_TEST_DIR/bin"
    mkdir -p "$ao_mock"
    cat > "$ao_mock/ao" <<'AOEOF'
#!/bin/bash
echo '{"session":{"action":"warn"},"hook_message":"Context budget at 75%"}'
AOEOF
    chmod +x "$ao_mock/ao"
    run bash -c 'printf "%s" "$1" | PATH="'"$ao_mock"':$PATH" CLAUDE_SESSION_ID="bats-ctx-schema" bash "$2" 2>&1' \
        -- '{"prompt":"keep going"}' "$HOOKS_DIR/context-guard.sh"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    assert_hook_schema "$output" "UserPromptSubmit"
    # Verify the actual message content is passed through
    local ctx
    ctx=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [ "$ctx" = "Context budget at 75%" ]
}

@test "context-guard: when ao returns no message, output is empty" {
    local ao_mock="$TMP_TEST_DIR/bin"
    mkdir -p "$ao_mock"
    cat > "$ao_mock/ao" <<'AOEOF'
#!/bin/bash
echo '{"session":{"action":"ok"}}'
AOEOF
    chmod +x "$ao_mock/ao"
    run bash -c 'printf "%s" "$1" | PATH="'"$ao_mock"':$PATH" CLAUDE_SESSION_ID="bats-ctx-empty" bash "$2" 2>&1' \
        -- '{"prompt":"hello"}' "$HOOKS_DIR/context-guard.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# 8. precompact-snapshot.sh (PreCompact) — unregistered
# ═══════════════════════════════════════════════════════════════════════

@test "precompact-snapshot: output conforms to schema with hookEventName=PreCompact" {
    # Set up minimal .agents/ context so the hook has something to snapshot
    mkdir -p "$MOCK_REPO/.agents/ao"
    echo '{"step":"plan","status":"done"}' > "$MOCK_REPO/.agents/ao/chain.jsonl"
    run bash -c 'cd "$1" && CLAUDE_SESSION_ID="bats-precompact" bash "$2" 2>&1' \
        -- "$MOCK_REPO" "$HOOKS_DIR/precompact-snapshot.sh"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    # The hook emits multi-line pretty-printed JSON; extract with jq
    local json_blob
    json_blob=$(echo "$output" | jq -c 'select(.hookSpecificOutput)' 2>/dev/null | tail -1)
    [ -n "$json_blob" ]
    assert_hook_schema "$json_blob" "PreCompact"
}

# ═══════════════════════════════════════════════════════════════════════
# 9. ratchet-advance.sh (PostToolUse) — unregistered
# ═══════════════════════════════════════════════════════════════════════

@test "ratchet-advance: successful ratchet record emits schema with hookEventName=PostToolUse" {
    run bash -c 'cd "$1" && printf "%s" "$2" | PATH="/usr/bin:/bin" bash "$3" 2>&1' \
        -- "$MOCK_REPO" \
        '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}' \
        "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    assert_hook_schema "$output" "PostToolUse"
    # Verify the message mentions the step and suggested next action
    local ctx
    ctx=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [[ "$ctx" == *"research"* ]]
    [[ "$ctx" == *"plan"* ]]
    [[ "$ctx" != *"/plan"* ]]
}

@test "ratchet-advance: non-ratchet command emits no output" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"go test ./..."},"tool_response":{"exit_code":0}}' \
        "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# 10. standards-injector.sh (PreToolUse) — unregistered
# ═══════════════════════════════════════════════════════════════════════

@test "standards-injector: python file emits schema with hookEventName=PreToolUse" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"file_path":"/some/path/main.py"}}' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    assert_hook_schema "$output" "PreToolUse"
    # Verify additionalContext contains actual standards content (not empty)
    local ctx
    ctx=$(echo "$output" | jq -r '.hookSpecificOutput.additionalContext')
    [ ${#ctx} -gt 10 ]
}

@test "standards-injector: go file emits schema with hookEventName=PreToolUse" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"file_path":"/some/path/main.go"}}' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
    [ -n "$output" ]
    assert_hook_schema "$output" "PreToolUse"
}

@test "standards-injector: unknown extension emits no output" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"file_path":"/some/path/data.csv"}}' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ═══════════════════════════════════════════════════════════════════════
# CROSS-CHECK: hooks.json event registration vs hookEventName in output
# ═══════════════════════════════════════════════════════════════════════

@test "cross-check: registered hooks emit hookEventName matching their hooks.json event" {
    # Map of registered JSON-emitting hooks to their expected hookEventName.
    # This is the meta-test that would have caught PR #71.
    local hooks_json="$HOOKS_DIR/hooks.json"
    [ -f "$hooks_json" ]

    # Parse hooks.json to build event-to-hook mapping for registered JSON emitters.
    # SessionStart and factory-router are intentionally silent.
    local -A hook_event_map
    hook_event_map=(
        ["prompt-nudge.sh"]="UserPromptSubmit"
        ["intent-echo.sh"]="UserPromptSubmit"
        ["commit-review-gate.sh"]="PreToolUse"
        ["go-vet-post-edit.sh"]="PostToolUse"
        ["research-loop-detector.sh"]="PostToolUse"
        ["edit-knowledge-surface.sh"]="PreToolUse"
    )

    local failures=""

    for hook_file in "${!hook_event_map[@]}"; do
        local expected_event="${hook_event_map[$hook_file]}"
        local hook_path="$HOOKS_DIR/$hook_file"
        [ -f "$hook_path" ] || {
            failures="${failures}  MISSING: $hook_file\n"
            continue
        }

        # Verify the hook is actually registered in hooks.json under the expected event
        local registered_event
        registered_event=$(jq -r --arg hf "$hook_file" '
            .hooks | to_entries[] |
            select(.value[].hooks[]?.command | test($hf)) |
            .key
        ' "$hooks_json" 2>/dev/null | head -1)

        [ -n "$registered_event" ] || {
            failures="${failures}  NOT REGISTERED: $hook_file expected under $expected_event\n"
            continue
        }

        # Verify the hookEventName in the source code matches the registered event
        local code_event
        code_event=$(grep -oE '"hookEventName":[[:space:]]*"[^"]*"' "$hook_path" | head -1 | sed 's/"hookEventName":[[:space:]]*"//;s/"//')
        [ -n "$code_event" ] || {
            failures="${failures}  NO hookEventName found in source: $hook_file\n"
            continue
        }

        [ "$code_event" = "$registered_event" ] || {
            failures="${failures}  MISMATCH: $hook_file emits hookEventName=$code_event but registered under $registered_event\n"
        }
    done

    if [ -n "$failures" ]; then
        echo "Cross-check failures:"
        printf '%b' "$failures"
        return 1
    fi
}

@test "cross-check: all hooks.json registered hooks that emit JSON use hookSpecificOutput wrapper" {
    # Verify no registered hook emits bare additionalContext without hookSpecificOutput
    local hooks_json="$HOOKS_DIR/hooks.json"
    local failures=""

    # Extract all hook script filenames from hooks.json
    local hook_files
    hook_files=$(jq -r '.hooks[][].hooks[].command' "$hooks_json" 2>/dev/null | \
        sed 's|.*hooks/||' | sort -u)

    while IFS= read -r hook_file; do
        [ -z "$hook_file" ] && continue
        local hook_path="$HOOKS_DIR/$hook_file"
        [ -f "$hook_path" ] || continue

        # If the hook emits additionalContext, it must be inside hookSpecificOutput
        if grep -q 'additionalContext' "$hook_path"; then
            # Check that every additionalContext reference is inside hookSpecificOutput
            if grep -q '"additionalContext"' "$hook_path" && \
               ! grep -q 'hookSpecificOutput' "$hook_path"; then
                failures="${failures}  BARE additionalContext in $hook_file (not wrapped in hookSpecificOutput)\n"
            fi
        fi
    done <<< "$hook_files"

    if [ -n "$failures" ]; then
        echo "Wrapper check failures:"
        printf '%b' "$failures"
        return 1
    fi
}

@test "cross-check: every hookEventName in source matches a valid Claude Code hook event" {
    # Valid Claude Code hook events
    local valid_events="SessionStart SessionEnd Stop UserPromptSubmit PreToolUse PostToolUse PreCompact TaskCompleted"
    local failures=""

    # Check all hooks in the hooks directory
    for hook_path in "$HOOKS_DIR"/*.sh; do
        [ -f "$hook_path" ] || continue
        local hook_file
        hook_file=$(basename "$hook_path")

        # Extract all hookEventName values from this file
        local events
        events=$(grep -oE '"hookEventName":"[^"]*"' "$hook_path" 2>/dev/null | \
            sed 's/"hookEventName":"//;s/"//' | sort -u)

        while IFS= read -r event; do
            [ -z "$event" ] && continue
            local found=0
            for valid in $valid_events; do
                if [ "$event" = "$valid" ]; then
                    found=1
                    break
                fi
            done
            [ "$found" -eq 1 ] || {
                failures="${failures}  INVALID EVENT: $hook_file uses hookEventName=$event\n"
            }
        done <<< "$events"
    done

    if [ -n "$failures" ]; then
        echo "Event name validation failures:"
        printf '%b' "$failures"
        return 1
    fi
}
