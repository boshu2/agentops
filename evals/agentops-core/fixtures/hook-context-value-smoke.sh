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

assert_empty() {
    local output="$1"
    local label="$2"
    if [[ -z "$output" ]]; then
        pass "$label stays silent"
    else
        fail "$label stays silent"
    fi
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

test_standards_injector() {
    local output ctx unsupported
    output="$(jq -n '{"tool_input":{"file_path":"cmd/example/main.go"}}' | bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)"
    json_event "$output" "PreToolUse" "standards-injector go"
    ctx="$(context_of "$output")"
    assert_contains "$ctx" "# Go Standards" "standards-injector go"
    assert_contains "$ctx" "gofmt" "standards-injector go"
    assert_contains "$ctx" "Full reference: skills/standards/references/go.md" "standards-injector go"
    assert_bytes_le "$ctx" 1200 "standards_go_bytes"

    unsupported="$(jq -n '{"tool_input":{"file_path":"notes.txt"}}' | bash "$HOOKS_DIR/standards-injector.sh" 2>&1 || true)"
    assert_empty "$unsupported" "standards-injector unsupported extension"
}

test_prompt_nudge() {
    local repo output ctx dedup_output
    repo="$TMP_ROOT/prompt-nudge"
    setup_git_repo "$repo"
    mkdir -p "$repo/.agents/ao" "$repo/bin"
    printf '{"step":"research","status":"done"}\n' > "$repo/.agents/ao/chain.jsonl"
    cat > "$repo/bin/ao" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" = "ratchet" && "${2:-}" = "status" ]]; then
    printf '{"steps":[{"step":"pre-mortem","status":"pending"},{"step":"vibe","status":"pending"}]}\n'
    exit 0
fi
exit 1
EOF
    chmod +x "$repo/bin/ao"

    output="$(cd "$repo" && jq -n '{"prompt":"implement the feature"}' | PATH="$repo/bin:$PATH" bash "$HOOKS_DIR/prompt-nudge.sh" 2>&1 || true)"
    json_event "$output" "UserPromptSubmit" "prompt-nudge"
    ctx="$(context_of "$output")"
    assert_contains "$ctx" "pre-mortem hasn't been run" "prompt-nudge"
    assert_bytes_le "$ctx" 200 "prompt_nudge_bytes"

    touch "$repo/.agents/ao/.ratchet-advance-fired"
    dedup_output="$(cd "$repo" && jq -n '{"prompt":"implement the feature"}' | PATH="$repo/bin:$PATH" bash "$HOOKS_DIR/prompt-nudge.sh" 2>&1 || true)"
    assert_empty "$dedup_output" "prompt-nudge dedup flag"
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

test_context_monitor() {
    local session bridge output ctx quiet stale
    session="hook-context-value-$$"
    bridge="/tmp/claude-ctx-${session}.json"
    printf '{"remaining_percent":20,"total_tokens":200000,"used_tokens":160000}\n' > "$bridge"
    output="$(printf '{"tool_name":"Read"}\n' | CLAUDE_SESSION_ID="$session" bash "$HOOKS_DIR/context-monitor.sh" 2>&1 || true)"
    json_event "$output" "PostToolUse" "context-monitor critical"
    ctx="$(context_of "$output")"
    assert_contains "$ctx" "20% remaining" "context-monitor critical"
    assert_bytes_le "$ctx" 240 "context_monitor_bytes"

    printf '{"remaining_percent":40}\n' > "$bridge"
    quiet="$(printf '{"tool_name":"Read"}\n' | CLAUDE_SESSION_ID="$session" bash "$HOOKS_DIR/context-monitor.sh" 2>&1 || true)"
    assert_empty "$quiet" "context-monitor above threshold"

    printf '{"remaining_percent":20}\n' > "$bridge"
    touch -t 202001010101 "$bridge"
    stale="$(printf '{"tool_name":"Read"}\n' | CLAUDE_SESSION_ID="$session" bash "$HOOKS_DIR/context-monitor.sh" 2>&1 || true)"
    assert_empty "$stale" "context-monitor stale bridge"
    rm -f "$bridge"
}

test_commit_review_gate() {
    local repo output ctx quiet
    repo="$TMP_ROOT/commit-review"
    setup_git_repo "$repo"
    printf 'seed\n' > "$repo/config.env"
    git -C "$repo" add config.env
    git -C "$repo" commit -q -m "seed"
    {
        printf 'TOKEN_FOR_REDACTION=secret-fixture-value\n'
        for i in $(seq 1 80); do
            printf 'line_%03d=value\n' "$i"
        done
    } > "$repo/config.env"
    git -C "$repo" add config.env

    output="$(cd "$repo" && jq -n '{"tool_name":"Bash","tool_input":{"command":"git commit -m eval"}}' | AGENTOPS_MANAGED_HOOK_BACKEND_DISABLED=1 AGENTOPS_COMMIT_REVIEW_DIFF_LINES=20 bash "$HOOKS_DIR/commit-review-gate.sh" 2>&1 || true)"
    json_event "$output" "PreToolUse" "commit-review-gate"
    ctx="$(context_of "$output")"
    assert_contains "$ctx" "SELF-REVIEW before committing" "commit-review-gate"
    assert_contains "$ctx" "showing first 20" "commit-review-gate"
    assert_contains "$ctx" "TOKEN_FOR_REDACTION=[REDACTED]" "commit-review-gate"
    assert_not_contains "$ctx" "secret-fixture-value" "commit-review-gate"
    assert_bytes_le "$ctx" 5000 "commit_review_bytes"

    git -C "$repo" reset -q
    quiet="$(cd "$repo" && jq -n '{"tool_name":"Bash","tool_input":{"command":"git commit -m eval"}}' | AGENTOPS_MANAGED_HOOK_BACKEND_DISABLED=1 bash "$HOOKS_DIR/commit-review-gate.sh" 2>&1 || true)"
    assert_empty "$quiet" "commit-review-gate without staged diff"
}

test_edit_knowledge_surface() {
    local repo file output ctx quiet
    repo="$TMP_ROOT/edit-knowledge"
    setup_git_repo "$repo"
    mkdir -p "$repo/.agents/learnings" "$repo/pkg"
    file="$repo/pkg/service.go"
    printf 'package pkg\n' > "$file"
    for i in 1 2 3 4; do
        cat > "$repo/.agents/learnings/${i}-service.md" <<EOF
# Service Learning $i

pkg/service.go requires careful validation.
EOF
    done

    output="$(cd "$repo" && jq -n --arg file "$file" '{"tool_name":"Edit","tool_input":{"file_path":$file}}' | bash "$HOOKS_DIR/edit-knowledge-surface.sh" 2>&1 || true)"
    json_event "$output" "PreToolUse" "edit-knowledge-surface"
    ctx="$(context_of "$output")"
    assert_contains "$ctx" "Relevant learnings for service.go" "edit-knowledge-surface"
    assert_contains "$ctx" "Review these before making changes" "edit-knowledge-surface"
    local learning_count
    learning_count="$(printf '%s\n' "$ctx" | grep -c '^- [0-9]-service.md:' || true)"
    record_metric "edit_knowledge_learning_count" "$learning_count"
    if [[ "$learning_count" -le 3 ]]; then
        pass "edit-knowledge-surface emits at most three learning titles ($learning_count)"
    else
        fail "edit-knowledge-surface emits at most three learning titles ($learning_count)"
    fi
    assert_bytes_le "$ctx" 700 "edit_knowledge_bytes"

    quiet="$(cd "$repo" && jq -n '{"tool_name":"Edit","tool_input":{"file_path":"pkg/other.go"}}' | bash "$HOOKS_DIR/edit-knowledge-surface.sh" 2>&1 || true)"
    assert_empty "$quiet" "edit-knowledge-surface unrelated file"
}

main() {
    require_tool jq
    require_tool git

    TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/hook-context-value.XXXXXX")"
    export TMP_ROOT
    trap 'rm -rf "$TMP_ROOT"' EXIT

    test_standards_injector
    test_prompt_nudge
    test_precompact_snapshot
    test_context_monitor
    test_commit_review_gate
    test_edit_knowledge_surface

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
