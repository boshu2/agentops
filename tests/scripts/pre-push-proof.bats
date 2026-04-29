#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/.githooks"
    /bin/cp "$REPO_ROOT/scripts/pre-push-proof.sh" "$FAKE_REPO/scripts/pre-push-proof.sh"
    /bin/cp "$REPO_ROOT/scripts/pre-push-gate.sh" "$FAKE_REPO/scripts/pre-push-gate.sh"
    /bin/cp "$REPO_ROOT/.githooks/pre-push" "$FAKE_REPO/.githooks/pre-push"
    chmod +x "$FAKE_REPO/scripts/pre-push-proof.sh"
    cd "$FAKE_REPO"
    git init --quiet
    git config user.email test@example.com
    git config user.name Test
    touch README.md
    git add README.md scripts/pre-push-proof.sh scripts/pre-push-gate.sh .githooks/pre-push
    git commit --quiet -m "initial"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "pre-push-proof write then check succeeds for same fingerprint" {
    run "$FAKE_REPO/scripts/pre-push-proof.sh" write --scope worktree --mode fast
    [ "$status" -eq 0 ]
    [[ "$output" == *"pre-push validation proof recorded"* ]]

    run "$FAKE_REPO/scripts/pre-push-proof.sh" check --scope worktree --mode fast
    [ "$status" -eq 0 ]
    [[ "$output" == *"pre-push validation proof current"* ]]
}

@test "pre-push-proof invalidates after changed file set changes" {
    "$FAKE_REPO/scripts/pre-push-proof.sh" write --scope worktree --mode fast >/dev/null

    echo "changed" >> README.md

    run "$FAKE_REPO/scripts/pre-push-proof.sh" check --scope worktree --mode fast
    [ "$status" -eq 1 ]
}

@test "pre-push-proof invalidates after gate script changes" {
    "$FAKE_REPO/scripts/pre-push-proof.sh" write --scope worktree --mode fast >/dev/null

    echo "# changed" >> "$FAKE_REPO/scripts/pre-push-gate.sh"

    run "$FAKE_REPO/scripts/pre-push-proof.sh" check --scope worktree --mode fast
    [ "$status" -eq 1 ]
}

@test "pre-push-proof supports explicit proof file path" {
    proof="$TMP_DIR/proof.tsv"

    run "$FAKE_REPO/scripts/pre-push-proof.sh" write --scope head --mode fast --file "$proof"
    [ "$status" -eq 0 ]
    [ -f "$proof" ]

    run "$FAKE_REPO/scripts/pre-push-proof.sh" check --scope head --mode fast --file "$proof"
    [ "$status" -eq 0 ]
}
