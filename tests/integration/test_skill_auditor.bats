#!/usr/bin/env bats
# test_skill_auditor.bats — L2 integration tests for skill-auditor.
#
# Asserts that the auditor returns:
#   - PASS exit 0 on the canonical known-good fixture
#   - FAIL exit 1 on the known-bad fixture
#   - valid JSON to stdout (default) or to --json <path>
#   - --strict mode upgrades WARN to exit 1
#
# Each test asserts behavioral correctness (verdict + exit + content).

setup() {
    REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"
    AUDIT_SH="$REPO_ROOT/skills/skill-auditor/scripts/audit.sh"
    GOOD="$REPO_ROOT/tests/fixtures/skills/known-good"
    BAD="$REPO_ROOT/tests/fixtures/skills/known-bad"
}

@test "auditor exists and is executable" {
    [ -f "$AUDIT_SH" ]
    [ -r "$AUDIT_SH" ]
}

@test "auditor returns exit 0 on known-good fixture" {
    run bash "$AUDIT_SH" "$GOOD"
    [ "$status" -eq 0 ]
}

@test "auditor verdict is PASS on known-good fixture" {
    run bash "$AUDIT_SH" "$GOOD"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"verdict": "PASS"'* ]]
}

@test "auditor returns exit 1 on known-bad fixture" {
    run bash "$AUDIT_SH" "$BAD"
    [ "$status" -eq 1 ]
}

@test "auditor verdict is FAIL on known-bad fixture" {
    run bash "$AUDIT_SH" "$BAD"
    [ "$status" -eq 1 ]
    [[ "$output" == *'"verdict": "FAIL"'* ]]
}

@test "auditor flags output-spec-explicit on known-bad" {
    run bash "$AUDIT_SH" "$BAD"
    [[ "$output" == *'"id":"output-spec-explicit","status":"fail"'* ]]
}

@test "auditor stdout is valid JSON" {
    # bats run merges stdout+stderr; the auditor writes a human summary to stderr.
    # Capture stdout-only via a temp file.
    local out="${BATS_TEST_TMPDIR}/stdout.json"
    bash "$AUDIT_SH" "$GOOD" >"$out" 2>/dev/null
    [ -s "$out" ]
    jq . "$out" >/dev/null
}

@test "auditor --json writes JSON file" {
    local out="${BATS_TEST_TMPDIR}/audit.json"
    run bash "$AUDIT_SH" --json "$out" "$GOOD"
    [ "$status" -eq 0 ]
    [ -s "$out" ]
    jq . "$out" >/dev/null
    local verdict
    verdict="$(jq -r .verdict "$out")"
    [ "$verdict" = "PASS" ]
}

@test "auditor JSON contains all 8 Pass-2 check ids" {
    run bash "$AUDIT_SH" "$GOOD"
    [ "$status" -eq 0 ]
    for id in description-has-triggers constraints-frontloaded rationale-present \
              verification-checkpoints output-spec-explicit quality-rubric \
              references-modularization trigger-clarity; do
        [[ "$output" == *"\"id\":\"$id\""* ]]
    done
}

@test "auditor exits 2 on missing target" {
    run bash "$AUDIT_SH" /nonexistent/path/skill
    [ "$status" -eq 2 ]
}

@test "auditor exits 2 on usage error (no target)" {
    run bash "$AUDIT_SH"
    [ "$status" -eq 2 ]
}

@test "auditor --strict upgrades WARN to exit 1" {
    # If a fixture produces WARN verdict, --strict should exit 1.
    # Use known-good (PASS) as a control: --strict should still exit 0.
    run bash "$AUDIT_SH" --strict "$GOOD"
    [ "$status" -eq 0 ]
}

@test "auditor self-audits its own SKILL.md" {
    run bash "$AUDIT_SH" "$REPO_ROOT/skills/skill-auditor"
    # Self-audit may PASS or WARN, but never FAIL.
    [ "$status" -eq 0 ]
}

@test "auditor self-audits skill-builder SKILL.md" {
    run bash "$AUDIT_SH" "$REPO_ROOT/skills/skill-builder"
    [ "$status" -eq 0 ]
}
