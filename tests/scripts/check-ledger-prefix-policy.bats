#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-ledger-prefix-policy.sh"
    TMP_DIR="$(mktemp -d)"
    LEDGER="$TMP_DIR/issues.jsonl"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "warns and reports foreign prefix counts without blocking" {
    cat > "$LEDGER" <<'JSONL'
{"id":"ag-good","title":"native"}
{"id":"soc-foreign","title":"legacy migrated"}
{"id":"ag-another","title":"native"}
JSONL

    run env LEDGER_PREFIX_POLICY_LEDGER="$LEDGER" bash "$SCRIPT"

    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
    [[ "$output" == *"1 foreign-prefix bead id(s)"* ]]
    [[ "$output" == *"soc-foreign"* ]]
    [[ "$output" == *"soc- 1"* ]]
}

@test "passes cleanly when every id uses ag prefix" {
    cat > "$LEDGER" <<'JSONL'
{"id":"ag-one","title":"native"}
{"id":"ag-two","title":"native"}
JSONL

    run env LEDGER_PREFIX_POLICY_LEDGER="$LEDGER" bash "$SCRIPT"

    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"all 2 bead id(s) use ag- prefix"* ]]
}

@test "skips gracefully when issues jsonl is absent" {
    run bash -c "cd '$TMP_DIR' && '$SCRIPT'"

    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP"* ]]
    [[ "$output" == *"_beads/issues.jsonl"* ]]
    [[ "$output" == *"gitignored"* ]]
}
