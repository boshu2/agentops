#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "hook preflight checks manifest-derived registered hooks beyond curated legacy list" {
    run bash "$REPO_ROOT/scripts/validate-hook-preflight.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"derived "* ]]
    [[ "$output" == *"manifest script exists: hooks/commit-review-gate.sh"* ]]
}
