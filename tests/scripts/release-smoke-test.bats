#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/release-smoke-test.sh"
}

@test "release smoke self-wraps with metadata guard" {
    run grep -q 'AGENTOPS_RELEASE_METADATA_GUARD_ACTIVE=1' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'check-release-agent-metadata-stable.sh' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "release smoke defaults citation-producing commands to no-mutation args" {
    run grep -q -- '--no-cite' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q -- '--dry-run' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'release_smoke_mutation_args flywheel-close-loop' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "release smoke exposes opt-in mutation flag" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"--allow-agent-mutations"* ]]
}
