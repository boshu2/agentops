#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/verify-release-ci.sh"

    TMP_DIR="$(mktemp -d)"
    WORK_REPO="$TMP_DIR/repo"
    STUB_BIN="$TMP_DIR/bin"
    GH_LOG="$TMP_DIR/gh.log"

    mkdir -p "$STUB_BIN" "$WORK_REPO/scripts"
    cp "$SCRIPT" "$WORK_REPO/scripts/verify-release-ci.sh"
    chmod +x "$WORK_REPO/scripts/verify-release-ci.sh"

    git init -b main "$WORK_REPO" >/dev/null
    git -C "$WORK_REPO" config user.name "Test User"
    git -C "$WORK_REPO" config user.email "test@example.com"
    printf 'release\n' > "$WORK_REPO/file.txt"
    git -C "$WORK_REPO" add file.txt scripts/verify-release-ci.sh
    git -C "$WORK_REPO" commit -m "release base" >/dev/null
    git -C "$WORK_REPO" tag -a v1.0.0 -m "Release v1.0.0"
    RELEASE_SHA="$(git -C "$WORK_REPO" rev-parse HEAD)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_gh_stub() {
    local list_payload="$1"
    local view_payload="${2:-$list_payload}"
    local watch_status="${3:-0}"

    cat > "$STUB_BIN/gh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
echo "\$*" >> "$GH_LOG"

if [[ "\${1:-}" == "run" && "\${2:-}" == "list" ]]; then
  cat <<'JSON'
$list_payload
JSON
  exit 0
fi

if [[ "\${1:-}" == "run" && "\${2:-}" == "view" ]]; then
  cat <<'JSON'
$view_payload
JSON
  exit 0
fi

if [[ "\${1:-}" == "run" && "\${2:-}" == "watch" ]]; then
  exit $watch_status
fi

echo "unexpected gh invocation: \$*" >&2
exit 1
EOF
    chmod +x "$STUB_BIN/gh"
}

@test "verify-release-ci reports GO for a successful exact-SHA Validate run" {
    write_gh_stub "[{\"databaseId\":123,\"headSha\":\"$RELEASE_SHA\",\"status\":\"completed\",\"conclusion\":\"success\",\"url\":\"https://example.invalid/runs/123\",\"createdAt\":\"2026-05-16T00:00:00Z\",\"event\":\"push\",\"displayTitle\":\"Validate\"}]"

    run env PATH="$STUB_BIN:$PATH" bash -c "cd '$WORK_REPO' && bash scripts/verify-release-ci.sh v1.0.0 --repo owner/repo --timeout 0 --poll-interval 0"
    [ "$status" -eq 0 ]
    [[ "$output" == *"GO release-ci"* ]]
    [[ "$output" == *"workflow=validate.yml"* ]]
    [[ "$output" == *"run_id=123"* ]]
    [[ "$output" == *"conclusion=success"* ]]
    [[ "$output" == *"sha=$RELEASE_SHA"* ]]
}

@test "verify-release-ci reports NO-GO for a failed exact-SHA Validate run" {
    write_gh_stub "[{\"databaseId\":124,\"headSha\":\"$RELEASE_SHA\",\"status\":\"completed\",\"conclusion\":\"failure\",\"url\":\"https://example.invalid/runs/124\",\"createdAt\":\"2026-05-16T00:00:00Z\",\"event\":\"push\",\"displayTitle\":\"Validate\"}]"

    run env PATH="$STUB_BIN:$PATH" bash -c "cd '$WORK_REPO' && bash scripts/verify-release-ci.sh v1.0.0 --repo owner/repo --timeout 0 --poll-interval 0"
    [ "$status" -eq 1 ]
    [[ "$output" == *"NO-GO release-ci"* ]]
    [[ "$output" == *"run_id=124"* ]]
    [[ "$output" == *"conclusion=failure"* ]]
}

@test "verify-release-ci watches an in-progress run before reporting GO" {
    write_gh_stub \
        "[{\"databaseId\":125,\"headSha\":\"$RELEASE_SHA\",\"status\":\"in_progress\",\"conclusion\":null,\"url\":\"https://example.invalid/runs/125\",\"createdAt\":\"2026-05-16T00:00:00Z\",\"event\":\"push\",\"displayTitle\":\"Validate\"}]" \
        "{\"databaseId\":125,\"headSha\":\"$RELEASE_SHA\",\"status\":\"completed\",\"conclusion\":\"success\",\"url\":\"https://example.invalid/runs/125\",\"createdAt\":\"2026-05-16T00:00:00Z\",\"event\":\"push\",\"displayTitle\":\"Validate\"}"

    run env PATH="$STUB_BIN:$PATH" bash -c "cd '$WORK_REPO' && bash scripts/verify-release-ci.sh v1.0.0 --repo owner/repo --timeout 0 --poll-interval 0"
    [ "$status" -eq 0 ]
    [[ "$output" == *"WAIT release-ci"* ]]
    [[ "$output" == *"GO release-ci"* ]]

    run grep -F "run watch 125 --repo owner/repo --exit-status" "$GH_LOG"
    [ "$status" -eq 0 ]
}

@test "verify-release-ci reports NO-GO when no run exists for the tagged SHA" {
    write_gh_stub "[]"

    run env PATH="$STUB_BIN:$PATH" bash -c "cd '$WORK_REPO' && bash scripts/verify-release-ci.sh v1.0.0 --repo owner/repo --timeout 0 --poll-interval 0"
    [ "$status" -eq 1 ]
    [[ "$output" == *"NO-GO release-ci: no validate.yml run found"* ]]
    [[ "$output" == *"sha=$RELEASE_SHA"* ]]
}

@test "verify-release-ci rejects targets that do not resolve locally" {
    write_gh_stub "[]"

    run env PATH="$STUB_BIN:$PATH" bash -c "cd '$WORK_REPO' && bash scripts/verify-release-ci.sh v9.9.9 --repo owner/repo --timeout 0 --poll-interval 0"
    [ "$status" -eq 1 ]
    [[ "$output" == *"target does not resolve to a commit"* ]]
}
