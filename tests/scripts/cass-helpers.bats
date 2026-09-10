#!/usr/bin/env bats
# Synthetic native records and stub CASS only; never reads the operator's corpus.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_DIR="$(mktemp -d)"
    export CASS_FIXTURE_ROOT="$TMP_DIR"
    export CASS_FIXTURE_MODE=fresh
    mkdir -p "$TMP_DIR/bin" "$TMP_DIR/.claude" "$TMP_DIR/.codex"
    cat > "$TMP_DIR/bin/cass" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$CASS_FIXTURE_ROOT/calls"
case "$1" in
    status)
        case "$CASS_FIXTURE_MODE" in
            timeout) exit 124 ;;
            malformed) printf 'not json\n'; exit 0 ;;
            unknown) printf '{}\n'; exit 0 ;;
            stale) printf '{"database":{"exists":true,"messages":12},"index":{"fresh":false,"stale":true,"documents":12}}\n' ;;
            missing)
                if [ -f "$CASS_FIXTURE_ROOT/repaired" ]; then
                    printf '{"database":{"exists":true,"messages":12},"index":{"fresh":true,"documents":12}}\n'
                else
                    printf '{"database":{"exists":false,"messages":0},"index":{"fresh":false,"documents":0}}\n'
                fi ;;
            *) printf '{"database":{"exists":true,"messages":12},"index":{"fresh":true,"documents":12}}\n' ;;
        esac ;;
    search)
        if [ "${CASS_FIXTURE_SEARCH_FAIL:-0}" = 1 ]; then exit 124; fi
        printf '{"aggregations":{"agent":{"buckets":[{"key":"codex","count":12}]},"date":{"buckets":[{"key":"2026-01-01","count":12}]}},"hits":[],"total_matches":0}\n' ;;
    doctor)
        touch "$CASS_FIXTURE_ROOT/repaired"
        printf '{"healthy":true,"checks":[],"auto_fix_actions":["repaired"]}\n' ;;
    *) echo "Unexpected CASS mutation: $*" >&2; exit 99 ;;
esac
STUB
    # Exercise the helper's cap plumbing without a live index or background job.
    cat > "$TMP_DIR/bin/timeout" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$CASS_FIXTURE_ROOT/caps"
shift
exec "$@"
STUB
    chmod +x "$TMP_DIR/bin/cass" "$TMP_DIR/bin/timeout"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "overview searches fresh, stale and missing state without indexing" {
    for CASS_FIXTURE_MODE in fresh stale missing; do
        export CASS_FIXTURE_MODE
        : > "$TMP_DIR/calls"
        : > "$TMP_DIR/caps"
        run env PATH="$TMP_DIR/bin:$PATH" bash "$REPO_ROOT/skills/cass/scripts/quick_analysis.sh" "$TMP_DIR"
        [ "$status" -eq 0 ]
        [[ "$output" == *"codex: 12 hits"* ]]
        [ "$(wc -l < "$TMP_DIR/calls" | tr -d ' ')" -eq 3 ]
        [ "$(wc -l < "$TMP_DIR/caps" | tr -d ' ')" -eq 3 ]
        ! grep -Eq '^(index|doctor|models|sources) ' "$TMP_DIR/calls"
        grep -q -- '--mode lexical --aggregate agent --limit 1 --fields minimal' "$TMP_DIR/calls"
    done
}

@test "overview preserves failed reads instead of reporting no sessions" {
    export CASS_FIXTURE_MODE=timeout
    export CASS_FIXTURE_SEARCH_FAIL=1
    run env PATH="$TMP_DIR/bin:$PATH" bash "$REPO_ROOT/skills/cass/scripts/quick_analysis.sh" "$TMP_DIR"
    [ "$status" -ne 0 ]
    [[ "$output" == *"CASS observation unavailable (exit 124)"* ]]
    [[ "$output" == *"Agent counts unavailable"* ]]
    [[ "$output" != *"No sessions found"* ]]
    ! grep -Eq '^(index|doctor) ' "$TMP_DIR/calls"
}

@test "selected recovery does not rebuild unobserved or malformed state" {
    for CASS_FIXTURE_MODE in timeout malformed unknown; do
        export CASS_FIXTURE_MODE
        : > "$TMP_DIR/calls"
        run env PATH="$TMP_DIR/bin:$PATH" bash "$REPO_ROOT/skills/cass/scripts/recover.sh"
        [ "$status" -eq 2 ]
        [[ "$output" == *"UNAVAILABLE: index state not observed"* ]]
        [ "$(cat "$TMP_DIR/calls")" = 'status --json' ]
    done
}

@test "selected recovery still repairs a diagnosed missing database" {
    export CASS_FIXTURE_MODE=missing
    run env PATH="$TMP_DIR/bin:$PATH" bash "$REPO_ROOT/skills/cass/scripts/recover.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"RECOVERED: doctor succeeded"* ]]
    [ -f "$TMP_DIR/repaired" ]
    ! grep -q '^index ' "$TMP_DIR/calls"
}

make_prompt_fixture() {
    python3 - "$TMP_DIR" <<'PY'
import json
import pathlib
import sys
root = pathlib.Path(sys.argv[1])
claude = [
    {"type": "assistant", "message": {"role": "assistant", "content": "early assistant"}},
    {"type": "user", "message": {"role": "user", "content": [{"type": "tool_result", "content": "tool output"}]}},
    {"type": "user", "message": {"role": "assistant", "content": "conflicting role"}},
]
claude += [{"type": "user", "timestamp": "2026-01-01T00:00:00Z", "message": {"role": "user", "content": "Retry this failed approach"}}] * 10
claude += [{"type": "user", "message": {"role": "user", "content": [
    {"type": "tool_result", "content": [{"type": "text", "text": "nested tool output"}]},
    {"type": "text", "text": "Actual mixed user text"},
]}}]
codex = [{"type": "session_meta", "payload": {"role": "user", "content": "metadata"}}] * 8
codex += [
    {"type": "response_item", "payload": {"type": "message", "role": "assistant", "content": [{"type": "output_text", "text": "assistant reply"}]}},
    {"type": "event_msg", "payload": {"type": "user_message", "message": "Late user text"}},
]
codex += [{"type": "response_item", "timestamp": "2026-01-01T00:00:00Z", "payload": {"type": "message", "role": "user", "content": [{"type": "input_text", "text": "Late user text"}]}}] * 2
codex += [{"role": "user", "content": "Flat user text"}]
for name, records in [(".claude", claude), (".codex", codex)]:
    (root / name / "fixture.jsonl").write_text("\n".join(json.dumps(record) for record in records) + "\n")
PY
}

@test "prompt miner uses native roles including late Codex records and excludes tool results" {
    make_prompt_fixture
    run python3 "$REPO_ROOT/skills/cass/scripts/prompt_miner.py" --glob "$TMP_DIR/.*/*.jsonl" --min-count 1 --json
    [ "$status" -eq 0 ]
    printf '%s' "$output" | jq -e '.total_prompts == 14 and (.repeated_prompts | length == 4)
        and (.repeated_prompts | map(.prompt) | sort) == ["Actual mixed user text", "Flat user text", "Late user text", "Retry this failed approach"]
        and (.repeated_prompts[] | select(.prompt == "Late user text") | .count == 2)' >/dev/null
}

@test "ten retries stay unassessed and legacy frequency flag remains usable" {
    make_prompt_fixture
    run python3 "$REPO_ROOT/skills/cass/scripts/prompt_miner.py" --glob "$TMP_DIR/.*/*.jsonl" --rituals-only --json
    [ "$status" -eq 0 ]
    printf '%s' "$output" | jq -e '(.repeated_prompts | length == 1)
        and (.repeated_prompts[0] | .count == 10 and .outcome == "unassessed" and (has("is_ritual") | not))' >/dev/null
}
