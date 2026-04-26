#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-next-work-schema-rows.sh"

    TMP_DIR="$(mktemp -d)"
    QUEUE="$TMP_DIR/next-work.jsonl"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "passes when queue is missing" {
    QUEUE="$TMP_DIR/missing.jsonl"
    run env QUEUE="$QUEUE" bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"not present"* ]]
}

@test "passes a clean v1.3 batch row" {
    cat > "$QUEUE" <<'EOF'
{"source_epic":"e1","timestamp":"2026-04-26T00:00:00Z","items":[{"title":"x","type":"tech-debt","severity":"high","source":"council-finding","description":"d"}],"consumed":false,"claim_status":"available"}
EOF
    run env QUEUE="$QUEUE" bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "rejects severity not in {low,medium,high}" {
    cat > "$QUEUE" <<'EOF'
{"source_epic":"e1","timestamp":"2026-04-26T00:00:00Z","items":[{"title":"bad","type":"tech-debt","severity":"critical","source":"council-finding","description":"d"}],"consumed":false,"claim_status":"available"}
EOF
    run env QUEUE="$QUEUE" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"severity=critical"* ]]
}

@test "rejects type not in schema enum" {
    cat > "$QUEUE" <<'EOF'
{"source_epic":"e1","timestamp":"2026-04-26T00:00:00Z","items":[{"title":"bad","type":"docs","severity":"medium","source":"council-finding","description":"d"}],"consumed":false,"claim_status":"available"}
EOF
    run env QUEUE="$QUEUE" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"type=docs"* ]]
}

@test "rejects source not in schema enum" {
    cat > "$QUEUE" <<'EOF'
{"source_epic":"e1","timestamp":"2026-04-26T00:00:00Z","items":[{"title":"bad","type":"tech-debt","severity":"medium","source":"post-mortem","description":"d"}],"consumed":false,"claim_status":"available"}
EOF
    run env QUEUE="$QUEUE" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"source=post-mortem"* ]]
}

@test "rejects legacy flat row (no items array)" {
    cat > "$QUEUE" <<'EOF'
{"title":"legacy","type":"tech-debt","severity":"medium","source":"council-finding","description":"flat"}
EOF
    run env QUEUE="$QUEUE" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"legacy flat row"* ]]
}

@test "rejects malformed JSON line" {
    printf '%s\n' '{not json' > "$QUEUE"
    run env QUEUE="$QUEUE" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"malformed JSON"* ]]
}

@test "rejects bad item claim_status value" {
    cat > "$QUEUE" <<'EOF'
{"source_epic":"e1","timestamp":"2026-04-26T00:00:00Z","items":[{"title":"x","type":"tech-debt","severity":"medium","source":"council-finding","description":"d","claim_status":"claimed"}],"consumed":false,"claim_status":"available"}
EOF
    run env QUEUE="$QUEUE" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"claim_status=claimed"* ]]
}

@test "skips empty lines" {
    printf '\n\n' > "$QUEUE"
    cat >> "$QUEUE" <<'EOF'
{"source_epic":"e1","timestamp":"2026-04-26T00:00:00Z","items":[{"title":"x","type":"tech-debt","severity":"high","source":"council-finding","description":"d"}],"consumed":false,"claim_status":"available"}
EOF
    run env QUEUE="$QUEUE" bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}
