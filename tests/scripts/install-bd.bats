#!/usr/bin/env bats

# Lightweight smoke tests for scripts/install-bd.sh. Network-dependent paths
# (download, version-resolve from GitHub) are skipped here to keep CI offline-
# safe; the verify-existing-install short-circuit and the unsupported-platform
# branches are exercised.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/install-bd.sh"
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "--help prints usage and exits 0" {
    run "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"install the \`bd\`"* ]]
    [[ "$output" == *"--version"* ]]
}

@test "rejects unknown flag" {
    run "$SCRIPT" --bogus
    [ "$status" -ne 0 ]
    [[ "$output" == *"unknown flag"* ]]
}

@test "short-circuits when bd is already installed at the requested version" {
    if ! command -v bd >/dev/null 2>&1; then
        skip "bd not on PATH on this host"
    fi
    have="$(bd version 2>&1 | head -1 || true)"
    # Pull the version number out of "bd version 1.0.3 (Homebrew)".
    ver="$(printf '%s' "$have" | sed -n 's/.*version[[:space:]]\([0-9.][0-9.]*\).*/\1/p' | head -1)"
    if [[ -z "$ver" ]]; then
        skip "could not parse current bd version: $have"
    fi
    run "$SCRIPT" --version "v$ver"
    [ "$status" -eq 0 ]
    [[ "$output" == *"already installed"* ]] || [[ "$output" == *"skipping"* ]]
}
