#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-no-tracked-agents.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO"
    cd "$FAKE_REPO"
    git init -q
    git config user.email test@example.com
    git config user.name "Test User"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_ignore() {
    printf '/.agents/\n' > "$FAKE_REPO/.gitignore"
}

@test "check-no-tracked-agents.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "passes when repo-root .agents is ignored and untracked" {
    write_ignore
    mkdir -p .agents/ao
    printf 'local\n' > .agents/ao/state.json

    run "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"no tracked repo-root .agents state"* ]]
}

@test "fails when repo-root .agents is tracked" {
    write_ignore
    mkdir -p .agents/rpi
    printf '{}\n' > .agents/rpi/execution-packet.json
    git add -f .agents/rpi/execution-packet.json

    run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"repo-root .agents paths are tracked"* ]]
    [[ "$output" == *".agents/rpi/execution-packet.json"* ]]
}

@test "allows staged deletion while removing .agents from the index" {
    write_ignore
    mkdir -p .agents/learnings
    printf 'secret\n' > .agents/learnings/item.md
    git add -f .agents/learnings/item.md
    git commit -qm "track legacy agents state"
    git rm -q --cached .agents/learnings/item.md

    run "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "fails when root .gitignore omits /.agents/" {
    printf '*.log\n' > .gitignore

    run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"must contain an explicit '/.agents/'"* ]]
}

@test "fails when root .gitignore re-includes .agents paths" {
    {
        printf '/.agents/\n'
        printf '!.agents/rpi/\n'
    } > .gitignore

    run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"re-includes repo-root .agents paths"* ]]
}

@test "permits nested .agents test fixtures outside repo root .agents" {
    write_ignore
    mkdir -p cli/cmd/ao/testdata/example/.agents
    printf '{}\n' > cli/cmd/ao/testdata/example/.agents/fixture.json
    git add cli/cmd/ao/testdata/example/.agents/fixture.json

    run "$SCRIPT"
    [ "$status" -eq 0 ]
}
