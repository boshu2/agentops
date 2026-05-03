#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/preflight-uat-binary.sh"
    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    FAKE_BIN="$TMP_DIR/bin"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_BIN"
    cp "$SCRIPT" "$FAKE_REPO/scripts/preflight-uat-binary.sh"
    chmod +x "$FAKE_REPO/scripts/preflight-uat-binary.sh"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "preflight UAT binary ignores nearer non-release tags" {
    git -C "$FAKE_REPO" init -q
    git -C "$FAKE_REPO" config user.email "test@example.com"
    git -C "$FAKE_REPO" config user.name "Test User"

    printf 'release\n' > "$FAKE_REPO/file.txt"
    git -C "$FAKE_REPO" add file.txt
    git -C "$FAKE_REPO" commit -q -m "release base"
    git -C "$FAKE_REPO" tag v1.2.3

    printf 'benchmark\n' >> "$FAKE_REPO/file.txt"
    git -C "$FAKE_REPO" commit -q -am "benchmark prep"
    git -C "$FAKE_REPO" tag pre-wave-1-baseline-eval-corpus-2026-05-01

    expected="$(git -C "$FAKE_REPO" describe --tags --match 'v[0-9]*.[0-9]*.[0-9]*' --always --dirty)"
    unfiltered="$(git -C "$FAKE_REPO" describe --tags --always --dirty)"
    [ "$unfiltered" = "pre-wave-1-baseline-eval-corpus-2026-05-01" ]
    [ "$expected" != "$unfiltered" ]

    cat > "$FAKE_BIN/ao" <<EOF
#!/usr/bin/env bash
echo "ao version $expected"
EOF
    chmod +x "$FAKE_BIN/ao"

    run env PATH="$FAKE_BIN:$PATH" bash -c "cd '$FAKE_REPO' && ./scripts/preflight-uat-binary.sh"

    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS: ao binary matches local build ($expected)"* ]]
}
