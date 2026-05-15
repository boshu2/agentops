#!/usr/bin/env bats
#
# Tests for hooks/update-principles-check.sh — verifies the fitness-delta
# warning behavior (docs/contracts/update-principles.md Principle 4).
#
# Sibling pattern: matches the test-shape of tests/scripts/check-three-gap-supergate.bats
# (operator exemplar commit 1b9d139c → f62295f7) — stub inputs as JSON via TOOL_INPUT,
# assert exit 0 + presence/absence of additionalContext WARN.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    HOOK="$REPO_ROOT/hooks/update-principles-check.sh"
    [ -x "$HOOK" ] || skip "hook not executable"
}

# Helper: invoke the hook with a synthesized tool-call JSON on stdin.
run_hook_with_command() {
    local command_str="$1"
    local input
    input=$(jq -nc --arg cmd "$command_str" \
        '{tool_name: "Bash", tool_input: {command: $cmd}}')
    echo "$input" | bash "$HOOK"
}

@test "fitness-delta in heredoc commit message → no warning" {
    cmd='git commit -m "$(cat <<EOF
chore(test): example commit

Code-driven fitness: 39 → 40 contracts catalogued.
EOF
)"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "fitness-delta as N/X → M/X pattern → no warning" {
    cmd='git commit -m "$(cat <<EOF
fix(gates): example

Code-driven fitness: 134/139 → 139/139.
EOF
)"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "missing fitness delta in heredoc → emits WARN additionalContext" {
    cmd='git commit -m "$(cat <<EOF
docs: update README with new section about the feature

Some longer prose describing the change with no numerical pair anywhere.
EOF
)"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
    [[ "$output" == *"Principle 4"* ]]
    [[ "$output" == *"fitness-delta"* ]]
}

@test "non-commit Bash command (ls) → silent, no parse" {
    cmd='ls -la docs/'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "AGENTOPS_HOOKS_DISABLED=1 short-circuits before any parse" {
    cmd='git commit -m "no delta here"'
    AGENTOPS_HOOKS_DISABLED=1 run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "AGENTOPS_UPDATE_PRINCIPLES_DISABLED=1 short-circuits before parse" {
    cmd='git commit -m "no delta here"'
    AGENTOPS_UPDATE_PRINCIPLES_DISABLED=1 run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}
