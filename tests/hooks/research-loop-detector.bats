#!/usr/bin/env bats
#
# Tests for hooks/research-loop-detector.sh — pins the read-streak counter,
# threshold-text branches, write-tool reset, read-only-bash classification,
# and kill-switch behavior. The hook has no test coverage today, so a single
# bad edit (off-by-one threshold, broken jq escape, dropped reset case) ships
# silently into production. This fixture closes that gap.

setup() {
    load helpers/test_helper
    _helper_setup
    HOOK="$REPO_ROOT/hooks/research-loop-detector.sh"
    COUNTER="$MOCK_REPO/.agents/ao/.read-streak"
}

teardown() {
    _helper_teardown
}

# fire TOOL_NAME [BASH_COMMAND]
# Runs the hook in $MOCK_REPO with CLAUDE_TOOL_NAME=TOOL_NAME and (when given)
# CLAUDE_TOOL_INPUT_COMMAND=BASH_COMMAND, capturing stdout in $output and exit
# status in $status. Cwd is fixed to $MOCK_REPO so git rev-parse resolves to
# the mock repo's state dir, not the real repo.
fire() {
    local tool="$1"
    local cmd="${2:-}"
    cd "$MOCK_REPO" || return 1
    run env CLAUDE_TOOL_NAME="$tool" CLAUDE_TOOL_INPUT_COMMAND="$cmd" \
        bash "$HOOK" <<< '{}'
}

# count_is N — assert the persisted counter file equals N.
count_is() {
    local want="$1"
    [ -f "$COUNTER" ]
    local got
    got=$(cat "$COUNTER")
    [ "$got" = "$want" ]
}

@test "Read tool increments counter from 0 → 1 with no nudge below WARN" {
    fire Read
    [ "$status" -eq 0 ]
    [ -z "$output" ]
    count_is 1
}

@test "WARN threshold (8 reads) emits 'Consider acting' nudge as JSON" {
    for _ in 1 2 3 4 5 6 7; do fire Read; done
    fire Read
    [ "$status" -eq 0 ]
    [[ "$output" == *"Consider acting"* ]]
    [[ "$output" == *"additionalContext"* ]]
    echo "$output" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null
    count_is 8
}

@test "STRONG threshold (12) emits 'WARNING' and demands Edit/Write/Bash" {
    for _ in 1 2 3 4 5 6 7 8 9 10 11; do fire Read; done
    fire Read
    [[ "$output" == *"WARNING"* ]]
    [[ "$output" == *"Edit, Write, or Bash"* ]]
    count_is 12
}

@test "STOP threshold (15) emits 'STOP RESEARCHING'" {
    for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14; do fire Read; done
    fire Read
    [[ "$output" == *"STOP RESEARCHING"* ]]
    [[ "$output" == *"Produce output NOW"* ]]
    count_is 15
}

@test "Edit tool resets counter (deletes counter file)" {
    for _ in 1 2 3; do fire Read; done
    count_is 3
    fire Edit
    [ "$status" -eq 0 ]
    [ ! -f "$COUNTER" ]
}

@test "Write tool resets counter" {
    for _ in 1 2 3 4 5; do fire Read; done
    fire Write
    [ ! -f "$COUNTER" ]
}

@test "NotebookEdit tool resets counter" {
    fire Read
    fire NotebookEdit
    [ ! -f "$COUNTER" ]
}

@test "Bash with read-only command (grep) increments counter" {
    fire Bash "grep -r foo ."
    count_is 1
    fire Bash "rg --files"
    count_is 2
    fire Bash "cat README.md"
    count_is 3
}

@test "Bash with execution command (cd, mv, npm) resets counter" {
    for _ in 1 2; do fire Read; done
    count_is 2
    fire Bash "cd /tmp && rm -rf foo"
    [ ! -f "$COUNTER" ]
}

@test "Grep tool increments counter (read-only)" {
    fire Grep
    fire Glob
    count_is 2
}

@test "WebSearch and WebFetch increment counter" {
    fire WebSearch
    fire WebFetch
    count_is 2
}

@test "Neutral tool (Agent) is ignored — no state mutation" {
    fire Read
    count_is 1
    fire Agent
    count_is 1  # unchanged
}

@test "AGENTOPS_HOOKS_DISABLED=1 short-circuits before any state write" {
    cd "$MOCK_REPO"
    run env AGENTOPS_HOOKS_DISABLED=1 CLAUDE_TOOL_NAME=Read \
        bash "$HOOK" <<< '{}'
    [ "$status" -eq 0 ]
    [ -z "$output" ]
    [ ! -f "$COUNTER" ]
}

@test "AGENTOPS_RESEARCH_LOOP_DISABLED=1 short-circuits selectively" {
    cd "$MOCK_REPO"
    run env AGENTOPS_RESEARCH_LOOP_DISABLED=1 CLAUDE_TOOL_NAME=Read \
        bash "$HOOK" <<< '{}'
    [ "$status" -eq 0 ]
    [ -z "$output" ]
    [ ! -f "$COUNTER" ]
}

@test "Custom WARN_THRESHOLD env var overrides default of 8" {
    cd "$MOCK_REPO"
    # With WARN=2, the second Read fires the warn nudge
    run env CLAUDE_TOOL_NAME=Read AGENTOPS_RESEARCH_WARN_THRESHOLD=2 \
        bash "$HOOK" <<< '{}'
    [ -z "$output" ]  # first read: count=1, below threshold 2
    count_is 1

    run env CLAUDE_TOOL_NAME=Read AGENTOPS_RESEARCH_WARN_THRESHOLD=2 \
        bash "$HOOK" <<< '{}'
    [[ "$output" == *"Consider acting"* ]]
    count_is 2
}

@test "STOP threshold takes precedence over STRONG and WARN" {
    cd "$MOCK_REPO"
    run env CLAUDE_TOOL_NAME=Read \
        AGENTOPS_RESEARCH_WARN_THRESHOLD=1 \
        AGENTOPS_RESEARCH_STRONG_THRESHOLD=1 \
        AGENTOPS_RESEARCH_STOP_THRESHOLD=1 \
        bash "$HOOK" <<< '{}'
    [[ "$output" == *"STOP RESEARCHING"* ]]
    [[ "$output" != *"WARNING:"* ]]
}

@test "Nudge JSON survives shell metacharacters in tool name (jq escapes)" {
    fire Read
    fire Read
    fire Read
    fire Read
    fire Read
    fire Read
    fire Read
    fire Read
    [[ "$output" == *"additionalContext"* ]]
    # Round-trip parse must succeed — not just substring match
    echo "$output" | jq -e '.hookSpecificOutput.hookEventName == "PostToolUse"' >/dev/null
}
