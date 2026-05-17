#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    HEALTH="$REPO_ROOT/scripts/check-compile-health.sh"
    OSCILLATION="$REPO_ROOT/scripts/check-compile-oscillation.sh"
    WORK="$BATS_TEST_TMPDIR/work"
    AGENTS="$WORK/.agents"
    mkdir -p "$AGENTS/defrag"
}

write_report() {
    local timestamp="$1"
    local stale_count="${2:-0}"
    local oscillating="${3:-[]}"
    cat > "$AGENTS/defrag/latest.json" <<JSON
{
  "timestamp": "$timestamp",
  "prune": { "stale_count": $stale_count },
  "oscillation": { "oscillating_goals": $oscillating }
}
JSON
}

fresh_ts() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

stale_ts() {
    date -u -d '48 hours ago' +"%Y-%m-%dT%H:%M:%SZ"
}

@test "compile-health skips when local defrag artifact is absent" {
    rm -rf "$AGENTS/defrag"
    run env AGENTS_DIR="$AGENTS" bash "$HEALTH"
    [ "$status" -eq 77 ]
    [[ "$output" == *"SKIP:"* ]]
}

@test "compile-health can require missing artifact as failure" {
    rm -rf "$AGENTS/defrag"
    run env AGENTS_DIR="$AGENTS" COMPILE_REQUIRE_ARTIFACT=1 bash "$HEALTH"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL:"* ]]
}

@test "compile-health passes fresh valid artifact" {
    write_report "$(fresh_ts)" 1
    run env AGENTS_DIR="$AGENTS" bash "$HEALTH"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS:"* ]]
}

@test "compile-health fails stale artifact" {
    write_report "$(stale_ts)" 0
    run env AGENTS_DIR="$AGENTS" COMPILE_MAX_AGE_HOURS=1 bash "$HEALTH"
    [ "$status" -eq 1 ]
    [[ "$output" == *"last defrag was"* ]]
}

@test "compile-health fails malformed artifact" {
    printf '{bad json\n' > "$AGENTS/defrag/latest.json"
    run env AGENTS_DIR="$AGENTS" bash "$HEALTH"
    [ "$status" -eq 1 ]
    [[ "$output" == *"could not read .timestamp"* ]]
}

@test "compile-health fails excessive stale count" {
    write_report "$(fresh_ts)" 9
    run env AGENTS_DIR="$AGENTS" COMPILE_MAX_STALE=5 bash "$HEALTH"
    [ "$status" -eq 1 ]
    [[ "$output" == *"stale learnings"* ]]
}

@test "compile-oscillation skips when local defrag artifact is absent" {
    rm -rf "$AGENTS/defrag"
    run env AGENTS_DIR="$AGENTS" bash "$OSCILLATION"
    [ "$status" -eq 77 ]
    [[ "$output" == *"SKIP:"* ]]
}

@test "compile-oscillation passes fresh valid artifact" {
    write_report "$(fresh_ts)" 0 "[]"
    run env AGENTS_DIR="$AGENTS" bash "$OSCILLATION"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS:"* ]]
}

@test "compile-oscillation fails stale artifact" {
    write_report "$(stale_ts)" 0 "[]"
    run env AGENTS_DIR="$AGENTS" COMPILE_MAX_AGE_HOURS=1 bash "$OSCILLATION"
    [ "$status" -eq 1 ]
    [[ "$output" == *"last defrag was"* ]]
}

@test "compile-oscillation fails malformed artifact" {
    printf '{bad json\n' > "$AGENTS/defrag/latest.json"
    run env AGENTS_DIR="$AGENTS" bash "$OSCILLATION"
    [ "$status" -eq 1 ]
    [[ "$output" == *"could not read .timestamp"* ]]
}

@test "compile-oscillation fails internally inconsistent artifact" {
    write_report "$(fresh_ts)" 0 '["goal-a"]'
    run env AGENTS_DIR="$AGENTS" bash "$OSCILLATION"
    [ "$status" -eq 1 ]
    [[ "$output" == *"oscillating goal"* ]]
}
