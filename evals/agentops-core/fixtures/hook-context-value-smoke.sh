#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROOT="$(cd "$ROOT/.." && pwd)"
HOOKS_DIR="$ROOT/hooks"

PASS=0
FAIL=0
METRICS=()

fail() {
    printf 'FAIL hook-context-value: %s\n' "$1" >&2
    FAIL=$((FAIL + 1))
}

pass() {
    printf 'PASS hook-context-value: %s\n' "$1"
    PASS=$((PASS + 1))
}

require_tool() {
    if ! command -v "$1" >/dev/null 2>&1; then
        printf 'FAIL hook-context-value: required tool missing: %s\n' "$1" >&2
        exit 1
    fi
}

record_metric() {
    METRICS+=("$1=$2")
}

json_event() {
    local output="$1"
    local event="$2"
    local label="$3"

    if ! printf '%s' "$output" | jq . >/dev/null 2>&1; then
        fail "$label emits valid JSON"
        return 1
    fi
    if printf '%s' "$output" | jq -e --arg event "$event" '.hookSpecificOutput.hookEventName == $event' >/dev/null 2>&1; then
        pass "$label emits $event hookSpecificOutput"
    else
        fail "$label emits $event hookSpecificOutput"
        return 1
    fi
}

context_of() {
    printf '%s' "$1" | jq -r '.hookSpecificOutput.additionalContext // empty'
}

byte_len() {
    printf '%s' "$1" | wc -c | tr -d ' '
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local label="$3"
    if [[ "$haystack" == *"$needle"* ]]; then
        pass "$label contains signal: $needle"
    else
        fail "$label contains signal: $needle"
    fi
}

assert_not_contains() {
    local haystack="$1"
    local needle="$2"
    local label="$3"
    if [[ "$haystack" != *"$needle"* ]]; then
        pass "$label excludes noise: $needle"
    else
        fail "$label excludes noise: $needle"
    fi
}

assert_bytes_le() {
    local text="$1"
    local max="$2"
    local label="$3"
    local bytes
    bytes="$(byte_len "$text")"
    record_metric "$label" "$bytes"
    if [[ "$bytes" -le "$max" ]]; then
        pass "$label bytes <= $max ($bytes)"
    else
        fail "$label bytes <= $max ($bytes)"
    fi
}

setup_git_repo() {
    local dir="$1"
    mkdir -p "$dir"
    git -C "$dir" init -q
    git -C "$dir" config user.email "eval@example.com"
    git -C "$dir" config user.name "Eval Runner"
}

test_precompact_snapshot() {
    local repo output ctx
    repo="$TMP_ROOT/precompact"
    setup_git_repo "$repo"
    mkdir -p "$repo/.agents"
    printf 'change\n' > "$repo/README.md"
    git -C "$repo" add README.md
    git -C "$repo" commit -q -m "seed"
    printf 'changed\n' > "$repo/README.md"

    output="$(cd "$repo" && bash "$HOOKS_DIR/precompact-snapshot.sh" 2>&1 || true)"
    json_event "$output" "PreCompact" "precompact-snapshot"
    ctx="$(context_of "$output")"
    assert_contains "$ctx" "branch=" "precompact-snapshot"
    assert_contains "$ctx" "files_changed=" "precompact-snapshot"
    assert_contains "$ctx" "snapshot=" "precompact-snapshot"
    assert_not_contains "$ctx" "README.md" "precompact-snapshot summary"
    assert_bytes_le "$ctx" 500 "precompact_bytes"
}

main() {
    require_tool jq
    require_tool git

    TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/hook-context-value.XXXXXX")"
    export TMP_ROOT
    trap 'rm -rf "$TMP_ROOT"' EXIT

    test_precompact_snapshot

    printf 'hook-context-value metrics:'
    for metric in "${METRICS[@]}"; do
        printf ' %s' "$metric"
    done
    printf '\n'

    if [[ "$FAIL" -gt 0 ]]; then
        printf 'RESULT hook-context-value FAIL pass=%d fail=%d\n' "$PASS" "$FAIL" >&2
        exit 1
    fi

    printf 'RESULT hook-context-value PASS pass=%d fail=%d\n' "$PASS" "$FAIL"
}

main "$@"
