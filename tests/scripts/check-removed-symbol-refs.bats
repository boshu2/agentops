#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-removed-symbol-refs.sh"

    TMP_DIR="$(mktemp -d)"
    WORK_REPO="$TMP_DIR/repo"

    git init -b main "$WORK_REPO" >/dev/null
    git -C "$WORK_REPO" config user.name "Test User"
    git -C "$WORK_REPO" config user.email "test@example.com"
    mkdir -p "$WORK_REPO/scripts" "$WORK_REPO/docs/releases" "$WORK_REPO/docs" "$WORK_REPO/skills/example" "$WORK_REPO/tests"
    cp "$SCRIPT" "$WORK_REPO/scripts/check-removed-symbol-refs.sh"
    chmod +x "$WORK_REPO/scripts/check-removed-symbol-refs.sh"
}

teardown() {
    rm -rf "$TMP_DIR"
}

commit_fixture() {
    git -C "$WORK_REPO" add .
    git -C "$WORK_REPO" commit -m "fixture" >/dev/null
}

@test "check-removed-symbol-refs fails on shell doc skill and test callsites" {
    printf 'ao defrag --old-flag\n' > "$WORK_REPO/scripts/run.sh"
    printf 'Use --old-flag here.\n' > "$WORK_REPO/docs/usage.md"
    printf 'Skill calls --old-flag.\n' > "$WORK_REPO/skills/example/SKILL.md"
    printf 'assert --old-flag gone\n' > "$WORK_REPO/tests/old.bats"
    printf 'historical --old-flag is allowed\n' > "$WORK_REPO/CHANGELOG.md"
    printf 'historical --old-flag is allowed\n' > "$WORK_REPO/docs/releases/2026-05-16-v1.0.0-notes.md"
    commit_fixture

    run bash -c "cd '$WORK_REPO' && bash scripts/check-removed-symbol-refs.sh -- --old-flag"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL removed-symbol-refs"* ]]
    [[ "$output" == *"scripts/run.sh"* ]]
    [[ "$output" == *"docs/usage.md"* ]]
    [[ "$output" == *"skills/example/SKILL.md"* ]]
    [[ "$output" == *"tests/old.bats"* ]]
    [[ "$output" != *"CHANGELOG.md"* ]]
    [[ "$output" != *"docs/releases/2026-05-16-v1.0.0-notes.md"* ]]
}

@test "check-removed-symbol-refs passes when only historical release references remain" {
    printf 'historical --old-flag is allowed\n' > "$WORK_REPO/CHANGELOG.md"
    printf 'historical --old-flag is allowed\n' > "$WORK_REPO/docs/releases/2026-05-16-v1.0.0-notes.md"
    printf 'no old flag here\n' > "$WORK_REPO/docs/usage.md"
    commit_fixture

    run bash -c "cd '$WORK_REPO' && bash scripts/check-removed-symbol-refs.sh -- --old-flag"
    [ "$status" -eq 0 ]
    [[ "$output" == *"OK removed-symbol-refs"* ]]
}

@test "check-removed-symbol-refs supports regex mode and extra excludes" {
    printf 'ao old-command --flag\n' > "$WORK_REPO/scripts/run.sh"
    printf 'fixture old-command --flag\n' > "$WORK_REPO/tests/fixture.txt"
    commit_fixture

    run bash -c "cd '$WORK_REPO' && bash scripts/check-removed-symbol-refs.sh --regex --exclude 'tests/**' -- 'old-command\\s+--flag'"
    [ "$status" -eq 1 ]
    [[ "$output" == *"scripts/run.sh"* ]]
    [[ "$output" != *"tests/fixture.txt"* ]]
}

@test "check-removed-symbol-refs requires -- before flag-like symbols" {
    commit_fixture

    run bash -c "cd '$WORK_REPO' && bash scripts/check-removed-symbol-refs.sh --old-flag"
    [ "$status" -eq 1 ]
    [[ "$output" == *"flag-like symbol before --"* ]]
}

@test "check-removed-symbol-refs reports invalid regex as an error" {
    printf 'content\n' > "$WORK_REPO/docs/usage.md"
    commit_fixture

    run bash -c "cd '$WORK_REPO' && bash scripts/check-removed-symbol-refs.sh --regex -- '['"
    [ "$status" -eq 1 ]
    [[ "$output" == *"git grep failed for removed symbol"* ]]
}
