#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW_PATH="$REPO_ROOT/.github/workflows/install-e2e.yml"
}

@test "install-e2e runs install-smoke directly on both OS jobs" {
    run grep -c "bash tests/install/test-install-smoke.sh" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
    [ "$output" -eq 2 ]
}

@test "install-e2e no longer carries the retired timeout wrapper" {
    run grep -E "run_with_timeout|command -v gtimeout|perl -e 'alarm shift @ARGV" "$WORKFLOW_PATH"
    [ "$status" -eq 1 ]
}

@test "install-e2e no longer shells directly to GNU timeout for install-smoke" {
    run grep -E "^[[:space:]]+timeout 60 bash tests/install/test-install-smoke.sh$" "$WORKFLOW_PATH"
    [ "$status" -eq 1 ]
}
