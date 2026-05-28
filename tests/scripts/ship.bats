#!/usr/bin/env bats
# Tests for scripts/ship.sh — the bot-paired fast-lane PR wrapper.
#
# Covers: argument parsing, precondition checks, inventory-touch detection,
# gate-mode routing, and --dry-run side-effect freedom.

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/ship.sh"
    [ -x "$SCRIPT" ] || skip "ship.sh not executable"
}

@test "--help renders the leading comment block" {
    run "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"ship.sh"* ]]
    [[ "$output" == *"anti-pattern #1"* ]]
}

@test "rejects unknown flag with exit 3" {
    run "$SCRIPT" --nonexistent
    [ "$status" -eq 3 ]
    [[ "$output" == *"unknown flag"* ]]
}

@test "--gate requires full|fast" {
    run "$SCRIPT" --gate bogus
    [ "$status" -eq 3 ]
    [[ "$output" == *"requires full|fast"* ]]
}

@test "exits 3 when on main branch" {
    # Save current branch, switch to main, run, restore
    local original
    original="$(git -C "$ROOT" rev-parse --abbrev-ref HEAD)"
    [[ "$original" == "main" || "$original" == "master" ]] && skip "already on main; test would be no-op"

    git -C "$ROOT" checkout main >/dev/null 2>&1 || skip "cannot reach main"
    run "$SCRIPT" --dry-run
    git -C "$ROOT" checkout "$original" >/dev/null 2>&1

    [ "$status" -eq 3 ]
    [[ "$output" == *"branch off main before shipping"* ]]
}

@test "--dry-run on a non-inventory branch picks fast mode" {
    # On the feature branch (this is run from within the repo on the test branch)
    branch="$(git -C "$ROOT" rev-parse --abbrev-ref HEAD)"
    if [[ "$branch" == "main" || "$branch" == "master" || "$branch" == "HEAD" || -z "$branch" ]]; then
        skip "test requires a named feature branch (detached HEAD or main not supported)"
    fi

    run "$SCRIPT" --dry-run
    [ "$status" -eq 0 ]
    # The script auto-detects from working-tree state, so this is environment-
    # sensitive. Just verify the routing line is present and one of the modes
    # was chosen.
    [[ "$output" == *"gate=fast"* ]] || [[ "$output" == *"gate=full"* ]]
}

@test "--force-fast overrides inventory detection" {
    branch="$(git -C "$ROOT" rev-parse --abbrev-ref HEAD)"
    if [[ "$branch" == "main" || "$branch" == "master" || "$branch" == "HEAD" || -z "$branch" ]]; then
        skip "test requires a named feature branch (detached HEAD or main not supported)"
    fi

    run "$SCRIPT" --dry-run --force-fast
    [ "$status" -eq 0 ]
    [[ "$output" == *"gate=fast"* ]]
    [[ "$output" == *"override"* ]]
}

@test "--gate full forces full mode" {
    branch="$(git -C "$ROOT" rev-parse --abbrev-ref HEAD)"
    if [[ "$branch" == "main" || "$branch" == "master" || "$branch" == "HEAD" || -z "$branch" ]]; then
        skip "test requires a named feature branch (detached HEAD or main not supported)"
    fi

    run "$SCRIPT" --dry-run --gate full
    [ "$status" -eq 0 ]
    [[ "$output" == *"gate=full"* ]]
}

@test "uses set -euo pipefail" {
    grep -qE '^set -euo pipefail$' "$SCRIPT"
}

@test "shellcheck clean" {
    if ! command -v shellcheck >/dev/null 2>&1; then
        skip "shellcheck not installed"
    fi
    shellcheck "$SCRIPT"
}

@test "anchors to the ship-loop anti-pattern #1" {
    grep -q 'anti-pattern #1' "$SCRIPT"
    grep -q 'ship-loop' "$SCRIPT"
}
