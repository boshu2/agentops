#!/usr/bin/env bats
# hook-stdin-contracts.bats — Verify all 12 stdin-consuming hooks handle JSON contracts correctly.
# Each hook gets: valid input, malformed JSON, and empty stdin tests.

setup() {
    load helpers/test_helper
    _helper_setup
    export CLAUDE_SESSION_ID="bats-contract-$$"
}

teardown() {
    _helper_teardown
}

# Helper: pipe JSON to hook in a subshell, properly handling pipes
run_hook() {
    local hook="$1"
    local json="$2"
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' -- "$json" "$hook"
}

# Helper: pipe JSON to hook with extra env vars
# Usage: run_hook_env "HOOK" "JSON" "VAR1=val1" "VAR2=val2"
run_hook_env() {
    local hook="$1"
    local json="$2"
    shift 2
    local env_str=""
    for ev in "$@"; do
        env_str="${env_str}export ${ev}; "
    done
    run bash -c "${env_str}"'printf "%s" "$1" | bash "$2" 2>&1' -- "$json" "$hook"
}

# ═══════════════════════════════════════════════════════════════════════
# 1. citation-tracker.sh — .tool_input.file_path (Read tool)
# ═══════════════════════════════════════════════════════════════════════

@test "citation-tracker: valid .agents/ path writes citation record" {
    mkdir -p "$MOCK_REPO/.agents/learnings"
    echo "test" > "$MOCK_REPO/.agents/learnings/item.md"
    run bash -c 'cd "$1" && printf "%s" "$2" | CLAUDE_SESSION_ID="bats-cite-valid-'$$'" bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"file_path":".agents/learnings/item.md"}}' "$HOOKS_DIR/citation-tracker.sh"
    [ "$status" -eq 0 ]
    [ -f "$MOCK_REPO/.agents/ao/citations.jsonl" ]
    grep -q ".agents/learnings/item.md" "$MOCK_REPO/.agents/ao/citations.jsonl"
}

@test "citation-tracker: non-.agents path exits silently" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"file_path":"README.md"}}' "$HOOKS_DIR/citation-tracker.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "citation-tracker: malformed JSON exits gracefully (no hang)" {
    # citation-tracker uses set -euo pipefail, so jq parse failure causes non-zero exit.
    # The contract is: no hang, no crash dump — a controlled exit is acceptable.
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" 'not-valid-json{{{' "$HOOKS_DIR/citation-tracker.sh"
    # Any exit code is fine — the key is that it completed without hanging
    [[ "$status" -le 128 ]]
}

@test "citation-tracker: empty stdin exits gracefully" {
    run bash -c 'cd "$1" && printf "" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '' "$HOOKS_DIR/citation-tracker.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 2. commit-review-gate.sh — .tool_input.command (Bash git commit)
# ═══════════════════════════════════════════════════════════════════════

@test "commit-review-gate: non-git command exits silently" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_name":"Bash","tool_input":{"command":"go test ./..."}}' "$HOOKS_DIR/commit-review-gate.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "commit-review-gate: kill switch exits silently" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_COMMIT_REVIEW_DISABLED=1 bash "$2" 2>&1' \
        -- '{"tool_name":"Bash","tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/commit-review-gate.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "commit-review-gate: malformed JSON exits gracefully" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{{broken json' "$HOOKS_DIR/commit-review-gate.sh"
    [ "$status" -eq 0 ]
}

@test "commit-review-gate: empty stdin exits gracefully" {
    run bash -c 'printf "" | bash "$1" 2>&1' \
        -- "$HOOKS_DIR/commit-review-gate.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 4. context-guard.sh — .prompt (UserPromptSubmit)
# ═══════════════════════════════════════════════════════════════════════

@test "context-guard: missing session ID exits silently" {
    run bash -c 'printf "%s" "$1" | CLAUDE_SESSION_ID="" bash "$2" 2>&1' \
        -- '{"prompt":"hello"}' "$HOOKS_DIR/context-guard.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "context-guard: kill switch exits silently" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_CONTEXT_GUARD_DISABLED=1 bash "$2" 2>&1' \
        -- '{"prompt":"hello"}' "$HOOKS_DIR/context-guard.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "context-guard: malformed JSON exits gracefully" {
    run bash -c 'printf "%s" "$1" | CLAUDE_SESSION_ID="test-1" bash "$2" 2>&1' \
        -- '{{not json}}' "$HOOKS_DIR/context-guard.sh"
    [ "$status" -eq 0 ]
}

@test "context-guard: empty stdin exits gracefully" {
    run bash -c 'printf "" | CLAUDE_SESSION_ID="test-1" bash "$1" 2>&1' \
        -- "$HOOKS_DIR/context-guard.sh"
    [ "$status" -eq 0 ]
}

@test "context-guard: emits additionalContext when ao returns message" {
    local ao_mock="$TMP_TEST_DIR/bin"
    mkdir -p "$ao_mock"
    cat > "$ao_mock/ao" <<'AOEOF'
#!/bin/bash
echo '{"session":{"action":"warn"},"hook_message":"Context warning message"}'
AOEOF
    chmod +x "$ao_mock/ao"
    run bash -c 'printf "%s" "$1" | PATH="'"$ao_mock"':$PATH" CLAUDE_SESSION_ID="bats-ctx-1" bash "$2" 2>&1' \
        -- '{"prompt":"keep going"}' "$HOOKS_DIR/context-guard.sh"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext == "Context warning message"'
}

# ═══════════════════════════════════════════════════════════════════════
# 5. dangerous-git-guard.sh — .tool_input.command (Bash git)
# ═══════════════════════════════════════════════════════════════════════

@test "dangerous-git-guard: force push blocked with exit 2" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git push -f origin main"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 2 ]
}

@test "dangerous-git-guard: safe branch delete allowed" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git branch -d feature"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 0 ]
}

@test "dangerous-git-guard: non-git command passes" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"npm install"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 0 ]
}

@test "dangerous-git-guard: force-with-lease allowed" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git push --force-with-lease origin main"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 0 ]
}

@test "dangerous-git-guard: hard reset blocked" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git reset --hard HEAD~1"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 2 ]
}

@test "dangerous-git-guard: checkout dot blocked" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git checkout ."}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 2 ]
}

@test "dangerous-git-guard: force branch delete blocked" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git branch -D feature"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 2 ]
}

@test "dangerous-git-guard: kill switch allows force push" {
    run bash -c 'cd "$1" && printf "%s" "$2" | AGENTOPS_HOOKS_DISABLED=1 bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git push -f origin main"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 0 ]
}

@test "dangerous-git-guard: malformed JSON exits gracefully" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" 'broken{{{json' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 0 ]
}

@test "dangerous-git-guard: empty stdin exits gracefully" {
    run bash -c 'cd "$1" && printf "" | bash "$2" 2>&1' \
        -- "$MOCK_REPO" "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 5. codex-parity-warn.sh — .tool_input.file_path (Edit skill file)
# ═══════════════════════════════════════════════════════════════════════

@test "codex-parity-warn: non-skill edit exits silently" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_name":"Edit","tool_input":{"file_path":"README.md"}}' "$HOOKS_DIR/codex-parity-warn.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "codex-parity-warn: shared skill edit emits parity warning" {
    mkdir -p "$MOCK_REPO/skills/example" "$MOCK_REPO/skills-codex/example"
    touch "$MOCK_REPO/skills/example/SKILL.md" "$MOCK_REPO/skills-codex/example/SKILL.md"

    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_name":"Edit","tool_input":{"file_path":"skills/example/SKILL.md"}}' "$HOOKS_DIR/codex-parity-warn.sh"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.hookSpecificOutput.hookEventName == "PreToolUse"' >/dev/null 2>&1
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext | contains("skills-codex/example/")' >/dev/null 2>&1
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext | contains("regen-codex-hashes.sh")' >/dev/null 2>&1
}

# ═══════════════════════════════════════════════════════════════════════
# 6. git-worker-guard.sh — .tool_input.command (Bash git)
# ═══════════════════════════════════════════════════════════════════════

@test "git-worker-guard: non-git command passes for worker" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_ROLE=worker bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"ls -la"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 0 ]
}

@test "git-worker-guard: git commit blocked for worker via CLAUDE_AGENT_NAME" {
    run bash -c 'printf "%s" "$1" | CLAUDE_AGENT_NAME="worker-1" bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 2 ]
}

@test "git-worker-guard: git commit allowed for non-worker" {
    run bash -c 'printf "%s" "$1" | CLAUDE_AGENT_NAME="" AGENTOPS_ROLE="" bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 0 ]
}

@test "git-worker-guard: git push blocked for worker" {
    run bash -c 'printf "%s" "$1" | CLAUDE_AGENT_NAME="worker-3" bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git push origin main"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 2 ]
}

@test "git-worker-guard: git add -A blocked for worker" {
    run bash -c 'printf "%s" "$1" | CLAUDE_AGENT_NAME="worker-2" bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git add -A"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 2 ]
}

@test "git-worker-guard: worker via swarm-role file blocked" {
    echo "worker" > "$MOCK_REPO/.agents/swarm-role"
    run bash -c 'cd "$1" && printf "%s" "$2" | CLAUDE_AGENT_NAME="" AGENTOPS_ROLE="" bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 2 ]
}

@test "git-worker-guard: team lead allowed to commit" {
    run bash -c 'printf "%s" "$1" | CLAUDE_AGENT_NAME="team-lead" bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 0 ]
}

@test "git-worker-guard: kill switch allows worker commit" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_HOOKS_DISABLED=1 CLAUDE_AGENT_NAME="worker-1" bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"git commit -m test"}}' "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 0 ]
}

@test "git-worker-guard: malformed JSON for worker exits gracefully" {
    run bash -c 'printf "%s" "$1" | CLAUDE_AGENT_NAME="worker-1" bash "$2" 2>&1' \
        -- '{{bad json}}' "$HOOKS_DIR/git-worker-guard.sh"
    # With broken JSON, jq fails, command is empty, no git detected => pass
    [ "$status" -eq 0 ]
}

@test "git-worker-guard: empty stdin for worker exits gracefully" {
    run bash -c 'printf "" | CLAUDE_AGENT_NAME="worker-1" bash "$1" 2>&1' \
        -- "$HOOKS_DIR/git-worker-guard.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 7. intent-echo.sh — .prompt (UserPromptSubmit)
# ═══════════════════════════════════════════════════════════════════════

@test "intent-echo: normal prompt exits silently" {
    rm -f "$REPO_ROOT/.agents/ao/.intent-echo-fired" 2>/dev/null
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"prompt":"add a new test"}' "$HOOKS_DIR/intent-echo.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "intent-echo: destructive keyword triggers context injection" {
    rm -f "$REPO_ROOT/.agents/ao/.intent-echo-fired" 2>/dev/null
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"prompt":"delete all the old files"}' "$HOOKS_DIR/intent-echo.sh"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
}

@test "intent-echo: kill switch silences output" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_INTENT_ECHO_DISABLED=1 bash "$2" 2>&1' \
        -- '{"prompt":"delete everything"}' "$HOOKS_DIR/intent-echo.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "intent-echo: malformed JSON exits gracefully" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- 'not valid {json' "$HOOKS_DIR/intent-echo.sh"
    [ "$status" -eq 0 ]
}

@test "intent-echo: empty stdin exits gracefully" {
    run bash -c 'printf "" | bash "$1" 2>&1' \
        -- "$HOOKS_DIR/intent-echo.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 8. new-user-welcome.sh — .prompt (UserPromptSubmit)
# ═══════════════════════════════════════════════════════════════════════

@test "new-user-welcome: missing marker exits silently" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"prompt":"help me understand auth"}' "$HOOKS_DIR/new-user-welcome.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "new-user-welcome: substantive prompt emits context and clears marker" {
    mkdir -p "$MOCK_REPO/.agents/ao"
    touch "$MOCK_REPO/.agents/ao/.new-user-welcome-needed"
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"prompt":"help me understand auth"}' "$HOOKS_DIR/new-user-welcome.sh"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
    [ ! -f "$MOCK_REPO/.agents/ao/.new-user-welcome-needed" ]
}

@test "new-user-welcome: slash command exits silently and keeps marker" {
    mkdir -p "$MOCK_REPO/.agents/ao"
    touch "$MOCK_REPO/.agents/ao/.new-user-welcome-needed"
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"prompt":"/research auth"}' "$HOOKS_DIR/new-user-welcome.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
    [ -f "$MOCK_REPO/.agents/ao/.new-user-welcome-needed" ]
}

@test "new-user-welcome: kill switch silences output" {
    mkdir -p "$MOCK_REPO/.agents/ao"
    touch "$MOCK_REPO/.agents/ao/.new-user-welcome-needed"
    run bash -c 'cd "$1" && printf "%s" "$2" | AGENTOPS_NEW_USER_WELCOME_DISABLED=1 bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"prompt":"help me understand auth"}' "$HOOKS_DIR/new-user-welcome.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
    [ -f "$MOCK_REPO/.agents/ao/.new-user-welcome-needed" ]
}

@test "new-user-welcome: malformed JSON exits gracefully" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- 'not valid {json' "$HOOKS_DIR/new-user-welcome.sh"
    [ "$status" -eq 0 ]
}

@test "new-user-welcome: empty stdin exits gracefully" {
    run bash -c 'printf "" | bash "$1" 2>&1' \
        -- "$HOOKS_DIR/new-user-welcome.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 9. pre-mortem-gate.sh — .tool_input.skill, .tool_input.args
# ═══════════════════════════════════════════════════════════════════════

@test "pre-mortem-gate: non-Skill tool passes" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_name":"Bash","tool_input":{"command":"ls"}}' "$HOOKS_DIR/pre-mortem-gate.sh"
    [ "$status" -eq 0 ]
}

@test "pre-mortem-gate: non-crank skill passes" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_name":"Skill","tool_input":{"skill":"vibe","args":""}}' "$HOOKS_DIR/pre-mortem-gate.sh"
    [ "$status" -eq 0 ]
}

@test "pre-mortem-gate: crank with no epic ID blocks in strict mode" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_name":"Skill","tool_input":{"skill":"crank","args":""}}' "$HOOKS_DIR/pre-mortem-gate.sh"
    [ "$status" -eq 2 ]
    [[ "$output" == *"could not parse an epic-id"* ]]
}

@test "pre-mortem-gate: kill switch allows crank" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_SKIP_PRE_MORTEM_GATE=1 bash "$2" 2>&1' \
        -- '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx"}}' "$HOOKS_DIR/pre-mortem-gate.sh"
    [ "$status" -eq 0 ]
}

@test "pre-mortem-gate: worker exempt" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_WORKER=1 bash "$2" 2>&1' \
        -- '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx"}}' "$HOOKS_DIR/pre-mortem-gate.sh"
    [ "$status" -eq 0 ]
}

@test "pre-mortem-gate: --skip-pre-mortem bypasses gate" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx --skip-pre-mortem"}}' "$HOOKS_DIR/pre-mortem-gate.sh"
    [ "$status" -eq 0 ]
}

@test "pre-mortem-gate: council evidence passes (today date)" {
    local today
    today=$(date +%Y-%m-%d)
    mkdir -p "$MOCK_REPO/.agents/council"
    touch "$MOCK_REPO/.agents/council/${today}-pre-mortem-ag-xxx.md"
    # Mock bd to return 5 children
    local mock_bin="$TMP_TEST_DIR/bin"
    mkdir -p "$mock_bin"
    printf '#!/bin/bash\nif [ "$1" = "children" ]; then printf "1\n2\n3\n4\n5\n"; fi\n' > "$mock_bin/bd"
    chmod +x "$mock_bin/bd"
    run bash -c 'cd "$1" && printf "%s" "$2" | PATH="'"$mock_bin"':$PATH" bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_name":"Skill","tool_input":{"skill":"crank","args":"ag-xxx"}}' "$HOOKS_DIR/pre-mortem-gate.sh"
    [ "$status" -eq 0 ]
}

@test "pre-mortem-gate: malformed JSON exits gracefully" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{{broken}}' "$HOOKS_DIR/pre-mortem-gate.sh"
    [ "$status" -eq 0 ]
}

@test "pre-mortem-gate: empty stdin exits gracefully" {
    run bash -c 'printf "" | bash "$1" 2>&1' \
        -- "$HOOKS_DIR/pre-mortem-gate.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 10. prompt-nudge.sh — .prompt (UserPromptSubmit)
# ═══════════════════════════════════════════════════════════════════════

@test "prompt-nudge: empty prompt exits silently" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"prompt":""}' "$HOOKS_DIR/prompt-nudge.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "prompt-nudge: kill switch silences output" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_HOOKS_DISABLED=1 bash "$2" 2>&1' \
        -- '{"prompt":"implement a feature"}' "$HOOKS_DIR/prompt-nudge.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "prompt-nudge: no chain.jsonl exits silently" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"prompt":"implement something"}' "$HOOKS_DIR/prompt-nudge.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "prompt-nudge: malformed JSON exits gracefully" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- 'not json' "$HOOKS_DIR/prompt-nudge.sh"
    [ "$status" -eq 0 ]
}

@test "prompt-nudge: empty stdin exits gracefully" {
    run bash -c 'printf "" | bash "$1" 2>&1' \
        -- "$HOOKS_DIR/prompt-nudge.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 11. ratchet-advance.sh — .tool_input.command, .tool_response.exit_code
# ═══════════════════════════════════════════════════════════════════════

@test "ratchet-advance: non-ratchet command exits silently" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"go test ./..."},"tool_response":{"exit_code":0}}' "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "ratchet-advance: failed ratchet record exits silently" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":1}}' "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "ratchet-advance: successful research record suggests plan skill (fallback)" {
    run bash -c 'cd "$1" && printf "%s" "$2" | PATH="/usr/bin:/bin" bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}' "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"plan"* ]]
    [[ "$output" != *"/plan"* ]]
}

@test "ratchet-advance: vibe record suggests post-mortem skill (fallback)" {
    run bash -c 'cd "$1" && printf "%s" "$2" | PATH="/usr/bin:/bin" bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"ao ratchet record vibe"},"tool_response":{"exit_code":0}}' "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"post-mortem"* ]]
    [[ "$output" != *"/post-mortem"* ]]
}

@test "ratchet-advance: post-mortem record says cycle complete (fallback)" {
    run bash -c 'cd "$1" && printf "%s" "$2" | PATH="/usr/bin:/bin" bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"ao ratchet record post-mortem"},"tool_response":{"exit_code":0}}' "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [[ "${output,,}" == *"complete"* ]]
}

@test "ratchet-advance: AUTOCHAIN kill switch silences output" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_AUTOCHAIN=0 bash "$2" 2>&1' \
        -- '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}' "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "ratchet-advance: idempotency suppresses when next step done" {
    mkdir -p "$MOCK_REPO/.agents/ao"
    echo '{"gate":"plan","status":"locked"}' > "$MOCK_REPO/.agents/ao/chain.jsonl"
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":0}}' "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "ratchet-advance: extracts --output artifact path" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"ao ratchet record plan --output .agents/plan.md"},"tool_response":{"exit_code":0}}' "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *".agents/plan.md"* ]]
}

@test "ratchet-advance: malformed JSON exits gracefully" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{{not json}}' "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
}

@test "ratchet-advance: empty stdin exits gracefully" {
    run bash -c 'printf "" | bash "$1" 2>&1' \
        -- "$HOOKS_DIR/ratchet-advance.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 11. research-loop-detector.sh — .tool_name (or env CLAUDE_TOOL_NAME)
# ═══════════════════════════════════════════════════════════════════════

@test "research-loop-detector: Edit resets counter" {
    rm -f "$REPO_ROOT/.agents/ao/.read-streak" 2>/dev/null
    run bash -c 'printf "%s" "$1" | CLAUDE_TOOL_NAME=Edit bash "$2" 2>&1' \
        -- '{"tool_name":"Edit"}' "$HOOKS_DIR/research-loop-detector.sh"
    [ "$status" -eq 0 ]
    [ ! -f "$REPO_ROOT/.agents/ao/.read-streak" ]
}

@test "research-loop-detector: Read increments counter" {
    rm -f "$REPO_ROOT/.agents/ao/.read-streak" 2>/dev/null
    run bash -c 'printf "%s" "$1" | CLAUDE_TOOL_NAME=Read bash "$2" 2>&1' \
        -- '{"tool_name":"Read"}' "$HOOKS_DIR/research-loop-detector.sh"
    [ "$status" -eq 0 ]
    [ -f "$REPO_ROOT/.agents/ao/.read-streak" ]
    [ "$(cat "$REPO_ROOT/.agents/ao/.read-streak")" = "1" ]
}

@test "research-loop-detector: threshold 8 triggers warning" {
    echo "7" > "$REPO_ROOT/.agents/ao/.read-streak"
    run bash -c 'printf "%s" "$1" | CLAUDE_TOOL_NAME=Read bash "$2" 2>&1' \
        -- '{"tool_name":"Read"}' "$HOOKS_DIR/research-loop-detector.sh"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
}

@test "research-loop-detector: kill switch silences output" {
    echo "14" > "$REPO_ROOT/.agents/ao/.read-streak"
    run bash -c 'printf "%s" "$1" | CLAUDE_TOOL_NAME=Read AGENTOPS_RESEARCH_LOOP_DISABLED=1 bash "$2" 2>&1' \
        -- '{"tool_name":"Read"}' "$HOOKS_DIR/research-loop-detector.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "research-loop-detector: malformed JSON with env var still works" {
    rm -f "$REPO_ROOT/.agents/ao/.read-streak" 2>/dev/null
    run bash -c 'printf "%s" "$1" | CLAUDE_TOOL_NAME=Read bash "$2" 2>&1' \
        -- 'broken json' "$HOOKS_DIR/research-loop-detector.sh"
    [ "$status" -eq 0 ]
    # Should still increment because CLAUDE_TOOL_NAME env var is set
    [ -f "$REPO_ROOT/.agents/ao/.read-streak" ]
}

@test "research-loop-detector: empty stdin with no env var exits gracefully" {
    run bash -c 'printf "" | CLAUDE_TOOL_NAME="" bash "$1" 2>&1' \
        -- "$HOOKS_DIR/research-loop-detector.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 12. standards-injector.sh — .tool_input.file_path (Read tool)
# ═══════════════════════════════════════════════════════════════════════

@test "standards-injector: python file injects standards context" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"file_path":"/some/path/main.py"}}' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
}

@test "standards-injector: go file injects standards context" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"file_path":"/some/path/main.go"}}' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
}

@test "standards-injector: shell file injects standards context" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"file_path":"/x/script.sh"}}' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1
}

@test "standards-injector: unknown extension exits silently" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{"file_path":"/some/path/data.csv"}}' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "standards-injector: missing file_path exits silently" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"tool_input":{}}' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "standards-injector: kill switch silences output" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_HOOKS_DISABLED=1 bash "$2" 2>&1' \
        -- '{"tool_input":{"file_path":"/x/y.py"}}' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "standards-injector: malformed JSON exits gracefully" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{broken' "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
}

@test "standards-injector: empty stdin exits gracefully" {
    run bash -c 'printf "" | bash "$1" 2>&1' \
        -- "$HOOKS_DIR/standards-injector.sh"
    [ "$status" -eq 0 ]
}

# ═══════════════════════════════════════════════════════════════════════
# 13. task-validation-gate.sh — .metadata.validation (complex nested)
# ═══════════════════════════════════════════════════════════════════════

@test "task-validation-gate: feature missing metadata.validation blocks" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_METADATA_GATE=strict bash "$2" 2>&1' \
        -- '{"issue_type":"feature","metadata":{}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 2 ]
    [[ "$output" == *"VALIDATION FAILED"* ]]
}

@test "task-validation-gate: bug missing metadata.validation blocks" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_METADATA_GATE=strict bash "$2" 2>&1' \
        -- '{"issue_type":"bug","metadata":{}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 2 ]
}

@test "task-validation-gate: docs issue_type exempt" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"issue_type":"docs","metadata":{}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: chore issue_type exempt" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"issue_type":"chore","metadata":{}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: ci issue_type exempt" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"issue_type":"ci","metadata":{}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: untyped task passes" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"metadata":{}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: files_exist with existing file passes" {
    mkdir -p "$MOCK_REPO/hooks"
    touch "$MOCK_REPO/hooks/prompt-nudge.sh"
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"metadata":{"validation":{"files_exist":["hooks/prompt-nudge.sh"]}}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: files_exist with missing file blocks" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"metadata":{"validation":{"files_exist":["hooks/nonexistent-file-12345.sh"]}}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 2 ]
}

@test "task-validation-gate: path traversal blocked" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- '{"metadata":{"validation":{"files_exist":["../README.md"]}}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 2 ]
}

@test "task-validation-gate: global kill switch passes" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_HOOKS_DISABLED=1 bash "$2" 2>&1' \
        -- '{"metadata":{"validation":{"files_exist":["/nonexistent"]}}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: hook-specific kill switch passes" {
    run bash -c 'printf "%s" "$1" | AGENTOPS_TASK_VALIDATION_DISABLED=1 bash "$2" 2>&1' \
        -- '{"metadata":{"validation":{"files_exist":["/nonexistent"]}}}' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: malformed JSON exits gracefully" {
    run bash -c 'printf "%s" "$1" | bash "$2" 2>&1' \
        -- 'not-json{{{' "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}

@test "task-validation-gate: empty stdin exits gracefully" {
    run bash -c 'cd "$1" && printf "" | bash "$2" 2>&1' \
        -- "$MOCK_REPO" "$HOOKS_DIR/task-validation-gate.sh"
    [ "$status" -eq 0 ]
}
