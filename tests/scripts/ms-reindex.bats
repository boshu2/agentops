#!/usr/bin/env bats
# Tests for scripts/ms-reindex.sh (age-22g0).
#
# Fixture fidelity: the mock `ms` emits the REAL on-disk shapes captured from a
# live ms 0.1.4 run 2026-07-02 — `ms index -O json` (status/indexed/errors/
# package_summary) and the two-line stdio JSON-RPC of `ms mcp serve` (initialize
# result + tools/call result whose content[0].text carries the nested search
# JSON). No hand-built shape ms never emits.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/ms-reindex.sh"
    TMP_DIR="$(mktemp -d)"
    MOCK="$TMP_DIR/ms"
    # Sandbox the ms data dir so step_clear_stale_locks never touches the real
    # ~/Library/Application Support/ms during tests. `env` inherits this export.
    export MS_DATA_DIR="$TMP_DIR/data"
    mkdir -p "$MS_DATA_DIR/index"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Write a mock `ms` binary. $1=indexed count, $2=errors JSON array,
# $3=serve line-2 (the tools/call result). Reads env for nothing else.
write_mock() {
    local indexed="$1" errors="$2" serve_l2="$3"
    cat > "$MOCK" <<EOF
#!/usr/bin/env bash
case "\$1" in
  index)
    printf '%s' '{"status":"partial","indexed":${indexed},"errors":${errors},"elapsed_ms":127,"package_summary":{"skills_discovered":177,"skills_with_companions":165,"total_companion_files":4517}}'
    ;;
  mcp)
    if [ "\$2" = "serve" ]; then
      cat >/dev/null   # consume the JSON-RPC stdin like the real server
      printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{"listChanged":false}},"serverInfo":{"name":"ms","version":"0.1.4"}}}'
      printf '%s\n' '${serve_l2}'
    fi
    ;;
esac
EOF
    chmod +x "$MOCK"
}

# A realistic tools/call result: content[0].text is escaped JSON with a results
# array. deadlock-finder-and-fixer present (the expected real id).
SERVE_GOOD='{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\n  \"query\": \"flaky concurrent test\",\n  \"count\": 3,\n  \"results\": [\n    {\n      \"id\": \"test\",\n      \"score\": 14.7\n    },\n    {\n      \"id\": \"deadlock-finder-and-fixer\",\n      \"score\": 5.4\n    },\n    {\n      \"id\": \"review\",\n      \"score\": 3.6\n    }\n  ]\n}"}]}}'

# Same, but with an orphan-class id (only a stale/pre-wipe server serves these).
SERVE_ORPHAN='{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\n  \"query\": \"flaky concurrent test\",\n  \"count\": 2,\n  \"results\": [\n    {\n      \"id\": \"deadlock-finder-and-fixer\",\n      \"score\": 5.4\n    },\n    {\n      \"id\": \"expected-all-pass\",\n      \"score\": 4.1\n    }\n  ]\n}"}]}}'

# Same, but WITHOUT the expected real id.
SERVE_MISSING='{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\n  \"query\": \"flaky concurrent test\",\n  \"count\": 1,\n  \"results\": [\n    {\n      \"id\": \"review\",\n      \"score\": 3.6\n    }\n  ]\n}"}]}}'

@test "full run passes: good index + no servers + fresh probe serves real id" {
    write_mock 176 '[{"path":"skills/_fixtures/bad-skill/SKILL.md","error":"missing skill id"}]' "$SERVE_GOOD"
    printf '' > "$TMP_DIR/empty_ps"
    run env MS_BIN="$MOCK" MS_REINDEX_PS_FIXTURE="$TMP_DIR/empty_ps" bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"index OK — indexed=176"* ]]
    [[ "$output" == *"probe OK"* ]]
    [[ "$output" == *"DONE"* ]]
}

@test "fails loud when indexed below the minimum" {
    write_mock 5 '[]' "$SERVE_GOOD"
    run env MS_BIN="$MOCK" MS_REINDEX_PS_FIXTURE=/dev/null bash "$SCRIPT"
    [ "$status" -ne 0 ]
    [[ "$output" == *"indexed=5 < required 170"* ]]
}

@test "fails loud when errors exceed the allowance" {
    write_mock 176 '[{"path":"a","error":"x"},{"path":"b","error":"y"}]' "$SERVE_GOOD"
    run env MS_BIN="$MOCK" MS_REINDEX_PS_FIXTURE=/dev/null bash "$SCRIPT"
    [ "$status" -ne 0 ]
    [[ "$output" == *"errors=2 > allowed 1"* ]]
}

@test "probe fails loud when an orphan-class id is present" {
    write_mock 176 '[]' "$SERVE_ORPHAN"
    run env MS_BIN="$MOCK" MS_REINDEX_PS_FIXTURE=/dev/null bash "$SCRIPT"
    [ "$status" -ne 0 ]
    [[ "$output" == *"ORPHAN id 'expected-all-pass' present"* ]]
}

@test "probe fails loud when the expected real id is missing" {
    write_mock 176 '[]' "$SERVE_MISSING"
    run env MS_BIN="$MOCK" MS_REINDEX_PS_FIXTURE=/dev/null bash "$SCRIPT"
    [ "$status" -ne 0 ]
    [[ "$output" == *"expected real id 'deadlock-finder-and-fixer' NOT in results"* ]]
}

@test "--print-serve-pids extracts pids from a full-command ps fixture" {
    cat > "$TMP_DIR/ps" <<'EOF'
  PID STARTED                  COMMAND
 12345 Wed Jul  2 20:14:26 2026 ms mcp serve
 67890 Wed Jul  2 20:15:00 2026 /Users/bo/.cargo/bin/ms mcp serve --tcp-port 9
 11111 Wed Jul  2 20:16:00 2026 vim scripts/ms-reindex.sh
 22222 Wed Jul  2 20:16:30 2026 grep -F ms mcp
EOF
    run env MS_REINDEX_PS_FIXTURE="$TMP_DIR/ps" bash "$SCRIPT" --print-serve-pids
    [ "$status" -eq 0 ]
    [[ "$output" == *"12345"* ]]
    [[ "$output" == *"67890"* ]]
    # non-server lines and the grep matcher line are excluded
    [[ "$output" != *"11111"* ]]
    [[ "$output" != *"22222"* ]]
}

@test "--print-serve-pids emits nothing when no servers run" {
    printf '  PID STARTED COMMAND\n 999 x y z bash -l\n' > "$TMP_DIR/ps"
    run env MS_REINDEX_PS_FIXTURE="$TMP_DIR/ps" bash "$SCRIPT" --print-serve-pids
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "sweep TERMs a real matching process (kill-list acted on, not just parsed)" {
    # spawn a real, killable dummy and label it as an ms mcp serve in the fixture
    sleep 300 &
    local dummy=$!
    printf ' %s Wed Jul  2 20:14:26 2026 ms mcp serve\n' "$dummy" > "$TMP_DIR/ps"
    run env MS_REINDEX_PS_FIXTURE="$TMP_DIR/ps" bash "$SCRIPT" --sweep-only
    [ "$status" -eq 0 ]
    [[ "$output" == *"sweep OK"* ]]
    # kill -0 must now fail: the process is gone
    run kill -0 "$dummy"
    [ "$status" -ne 0 ]
}
