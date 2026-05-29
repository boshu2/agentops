#!/usr/bin/env bats

# Guard layer 4 (ag-hdqu0.6): the deny-by-default holdout-leak gate. An Outcomes
# rubric/score payload must NEVER carry holdout ground truth (Managed Agents are
# not ZDR). check-outcomes-holdout-leak.sh exits nonzero on any payload with a
# target/ground_truth key, zero only when all payloads are clean, and nonzero on
# an unreadable file (deny-by-default).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-outcomes-holdout-leak.sh"
    TMP_DIR="$(mktemp -d)"

    cat > "$TMP_DIR/clean.json" <<'EOF'
{"schema_version":1,"source_task_id":"t1","judge_content_hash":"sha256:abc","criteria":[{"id":"accuracy","description":"names the capital","weight":0.7}]}
EOF
    cat > "$TMP_DIR/leak-target.json" <<'EOF'
{"source_task_id":"t1","criteria":[{"id":"accuracy"}],"target":"Ouagadougou"}
EOF
    cat > "$TMP_DIR/leak-gt.json" <<'EOF'
{"source_task_id":"t1","ground_truth":[{"id":"q1","value":"Antananarivo"}]}
EOF
}

teardown() { rm -rf "$TMP_DIR"; }

@test "clean payload exits zero" {
    run bash "$SCRIPT" "$TMP_DIR/clean.json"
    [ "$status" -eq 0 ]
}

@test "payload with target key exits nonzero" {
    run bash "$SCRIPT" "$TMP_DIR/leak-target.json"
    [ "$status" -ne 0 ]
    [[ "$output" == *"holdout key"* ]]
}

@test "payload with ground_truth key exits nonzero" {
    run bash "$SCRIPT" "$TMP_DIR/leak-gt.json"
    [ "$status" -ne 0 ]
}

@test "one leaking payload among clean ones fails the whole run" {
    run bash "$SCRIPT" "$TMP_DIR/clean.json" "$TMP_DIR/leak-target.json"
    [ "$status" -ne 0 ]
}

@test "unreadable file is denied by default" {
    run bash "$SCRIPT" "$TMP_DIR/does-not-exist.json"
    [ "$status" -ne 0 ]
}

@test "no arguments is a usage error" {
    run bash "$SCRIPT"
    [ "$status" -ne 0 ]
}
