#!/usr/bin/env bats
#
# Tests for hooks/check-test-pair-on-commit.sh — verifies the warn-on-
# code-without-test behavior (docs/contracts/update-principles.md
# Principle 2).
#
# Sibling pattern: matches tests/hooks/test-update-principles-check.bats
# (cycle 54 commit ecb3b3ba) — stub a temp git repo, stage files via
# git add, invoke the hook with a synthesized PreToolUse JSON input,
# assert exit 0 + presence/absence of WARN additionalContext.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    HOOK="$REPO_ROOT/hooks/check-test-pair-on-commit.sh"
    [ -x "$HOOK" ] || skip "hook not executable"

    # Build a temp git repo so `git diff --cached --name-only` works
    TMP_REPO="$(mktemp -d)"
    cd "$TMP_REPO"
    git init -q
    git config user.email "test@example.com"
    git config user.name "Test"
    mkdir -p hooks scripts lib cli/embedded/hooks cli/cmd/ao
}

teardown() {
    cd /
    rm -rf "$TMP_REPO"
}

# Helper: stage the given files (touched empty) then invoke the hook
# with a synthesized Bash tool input for `git commit -m`.
stage_and_run() {
    for path in "$@"; do
        mkdir -p "$(dirname "$path")"
        : > "$path"
        git add "$path"
    done
    local input
    input=$(jq -nc '{tool_name: "Bash", tool_input: {command: "git commit -m \"chore: test\""}}')
    echo "$input" | bash "$HOOK"
}

@test "staged .go without _test.go → WARN" {
    run stage_and_run "cli/cmd/ao/foo.go"
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
    [[ "$output" == *"*.go change without paired *_test.go"* ]]
}

@test "staged .go with paired _test.go → no warning" {
    run stage_and_run "cli/cmd/ao/foo.go" "cli/cmd/ao/foo_test.go"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "staged hooks/foo.sh without .bats → WARN" {
    run stage_and_run "hooks/foo.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
    [[ "$output" == *"hooks/scripts/lib without paired *.bats"* ]]
}

@test "staged hooks/foo.sh with paired .bats → no warning" {
    run stage_and_run "hooks/foo.sh" "tests/hooks/test-foo.bats"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "staged generated cli/embedded/ shell → no warning (exempt)" {
    run stage_and_run "cli/embedded/hooks/some-generated.sh"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "staged testdata fixture .go → no warning (exempt)" {
    run stage_and_run "cli/cmd/ao/testdata/fixture.go"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "docs-only commit → no warning" {
    run stage_and_run "docs/foo.md" "README.md"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "non-commit Bash command (ls) → silent" {
    : > "cli/cmd/ao/foo.go"
    git add "cli/cmd/ao/foo.go"
    local input
    input=$(jq -nc '{tool_name: "Bash", tool_input: {command: "ls -la"}}')
    run bash -c "echo '$input' | bash $HOOK"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "AGENTOPS_HOOKS_DISABLED=1 short-circuits" {
    : > "cli/cmd/ao/foo.go"
    git add "cli/cmd/ao/foo.go"
    local input
    input=$(jq -nc '{tool_name: "Bash", tool_input: {command: "git commit -m \"x\""}}')
    AGENTOPS_HOOKS_DISABLED=1 run bash -c "echo '$input' | bash $HOOK"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}

@test "AGENTOPS_TEST_PAIR_CHECK_DISABLED=1 short-circuits" {
    : > "cli/cmd/ao/foo.go"
    git add "cli/cmd/ao/foo.go"
    local input
    input=$(jq -nc '{tool_name: "Bash", tool_input: {command: "git commit -m \"x\""}}')
    AGENTOPS_TEST_PAIR_CHECK_DISABLED=1 run bash -c "echo '$input' | bash $HOOK"
    [ "$status" -eq 0 ]
    [[ "$output" != *"WARN"* ]]
}
