#!/usr/bin/env bats
#
# Behavioral spec for scripts/check-test-count-regression.sh — the test-count
# non-regression ratchet for cli/internal/ packages (ag-h2z).
#
# Each test builds a throwaway git repo, commits a baseline of _test.go files
# under cli/internal/<pkg>/, captures the baseline SHA, mutates + commits, then
# runs the gate with BASE_REF=<baseline-sha>. The script operates purely on the
# current working directory's git state, so it is invoked by absolute path from
# inside the throwaway repo (no copy into the fixture required).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-test-count-regression.sh"

    TMP_DIR="$(mktemp -d)"
    WORK_REPO="$TMP_DIR/repo"

    git init -b main "$WORK_REPO" >/dev/null
    git -C "$WORK_REPO" config user.name "Test User"
    git -C "$WORK_REPO" config user.email "test@example.com"
    mkdir -p "$WORK_REPO/cli/internal/foo"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# write_test_file <repo-relative-path> <TestFn>...
write_test_file() {
    local path="$1"
    shift
    mkdir -p "$WORK_REPO/$(dirname "$path")"
    {
        echo "package foo"
        echo ""
        echo 'import "testing"'
        echo ""
        local fn
        for fn in "$@"; do
            printf 'func %s(t *testing.T) {}\n' "$fn"
        done
    } >"$WORK_REPO/$path"
}

commit_all() {
    git -C "$WORK_REPO" add -A
    git -C "$WORK_REPO" commit -q -m "$1"
}

run_gate() {
    # $1 = BASE_REF ; remaining = extra "VAR=val" env assignments
    local base="$1"
    shift
    run bash -c "cd '$WORK_REPO' && $* BASE_REF='$base' bash '$SCRIPT'"
}

@test "adds tests: count increases, gate passes" {
    write_test_file cli/internal/foo/foo_test.go TestA TestB
    commit_all "baseline"
    base="$(git -C "$WORK_REPO" rev-parse HEAD)"
    write_test_file cli/internal/foo/foo_test.go TestA TestB TestC
    commit_all "add TestC"

    run_gate "$base"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"cli/internal/foo"* ]]
}

@test "removes a test without trailer: gate fails naming the test and pkg" {
    write_test_file cli/internal/foo/foo_test.go TestA TestB TestC
    commit_all "baseline"
    base="$(git -C "$WORK_REPO" rev-parse HEAD)"
    write_test_file cli/internal/foo/foo_test.go TestA TestB
    commit_all "remove TestC"

    run_gate "$base"
    [ "$status" -eq 1 ]
    [[ "$output" == *"cli/internal/foo"* ]]
    [[ "$output" == *"TestC"* ]]
    [[ "$output" == *"3 -> 2"* ]]
}

@test "removes a test WITH Test-Removal-Reason trailer: gate passes" {
    write_test_file cli/internal/foo/foo_test.go TestA TestB TestC
    commit_all "baseline"
    base="$(git -C "$WORK_REPO" rev-parse HEAD)"
    write_test_file cli/internal/foo/foo_test.go TestA TestB
    git -C "$WORK_REPO" add -A
    git -C "$WORK_REPO" commit -q -m "remove TestC

Test-Removal-Reason: deduplication after refactor"

    run_gate "$base"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Test-Removal-Reason"* ]]
}

@test "renames a test (Test_X -> TestX): net count stable, gate passes" {
    write_test_file cli/internal/foo/foo_test.go Test_X TestY
    commit_all "baseline"
    base="$(git -C "$WORK_REPO" rev-parse HEAD)"
    write_test_file cli/internal/foo/foo_test.go TestX TestY
    commit_all "rename Test_X to TestX"

    run_gate "$base"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "AGENTOPS_TEST_COUNT_NOREGRESS=skip bypasses the gate" {
    write_test_file cli/internal/foo/foo_test.go TestA TestB TestC
    commit_all "baseline"
    base="$(git -C "$WORK_REPO" rev-parse HEAD)"
    write_test_file cli/internal/foo/foo_test.go TestA
    commit_all "remove two tests"

    run_gate "$base" "AGENTOPS_TEST_COUNT_NOREGRESS=skip"
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP"* ]]
}

@test "no cli/internal test changes: gate is a no-op pass" {
    mkdir -p "$WORK_REPO/docs"
    echo "doc" >"$WORK_REPO/docs/a.md"
    commit_all "baseline"
    base="$(git -C "$WORK_REPO" rev-parse HEAD)"
    echo "more" >>"$WORK_REPO/docs/a.md"
    commit_all "doc-only change"

    run_gate "$base"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "unresolvable BASE_REF: fail-open SKIP" {
    write_test_file cli/internal/foo/foo_test.go TestA
    commit_all "baseline"

    run_gate "origin/this-ref-does-not-exist"
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP"* ]]
}

@test "test moved between files in same pkg: net count stable, gate passes" {
    write_test_file cli/internal/foo/foo_test.go TestA TestB
    commit_all "baseline"
    base="$(git -C "$WORK_REPO" rev-parse HEAD)"
    write_test_file cli/internal/foo/foo_test.go TestA
    write_test_file cli/internal/foo/bar_test.go TestB
    commit_all "move TestB to bar_test.go"

    run_gate "$base"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}
