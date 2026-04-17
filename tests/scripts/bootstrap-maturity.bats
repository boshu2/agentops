#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/bootstrap-maturity.sh"

    TMP_DIR="$(mktemp -d)"
    LEARN="$TMP_DIR/learnings"
    mkdir -p "$LEARN"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "bootstrap-maturity.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "bootstrap-maturity.sh preserves existing maturity in JSONL lines" {
    cat > "$LEARN/mixed.jsonl" <<'EOF'
{"id":"a","claim":"thing1"}
{"id":"b","claim":"thing2","maturity":"stable"}
{"id":"c","claim":"thing3"}
EOF

    run bash "$SCRIPT" "$LEARN"
    [ "$status" -eq 0 ]

    # Entry b must still be "stable", not overwritten to "provisional".
    run jq -r 'select(.id=="b") | .maturity' "$LEARN/mixed.jsonl"
    [ "$status" -eq 0 ]
    [ "$output" = "stable" ]

    # Entry a and c must have gained "provisional".
    run jq -r 'select(.id=="a") | .maturity' "$LEARN/mixed.jsonl"
    [ "$output" = "provisional" ]
    run jq -r 'select(.id=="c") | .maturity' "$LEARN/mixed.jsonl"
    [ "$output" = "provisional" ]
}

@test "bootstrap-maturity.sh emits compact JSONL (one object per line)" {
    cat > "$LEARN/compact.jsonl" <<'EOF'
{"id":"x","claim":"plain"}
{"id":"y","claim":"plain2"}
EOF

    run bash "$SCRIPT" "$LEARN"
    [ "$status" -eq 0 ]

    # Line count must equal the original two objects. Pretty-printed output
    # would inflate this to 8+ lines.
    count=$(wc -l < "$LEARN/compact.jsonl" | tr -d ' ')
    [ "$count" -eq 2 ]
}

@test "bootstrap-maturity.sh is idempotent on fully-populated files" {
    cat > "$LEARN/all.jsonl" <<'EOF'
{"id":"x","maturity":"stable"}
{"id":"y","maturity":"provisional"}
EOF

    checksum_before=$(sha256sum "$LEARN/all.jsonl" | awk '{print $1}')
    run bash "$SCRIPT" "$LEARN"
    [ "$status" -eq 0 ]

    checksum_after=$(sha256sum "$LEARN/all.jsonl" | awk '{print $1}')
    [ "$checksum_before" = "$checksum_after" ]
}
