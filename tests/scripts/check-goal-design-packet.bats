#!/usr/bin/env bats
# Acceptance surface for goal-design artifacts: schema-backed intent.md and
# driver.md packets, including driver-to-intent digest integrity.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-goal-design-packet.sh"
    INTENT_SCHEMA="$REPO_ROOT/schemas/goal-design-intent.v1.schema.json"
    DRIVER_SCHEMA="$REPO_ROOT/schemas/goal-design-driver.v1.schema.json"
    FIX="$REPO_ROOT/tests/fixtures/goal-design"
    if ! command -v python3 >/dev/null 2>&1 || ! python3 -c 'import yaml, jsonschema' >/dev/null 2>&1; then
        HAVE_SCHEMA_DEPS=0
    else
        HAVE_SCHEMA_DEPS=1
    fi
}

@test "checker and schemas exist" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
    [ -f "$INTENT_SCHEMA" ]
    [ -f "$DRIVER_SCHEMA" ]
}

@test "schemas are strict JSON Schema documents" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    run python3 - "$INTENT_SCHEMA" "$DRIVER_SCHEMA" <<'PY'
import json
import sys
from pathlib import Path
from jsonschema import Draft202012Validator

for path in sys.argv[1:]:
    schema = json.loads(Path(path).read_text(encoding="utf-8"))
    Draft202012Validator.check_schema(schema)
    assert schema["$schema"] == "https://json-schema.org/draft/2020-12/schema"
    assert schema["additionalProperties"] is False
PY
    [ "$status" -eq 0 ]
}

@test "valid goal-design packet passes" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/valid"
    [ "$status" -eq 0 ]
    [[ "$output" == *"goal-design packet valid"* ]]
}

@test "missing BDD behavior fails" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/missing-bdd"
    [ "$status" -ne 0 ]
    [[ "$output" == *"intent.md schema violation"* ]]
    [[ "$output" == *"bdd"* ]]
}

@test "missing first failing proof fails" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/missing-first-failing-proof"
    [ "$status" -ne 0 ]
    [[ "$output" == *"driver.md schema violation"* ]]
    [[ "$output" == *"first_failing_proof"* ]]
}

@test "missing route-back rules fail" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/missing-route-back-rules"
    [ "$status" -ne 0 ]
    [[ "$output" == *"driver.md schema violation"* ]]
    [[ "$output" == *"route_back_rules"* ]]
}

@test "stale driver intent digest fails" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/stale-driver-digest"
    [ "$status" -ne 0 ]
    [[ "$output" == *"driver intent_ref.sha256 is stale"* ]]
}

@test "self-grading language fails" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    run "$SCRIPT" "$FIX/self-grading-language"
    [ "$status" -ne 0 ]
    [[ "$output" == *"self-grading"* ]]
}
