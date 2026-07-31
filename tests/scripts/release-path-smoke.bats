#!/usr/bin/env bats
# Negative witness for the between-releases release-path smoke
# (tests/scripts/lib/release-snapshot-smoke.sh, .github/workflows/release-path-smoke.yml).
#
# Repo law: every gate must be proven able to FAIL on the thing it claims to
# detect. A release-path smoke that only ever ran on a healthy config would be
# indistinguishable from `exit 0` — which is exactly how the retired
# `before.hooks` entry survived 20 days undetected
# (docs/audits/release-readiness-v3.3.0.md, "Process lesson").
#
# RED : a .goreleaser.yml copy carrying a retired before-hook -> smoke exits 1.
# RED : a structurally invalid .goreleaser.yml copy            -> smoke exits 1.
# GREEN: the real, current .goreleaser.yml                     -> smoke exits 0.
#
# The GREEN case is a full cross-platform snapshot build. It is skipped unless
# AGENTOPS_RELEASE_SMOKE_FULL=1 so the generic `bats tests/scripts/*.bats` job
# stays fast; the release-path-smoke workflow sets it and runs this file after
# installing goreleaser, so the green leg is genuinely exercised in CI.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SMOKE="$REPO_ROOT/tests/scripts/lib/release-snapshot-smoke.sh"
    WORKFLOW="$REPO_ROOT/.github/workflows/release-path-smoke.yml"
    REAL_CONFIG="$REPO_ROOT/.goreleaser.yml"
    FIX="$(mktemp -d "${BATS_TMPDIR:-/tmp}/relsmoke.XXXXXX")"
}

teardown() {
    [ -n "${FIX:-}" ] && [ -d "$FIX" ] && rm -rf "$FIX"
    return 0
}

require_goreleaser() {
    command -v goreleaser > /dev/null 2>&1 || skip "goreleaser not installed"
}

@test "smoke script exists and is executable" {
    [ -f "$SMOKE" ]
    [ -x "$SMOKE" ]
}

@test "--help prints usage and exits 0" {
    run bash "$SMOKE" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"--config"* ]]
    [[ "$output" == *"--timeout"* ]]
}

@test "usage error: unknown flag exits 2 (not confused with a broken release path)" {
    run bash "$SMOKE" --nope
    [ "$status" -eq 2 ]
    [[ "$output" == *"unknown argument"* ]]
}

@test "usage error: missing config exits 2" {
    run bash "$SMOKE" --config "$FIX/does-not-exist.yml"
    [ "$status" -eq 2 ]
    [[ "$output" == *"config not found"* ]]
}

@test "the real .goreleaser.yml is structurally parseable and declares the release path" {
    # Cheap structural assertion that runs everywhere, goreleaser or not: the
    # config the smoke exercises must still be the release config.
    [ -f "$REAL_CONFIG" ]
    grep -q '^project_name: ao$' "$REAL_CONFIG"
    grep -q '^builds:' "$REAL_CONFIG"
    grep -q '^archives:' "$REAL_CONFIG"
}

@test "red: a retired before-hook makes the release path FAIL" {
    # Reproduces the exact 20-day-undetected break: .goreleaser.yml referencing
    # a script that no longer exists. Hooks run before any build, so this is
    # fast even though it drives the real pipeline.
    require_goreleaser
    {
        echo "before:"
        echo "  hooks:"
        echo "    - ./scripts/this-script-was-retired.sh"
        cat "$REAL_CONFIG"
    } > "$FIX/broken-hook.yml"

    run bash "$SMOKE" --config "$FIX/broken-hook.yml" --timeout 5m
    [ "$status" -eq 1 ]
    [[ "$output" == *"release path is broken"* ]]
}

@test "red: a structurally invalid config makes the release path FAIL" {
    require_goreleaser
    printf 'version: 2\nproject_name: ao\nbuilds:\n  - id: ao\n    goos: not-a-list\n' \
        > "$FIX/invalid.yml"

    run bash "$SMOKE" --config "$FIX/invalid.yml" --timeout 5m
    [ "$status" -eq 1 ]
    [[ "$output" == *"release path is broken"* ]]
}

@test "green: the current .goreleaser.yml builds end-to-end" {
    require_goreleaser
    [ "${AGENTOPS_RELEASE_SMOKE_FULL:-0}" = "1" ] \
        || skip "full snapshot build: set AGENTOPS_RELEASE_SMOKE_FULL=1"

    run bash "$SMOKE"
    [ "$status" -eq 0 ]
    [[ "$output" == *"release path built end-to-end"* ]]
}

@test "the release-path smoke workflow runs the shared smoke script" {
    # Guards drift between the negative witness above and what CI actually runs.
    [ -f "$WORKFLOW" ]
    grep -q 'tests/scripts/lib/release-snapshot-smoke.sh' "$WORKFLOW"
    grep -q 'tests/scripts/release-path-smoke.bats' "$WORKFLOW"
}

@test "nightly calls the release-path smoke workflow" {
    nightly="$REPO_ROOT/.github/workflows/nightly.yml"
    [ -f "$nightly" ]
    grep -q './.github/workflows/release-path-smoke.yml' "$nightly"
}
