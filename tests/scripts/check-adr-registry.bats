#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-adr-registry.sh"
    FIX="$(mktemp -d)"
}

teardown() {
    rm -rf "$FIX"
}

# Write a minimal well-formed ADR into the fixture dir.
# usage: write_adr <filename> <title-number> [<status-line>]
write_adr() {
    local fname="$1" title_num="$2" status="${3:-- **Status:** Accepted (2026-01-01)}"
    {
        echo "# ADR-${title_num}: Fixture ${title_num}"
        echo ""
        echo "$status"
        echo ""
        echo "## Context"
        echo "Fixture body."
    } > "$FIX/$fname"
}

@test "checker exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "green: clean fixture (unique numbers, matching titles, Status present) passes" {
    write_adr "ADR-0001-alpha.md" "0001"
    write_adr "ADR-0002-beta.md" "0002"
    run bash "$SCRIPT" --dir "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "red: duplicate number names both colliding files" {
    write_adr "ADR-0004-alpha.md" "0004"
    write_adr "ADR-0004-beta.md" "0004"
    run bash "$SCRIPT" --dir "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"duplicate ADR number 0004"* ]]
    [[ "$output" == *"ADR-0004-alpha.md"* ]]
    [[ "$output" == *"ADR-0004-beta.md"* ]]
}

@test "red: filename number != in-file title number" {
    write_adr "ADR-0005-alpha.md" "0006"
    run bash "$SCRIPT" --dir "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"filename number 0005 != in-file title number 0006"* ]]
}

@test "red: missing Status line fails" {
    # Deliberately omit the Status line.
    {
        echo "# ADR-0007: No Status"
        echo ""
        echo "## Context"
        echo "Body without a status line."
    } > "$FIX/ADR-0007-nostatus.md"
    run bash "$SCRIPT" --dir "$FIX"
    [ "$status" -eq 1 ]
    [[ "$output" == *"no Status line"* ]]
}

@test "green: blockquote Status shape ('> **Status:**') is accepted" {
    write_adr "ADR-0008-blockquote.md" "0008" "> **Status:** Accepted (2026-01-01)"
    run bash "$SCRIPT" --dir "$FIX"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "green: real repo ADRs pass" {
    run bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}
