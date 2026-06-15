#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-provenance-feed-health.sh"
    TMP_DIR="$(mktemp -d)"
    LEDGER="$TMP_DIR/ledger.jsonl"
}

teardown() {
    rm -rf "$TMP_DIR"
}

genesis_row() {
    printf '{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-8jf97","from_type":"bead","to_id":"feat/ag-8jf97-provenance-ledger","to_type":"commit","relation":"wasGeneratedBy","evidence_ref":"_beads/issues.jsonl#ag-8jf97","trust_tier":"authored","ts":"2026-06-13T23:55:06Z","prev_hash":"","payload_hash":"52b578a2","hash":"ae78526f"}\n'
}

real_edge() {
    printf '{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-zqqm","from_type":"bead","to_id":"38368750c","to_type":"commit","relation":"wasGeneratedBy","evidence_ref":"commit 38368750c","trust_tier":"inferred","ts":"2026-06-15T10:49:22Z","prev_hash":"ae78526f","payload_hash":"7883ec9b","hash":"171e1efb"}\n'
}

@test "passes when ledger has real edges beyond genesis" {
    genesis_row > "$LEDGER"
    real_edge >> "$LEDGER"

    run env PROVENANCE_LEDGER="$LEDGER" bash "$SCRIPT"

    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"2 edges"* ]]
}

@test "warn-only when ledger has only genesis row (default mode)" {
    genesis_row > "$LEDGER"

    run env PROVENANCE_LEDGER="$LEDGER" bash "$SCRIPT"

    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
    [[ "$output" == *"dead sensor"* ]]
}

@test "strict mode fails when only genesis exists" {
    genesis_row > "$LEDGER"

    run env PROVENANCE_LEDGER="$LEDGER" bash "$SCRIPT" --strict

    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"dead sensor"* ]]
}

@test "strict via env var fails when only genesis exists" {
    genesis_row > "$LEDGER"

    run env PROVENANCE_LEDGER="$LEDGER" AGENTOPS_PROVENANCE_FEED_STRICT=1 bash "$SCRIPT"

    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
}

@test "exits 2 when ledger file is missing" {
    run env PROVENANCE_LEDGER="$TMP_DIR/nonexistent.jsonl" bash "$SCRIPT"

    [ "$status" -eq 2 ]
    [[ "$output" == *"ledger missing"* ]]
}

@test "passes with many edges" {
    genesis_row > "$LEDGER"
    real_edge >> "$LEDGER"
    real_edge >> "$LEDGER"
    real_edge >> "$LEDGER"

    run env PROVENANCE_LEDGER="$LEDGER" bash "$SCRIPT"

    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"4 edges"* ]]
}
