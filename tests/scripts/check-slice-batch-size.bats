#!/usr/bin/env bats
#
# Tests for scripts/check-slice-batch-size.sh (age-74yi) — the small-batch-by-
# Gherkin ENFORCEMENT gate: one slice bead == one behavior == one Gherkin
# scenario, fail-on-multi.
#
# The enforcement half of the flywheel discipline
# (docs/architecture/the-flywheel.md): the discovery/behavior-first skills SAY
# "seed slices small / one scenario per slice" but nothing FAILS on a
# multi-behavior slice. This gate makes the batch unit COUNTABLE.
#
# Tracker-agnostic: the script reads the bead body via `ao beads exec show <id>
# --json`. These tests STUB `ao` on PATH (a fake that emits canned JSON) so the
# suite never depends on a live tracker — mirroring how the other script tests
# stub `ao`/`br`.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-slice-batch-size.sh"

    TMP_DIR="$(mktemp -d)"
    STUB_JSON="$TMP_DIR/bead.json"
    export STUB_JSON
    STUB_READY_JSON="$TMP_DIR/ready.json"
    export STUB_READY_JSON

    # Fake `ao` on PATH: `ao beads exec show <id> --json` cats the canned JSON.
    FAKE_BIN="$TMP_DIR/bin"
    mkdir -p "$FAKE_BIN"
    cat > "$FAKE_BIN/ao" <<'STUB'
#!/usr/bin/env bash
# Fake `ao` for check-slice-batch-size tests. Only implements the two verbs the
# script uses: `beads exec show <id> --json` and `beads dir`.
if [[ "$1" == "beads" && "$2" == "dir" ]]; then
    echo "${STUB_BEADS_DIR:-/tmp}"
    exit 0
fi
if [[ "$1" == "beads" && "$2" == "exec" && "$3" == "ready" ]]; then
    if [[ -f "$STUB_READY_JSON" ]]; then
        cat "$STUB_READY_JSON"
        exit 0
    fi
    exit 1
fi
if [[ "$1" == "beads" && "$2" == "exec" && "$3" == "show" ]]; then
    if [[ -f "$STUB_JSON" ]]; then
        cat "$STUB_JSON"
        exit 0
    fi
    exit 1
fi
echo "fake ao: unhandled invocation: $*" >&2
exit 2
STUB
    chmod +x "$FAKE_BIN/ao"
    export PATH="$FAKE_BIN:$PATH"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Build the canned `ao beads exec show --json` payload (a JSON array, exactly
# what a live `br`/`bd` passthrough emits) from a raw body file. jq handles all
# JSON escaping of the Gherkin body.
make_bead() {
    local id="$1" bodyfile="$2"
    jq -n --arg id "$id" --arg desc "$(cat "$bodyfile")" \
        '[{id:$id,title:"a slice",description:$desc,status:"open",priority:1,issue_type:"task"}]' \
        > "$STUB_JSON"
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

# --- Acceptance scenario 1: a multi-behavior slice FAILS ----------------------
@test "FAIL: a slice bead with TWO Gherkin scenarios fails and directs a split" {
    cat > "$TMP_DIR/body.md" <<'EOF'
## Context

This slice tried to do two things at once.

## Scenarios

Scenario: the check flags a multi-behavior slice
  Given a bead body with two scenarios
  When the batch check runs
  Then it fails

Scenario: the check passes a single-behavior slice
  Given a bead body with one scenario
  When the batch check runs
  Then it passes
EOF
    make_bead "age-multi" "$TMP_DIR/body.md"
    run bash "$SCRIPT" age-multi
    [ "$status" -eq 1 ]
    [[ "$output" == *"SLICE-BATCH: FAIL"* ]]
    [[ "$output" == *"age-multi"* ]]
    [[ "$output" == *"2 behaviors"* ]]
    [[ "$output" == *"split"* ]]
    # Names the detected scenarios so the operator knows where to cut.
    [[ "$output" == *"the check flags a multi-behavior slice"* ]]
    [[ "$output" == *"the check passes a single-behavior slice"* ]]
}

@test "FAIL: two bare Given/When/Then stanzas (no Scenario: headers) also fail" {
    cat > "$TMP_DIR/body.md" <<'EOF'
Prose intro describing the slice.

Given the first behavior's precondition
When the first action happens
Then the first outcome holds

Given the second behavior's precondition
When the second action happens
Then the second outcome holds
EOF
    make_bead "age-bare2" "$TMP_DIR/body.md"
    run bash "$SCRIPT" age-bare2
    [ "$status" -eq 1 ]
    [[ "$output" == *"SLICE-BATCH: FAIL"* ]]
    [[ "$output" == *"2 behaviors"* ]]
}

# --- Acceptance scenario 2: a single-behavior slice PASSES --------------------
@test "PASS: a slice with exactly one Given/When/Then (+ edge/AND lines) passes" {
    cat > "$TMP_DIR/body.md" <<'EOF'
## Context

One behavior, sliced thin.

## Scenarios

Scenario: the check passes a one-behavior slice
  Given a bead body with a single scenario
  And an executed-red acceptance test tests/scripts/check-slice-batch-size.bats
  When the batch check runs
  Then it passes with exit 0
EOF
    make_bead "age-one" "$TMP_DIR/body.md"
    run bash "$SCRIPT" age-one
    [ "$status" -eq 0 ]
    [[ "$output" == *"SLICE-BATCH: PASS"* ]]
    [[ "$output" == *"age-one"* ]]
}

@test "PASS: one bare Given/When/Then stanza (no Scenario: header) passes" {
    cat > "$TMP_DIR/body.md" <<'EOF'
Add the flag to the parse loop.

Given a bead body using a single bare-GWT stanza
When the batch check runs
Then it passes
EOF
    make_bead "age-bare1" "$TMP_DIR/body.md"
    run bash "$SCRIPT" age-bare1
    [ "$status" -eq 0 ]
    [[ "$output" == *"SLICE-BATCH: PASS"* ]]
}

# --- 0-scenario decision: WARN (not a hard FAIL) ------------------------------
@test "WARN: a task bead with no Gherkin scenario warns (exit 0), never a hard FAIL" {
    cat > "$TMP_DIR/body.md" <<'EOF'
## Context

A plain task bead with prose acceptance only — common in the tracker today.
No Given/When/Then block yet. The word Scenario appears here in prose but not
as a header.
EOF
    make_bead "age-zero" "$TMP_DIR/body.md"
    run bash "$SCRIPT" age-zero
    [ "$status" -eq 0 ]
    [[ "$output" == *"SLICE-BATCH: WARN"* ]]
    [[ "$output" == *"age-zero"* ]]
    [[ "$output" == *"no Gherkin scenario"* ]]
}

# --- Robustness ---------------------------------------------------------------
@test "robust: a fenced code block containing Scenario:/GWT text is parse-inert" {
    # Only ONE real scenario; a second lives inside a ``` fence and must not count.
    cat > "$TMP_DIR/body.md" <<'EOF'
## Scenarios

Scenario: the only real behavior
  Given a real precondition
  When it runs
  Then it works

```yaml
acceptance:
  example: |
    Scenario: phantom inside a fence
      Given a fenced line
      When parsed
      Then it must not count
```
EOF
    make_bead "age-fence" "$TMP_DIR/body.md"
    run bash "$SCRIPT" age-fence
    [ "$status" -eq 0 ]
    [[ "$output" == *"SLICE-BATCH: PASS"* ]]
}

@test "robust: inline mid-sentence Given/When/Then in prose does not count as a scenario" {
    cat > "$TMP_DIR/body.md" <<'EOF'
## Context

The slice carries exactly one happy-path Given/When/Then triad. We reference
When and Then inline in a sentence, which must not be miscounted as behaviors.

## Scenarios

Scenario: one true behavior
  Given a precondition
  When an action
  Then an outcome
EOF
    make_bead "age-inline" "$TMP_DIR/body.md"
    run bash "$SCRIPT" age-inline
    [ "$status" -eq 0 ]
    [[ "$output" == *"SLICE-BATCH: PASS"* ]]
}

@test "--json emits a machine-readable summary" {
    cat > "$TMP_DIR/body.md" <<'EOF'
## Scenarios

Scenario: a
  Given g
  When w
  Then t

Scenario: b
  Given g
  When w
  Then t
EOF
    make_bead "age-json" "$TMP_DIR/body.md"
    run bash "$SCRIPT" --json age-json
    [ "$status" -eq 1 ]
    [[ "$output" == *'"bead":"age-json"'* ]]
    [[ "$output" == *'"behaviors":2'* ]]
    [[ "$output" == *'"result":"fail"'* ]]
}

# --- Misuse / infra -----------------------------------------------------------
@test "misuse: no bead id exits 2" {
    run bash "$SCRIPT"
    [ "$status" -eq 2 ]
    [[ "$output" == *"bead id"* ]]
}

@test "infra: bead show returning nothing exits 2, not a policy verdict" {
    rm -f "$STUB_JSON"   # stub `ao ... show` will exit 1 / emit nothing
    run bash "$SCRIPT" age-missing
    [ "$status" -eq 2 ]
}

@test "infra: bead show returning MALFORMED JSON exits 2, not a fail-open WARN (age-74yi refute-fix)" {
    # Non-empty but unparseable tracker output. Before the fix, jq's parse error
    # was swallowed into an empty body -> SLICE-BATCH: WARN exit 0 (fail-open).
    printf 'not-json\n' > "$STUB_JSON"
    run bash "$SCRIPT" age-bad
    [ "$status" -eq 2 ]
    [[ "$output" == *"malformed"* || "$output" == *"infra"* ]]
}

@test "infra: --all-ready with MALFORMED ready JSON exits 2, not a fail-open sweep over zero beads (age-74yi refute-fix 2)" {
    printf 'not-json\n' > "$STUB_READY_JSON"
    run bash "$SCRIPT" --all-ready
    [ "$status" -eq 2 ]
}

@test "infra: --all-ready where one ready bead's show is MALFORMED fails CLOSED (exit 2), not skipped (age-74yi refute-fix 2)" {
    printf '{"issues":[{"id":"age-x"}]}\n' > "$STUB_READY_JSON"  # one ready id
    printf 'not-json\n' > "$STUB_JSON"                            # its show is malformed
    run bash "$SCRIPT" --all-ready
    [ "$status" -eq 2 ]
}

@test "PASS: --all-ready over valid single-scenario ready beads exits 0" {
    printf '{"issues":[{"id":"age-x"}]}\n' > "$STUB_READY_JSON"
    printf 'GIVEN a\nWHEN b\nTHEN c\n' > "$TMP_DIR/body.txt"
    make_bead age-x "$TMP_DIR/body.txt"                          # valid single-scenario show
    run bash "$SCRIPT" --all-ready
    [ "$status" -eq 0 ]
}

@test "PASS: a Background: block before a single Scenario is shared setup, not a 2nd behavior (age-74yi refute-fix 3)" {
    # Gherkin Background = shared setup steps; it must NOT count as a behavior.
    printf 'Background:\n  Given shared setup\n\nScenario: one behavior\n  Given x\n  When y\n  Then z\n' > "$TMP_DIR/body.txt"
    make_bead age-bg "$TMP_DIR/body.txt"
    run bash "$SCRIPT" age-bg
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}
