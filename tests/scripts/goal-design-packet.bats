#!/usr/bin/env bats
# Acceptance surface for digest-safe goal-design packet authoring helpers.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TOOL="$REPO_ROOT/scripts/goal-design-packet.py"
    CHECKER="$REPO_ROOT/scripts/check-goal-design-packet.sh"
    if ! command -v python3 >/dev/null 2>&1 || ! python3 -c 'import yaml, jsonschema' >/dev/null 2>&1; then
        HAVE_SCHEMA_DEPS=0
    else
        HAVE_SCHEMA_DEPS=1
    fi
}

@test "new creates a checker-clean packet with matching digest" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    run "$TOOL" new generated-packet \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Create a digest-safe goal-design packet" \
        --scenario-name "Create a digest-safe goal-design packet" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py"
    [ "$status" -eq 0 ]
    [[ "$output" == *"goal-design packet valid"* ]]

    intent="$BATS_TEST_TMPDIR/.agents/goal-design/generated-packet/intent.md"
    driver="$BATS_TEST_TMPDIR/.agents/goal-design/generated-packet/driver.md"
    expected="$(sha256sum "$intent" | awk '{print $1}')"
    run grep -F "sha256: $expected" "$driver"
    [ "$status" -eq 0 ]
}

@test "refresh-digest repairs a stale driver after intent edit" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new stale-packet \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Repair a stale goal-design digest" \
        --scenario-name "Repair a stale goal-design digest" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/stale-packet"

    printf '\nDigest-changing edit.\n' >> "$packet/intent.md"
    run "$CHECKER" "$packet"
    [ "$status" -ne 0 ]
    [[ "$output" == *"driver intent_ref.sha256 is stale"* ]]

    run "$TOOL" refresh-digest "$packet"
    [ "$status" -eq 0 ]
    [[ "$output" == *"goal-design packet valid"* ]]
}

@test "check delegates to the canonical packet checker" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    run "$TOOL" check "$REPO_ROOT/tests/fixtures/goal-design/mismatched-slug"
    [ "$status" -ne 0 ]
    [[ "$output" == *"slug mismatch"* ]]
}
