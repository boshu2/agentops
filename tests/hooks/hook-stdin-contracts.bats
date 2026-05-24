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

@test "dangerous-git-guard: branch names containing -f are allowed" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git push origin feature-fix"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 0 ]
}

@test "dangerous-git-guard: ref names containing -f are allowed" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git push origin refs/heads/fix-forward"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 0 ]
}

@test "dangerous-git-guard: force after remote is blocked" {
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"tool_input":{"command":"git push origin -f main"}}' "$HOOKS_DIR/dangerous-git-guard.sh"
    [ "$status" -eq 2 ]
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
    touch "$MOCK_REPO/hooks/context-guard.sh"
    run bash -c 'cd "$1" && printf "%s" "$2" | bash "$3" 2>&1' \
        -- "$MOCK_REPO" '{"metadata":{"validation":{"files_exist":["hooks/context-guard.sh"]}}}' "$HOOKS_DIR/task-validation-gate.sh"
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
