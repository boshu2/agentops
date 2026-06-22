#!/usr/bin/env bats
# L2 tests for scripts/verify-pushed-commit-builds.sh — the partial-commit-lands-broken
# guard (age-yy24). The "build" is stubbed via AGENTOPS_COMMIT_BUILD_CMD so the test is
# fast + hermetic: "the committed tree builds" == "helper.sh exists in it". The escape
# is a commit MISSING a file the code needs while the worktree still has it.

setup() {
    SCRIPT="$(git rev-parse --show-toplevel)/scripts/verify-pushed-commit-builds.sh"
    TMP="$(mktemp -d)"
    cd "$TMP"
    git init -q .
    git config user.email t@t && git config user.name t
    # The committed tree "builds" iff helper.sh is present; watch the whole repo.
    export AGENTOPS_COMMIT_BUILD_CMD="test -f helper.sh"
    export AGENTOPS_COMMIT_BUILD_PATHS="."
    unset AGENTOPS_PREPUSH_SKIP_COMMIT_BUILD
}

teardown() {
    cd /
    rm -rf "$TMP"
}

@test "partial commit (needed file left untracked) is refused" {
    # Commit code.sh but NOT helper.sh; helper.sh exists only in the worktree
    # (untracked) — the worktree 'builds' but the COMMIT does not.
    echo 'code' > code.sh
    git add code.sh && git commit -qm "partial: code without helper"
    sha="$(git rev-parse HEAD)"
    echo 'helper' > helper.sh   # untracked -> dirty source tree, present in worktree only
    run bash "$SCRIPT" "$sha"
    [ "$status" -eq 1 ]
    [[ "$output" == *"does NOT build"* ]]
}

@test "complete commit on a dirty tree is allowed" {
    # helper.sh IS committed (the commit builds); an unrelated untracked .sh makes the
    # tree dirty so the build path actually runs (not the clean-tree skip).
    echo 'code' > code.sh; echo 'helper' > helper.sh
    git add code.sh helper.sh && git commit -qm "complete"
    sha="$(git rev-parse HEAD)"
    echo 'scratch' > unrelated.sh   # untracked, makes tree dirty
    run bash "$SCRIPT" "$sha"
    [ "$status" -eq 0 ]
}

@test "clean tree is skipped on the fast path (worktree == HEAD)" {
    echo 'code' > code.sh; echo 'helper' > helper.sh
    git add code.sh helper.sh && git commit -qm "complete + clean"
    sha="$(git rev-parse HEAD)"
    # No dirty source -> the gate's worktree build already validated the commit -> skip.
    run bash "$SCRIPT" "$sha"
    [ "$status" -eq 0 ]
}

@test "emergency bypass skips the check" {
    echo 'code' > code.sh
    git add code.sh && git commit -qm "partial"
    sha="$(git rev-parse HEAD)"
    echo 'helper' > helper.sh   # would otherwise fail
    AGENTOPS_PREPUSH_SKIP_COMMIT_BUILD=1 run bash "$SCRIPT" "$sha"
    [ "$status" -eq 0 ]
}

@test "a broken INTERMEDIATE commit is caught even when the tip builds (range, clean tree)" {
    # base builds; commit A removes helper.sh (broken); commit B restores it (tip builds).
    # Pushing base..B must be refused because A does not build — even on a clean tree
    # (the worktree build only ever validated the tip). Regression for the 2026-06-22 refute.
    echo 'code' > code.sh; echo 'helper' > helper.sh
    git add code.sh helper.sh && git commit -qm "base (builds)"
    base="$(git rev-parse HEAD)"
    git rm -q helper.sh && git commit -qm "A: drop helper (broken)"
    echo 'helper' > helper.sh && git add helper.sh && git commit -qm "B: restore helper (tip builds)"
    tip="$(git rev-parse HEAD)"
    # Clean tree now; tip builds. Stdin: <lref> <lsha=tip> <rref> <rsha=base>.
    run bash -c "printf 'refs/heads/main %s refs/heads/main %s\n' '$tip' '$base' | bash '$SCRIPT'"
    [ "$status" -eq 1 ]
    [[ "$output" == *"does NOT build"* ]]
}

@test "reads shas from git pre-push stdin (field 2)" {
    echo 'code' > code.sh
    git add code.sh && git commit -qm "partial"
    sha="$(git rev-parse HEAD)"
    echo 'helper' > helper.sh
    # Simulate git's pre-push stdin: <local_ref> <local_sha> <remote_ref> <remote_sha>
    run bash -c "printf 'refs/heads/main %s refs/heads/main %s\n' '$sha' '0000000000000000000000000000000000000000' | bash '$SCRIPT'"
    [ "$status" -eq 1 ]
}
