#!/usr/bin/env bats
# Tests for scripts/generate-cli-reference.sh portability.
#
# The script's awk command-extraction patterns must work under BOTH gawk
# (CI on ubuntu-latest) and mawk (Ubuntu default on local dev). The original
# pattern `[[:space:]]{2,}` used POSIX interval expressions which older mawk
# (pre 1.3.4-20240123) silently rejected, producing a 14-line empty
# COMMANDS.md on mawk hosts.
#
# Fix: use [[:space:]][[:space:]]+ (explicit-repetition form) — works on every
# awk implementation we ship to.
#
# Source: harvested item from .agents/rpi/next-work.jsonl
# (discovery-2026-05-12-ddd-hexagonal, "Make scripts/generate-cli-reference.sh
# portable to mawk").

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/generate-cli-reference.sh"
    [ -f "$SCRIPT" ] || skip "script not found: $SCRIPT"
}

@test "script does not use POSIX interval expressions ({N,} / {N,M})" {
    # Find any [[:space:]]{N,} or {N,M} that would break on older mawk
    if grep -nE '\[\[:[a-z]+:\]\]\{[0-9]+,[0-9]*\}' "$SCRIPT"; then
        echo "Found POSIX interval expression — older mawk rejects these silently."
        echo "Use [[:space:]][[:space:]]+ or other explicit-repetition form."
        return 1
    fi
}

@test "command-extraction regex matches indented commands under both awks" {
    local pattern='/^[[:space:]][[:space:]]+[a-z0-9][a-z0-9-]*([[:space:]]+|$)/'
    local input
    input=$(printf '  build         Build the binary\n  check-deps    Check\n  validate-job  Job\n')

    # gawk
    if command -v gawk >/dev/null; then
        gawk_out="$(echo "$input" | gawk "$pattern { print \$1 }")"
        [ "$gawk_out" = "$(printf 'build\ncheck-deps\nvalidate-job')" ]
    fi

    # mawk
    if command -v mawk >/dev/null; then
        mawk_out="$(echo "$input" | mawk "$pattern { print \$1 }")"
        [ "$mawk_out" = "$(printf 'build\ncheck-deps\nvalidate-job')" ]
    fi
}

@test "regex does not match single-space indents or unindented lines" {
    local pattern='/^[[:space:]][[:space:]]+[a-z0-9][a-z0-9-]*([[:space:]]+|$)/'
    local input
    input=$(printf 'unindented\n single-space\n  double-space\n')
    local out
    out="$(echo "$input" | gawk "$pattern { print \$1 }")"
    [ "$out" = "double-space" ]
}
