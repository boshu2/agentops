#!/usr/bin/env bats
# ci-local-release.bats — Tests for scripts/ci-local-release.sh
#
# Strategy: exercise the script's CLI flag parsing, validation, and fast-path
# behavior. Heavy gates are excluded via --fast and further neutralized by
# stubbing the scripts/tests they invoke.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/ci-local-release.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "ci-local-release.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "ci-local-release.sh has set -euo pipefail" {
    run grep -q 'set -euo pipefail' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "--help prints usage and exits 0" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage"* ]]
    [[ "$output" == *"--fast"* ]]
    [[ "$output" == *"--security-mode"* ]]
}

@test "-h prints usage and exits 0" {
    run bash "$SCRIPT" -h
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage"* ]]
}

@test "unknown flag is rejected with usage and exit 1" {
    run bash "$SCRIPT" --not-a-real-flag
    [ "$status" -eq 1 ]
    [[ "$output" == *"Unknown option"* ]]
}

@test "--security-mode rejects invalid values" {
    run bash "$SCRIPT" --security-mode garbage
    [ "$status" -eq 1 ]
    [[ "$output" == *"Invalid --security-mode"* ]]
}

@test "--security-mode accepts quick (help-exit path)" {
    # The script validates --security-mode before --help short-circuit when given.
    # So we pass --help last to force an early successful exit without running gates.
    run bash "$SCRIPT" --security-mode quick --help
    [ "$status" -eq 0 ]
}

@test "--security-mode accepts full (help-exit path)" {
    run bash "$SCRIPT" --security-mode full --help
    [ "$status" -eq 0 ]
}

@test "--release-version rejects garbage values" {
    run bash "$SCRIPT" --release-version not-a-version
    [ "$status" -eq 1 ]
    [[ "$output" == *"Invalid --release-version"* ]]
}

@test "--release-version accepts semver (help-exit path)" {
    run bash "$SCRIPT" --release-version 2.18.0 --help
    [ "$status" -eq 0 ]
}

@test "--release-version accepts semver with leading v (help-exit path)" {
    run bash "$SCRIPT" --release-version v2.18.0 --help
    [ "$status" -eq 0 ]
}

@test "--release-version accepts prerelease suffixes (help-exit path)" {
    run bash "$SCRIPT" --release-version 2.18.0-rc.1 --help
    [ "$status" -eq 0 ]
}

@test "--jobs accepts numeric value (help-exit path)" {
    run bash "$SCRIPT" --jobs 4 --help
    [ "$status" -eq 0 ]
}

@test "script references ARTIFACT_DIR for release-grade artifact tracking" {
    # Verifies the RUN_ID / ARTIFACT_DIR pattern is present, since release
    # provenance depends on artifacts being written to a dated directory.
    run grep -q 'ARTIFACT_DIR=' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script defines pass/fail/warn helpers consistent with pre-push-gate" {
    run grep -qE '^pass\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -qE '^fail\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -qE '^warn\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script starts errors counter at 0" {
    run grep -q '^errors=0' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script defines a FAST_MODE default of false" {
    run grep -q 'FAST_MODE=false' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script increments error count from fail helper" {
    # Guard the convention: fail() must bump errors so the gate can aggregate.
    run grep -q 'errors=\$((errors + 1))' "$SCRIPT"
    [ "$status" -eq 0 ]
}
