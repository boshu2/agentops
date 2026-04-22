#!/usr/bin/env bats
#
# Covers scripts/check-dream-throughput.sh — the gate that detects the
# chicken-and-egg close-loop deadlock documented in
# .agents/learnings/2026-04-22-close-loop-citation-gate-deadlock.md.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-dream-throughput.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "check-dream-throughput.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "PASS when ingested candidates are auto-promoted" {
    cat > "$TMP_DIR/close-loop.json" <<'EOF'
{"ingest":{"added":5},"auto_promote":{"promoted":3}}
EOF
    run bash "$SCRIPT" "$TMP_DIR/close-loop.json"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"3"* ]]
    [[ "$output" == *"5"* ]]
}

@test "OK when nothing was ingested this run" {
    cat > "$TMP_DIR/close-loop.json" <<'EOF'
{"ingest":{"added":0},"auto_promote":{"promoted":0}}
EOF
    run bash "$SCRIPT" "$TMP_DIR/close-loop.json"
    [ "$status" -eq 0 ]
    [[ "$output" == *"no candidates ingested"* ]]
}

@test "FAIL on the deadlock pattern: added>0 but promoted==0" {
    cat > "$TMP_DIR/close-loop.json" <<'EOF'
{"ingest":{"added":43},"auto_promote":{"promoted":0}}
EOF
    run bash "$SCRIPT" "$TMP_DIR/close-loop.json"
    [ "$status" -eq 1 ]
    [[ "$output" == *"stall"* ]]
    [[ "$output" == *"43"* ]]
}

@test "ERROR (exit 2) when input file is missing" {
    run bash "$SCRIPT" "$TMP_DIR/does-not-exist.json"
    [ "$status" -eq 2 ]
    [[ "$output" == *"not found"* ]]
}

@test "ERROR (exit 2) when no argument given" {
    run bash "$SCRIPT"
    [ "$status" -eq 2 ]
    [[ "$output" == *"usage"* ]]
}

@test "ERROR (exit 2) when JSON keys are not integers" {
    cat > "$TMP_DIR/close-loop.json" <<'EOF'
{"ingest":{"added":"many"},"auto_promote":{"promoted":0}}
EOF
    run bash "$SCRIPT" "$TMP_DIR/close-loop.json"
    [ "$status" -eq 2 ]
    [[ "$output" == *"integers"* ]]
}

@test "PASS defaults when keys are missing (treated as 0)" {
    echo '{}' > "$TMP_DIR/close-loop.json"
    run bash "$SCRIPT" "$TMP_DIR/close-loop.json"
    [ "$status" -eq 0 ]
    [[ "$output" == *"no candidates ingested"* ]]
}
