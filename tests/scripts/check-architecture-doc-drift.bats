#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-architecture-doc-drift.sh"
}

@test "checker exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "green: real repo architecture docs pass" {
    run bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "overview facts match measured build profiles" {
    overview="$REPO_ROOT/docs/architecture/codebase-overview.md"

    run rg -F '| CLI top-level commands | 32 default / 89 with `flywheel legacy` (`go run [-tags profile] ./cmd/ao --help`) |' "$overview"
    [ "$status" -eq 0 ]

    run rg -F '| Active skills | 62 (`git ls-files skills | awk -F/ '\''NF == 3 && $3 == "SKILL.md"'\''`) |' "$overview"
    [ "$status" -eq 0 ]

    run rg -F '| Shell scripts | 371 (`git ls-files scripts | awk '\''/\.sh$/'\''`) |' "$overview"
    [ "$status" -eq 0 ]

    run rg -F '| Bats test files | 293 (`git ls-files tests | awk '\''/\.bats$/'\''`) |' "$overview"
    [ "$status" -eq 0 ]

    run rg -F '| `ao codex *` | `legacy`-tagged archive; absent from the default spine |' "$overview"
    [ "$status" -eq 0 ]
}

@test "red: stale measured fact is rejected from fixture" {
    fixture="$BATS_TEST_TMPDIR/codebase-overview.md"
    sed 's/| Active skills | 62 (/| Active skills | 999 (/' \
        "$REPO_ROOT/docs/architecture/codebase-overview.md" >"$fixture"

    run env ARCHITECTURE_OVERVIEW_DOC="$fixture" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"must contain measured fact: | Active skills | 62"* ]]
}
