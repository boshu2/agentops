#!/usr/bin/env bats
# Acceptance surface for ag-hguuf: the Outcomes-rubric projection contract +
# schema. The validator enforces the committed JSON Schema
# (schemas/outcomes-rubric.v1.schema.json) over the three tracked fixtures with
# the expected pass/pass/fail, and rejects any payload that smuggles a leak
# field (target / ground_truth / expected_output) — the holdout-isolation
# invariant at the cloud boundary (Managed Agents are not ZDR).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-outcomes-rubric.sh"
    SCHEMA="$REPO_ROOT/schemas/outcomes-rubric.v1.schema.json"
    FIX="$REPO_ROOT/tests/fixtures/outcomes-rubric"
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

@test "schema is valid JSON and forbids extra properties at every level" {
    run python3 -c "import json,sys; s=json.load(open('$SCHEMA')); sys.exit(0 if s.get('additionalProperties') is False and s['properties']['criteria']['items'].get('additionalProperties') is False else 1)"
    [ "$status" -eq 0 ]
}

@test "three fixtures exist (valid-dev, valid-holdout-criteria-only, invalid-contains-target)" {
    [ -f "$FIX/valid-dev.json" ]
    [ -f "$FIX/valid-holdout-criteria-only.json" ]
    [ -f "$FIX/invalid-contains-target.json" ]
}

@test "selftest validates the three fixtures pass/pass/fail" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    run "$SCRIPT" --selftest
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS: valid-dev.json"* ]]
    [[ "$output" == *"PASS: valid-holdout-criteria-only.json"* ]]
    [[ "$output" == *"PASS: invalid-contains-target.json"* ]]
}

@test "valid-dev fixture validates" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/valid-dev.json"
    [ "$status" -eq 0 ]
}

@test "valid-holdout-criteria-only fixture validates (no instructions)" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/valid-holdout-criteria-only.json"
    [ "$status" -eq 0 ]
}

@test "invalid-contains-target fixture is rejected (holdout leak field)" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/invalid-contains-target.json"
    [ "$status" -ne 0 ]
}

@test "a payload smuggling ground_truth is rejected" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    tmp="$BATS_TEST_TMPDIR/leak.json"
    cat > "$tmp" <<'JSON'
{"schema_version":1,"source_task_id":"t","judge_content_hash":"sha256:aa","ground_truth":"secret","criteria":[{"id":"a","description":"d","weight":1.0}]}
JSON
    run "$SCRIPT" "$tmp"
    [ "$status" -ne 0 ]
}

@test "a criterion smuggling expected_output is rejected (every-level guard)" {
    if [ "$HAVE_JSONSCHEMA" -eq 0 ]; then skip "python3 jsonschema unavailable"; fi
    tmp="$BATS_TEST_TMPDIR/leak2.json"
    cat > "$tmp" <<'JSON'
{"schema_version":1,"source_task_id":"t","judge_content_hash":"sha256:aa","criteria":[{"id":"a","description":"d","weight":1.0,"expected_output":"42"}]}
JSON
    run "$SCRIPT" "$tmp"
    [ "$status" -ne 0 ]
}
