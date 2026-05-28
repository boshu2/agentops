#!/usr/bin/env bats
# Acceptance surface for ag-yma: machine-checkable typed contract for the
# .agents/rpi/next-work.jsonl harvest output. The validator enforces the
# committed JSON Schema (schemas/next-work-{item,batch}.v1.schema.json) and
# names the offending field on every violation.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-next-work.sh"
    FIX="$REPO_ROOT/tests/fixtures/next-work"
}

@test "validator exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "committed JSON Schema files exist and are valid JSON" {
    [ -f "$REPO_ROOT/schemas/next-work-item.v1.schema.json" ]
    [ -f "$REPO_ROOT/schemas/next-work-batch.v1.schema.json" ]
    jq empty "$REPO_ROOT/schemas/next-work-item.v1.schema.json"
    jq empty "$REPO_ROOT/schemas/next-work-batch.v1.schema.json"
}

@test "valid fixture passes in strict mode" {
    run bash "$SCRIPT" --strict "$FIX/valid.jsonl"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "strict mode rejects invalid type and names the field" {
    run bash "$SCRIPT" --strict "$FIX/bad-type.jsonl"
    [ "$status" -ne 0 ]
    [[ "$output" == *"type=bogus"* ]]
}

@test "strict mode rejects invalid severity and names the field" {
    run bash "$SCRIPT" --strict "$FIX/bad-severity.jsonl"
    [ "$status" -ne 0 ]
    [[ "$output" == *"severity=critical"* ]]
}

@test "strict mode rejects an item missing the required title" {
    run bash "$SCRIPT" --strict "$FIX/missing-title.jsonl"
    [ "$status" -ne 0 ]
    [[ "$output" == *"title"* ]]
}

@test "strict mode rejects a completed_run proof_ref with no run_id" {
    run bash "$SCRIPT" --strict "$FIX/bad-proof-ref.jsonl"
    [ "$status" -ne 0 ]
    [[ "$output" == *"proof_ref"* ]]
    [[ "$output" == *"run_id"* ]]
}

@test "strict mode rejects a malformed JSON line and names the line" {
    run bash "$SCRIPT" --strict "$FIX/malformed.jsonl"
    [ "$status" -ne 0 ]
    [[ "$output" == *"malformed JSON"* ]]
}

@test "advisory mode (default) reports violations but exits zero" {
    run bash "$SCRIPT" "$FIX/bad-type.jsonl"
    [ "$status" -eq 0 ]
    [[ "$output" == *"type=bogus"* ]]
}

@test "json output emits a machine-readable verdict" {
    run bash "$SCRIPT" --json --strict "$FIX/bad-type.jsonl"
    [ "$status" -ne 0 ]
    echo "$output" | jq -e '.valid == false'
    echo "$output" | jq -e '.violations | length >= 1'
}

@test "json output marks a clean file valid" {
    run bash "$SCRIPT" --json "$FIX/valid.jsonl"
    [ "$status" -eq 0 ]
    echo "$output" | jq -e '.valid == true'
}

@test "passes gracefully when the queue file is absent" {
    run bash "$SCRIPT" --strict "$BATS_TEST_TMPDIR/missing.jsonl"
    [ "$status" -eq 0 ]
    [[ "$output" == *"not present"* ]] || [[ "$output" == *"PASS"* ]]
}

@test "unknown flag returns a usage hint" {
    run bash "$SCRIPT" --bogus-flag "$FIX/valid.jsonl"
    [ "$status" -ne 0 ]
    [[ "$output" == *"unknown"* ]] || [[ "$output" == *"Usage"* ]]
}
