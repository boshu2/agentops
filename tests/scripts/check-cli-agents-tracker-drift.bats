#!/usr/bin/env bats
# Acceptance surface for scripts/check-cli-agents-tracker-drift.sh

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-cli-agents-tracker-drift.sh"
    FIXTURE="$(mktemp -d "$BATS_TMPDIR/cli-agents.XXXXXX")"
}

teardown() {
    [ -n "${FIXTURE:-}" ] && rm -rf "$FIXTURE"
}

@test "checker exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "green: real cli/AGENTS.md pointer stub passes" {
    run bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "red: bd-era cli/AGENTS.md fails" {
    cat > "$FIXTURE/AGENTS.md" <<'EOF'
# Agent Instructions
Use bd ready for work tracking.
Read ../AGENTS.md and docs/architecture/codebase-overview.md
BEADS_DIR=$PWD/_beads br ready
EOF
    run bash "$SCRIPT" --agents-file "$FIXTURE/AGENTS.md"
    [ "$status" -ne 0 ]
    [[ "$output" == *"bd"* ]]
}

@test "red: missing root AGENTS link fails" {
    cat > "$FIXTURE/AGENTS.md" <<'EOF'
# CLI
BEADS_DIR=$PWD/_beads br ready
See docs/architecture/codebase-overview.md
EOF
    run bash "$SCRIPT" --agents-file "$FIXTURE/AGENTS.md"
    [ "$status" -ne 0 ]
    [[ "$output" == *"root AGENTS.md"* ]]
}

@test "red: missing br ready invocation fails" {
    cat > "$FIXTURE/AGENTS.md" <<'EOF'
# CLI
Read ../AGENTS.md and docs/architecture/codebase-overview.md
EOF
    run bash "$SCRIPT" --agents-file "$FIXTURE/AGENTS.md"
    [ "$status" -ne 0 ]
    [[ "$output" == *"br ready"* ]]
}
