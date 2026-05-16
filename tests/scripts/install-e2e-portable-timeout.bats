#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    WORKFLOW_PATH="$REPO_ROOT/.github/workflows/install-e2e.yml"
}

@test "install-e2e install-smoke uses portable timeout wrapper" {
    run grep -c "run_with_timeout 60 bash tests/install/test-install-smoke.sh" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
    [ "$output" -eq 2 ]
}

@test "install-e2e wrapper supports macOS gtimeout and perl fallback" {
    run grep -c "command -v gtimeout" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
    [ "$output" -eq 2 ]

    run grep -c "perl -e 'alarm shift @ARGV" "$WORKFLOW_PATH"
    [ "$status" -eq 0 ]
    [ "$output" -eq 2 ]
}

@test "install-e2e no longer shells directly to GNU timeout for install-smoke" {
    run grep -E "^[[:space:]]+timeout 60 bash tests/install/test-install-smoke.sh$" "$WORKFLOW_PATH"
    [ "$status" -eq 1 ]
}
