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

@test "prompt emits a small dispatch prompt for a validated packet" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new dispatch-packet \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Emit a dispatch prompt for goal APIs" \
        --scenario-name "Emit a dispatch prompt for goal APIs" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/dispatch-packet"

    sed -i.bak 's/^status: draft$/status: validated/' "$packet/intent.md" "$packet/driver.md"
    rm -f "$packet/intent.md.bak" "$packet/driver.md.bak"
    "$TOOL" refresh-digest "$packet" >/dev/null

    run "$TOOL" prompt "$packet"
    [ "$status" -eq 0 ]
    [[ "$output" == *"$packet/intent.md"* ]]
    [[ "$output" == *"$packet/driver.md"* ]]
    [[ "$output" == *"first_failing_proof"* ]]
    [[ "$output" == *"B1"* ]]
    [ "${#output}" -lt 4000 ]
}

@test "prompt refuses a draft packet without --allow-draft" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new draft-dispatch \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Refuse to dispatch a draft packet" \
        --scenario-name "Refuse to dispatch a draft packet" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/draft-dispatch"

    run "$TOOL" prompt "$packet"
    [ "$status" -ne 0 ]
    [[ "$output" == *"not validated"* ]]

    run "$TOOL" prompt "$packet" --allow-draft
    [ "$status" -eq 0 ]
    [[ "$output" == *"DRAFT"* ]]
}

@test "prompt fails closed when over the max-chars budget" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new oversize-dispatch \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Overflow the dispatch prompt budget" \
        --scenario-name "Overflow the dispatch prompt budget" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/oversize-dispatch"

    run "$TOOL" prompt "$packet" --allow-draft --max-chars 50
    [ "$status" -ne 0 ]
    [[ "$output" == *"max-chars"* ]]
}

@test "prompt cannot raise or disable the goal-API ceiling via --max-chars" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new ceiling-dispatch \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Hold the goal-API prompt ceiling" \
        --scenario-name "Hold the goal-API prompt ceiling" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/ceiling-dispatch"

    run "$TOOL" prompt "$packet" --allow-draft --max-chars 5000
    [ "$status" -ne 0 ]
    [[ "$output" == *"1..4000"* ]]

    run "$TOOL" prompt "$packet" --allow-draft --max-chars 0
    [ "$status" -ne 0 ]
    [[ "$output" == *"1..4000"* ]]
}

@test "prompt refuses a checker-dirty packet (stale digest after intent edit)" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new stale-dispatch \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Refuse to dispatch a stale packet" \
        --scenario-name "Refuse to dispatch a stale packet" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/stale-dispatch"
    sed -i.bak 's/^status: draft$/status: validated/' "$packet/intent.md" "$packet/driver.md"
    rm -f "$packet/intent.md.bak" "$packet/driver.md.bak"
    "$TOOL" refresh-digest "$packet" >/dev/null

    printf '\nStale-making edit.\n' >> "$packet/intent.md"
    run "$TOOL" prompt "$packet"
    [ "$status" -ne 0 ]
    [[ "$output" == *"checker failed"* ]]
}

@test "mark-validated stamps verdict, flips status, refreshes digest" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new marked-packet \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Mark a packet validated with evidence" \
        --scenario-name "Mark a packet validated with evidence" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/marked-packet"

    run "$TOOL" mark-validated "$packet" --verdict "PASS (codex exec cross-family, round 1, 2026-07-09)"
    [ "$status" -eq 0 ]
    [[ "$output" == *"goal-design packet valid"* ]]
    grep -q "^status: validated" "$packet/intent.md"
    grep -q "^status: validated" "$packet/driver.md"
    grep -qF -- "- Last validation verdict: PASS (codex exec cross-family, round 1, 2026-07-09)" "$packet/driver.md"

    run "$TOOL" prompt "$packet"
    [ "$status" -eq 0 ]
}

@test "mark-validated refuses a non-PASS/WARN or empty verdict" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new unmarked-packet \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Refuse weak validation evidence" \
        --scenario-name "Refuse weak validation evidence" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/unmarked-packet"

    run "$TOOL" mark-validated "$packet" --verdict "FAIL blockers remain"
    [ "$status" -ne 0 ]
    [[ "$output" == *"PASS or WARN"* ]]
    grep -q "^status: draft" "$packet/intent.md"
}

@test "new scaffolds the andon router into the driver body" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new andon-packet \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Scaffold the andon router" \
        --scenario-name "Scaffold the andon router" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    driver="$BATS_TEST_TMPDIR/.agents/goal-design/andon-packet/driver.md"
    grep -q "## Andon Router" "$driver"
    grep -q "ao gate check --fast --scope head" "$driver"
    grep -q "refusal lane" "$driver"
}

@test "mark-validated rolls back when the checker rejects the stamped packet" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new rollback-packet \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Never leave a broken packet certified" \
        --scenario-name "Never leave a broken packet certified" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/rollback-packet"

    sed -i.bak 's/^objective: .*$/objective: ""/' "$packet/intent.md"
    rm -f "$packet/intent.md.bak"
    before_intent="$(sha256sum "$packet/intent.md" | awk '{print $1}')"
    before_driver="$(sha256sum "$packet/driver.md" | awk '{print $1}')"

    run "$TOOL" mark-validated "$packet" --verdict "PASS (validator, 2026-07-09)"
    [ "$status" -ne 0 ]
    [[ "$output" == *"restored"* ]]
    [ "$(sha256sum "$packet/intent.md" | awk '{print $1}')" = "$before_intent" ]
    [ "$(sha256sum "$packet/driver.md" | awk '{print $1}')" = "$before_driver" ]
    grep -q "^status: draft" "$packet/intent.md"

    run "$TOOL" mark-validated "$packet" --verdict "PASS (validator)" --no-check
    [ "$status" -ne 0 ]
}

@test "mark-validated leaves a malformed-driver packet byte-identical" {
    if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
    "$TOOL" new malformed-driver \
        --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
        --objective "Never partially certify a packet" \
        --scenario-name "Never partially certify a packet" \
        --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
        --write-scope "scripts/goal-design-packet.py" >/dev/null
    packet="$BATS_TEST_TMPDIR/.agents/goal-design/malformed-driver"

    printf 'not frontmatter at all\n' > "$packet/driver.md"
    before_intent="$(sha256sum "$packet/intent.md" | awk '{print $1}')"
    before_driver="$(sha256sum "$packet/driver.md" | awk '{print $1}')"

    run "$TOOL" mark-validated "$packet" --verdict "PASS (validator, 2026-07-09)"
    [ "$status" -ne 0 ]
    [ "$(sha256sum "$packet/intent.md" | awk '{print $1}')" = "$before_intent" ]
    [ "$(sha256sum "$packet/driver.md" | awk '{print $1}')" = "$before_driver" ]
    grep -q "^status: draft" "$packet/intent.md"
}
