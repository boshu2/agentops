#!/usr/bin/env bats
#
# Drift-blocking test for the `bats` change-filter wiring in
# .github/workflows/validate.yml — verifies that:
#
# 1. A `bats:` filter is declared (matches `**/*.bats`)
# 2. `bats` is exposed in the changes job outputs
# 3. The bats-tests job triggers on `needs.changes.outputs.bats == 'true'`
#
# Why this exists: the bats-tests CI job runs `bats tests/{hooks,scripts}/*.bats`.
# Before this filter existed, a bats-only commit (e.g. cycle 66 ee9e627b) did
# not match `hooks|shell|ci`, so bats-tests SKIPPED, masking the fact that
# the bats fixture-stub-tracking test could have caught the cycle-64 drift.
#
# Sibling pattern: tests/scripts/check-three-gap-supergate.bats (cycle 63)
# — same shape: grep the artifact-under-test for the expected wiring strings
# and assert each is present.

WORKFLOW_PATH="${WORKFLOW_PATH:-$BATS_TEST_DIRNAME/../../.github/workflows/validate.yml}"

@test "validate.yml declares a bats: filter under the changes job" {
    run grep -E "^            bats:" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
    [[ "$output" == *"bats:"* ]]
}

@test "validate.yml bats filter matches **/*.bats" {
    # Two lines after `bats:` should be `- '**/*.bats'`
    run bash -c "grep -A 1 '^            bats:' '$WORKFLOW_PATH' | tail -1"
    [ "$status" -eq 0 ]
    [[ "$output" == *"**/*.bats"* ]]
}

@test "validate.yml changes job exposes bats output" {
    run grep -F "      bats: \${{ steps.release.outputs.release == 'true' || steps.filter.outputs.bats }}" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
}

@test "validate.yml bats-tests job triggers on needs.changes.outputs.bats" {
    # The bats-tests block must include the bats output in its `if:` clause
    run bash -c "awk '/^  bats-tests:/{inblock=1} inblock && /^    if:/{print; exit}' '$WORKFLOW_PATH'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"needs.changes.outputs.bats == 'true'"* ]]
}
