#!/usr/bin/env bats

# Regression tests for scripts/check-bead-scenario-coverage.sh (ag-9jle.4).
#
# This is the LEAF-validator, per-bead scenario->test coverage gate — the
# forward-from-Gherkin sibling of the corpus-wide check-scenario-test-linkage.sh.
# The script resolves REPO_ROOT relative to its own location, so we copy it into
# a self-contained fake repo and resolve @covered-by targets there.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-bead-scenario-coverage.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/tests/e2e" "$FAKE_REPO/work"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/check-bead-scenario-coverage.sh"
    chmod +x "$FAKE_REPO/scripts/check-bead-scenario-coverage.sh"
    FAKE_SCRIPT="$FAKE_REPO/scripts/check-bead-scenario-coverage.sh"

    # A passing covering test (exit 0) with a named function.
    cat > "$FAKE_REPO/tests/e2e/pass.sh" <<'EOF'
#!/usr/bin/env bash
TestCoversScenario() { :; }
exit 0
EOF
    # A failing covering test (exit 1) — present but not passing.
    cat > "$FAKE_REPO/tests/e2e/fail.sh" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Write a bead-body file with a `## Scenarios` block. $1=outfile; rest = lines
# of the form "TAG|||SCENARIO_NAME" (TAG may be empty).
write_bead() {
    local out="$1"; shift
    {
        printf '## Background\nWhy this matters.\n\n## Scenarios\n'
        for pair in "$@"; do
            local tag="${pair%%|||*}" name="${pair##*|||}"
            [ -n "$tag" ] && printf '  %s\n' "$tag"
            printf 'Scenario: %s\n  Given a thing\n  When it runs\n  Then it works\n\n' "$name"
        done
        printf '## Logging\nLog it.\n'
    } > "$out"
}

# Write a bare .feature file. $1=outfile; rest = "TAG|||SCENARIO_NAME".
write_feature() {
    local out="$1"; shift
    {
        printf '# fake spec\n\nFeature: leaf coverage\n\n'
        for pair in "$@"; do
            local tag="${pair%%|||*}" name="${pair##*|||}"
            [ -n "$tag" ] && printf '  %s\n' "$tag"
            printf '  Scenario: %s\n    When x\n    Then y\n\n' "$name"
        done
    } > "$out"
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "PASS: all scenarios in a bead body covered (M==N)" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/pass.sh|||first scenario" \
        "@covered-by:tests/e2e/pass.sh|||second scenario"
    run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"2/2"* ]]
}

@test "FAIL: a scenario lacks any covering test (M<N), not merely 'tests exist'" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/pass.sh|||covered scenario" \
        "|||uncovered scenario"
    run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
    [[ "$output" == *"no covering test"* ]]
    [[ "$output" == *"uncovered scenario"* ]]
}

@test "FAIL: dangling @covered-by (test file missing)" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/missing.sh|||scenario one"
    run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
    [[ "$output" == *"dangling"* ]]
    [[ "$output" == *"test path does not exist"* ]]
}

@test "FAIL: dangling @covered-by path::Name (name missing in file)" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/pass.sh::TestDoesNotExist|||named dangling"
    run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
    [[ "$output" == *"not found in"* ]]
}

@test "PASS: @covered-by path::Name resolves when the name exists" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/pass.sh::TestCoversScenario|||named cover"
    run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
}

@test "--run PASS: covering test present AND exits 0" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/pass.sh|||runnable scenario"
    run bash "$FAKE_SCRIPT" --run "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
    [[ "$output" == *"passing"* ]]
}

@test "--run FAIL: covering test present but exits non-zero (not passing)" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/fail.sh|||failing scenario"
    run bash "$FAKE_SCRIPT" --run "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
    [[ "$output" == *"did not pass"* ]]
}

@test "without --run, a present-but-failing test still counts as covered" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/fail.sh|||present scenario"
    run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
}

@test "bare .feature file: scenarios are in scope without a ## Scenarios heading" {
    write_feature "$FAKE_REPO/work/x.feature" \
        "@covered-by:tests/e2e/pass.sh|||feature scenario one" \
        "@covered-by:tests/e2e/pass.sh|||feature scenario two"
    run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/x.feature"
    [ "$status" -eq 0 ]
    [[ "$output" == *"2/2"* ]]
}

@test "scenarios outside the ## Scenarios block are not counted" {
    # A Scenario: line under a later H2 must not be counted.
    {
        printf '## Scenarios\n'
        printf '@covered-by:tests/e2e/pass.sh\n'
        printf 'Scenario: real scenario\n  Then y\n\n'
        printf '## Notes\n'
        printf 'Scenario: not a real scenario, just prose\n'
    } > "$FAKE_REPO/work/bead.md"
    run bash "$FAKE_SCRIPT" --json "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"scenarios_total":1'* ]]
}

@test "--min-covered threshold below N still passes when met" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/pass.sh|||covered scenario" \
        "|||uncovered scenario"
    run bash "$FAKE_SCRIPT" --min-covered=1 "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
}

@test "--json emits a machine-readable summary" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/pass.sh|||covered scenario"
    run bash "$FAKE_SCRIPT" --json "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"result":"pass"'* ]]
    [[ "$output" == *'"scenarios_covered":1'* ]]
}

@test "--json reports fail with covered<total" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/pass.sh|||covered scenario" \
        "|||uncovered scenario"
    run bash "$FAKE_SCRIPT" --json "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
    [[ "$output" == *'"result":"fail"'* ]]
    [[ "$output" == *'"scenarios_total":2'* ]]
    [[ "$output" == *'"scenarios_covered":1'* ]]
}

@test "--warn-only downgrades a failure to exit 0" {
    write_bead "$FAKE_REPO/work/bead.md" "|||uncovered scenario"
    run bash "$FAKE_SCRIPT" --warn-only "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
}

@test "stdin source via '-'" {
    write_bead "$FAKE_REPO/work/bead.md" \
        "@covered-by:tests/e2e/pass.sh|||piped scenario"
    run bash -c "cat '$FAKE_REPO/work/bead.md' | bash '$FAKE_SCRIPT' -"
    [ "$status" -eq 0 ]
    [[ "$output" == *"1/1"* ]]
}

@test "misuse: missing source exits 2" {
    run bash "$FAKE_SCRIPT"
    [ "$status" -eq 2 ]
    [[ "$output" == *"no source given"* ]]
}

@test "misuse: nonexistent source file exits 2" {
    run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/nope.md"
    [ "$status" -eq 2 ]
    [[ "$output" == *"does not exist"* ]]
}

@test "misuse: unknown flag exits 2" {
    run bash "$FAKE_SCRIPT" --bogus "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 2 ]
}

# --- --admission mode (ag-iruq3.1) -------------------------------------------
# Plan-time admission gate: PASS iff >=1 structurally complete scenario unit
# (Scenario:/Scenario Outline: block OR contiguous bare-GWT stanza). Fixtures
# below are REAL bead-body-shaped markdown (## sections, fenced yaml
# acceptance_criteria blocks) per the fixture-fidelity rule.

@test "admission: bead body without Scenarios block is inadmissible" {
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
## Context

Audit (2026-06-12) found acceptance enforcement is prose-only everywhere.
The mechanical tool exists but only runs at validate time.

## Approach

Add the flag to the parse loop; keep existing modes byte-identical.

## Notes

Free-text acceptance: the script should work correctly when done.
EOF
    run bash "$FAKE_SCRIPT" --admission "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
    [[ "$output" == *"## Scenarios"* ]]
}

@test "admission: free-text acceptance (heading but zero units) is inadmissible" {
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
## Context

Migrate the --bead mode off the retired tracker.

## Scenarios

The script should fetch the bead body via the new tracker and fail
gracefully when it is absent. All existing behavior stays the same.

## Logging

Log it.
EOF
    run bash "$FAKE_SCRIPT" --admission "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
}

@test "admission: Scenario-header GWT without covered-by passes" {
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
## Context

Plan-time: tests are not written yet, so no @covered-by tags exist.

## Scenarios

Scenario: gate rejects beads without runnable acceptance
  Given a bead body with no Scenarios block
  When the admission gate runs
  Then it exits 1 naming the bead

## Logging

Log it.
EOF
    run bash "$FAKE_SCRIPT" --admission "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
}

@test "admission: bare-GWT stanza without Scenario header passes" {
    # Real corpus style (~97 bodies): contiguous Given/When/Then lines with no
    # Scenario: header, plus the fenced yaml acceptance block bead bodies carry.
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
Add plan-time admission semantics to the leaf gate; migrate --bead off retired bd.

## Scenarios

Given a bead body using bare-GWT stanzas without Scenario headers
When the admission gate runs
And both corpus styles are considered
Then it exits 0

```yaml
acceptance_criteria:
  - id: ac-1
    description: "admission accepts both corpus styles"
    check_type: command_exit_zero
```
EOF
    run bash "$FAKE_SCRIPT" --admission "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
}

@test "admission: blank line inside a Scenario block does not split the unit" {
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
## Scenarios

Scenario: blank line is body formatting, not a unit boundary
  Given a Scenario block with a blank line between its lines

  Then it still counts as one complete unit
EOF
    run bash "$FAKE_SCRIPT" --admission --json "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"units_total":1'* ]]
    [[ "$output" == *'"structurally_complete":1'* ]]
}

@test "admission: structurally incomplete LAST unit is inadmissible" {
    # The final unit lacks a Then line — exercises the EOF verdict flush (F7).
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
## Scenarios

Scenario: complete unit lands first
  Given a thing
  When it runs
  Then it works

Scenario: final unit has no Then
  Given a dangling setup
  When nothing is asserted
EOF
    run bash "$FAKE_SCRIPT" --admission "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
}

@test "admission: missing When warns but passes" {
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
## Scenarios

Scenario: declarative unit with no action line
  Given a precondition
  Then the postcondition holds
EOF
    run bash "$FAKE_SCRIPT" --admission --json "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 0 ]
    [[ "$output" == *"WARN"* ]]
    [[ "$output" == *'"warnings":1'* ]]
}

@test "admission: fenced content is parse-inert" {
    # Only fenced 'Scenario:'/GWT text — must NOT count as a unit (F4).
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
## Scenarios

```yaml
acceptance_criteria:
  - id: ac-1
    description: |
      Scenario: phantom unit inside a fence
        Given a fenced line
        Then it must not be parsed
```
EOF
    run bash "$FAKE_SCRIPT" --admission --json "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
    [[ "$output" == *'"units_total":0'* ]]
}

@test "admission: empty input is infra failure not policy rejection (exit 2)" {
    run bash -c "printf '' | bash '$FAKE_SCRIPT' --admission -"
    [ "$status" -eq 2 ]
    [[ "$output" == *"no content"* ]]
}

@test "admission: --run with --admission is misuse exit 2" {
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
## Scenarios

Scenario: any
  Given a thing
  Then it works
EOF
    run bash "$FAKE_SCRIPT" --admission --run "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 2 ]
    [[ "$output" == *"cannot be combined"* ]]
}

@test "admission: zero-scenario loophole is closed (--min-covered=0 still fails)" {
    cat > "$FAKE_REPO/work/bead.md" <<'EOF'
## Scenarios

Acceptance is described in free text only; no GWT units here.
EOF
    run bash "$FAKE_SCRIPT" --admission --min-covered=0 "$FAKE_REPO/work/bead.md"
    [ "$status" -eq 1 ]
}
