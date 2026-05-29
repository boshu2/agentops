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

# Resolve WORKFLOW_PATH inside setup() (per-test), NOT at file scope. Under
# bats >=1.12 (CI) $BATS_TEST_DIRNAME is reliable in setup() but a file-scope
# value resolved against a different cwd/leaked path, making this self-test read
# stale workflow content and miss the new grouped-job wiring (ag-877). The
# sibling validate-release-tag-full-ci.bats uses this same setup() pattern.
setup() {
    WORKFLOW_PATH="$BATS_TEST_DIRNAME/../../.github/workflows/validate.yml"
}

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

@test "validate.yml bats step triggers on needs.changes.outputs.bats" {
    # Post-rebuild (ag-877): bats is no longer a standalone `bats-tests:` job —
    # it runs as the "Run bats tests" step inside the `correctness` job. Its
    # step-level `if:` must still gate on the bats path-filter output so a
    # .bats-only change re-runs it.
    run bash -c "awk '/name: Run bats tests/{inblock=1} inblock && /^        if:/{print; exit}' '$WORKFLOW_PATH'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"needs.changes.outputs.bats == 'true'"* ]]
}
