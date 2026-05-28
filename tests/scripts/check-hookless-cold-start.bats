#!/usr/bin/env bats
# Tests for scripts/check-hookless-cold-start.sh (soc-qh2g1 scenario 2).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-hookless-cold-start.sh"
    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/docs/architecture"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/check-hookless-cold-start.sh"
    chmod +x "$FAKE_REPO/scripts/check-hookless-cold-start.sh"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "real cold-start surfaces carry no un-hedged hook promises" {
    run bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "fails when a scoped surface presents a hook path as a current surface" {
    cat > "$FAKE_REPO/docs/architecture/primitive-chains.md" <<'EOF'
# Primitive Chains

| Continuity Surface | Role |
|--------------------|------|
| `hooks/session-start.sh` | Injects lightweight repo context |
EOF

    run "$FAKE_REPO/scripts/check-hookless-cold-start.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"primitive-chains.md:5"* ]]
    [[ "$output" == *"without an opt-in/historical hedge"* ]]
}

@test "passes when a hook path is hedged as opt-in / author-it-yourself" {
    cat > "$FAKE_REPO/docs/architecture/primitive-chains.md" <<'EOF'
# Primitive Chains

Startup context loads via `ao session bootstrap` / `ao inject` (run explicitly).
A `hooks/session-start.sh` is opt-in only (author one via the hooks-authoring skill).
EOF

    run "$FAKE_REPO/scripts/check-hookless-cold-start.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "passes when no hook paths appear at all" {
    cat > "$FAKE_REPO/docs/architecture/primitive-chains.md" <<'EOF'
# Primitive Chains

Startup orientation: run `ao session bootstrap`. Nothing auto-injects in 3.0.
EOF

    run "$FAKE_REPO/scripts/check-hookless-cold-start.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}

@test "errors when no scoped surface exists under root" {
    run "$FAKE_REPO/scripts/check-hookless-cold-start.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"no cold-start surfaces found"* ]]
}
