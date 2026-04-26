#!/usr/bin/env bats

# Regression test for evolve-coverage-delta.sh's per-package coverage parser.
# The previous grep accepted any "coverage: N" prefix and silently consumed
# Go's event-coverage line ("coverage: 1/12 events") as 1.0%, dragging the
# project average down. This test pins the % anchor.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/evolve-coverage-delta.sh"
}

# Re-implement the script's pure parsing pipeline so the test does not need
# `go` on PATH (some CI runners that exercise scripts/ don't pull go).
parse_avg() {
    local input="$1"
    printf '%s' "$input" \
        | grep -oE 'coverage: [0-9]+(\.[0-9]+)?%' \
        | grep -oE '[0-9]+(\.[0-9]+)?' \
        | awk '{sum += $1; count++} END { if (count > 0) printf "%.1f", sum/count; else print "0.0" }'
}

@test "script file exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "parser regex requires the % suffix" {
    # The whole point of this test file: ensure the script's coverage
    # extractor keeps the trailing % anchor so Go's event-coverage
    # informational line (`coverage: 1/12 events`) is not consumed.
    # Use awk for robust substring matching that side-steps grep regex
    # escaping headaches.
    awk '/grep -oE/ && /coverage:/ && /\)\?%/ {found=1} END {exit !found}' "$SCRIPT"
}

@test "ignores Go event-coverage informational line" {
    # cmd/ao under -coverpkg with no _test.go files emits this format.
    input="ok  	github.com/boshu2/agentops/cli/cmd/ao	70.187s	coverage: 1/12 events (informational)"
    result="$(parse_avg "$input")"
    [ "$result" = "0.0" ]
}

@test "parses standard percent-format coverage lines" {
    input="ok  	github.com/boshu2/agentops/cli/internal/goals	5.0s	coverage: 76.5% of statements"
    result="$(parse_avg "$input")"
    [ "$result" = "76.5" ]
}

@test "averages multiple percent lines and skips the event line" {
    input="ok  	github.com/boshu2/agentops/cli/cmd/ao	70.187s	coverage: 1/12 events (informational)
ok  	github.com/boshu2/agentops/cli/internal/goals	5.0s	coverage: 76.5% of statements
ok  	github.com/boshu2/agentops/cli/internal/ratchet	11.3s	coverage: 88.2% of statements"
    result="$(parse_avg "$input")"
    # Average of 76.5 and 88.2; the 1/12 events line is excluded.
    [ "$result" = "82.4" ] || [ "$result" = "82.3" ]
}

@test "handles integer-percent lines without decimal" {
    input="ok  	github.com/boshu2/agentops/cli/internal/foo	1.0s	coverage: 50% of statements"
    result="$(parse_avg "$input")"
    [ "$result" = "50.0" ]
}

@test "returns 0.0 when nothing matches" {
    result="$(parse_avg "no coverage lines at all here")"
    [ "$result" = "0.0" ]
}
