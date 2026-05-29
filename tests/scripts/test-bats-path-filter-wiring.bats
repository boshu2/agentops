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

# Read the COMMITTED workflow (git show HEAD:) into a private temp file rather
# than the working tree. Under bats >=1.12 (CI) the working-tree validate.yml
# transiently held stale content when this suite ran (a sibling suite's mid-run
# git operation, order-dependent — does not repro under local bats 1.10), so
# working-tree reads missed the new grouped-job wiring (ag-877). The committed
# blob at HEAD is immutable and is exactly what CI executes.
setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW_PATH="$BATS_TEST_TMPDIR/validate.yml"
    git -C "$REPO_ROOT" show HEAD:.github/workflows/validate.yml > "$WORKFLOW_PATH" 2>/dev/null \
        || cp "$REPO_ROOT/.github/workflows/validate.yml" "$WORKFLOW_PATH"
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
    echo "DEBUG repo=$REPO_ROOT" >&2
    echo "DEBUG wf=$WORKFLOW_PATH exists=$([ -f "$WORKFLOW_PATH" ] && echo Y || echo N)" >&2
    echo "DEBUG headsha=$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>&1)" >&2
    echo "DEBUG tmpfile_dp=$(grep -c '^  doctrine-proof:' "$WORKFLOW_PATH" 2>/dev/null)" >&2
    echo "DEBUG head_dp=$(git -C "$REPO_ROOT" show HEAD:.github/workflows/validate.yml 2>&1 | grep -c '^  doctrine-proof:')" >&2
    echo "DEBUG wt_dp=$(grep -c '^  doctrine-proof:' "$REPO_ROOT/.github/workflows/validate.yml" 2>/dev/null)" >&2
    echo "DEBUG nbatslines=$(grep -c 'name: Run bats tests' "$WORKFLOW_PATH" 2>/dev/null)" >&2
    run bash -c "awk '/name: Run bats tests/{inblock=1} inblock && /^        if:/{print; exit}' '$WORKFLOW_PATH'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"needs.changes.outputs.bats == 'true'"* ]]
}
