#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/retag-release.sh"

    TMP_DIR="$(mktemp -d)"
    REMOTE_REPO="$TMP_DIR/remote.git"
    WORK_REPO="$TMP_DIR/repo"
    STUB_BIN="$TMP_DIR/bin"
    GH_LOG="$TMP_DIR/gh.log"

    mkdir -p "$STUB_BIN"
    git init --bare "$REMOTE_REPO" >/dev/null
    git init -b main "$WORK_REPO" >/dev/null
    git -C "$WORK_REPO" remote add origin "$REMOTE_REPO"
    mkdir -p "$WORK_REPO/scripts"
    cp "$SCRIPT" "$WORK_REPO/scripts/retag-release.sh"
    chmod +x "$WORK_REPO/scripts/retag-release.sh"

    git -C "$WORK_REPO" config user.name "Test User"
    git -C "$WORK_REPO" config user.email "test@example.com"
    printf 'one\n' > "$WORK_REPO/file.txt"
    git -C "$WORK_REPO" add file.txt scripts/retag-release.sh
    git -C "$WORK_REPO" commit -m "feat: initial release" >/dev/null
    git -C "$WORK_REPO" tag -a v1.0.0 -m $'Release v1.0.0\n\nOriginal annotation'
    git -C "$WORK_REPO" push origin main >/dev/null
    git -C "$WORK_REPO" push origin v1.0.0 >/dev/null

    printf 'two\n' >> "$WORK_REPO/file.txt"
    git -C "$WORK_REPO" add file.txt
    git -C "$WORK_REPO" commit -m "fix: post-tag patch" >/dev/null

    cat > "$STUB_BIN/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

LOG_FILE="${GH_LOG:?}"
echo "$*" >> "$LOG_FILE"

if [[ "${1:-}" == "release" && "${2:-}" == "delete" ]]; then
  exit 0
fi

if [[ "${1:-}" == "run" && "${2:-}" == "list" ]]; then
  if [[ "$*" == *"| .url"* ]]; then
    printf '%s\n' "https://example.invalid/runs/123"
  else
    printf '%s\n' "123"
  fi
  exit 0
fi

if [[ "${1:-}" == "run" && "${2:-}" == "watch" ]]; then
  exit 0
fi

echo "unexpected gh invocation: $*" >&2
exit 1
EOF

    cat > "$STUB_BIN/brew" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

    cat > "$STUB_BIN/ao" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "version" ]]; then
  printf '%s\n' "ao version 1.0.0"
  exit 0
fi
exit 0
EOF

    chmod +x "$STUB_BIN/gh" "$STUB_BIN/brew" "$STUB_BIN/ao"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "retag-release preserves annotation and waits on the tag-push run only" {
    run env PATH="$STUB_BIN:$PATH" GH_LOG="$GH_LOG" \
        bash -c "cd '$WORK_REPO' && bash scripts/retag-release.sh v1.0.0 owner/repo"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Release workflow succeeded."* ]]
    [[ "$output" == *"Verified annotated tag object for v1.0.0 (retag)"* ]]

    run git -C "$WORK_REPO" cat-file -t v1.0.0
    [ "$status" -eq 0 ]
    [ "$output" = "tag" ]

    run git -C "$WORK_REPO" rev-parse v1.0.0^{}
    [ "$status" -eq 0 ]
    tagged_commit="$output"
    run git -C "$WORK_REPO" rev-parse HEAD
    [ "$status" -eq 0 ]
    [ "$tagged_commit" = "$output" ]

    run git -C "$WORK_REPO" tag -l v1.0.0 --format='%(contents)'
    [ "$status" -eq 0 ]
    [ "$output" = $'Release v1.0.0\n\nOriginal annotation' ]

    run git --git-dir="$REMOTE_REPO" cat-file -t refs/tags/v1.0.0
    [ "$status" -eq 0 ]
    [ "$output" = "tag" ]

    run grep -F "workflow run" "$GH_LOG"
    [ "$status" -eq 1 ]
}

@test "retag-release recreates a lightweight tag at HEAD as annotated" {
    git -C "$WORK_REPO" tag -f v1.0.0 HEAD >/dev/null 2>&1

    run env PATH="$STUB_BIN:$PATH" GH_LOG="$GH_LOG" \
        bash -c "cd '$WORK_REPO' && bash scripts/retag-release.sh v1.0.0 owner/repo"
    [ "$status" -eq 0 ]
    [[ "$output" == *"points at HEAD but is a commit object; recreating annotated tag."* ]]
    [[ "$output" == *"Verified annotated tag object for v1.0.0 (retag)"* ]]

    run git -C "$WORK_REPO" cat-file -t refs/tags/v1.0.0
    [ "$status" -eq 0 ]
    [ "$output" = "tag" ]

    run git -C "$WORK_REPO" rev-parse v1.0.0^{}
    [ "$status" -eq 0 ]
    tagged_commit="$output"
    run git -C "$WORK_REPO" rev-parse HEAD
    [ "$status" -eq 0 ]
    [ "$tagged_commit" = "$output" ]

    run git -C "$WORK_REPO" tag -l v1.0.0 --format='%(contents)'
    [ "$status" -eq 0 ]
    [ "$output" = "Release v1.0.0" ]

    run git --git-dir="$REMOTE_REPO" cat-file -t refs/tags/v1.0.0
    [ "$status" -eq 0 ]
    [ "$output" = "tag" ]
}
