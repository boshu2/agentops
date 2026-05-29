#!/usr/bin/env bats
#
# Drift-blocking test for the soc-33bw CI wiring in
# .github/workflows/validate.yml — verifies that the
# validate-three-gap-supergate job:
#
# 1. Uses fetch-depth: 0 on its checkout (so git log main..HEAD works)
# 2. Has an advisory step that invokes --strict-coverage on Gap 1
# 3. The advisory step uses `|| echo "ADVISORY-WARN: ..."` instead of
#    the step-level GH Actions opt-out keyword, because the
#    validate-ci-policy-parity.sh awk parser misattributes step-level
#    use of that keyword to the parent job and flips its blocking
#    classification
#
# Sibling pattern: tests/scripts/test-bats-path-filter-wiring.bats
# (cycle 68) — same shape: grep validate.yml for the wired pattern and
# assert each piece is present in the right place.

# Anchor unconditionally to this repo's workflow — do NOT honor an inherited
# WORKFLOW_PATH. Under bats >=1.12 (CI) a sibling suite's WORKFLOW_PATH can leak
# across files, which made this self-test read the wrong workflow (ag-877).
WORKFLOW_PATH="$BATS_TEST_DIRNAME/../../.github/workflows/validate.yml"

@test "doctrine-proof (host of three-gap-supergate) uses fetch-depth: 0" {
    # Post-rebuild (ag-877): the three-gap-supergate checks run as steps inside
    # the `doctrine-proof` job, whose checkout must use fetch-depth: 0 so the
    # Gap 1 --strict-coverage advisory step can resolve `main..HEAD`.
    run bash -c "awk '/^  doctrine-proof:/{p=NR} p && NR>=p && NR<=p+20' '$WORKFLOW_PATH' | grep -E 'fetch-depth:[[:space:]]*0'"
    [ "$status" -eq 0 ]
}

@test "validate-three-gap-supergate has a Gap 1 --strict-coverage advisory step" {
    run grep -E 'Gap 1 --strict-coverage \(advisory' "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
    [[ "$output" == *"soc-33bw"* ]]
    [[ "$output" == *"soc-w6vh.6"* ]]
}

@test "advisory step invokes --strict-coverage flag" {
    # Within ~10 lines after the step name, expect the strict-coverage invocation
    run bash -c "awk '/Gap 1 --strict-coverage \(advisory/{p=NR} p && NR>=p && NR<=p+10' '$WORKFLOW_PATH' | grep -E '\\-\\-strict-coverage'"
    [ "$status" -eq 0 ]
}

@test "advisory step uses ADVISORY-WARN echo, not the step-level keyword" {
    run bash -c "awk '/Gap 1 --strict-coverage \(advisory/{p=NR} p && NR>=p && NR<=p+10' '$WORKFLOW_PATH' | grep -E 'ADVISORY-WARN'"
    [ "$status" -eq 0 ]
}
