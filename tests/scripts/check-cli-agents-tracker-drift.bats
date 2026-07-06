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
BEADS_DIR="$(ao beads dir)" br ready
EOF
    run bash "$SCRIPT" --agents-file "$FIXTURE/AGENTS.md"
    [ "$status" -ne 0 ]
    [[ "$output" == *"bd"* ]]
}

@test "red: stale worktree-local BEADS_DIR fails" {
    cat > "$FIXTURE/AGENTS.md" <<'EOF'
# CLI
BEADS_DIR=$PWD/_beads br ready
Read ../AGENTS.md and docs/architecture/codebase-overview.md
EOF
    run bash "$SCRIPT" --agents-file "$FIXTURE/AGENTS.md"
    [ "$status" -ne 0 ]
    [[ "$output" == *"BEADS_DIR="* ]]
}

@test "red: hard-coded private ledger git path fails" {
    cat > "$FIXTURE/AGENTS.md" <<'EOF'
# CLI
BEADS_DIR="$(ao beads dir)" br ready
git -C _beads push
Read ../AGENTS.md and docs/architecture/codebase-overview.md
EOF
    run bash "$SCRIPT" --agents-file "$FIXTURE/AGENTS.md"
    [ "$status" -ne 0 ]
    [[ "$output" == *"git -C _beads"* ]]
}

@test "red: missing root AGENTS link fails" {
    cat > "$FIXTURE/AGENTS.md" <<'EOF'
# CLI
BEADS_DIR="$(ao beads dir)" br ready
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

# Substrate carve-out (age-gc-adoption-u0he): bd/dolt is the gascity SUBSTRATE
# store, first-class and embraced — a different layer from this repo's br tracker.
# A line documenting the substrate and marked `gascity-substrate` is legitimate.
@test "green: substrate-marked bd/dolt reference is exempt" {
    cat > "$FIXTURE/AGENTS.md" <<'EOF'
# CLI
BEADS_DIR="$(ao beads dir)" br ready
The gascity substrate store runs bd dolt natively — gascity-substrate layer, not this repo's tracker.
Read ../AGENTS.md and docs/architecture/codebase-overview.md
EOF
    run bash "$SCRIPT" --agents-file "$FIXTURE/AGENTS.md"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

# The carve-out is scoped: an unmarked live bd command still fails even when a
# separate substrate-marked line is present.
@test "red: unmarked bd command fails despite a substrate-marked line" {
    cat > "$FIXTURE/AGENTS.md" <<'EOF'
# CLI
Use bd ready for this repo's work tracking.
The gascity substrate store runs on bd dolt — gascity-substrate layer.
BEADS_DIR="$(ao beads dir)" br ready
Read ../AGENTS.md and docs/architecture/codebase-overview.md
EOF
    run bash "$SCRIPT" --agents-file "$FIXTURE/AGENTS.md"
    [ "$status" -ne 0 ]
    [[ "$output" == *"bd"* ]]
}
