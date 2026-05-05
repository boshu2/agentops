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
    # Force commit signing off in this fake repo so tests that need a
    # commit (e.g. "allows staged deletion") aren't blocked by host-level
    # signing config inherited via /etc/gitconfig.
    git config commit.gpgsign false
    git config tag.gpgsign false
    git config gpg.format openpgp
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
    [[ "$output" == *"no disallowed tracked repo-root .agents state"* ]]
}

@test "fails when repo-root .agents is tracked outside the audit-truth allowlist" {
    write_ignore
    mkdir -p .agents/rpi
    printf '{}\n' > .agents/rpi/execution-packet.json
    git add -f .agents/rpi/execution-packet.json

    run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"tracked outside the audit-truth allowlist"* ]]
    [[ "$output" == *".agents/rpi/execution-packet.json"* ]]
}

@test "permits tracked allowlisted audit-truth files (nightly snapshots)" {
    {
        printf '/.agents/*\n'
        printf '/.agents/**/*\n'
        printf '!/.agents/\n'
        printf '!/.agents/nightly/\n'
        printf '!/.agents/nightly/**\n'
    } > .gitignore
    mkdir -p .agents/nightly/2026-05-05
    printf '{}\n' > .agents/nightly/2026-05-05/baseline-goals.json
    git add .agents/nightly/2026-05-05/baseline-goals.json

    run "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"no disallowed tracked repo-root .agents state"* ]]
}

@test "permits tracked allowlisted audit-truth files (rpi/next-work, evolve, goals attempts, findings registry)" {
    {
        printf '/.agents/*\n'
        printf '/.agents/**/*\n'
        printf '!/.agents/\n'
        printf '!/.agents/rpi/\n'
        printf '!/.agents/rpi/next-work.jsonl\n'
        printf '!/.agents/evolve/\n'
        printf '!/.agents/evolve/cycle-history.jsonl\n'
        printf '!/.agents/evolve/session-state.json\n'
        printf '!/.agents/goals/\n'
        printf '!/.agents/goals/**/\n'
        printf '!/.agents/goals/**/attempts.jsonl\n'
        printf '!/.agents/findings/\n'
        printf '!/.agents/findings/registry.jsonl\n'
    } > .gitignore
    mkdir -p .agents/rpi .agents/evolve .agents/goals/g-one .agents/findings
    printf '{}\n'      > .agents/rpi/next-work.jsonl
    printf '{}\n'      > .agents/evolve/cycle-history.jsonl
    printf '{}\n'      > .agents/evolve/session-state.json
    printf '{}\n'      > .agents/goals/g-one/attempts.jsonl
    printf '{}\n'      > .agents/findings/registry.jsonl
    git add .agents/rpi/next-work.jsonl .agents/evolve/cycle-history.jsonl .agents/evolve/session-state.json .agents/goals/g-one/attempts.jsonl .agents/findings/registry.jsonl

    run "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"no disallowed tracked repo-root .agents state"* ]]
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

@test "fails when root .gitignore re-includes .agents paths outside the allowlist" {
    {
        printf '/.agents/\n'
        printf '!.agents/learnings/\n'
    } > .gitignore

    run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"outside the audit-truth allowlist"* ]]
    [[ "$output" == *"!.agents/learnings/"* ]]
}

@test "permits nested .agents test fixtures outside repo root .agents" {
    write_ignore
    mkdir -p cli/cmd/ao/testdata/example/.agents
    printf '{}\n' > cli/cmd/ao/testdata/example/.agents/fixture.json
    git add cli/cmd/ao/testdata/example/.agents/fixture.json

    run "$SCRIPT"
    [ "$status" -eq 0 ]
}
