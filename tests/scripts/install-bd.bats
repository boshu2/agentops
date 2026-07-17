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

@test "bootstrap skill keeps bd install out of scope (install-bd.sh is the owner)" {
    # Cathedral cut: bootstrap no longer installs runtimes; install-bd.sh owns beads.
    run grep -q "install-bd.sh" "$REPO_ROOT/skills/bootstrap/SKILL.md"
    [ "$status" -ne 0 ]
    run grep -q "installing or invoking \`ao\`, \`br\`, \`bd\`" "$REPO_ROOT/skills/bootstrap/SKILL.md"
    [ "$status" -eq 0 ]
    [ -x "$SCRIPT" ]
}

@test "installer-common.sh pin in install-bd.sh matches the file on disk" {
    # The curl|bash path of install-bd.sh sources installer-common.sh only
    # after verifying it against INSTALLER_COMMON_SHA256. Any edit to
    # installer-common.sh must bump that pin or remote installs fail closed.
    pin="$(sed -n 's/^INSTALLER_COMMON_SHA256="\([0-9a-f]\{64\}\)"$/\1/p' "$SCRIPT")"
    [ -n "$pin" ]
    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "$REPO_ROOT/scripts/lib/installer-common.sh" | awk '{print $1}')"
    else
        actual="$(shasum -a 256 "$REPO_ROOT/scripts/lib/installer-common.sh" | awk '{print $1}')"
    fi
    [ "$pin" = "$actual" ]
}
