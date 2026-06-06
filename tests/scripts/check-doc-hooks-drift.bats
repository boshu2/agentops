#!/usr/bin/env bats
# Tests for scripts/check-doc-hooks-drift.sh (ag-rryf — hooks-runtime doc gate).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-doc-hooks-drift.sh"
    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/docs" "$FAKE_REPO/docs/contracts" "$FAKE_REPO/docs/releases"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/check-doc-hooks-drift.sh"
    chmod +x "$FAKE_REPO/scripts/check-doc-hooks-drift.sh"
    # A clean baseline doc so "scanned > 0" holds and the default case passes.
    cat > "$FAKE_REPO/docs/index.md" <<'EOF'
# Index
Run `ao session bootstrap`. AgentOps 3.0 is hookless — nothing auto-injects.
EOF
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "real live-facing docs/ carry no live-hooks references" {
    run bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "fails when a doc presents a hooks/*.sh runtime path as live" {
    cat > "$FAKE_REPO/docs/newcomer-guide.md" <<'EOF'
# Newcomer Guide
At session start, `hooks/session-start.sh` injects repo context automatically.
EOF
    run "$FAKE_REPO/scripts/check-doc-hooks-drift.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"newcomer-guide.md:2"* ]]
    [[ "$output" == *"presents a hooks runtime as live"* ]]
}

@test "fails when a doc presents 'ao hooks' as a live command" {
    cat > "$FAKE_REPO/docs/getting-started.md" <<'EOF'
# Getting Started
Run `ao hooks install` to wire the gates.
EOF
    run "$FAKE_REPO/scripts/check-doc-hooks-drift.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"getting-started.md:2"* ]]
}

@test "fails when a doc presents hooks/hooks.json as a live manifest" {
    cat > "$FAKE_REPO/docs/architecture.md" <<'EOF'
# Architecture
Hooks are wired through `hooks/hooks.json` and fire on every event.
EOF
    run "$FAKE_REPO/scripts/check-doc-hooks-drift.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"architecture.md:2"* ]]
}

@test "passes when a hook ref is hedged as opt-in / hooks-authoring" {
    cat > "$FAKE_REPO/docs/newcomer-guide.md" <<'EOF'
# Newcomer Guide
A `hooks/session-start.sh` is opt-in only (author one via the hooks-authoring skill).
EOF
    run "$FAKE_REPO/scripts/check-doc-hooks-drift.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "passes when a hook ref is hedged as removed / historical" {
    cat > "$FAKE_REPO/docs/migration.md" <<'EOF'
# Migration
The `ao hooks install` command and `hooks/hooks.json` manifest are gone (removed in 3.0).
EOF
    run "$FAKE_REPO/scripts/check-doc-hooks-drift.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "ignores archival + opt-in-subsystem surfaces (releases/, contracts/)" {
    cat > "$FAKE_REPO/docs/releases/v3.md" <<'EOF'
# v3 notes
`ao hooks install` and `hooks/session-start.sh` were removed.
EOF
    # Unhedged live-presenting ref inside an excluded subtree must NOT fail.
    cat > "$FAKE_REPO/docs/contracts/eval.md" <<'EOF'
# Eval contract
The runtime executor is `hooks/task-validation-gate.sh` and it runs during validation.
EOF
    run "$FAKE_REPO/scripts/check-doc-hooks-drift.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "errors when docs/ does not exist under root" {
    rm -rf "$FAKE_REPO/docs"
    run "$FAKE_REPO/scripts/check-doc-hooks-drift.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/ not found"* ]]
}
