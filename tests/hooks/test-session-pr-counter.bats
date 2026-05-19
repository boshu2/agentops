#!/usr/bin/env bats
#
# Tests for hooks/session-pr-counter.sh — verifies the session-scope post-mortem
# reminder (soc-1aou, mechanical enforcement of the soc-waxr rule).
#
# Sibling pattern: tests/hooks/test-check-test-pair-on-commit.bats — synthesize
# PreToolUse JSON input, mock `gh` via PATH stub, assert exit code + emitted
# additionalContext.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    HOOK="$REPO_ROOT/hooks/session-pr-counter.sh"
    [ -x "$HOOK" ] || skip "hook not executable"

    TMP_DIR="$(mktemp -d)"
    MOCK_BIN="$TMP_DIR/bin"
    mkdir -p "$MOCK_BIN"
    # Always ensure our mock bin is in front of real PATH.
    export ORIG_PATH="$PATH"
    export PATH="$MOCK_BIN:$PATH"
}

teardown() {
    export PATH="${ORIG_PATH:-$PATH}"
    if [ -n "${TMP_DIR:-}" ] && [ -d "$TMP_DIR" ]; then
        rm -rf "$TMP_DIR"
    fi
}

# Stub `gh` to return a JSON array of length N (zero or more PRs).
mock_gh_pr_count() {
    local n="$1"
    cat >"$MOCK_BIN/gh" <<EOF
#!/bin/bash
# Emit JSON array of N empty objects; mimics gh pr list --json output.
python3 -c "import json,sys; print(json.dumps([{} for _ in range($n)]))"
EOF
    chmod +x "$MOCK_BIN/gh"
}

# Synthesize the PreToolUse JSON the hook receives on stdin.
input_json() {
    local cmd="${1:-gh pr create -t foo -b bar}"
    jq -nc --arg c "$cmd" '{tool_name: "Bash", tool_input: {command: $c}}'
}

# ---- early-exit / no-fire cases (exit 0, no context emitted) ----

@test "no-op when AGENTOPS_HOOKS_DISABLED=1" {
    mock_gh_pr_count 10
    run env AGENTOPS_HOOKS_DISABLED=1 bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "no-op when AGENTOPS_SESSION_PR_COUNTER_DISABLED=1" {
    mock_gh_pr_count 10
    run env AGENTOPS_SESSION_PR_COUNTER_DISABLED=1 bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "no-op when tool is not Bash" {
    mock_gh_pr_count 10
    local non_bash
    non_bash=$(jq -nc '{tool_name: "Edit", tool_input: {file_path: "/tmp/x"}}')
    run bash -c "echo '$non_bash' | $HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "no-op when Bash command is not gh pr create" {
    mock_gh_pr_count 10
    run bash -c "echo '$(input_json "ls -la")' | $HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "fails open when gh returns malformed output (non-numeric count)" {
    # Mock gh to return garbage; hook should fail open (exit 0, no context).
    # This exercises the case-statement number-validation guard.
    cat >"$MOCK_BIN/gh" <<'EOF'
#!/bin/bash
echo "not-json garbage"
EOF
    chmod +x "$MOCK_BIN/gh"
    run bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ---- under-threshold cases (exit 0, no context) ----

@test "under threshold: 0 PRs in window → silent" {
    mock_gh_pr_count 0
    run bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "under threshold: 3 PRs in window → silent (next would be #4, threshold 5)" {
    mock_gh_pr_count 3
    run bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

# ---- at-or-over threshold cases (exit 0, advisory context emitted) ----

@test "at threshold: 4 PRs → emits notice (next would be #5)" {
    mock_gh_pr_count 4
    run bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 0 ]
    [[ "$output" == *"SESSION-SCOPE NOTICE"* ]]
    [[ "$output" == *"soc-waxr"* ]]
    [[ "$output" == *"post-mortem"* ]]
}

@test "over threshold: 7 PRs → emits notice with the count" {
    mock_gh_pr_count 7
    run bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 0 ]
    [[ "$output" == *"7 PR"* ]]
    [[ "$output" == *"#8"* ]]
}

@test "custom threshold via env: AGENTOPS_SESSION_PR_THRESHOLD=3 fires at 2 PRs" {
    mock_gh_pr_count 2
    run env AGENTOPS_SESSION_PR_THRESHOLD=3 bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 0 ]
    [[ "$output" == *"SESSION-SCOPE NOTICE"* ]]
    [[ "$output" == *"#3"* ]]
}

# ---- hard-block mode (exit 2 with clear reason) ----

@test "hard-block mode: 5 PRs + AGENTOPS_SESSION_PR_BLOCK=1 → exit 2" {
    mock_gh_pr_count 5
    run env AGENTOPS_SESSION_PR_BLOCK=1 bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 2 ]
    [[ "$output" == *"BLOCKED"* ]]
    [[ "$output" == *"post-mortem"* ]]
}

@test "hard-block mode under threshold: 3 PRs + block=1 → exit 0 (no fire)" {
    mock_gh_pr_count 3
    run env AGENTOPS_SESSION_PR_BLOCK=1 bash -c "echo '$(input_json)' | $HOOK"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}
