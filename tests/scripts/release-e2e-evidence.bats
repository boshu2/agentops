#!/usr/bin/env bats
setup() {
    CHECK="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)/tests/integration/test-release-e2e-validation.sh"
    LOG="$BATS_TEST_TMPDIR/release.log"
}

@test "current successful release check lines pass with ANSI colors" {
    for label in 'Codex runtime sections' 'Codex artifact metadata' 'Install surface smoke' 'ao init + live-waist smoke'; do
        printf '\033[0;32m  ✓\033[0m %s\n' "$label" >> "$LOG"
    done
    run bash -c 'source "$1"; verify_release_output "$2"' _ "$CHECK" "$LOG"
    [ "$status" -eq 0 ]
}

@test "headers or failed release checks cannot count as successful evidence" {
    for label in 'Codex runtime sections' 'Codex artifact metadata' 'Install surface smoke' 'ao init + live-waist smoke'; do
        printf '== %s ==\n  ✗ %s\n' "$label" "$label" >> "$LOG"
    done
    run bash -c 'source "$1"; verify_release_output "$2"' _ "$CHECK" "$LOG"
    [ "$status" -ne 0 ]
    [[ "$output" == *'missing successful completion of Install surface smoke'* ]]
}

@test "retired successful labels cannot substitute for current checks" {
    printf '  ✓ Codex runtime sections\n  ✓ Codex artifact metadata\n  ✓ Hook install smoke (minimal + full)\n  ✓ ao init --hooks + ao rpi smoke\n' > "$LOG"
    run bash -c 'source "$1"; verify_release_output "$2"' _ "$CHECK" "$LOG"
    [ "$status" -ne 0 ]
}

@test "large trailing output does not turn an earlier successful marker into SIGPIPE failure" {
    for label in 'Codex runtime sections' 'Codex artifact metadata' 'Install surface smoke' 'ao init + live-waist smoke'; do
        printf '  ✓ %s\n' "$label" >> "$LOG"
    done
    awk 'BEGIN {for (i=0; i<10000; i++) print "remaining diagnostic line"}' >> "$LOG"
    run bash -c 'source "$1"; verify_release_output "$2"' _ "$CHECK" "$LOG"
    [ "$status" -eq 0 ]
}
