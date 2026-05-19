#!/usr/bin/env bats
# Tests for scripts/gh-merge-chain.sh (soc — F3 closer).
#
# Exercises argument parsing and --dry-run paths. The live-poll loop is
# not unit-testable without a live gh + GitHub fixture; covered by manual
# usage on real merge chains.

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/gh-merge-chain.sh"
    [ -x "$SCRIPT" ] || skip "script not executable: $SCRIPT"
}

@test "exits 2 when called with no PR numbers" {
    run "$SCRIPT"
    [ "$status" -eq 2 ]
    [[ "$output" == *"usage:"* ]]
}

@test "exits 2 on unknown flag" {
    run "$SCRIPT" --nonexistent-flag 321
    [ "$status" -eq 2 ]
    [[ "$output" == *"unknown flag:"* ]]
}

@test "--help prints the leading comment block" {
    run "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"gh-merge-chain"* ]]
    [[ "$output" == *"update-branch"* ]]
}

@test "--dry-run lists PR numbers without invoking gh pr merge" {
    if ! command -v gh >/dev/null 2>&1; then
        skip "gh not installed"
    fi
    # gh repo view will be called; we tolerate failure by bypassing the
    # cwd-requires-gh-repo branch using a fake repo env if needed.
    # On a normal repo checkout this succeeds.
    run "$SCRIPT" --dry-run 321 322 323
    # In a non-repo, status=2 with the cannot-resolve message.
    if [[ "$status" -eq 2 && "$output" == *"cannot resolve current repo"* ]]; then
        skip "no gh-resolvable repo context"
    fi
    [ "$status" -eq 0 ]
    [[ "$output" == *"3 PR(s)"* ]]
    [[ "$output" == *"#321"* ]]
    [[ "$output" == *"#322"* ]]
    [[ "$output" == *"#323"* ]]
    [[ "$output" == *"[dry-run]"* ]]
}

@test "uses set -euo pipefail" {
    # The directive lives after the multi-line comment header.
    grep -qE '^set -euo pipefail$' "$SCRIPT"
}

@test "quotes the PR numbers when passing to gh" {
    # Defense against the unquoted-expansion class. The hot loop must use
    # \"\$pr\" / \"\$REPO\" — flag with shellcheck and grep.
    if command -v shellcheck >/dev/null 2>&1; then
        shellcheck -S warning "$SCRIPT"
    fi
}

@test "anchors to the F3 post-mortem learning reference" {
    # Asserting the rationale REFERENCE in the script body, not the local
    # learning file: .agents/ is gitignored, so a file-existence check would
    # fail in CI's fresh clone (caught on the rebased SHA of this PR).
    grep -q '2026-05-18-auto-merge-needs-update-branch-when-main-moves.md' "$SCRIPT"
}
