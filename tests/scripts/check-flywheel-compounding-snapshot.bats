#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-flywheel-compounding-snapshot.sh"
    TMP_DIR="$(mktemp -d)"
    SNAPSHOT="$TMP_DIR/flywheel-compounding-snapshot.json"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_snapshot() {
    local recorded_at="$1"
    local compounding="$2"
    cat > "$SNAPSHOT" <<JSON
{
  "recorded_at": "$recorded_at",
  "git_sha": "test-sha",
  "git_branch": "test",
  "evidence": {
    "status": "DECAYING",
    "delta": 21.9,
    "sigma_rho": 0.002,
    "escape_velocity_compounding": $compounding
  }
}
JSON
}

@test "fresh non-compounding snapshot passes as health evidence" {
    write_snapshot "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "false"

    run env SNAPSHOT_PATH="$SNAPSHOT" bash "$SCRIPT"

    [ "$status" -eq 0 ]
    [[ "$output" == *"compounding=false"* ]]
    [[ "$output" == *"action signal"* ]]
}

@test "strict mode fails a fresh non-compounding snapshot" {
    write_snapshot "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "false"

    run env SNAPSHOT_PATH="$SNAPSHOT" AGENTOPS_FLYWHEEL_SNAPSHOT_REQUIRE_COMPOUNDING=1 bash "$SCRIPT"

    [ "$status" -eq 1 ]
    [[ "$output" == *"strict mode"* ]]
}

@test "stale snapshot still fails" {
    write_snapshot "2000-01-01T00:00:00Z" "true"

    run env SNAPSHOT_PATH="$SNAPSHOT" bash "$SCRIPT"

    [ "$status" -eq 1 ]
    [[ "$output" == *"snapshot is"* ]]
}
