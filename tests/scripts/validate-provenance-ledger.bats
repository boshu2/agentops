#!/usr/bin/env bats
# Acceptance surface for ag-x31t.2: the provenance ledger event contract +
# schema (schemas/agentops-sdlc-provenance.v1.schema.json). The validator
# enforces the committed JSON Schema over the tracked fixtures — a
# decision_produces_artifact edge VALIDATES, and an otherwise-identical edge
# missing trust_tier is REJECTED (the bead's named pass/fail case). Each event
# is one line of the hash-chained audit ledger at docs/provenance/ledger.jsonl;
# the committed ledger is the audit authority (council 2026-05-30).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-provenance-ledger.sh"
    SCHEMA="$REPO_ROOT/schemas/agentops-sdlc-provenance.v1.schema.json"
    FIX="$REPO_ROOT/tests/fixtures/provenance"
    if ! command -v python3 >/dev/null 2>&1 || ! python3 -c 'import jsonschema' >/dev/null 2>&1; then
        HAVE_JSONSCHEMA=0
    else
        HAVE_JSONSCHEMA=1
    fi
}

@test "validator and schema exist" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
    [ -f "$SCHEMA" ]
}

@test "schema is valid JSON and forbids extra properties at the event level" {
    run python3 -c "import json,sys; s=json.load(open('$SCHEMA')); sys.exit(0 if s.get('additionalProperties') is False else 1)"
    [ "$status" -eq 0 ]
}

@test "schema requires the edge + hash-chain fields" {
    run python3 -c "import json,sys; s=json.load(open('$SCHEMA')); req=set(s['required']); need={'from_id','from_type','to_id','to_type','relation','trust_tier','ts','prev_hash','payload_hash','hash'}; sys.exit(0 if need <= req else 1)"
    [ "$status" -eq 0 ]
}

@test "trust_tier enum is exactly authored|inferred|mined" {
    run python3 -c "import json,sys; s=json.load(open('$SCHEMA')); sys.exit(0 if sorted(s['properties']['trust_tier']['enum'])==['authored','inferred','mined'] else 1)"
    [ "$status" -eq 0 ]
}

@test "decision_produces_artifact is a valid relation" {
    run python3 -c "import json,sys; s=json.load(open('$SCHEMA')); sys.exit(0 if 'decision_produces_artifact' in s['\$defs']['relation']['enum'] else 1)"
    [ "$status" -eq 0 ]
}

@test "fixtures exist (valid decision edge + invalid missing-trust_tier)" {
    [ -f "$FIX/valid-decision-produces-artifact.json" ]
    [ -f "$FIX/invalid-missing-trust-tier.json" ]
}

@test "selftest validates the fixtures pass/fail" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    run "$SCRIPT" --selftest
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS: valid-decision-produces-artifact.json"* ]]
    [[ "$output" == *"PASS: invalid-missing-trust-tier.json"* ]]
}

@test "decision_produces_artifact edge validates" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/valid-decision-produces-artifact.json"
    [ "$status" -eq 0 ]
}

@test "an edge missing trust_tier is rejected" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/invalid-missing-trust-tier.json"
    [ "$status" -ne 0 ]
}

@test "an edge with an unknown trust_tier value is rejected" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    tmp="$BATS_TEST_TMPDIR/badtier.json"
    cat > "$tmp" <<'JSON'
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-x31t.2","from_type":"decision","to_id":"schemas/x.json","to_type":"artifact","relation":"decision_produces_artifact","trust_tier":"guessed","ts":"2026-05-31T00:00:00Z","prev_hash":"","payload_hash":"cd137deeb225f94ee884b3703485a0effd56937b57f191a10981cc3cc4d4dcee","hash":"0e7224e46af26b3e7b35cfcad9b4bc9838629ae084e48fe024045ea51a5c9ada"}
JSON
    run "$SCRIPT" "$tmp"
    [ "$status" -ne 0 ]
}

@test "an event carrying an unknown extra property is rejected" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    tmp="$BATS_TEST_TMPDIR/extra.json"
    cat > "$tmp" <<'JSON'
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-x31t.2","from_type":"decision","to_id":"schemas/x.json","to_type":"artifact","relation":"decision_produces_artifact","trust_tier":"authored","ts":"2026-05-31T00:00:00Z","prev_hash":"","payload_hash":"cd137deeb225f94ee884b3703485a0effd56937b57f191a10981cc3cc4d4dcee","hash":"0e7224e46af26b3e7b35cfcad9b4bc9838629ae084e48fe024045ea51a5c9ada","smuggled":"x"}
JSON
    run "$SCRIPT" "$tmp"
    [ "$status" -ne 0 ]
}

@test "a non-hex hash is rejected (chain-field guard)" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    tmp="$BATS_TEST_TMPDIR/badhash.json"
    cat > "$tmp" <<'JSON'
{"schema_version":"agentops-sdlc-provenance.v1","from_id":"ag-x31t.2","from_type":"decision","to_id":"schemas/x.json","to_type":"artifact","relation":"decision_produces_artifact","trust_tier":"authored","ts":"2026-05-31T00:00:00Z","prev_hash":"","payload_hash":"not-a-sha","hash":"0e7224e46af26b3e7b35cfcad9b4bc9838629ae084e48fe024045ea51a5c9ada"}
JSON
    run "$SCRIPT" "$tmp"
    [ "$status" -ne 0 ]
}
