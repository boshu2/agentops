#!/usr/bin/env bats
#
# Tests for hooks/check-sibling-citation-on-commit.sh — verifies
# update-principles.md Principle 3 (sibling-pattern citation).
#
# Sibling pattern: matches tests/hooks/test-update-principles-check.bats
# (cycle 54, P4) and tests/hooks/test-check-test-pair-on-commit.bats
# (cycle 55, P2) — same JSON-stdin input shape, same exit-0 + WARN
# assertion model.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    HOOK="$REPO_ROOT/hooks/check-sibling-citation-on-commit.sh"
    [ -x "$HOOK" ] || skip "hook not executable"
}

# Helper
run_hook_with_command() {
    local command_str="$1"
    local input
    input=$(jq -nc --arg cmd "$command_str" \
        '{tool_name: "Bash", tool_input: {command: $cmd}}')
    echo "$input" | bash "$HOOK"
}

@test "explicit 'matching the X pattern' → no warning" {
    cmd='git commit -m "$(cat <<EOF
fix(thing): improve behavior of foo

The change matches the Gap 1 council-coverage SKIP pattern so the
greenfield case is handled consistently. Code-driven fitness: 1 → 2.
EOF
)"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "'follows the X shape' → no warning" {
    cmd='git commit -m "$(cat <<EOF
docs(plans): track new plan

The plan follows the docs/plans/2026-05-11-evolution-roadmap.md shape,
keeping plan tracked alongside bd state.
EOF
)"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "'Sibling pattern:' → no warning" {
    cmd='git commit -m "$(cat <<EOF
feat(hooks): new hook for X

Sibling pattern: hooks/update-principles-check.sh — same PreToolUse:Bash
matcher shape with the same kill-switch layering.
EOF
)"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "'shape from commit <sha>' → no warning" {
    cmd='git commit -m "$(cat <<EOF
feat(scripts): refactor X

Bats coverage shape from commit f62295f7: 4 tests covering all paths.
Fitness: 0 → 4 tests.
EOF
)"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "longer body with no sibling citation → emits WARN" {
    cmd='git commit -m "$(cat <<EOF
feat(thing): add new feature

This commit introduces a completely new capability for the codebase.
It does not relate to any prior work in obvious ways and does not
reference any existing pattern in the repository as inspiration.
Length is well over the trivial threshold so the check fires.
EOF
)"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
    [[ "$output" == *"Principle 3"* ]]
}

@test "trivial body (< 80 chars) → silent exemption" {
    cmd='git commit -m "chore: bump dep version"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "[no-sibling] tag in body → silent exemption" {
    cmd='git commit -m "$(cat <<EOF
feat(thing): first-of-kind capability

[no-sibling] No precedent exists in repo because this introduces a
genuinely new domain concept (bounded context X with no prior peers).
EOF
)"'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "non-commit Bash command → silent" {
    cmd='ls -la docs/'
    run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "AGENTOPS_HOOKS_DISABLED=1 short-circuits" {
    cmd='git commit -m "$(cat <<EOF
feat(thing): long body without citation that would otherwise warn but kill switch is set so silent.
EOF
)"'
    AGENTOPS_HOOKS_DISABLED=1 run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "AGENTOPS_SIBLING_CITATION_DISABLED=1 short-circuits" {
    cmd='git commit -m "$(cat <<EOF
feat(thing): long body without citation that would otherwise warn but per-hook kill switch silences.
EOF
)"'
    AGENTOPS_SIBLING_CITATION_DISABLED=1 run run_hook_with_command "$cmd"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}
