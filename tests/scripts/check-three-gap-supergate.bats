#!/usr/bin/env bats
#
# Tests for scripts/check-three-gap-supergate.sh — verifies Gap 2's
# compile-health sub-check SKIPs (rather than fails) when neither the
# canonical .agents/defrag/latest.json nor an overnight Dream preview
# is present, matching the existing Gap 1 council-coverage SKIP shape
# for operator-side surfaces.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    TMP_HOME="$(mktemp -d)"

    SHIM_ROOT="$TMP_HOME/repo"
    mkdir -p "$SHIM_ROOT/scripts" "$SHIM_ROOT/.agents" "$SHIM_ROOT/cli/bin"
    cp "$REPO_ROOT/scripts/check-three-gap-supergate.sh" "$SHIM_ROOT/scripts/"
    cat > "$SHIM_ROOT/scripts/check-flywheel-compounding-snapshot.sh" <<'EOF'
#!/usr/bin/env bash
echo "stub flywheel-compounding-snapshot OK"
exit 0
EOF
    cat > "$SHIM_ROOT/scripts/proof-run.sh" <<'EOF'
#!/usr/bin/env bash
echo "stub proof-run OK"
exit 0
EOF
    cat > "$SHIM_ROOT/scripts/check-compile-health.sh" <<'EOF'
#!/usr/bin/env bash
# In tests we never want the real compile-health path to fail — if the
# supergate decides to invoke it (artifact present), we treat it as PASS.
echo "stub compile-health OK"
exit 0
EOF
    chmod +x "$SHIM_ROOT/scripts/"*.sh
    : > "$SHIM_ROOT/cli/bin/ao"
    chmod +x "$SHIM_ROOT/cli/bin/ao"
}

teardown() {
    rm -rf "$TMP_HOME"
}

@test "Gap 2 SKIPs compile-health when no defrag artifact is present" {
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=durable-learning
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP  compile-health"* ]]
    [[ "$output" == *"three-gap super-gate (durable-learning): PASS"* ]]
}

@test "Gap 2 runs compile-health when overnight preview exists" {
    mkdir -p "$SHIM_ROOT/.agents/overnight/run-1/defrag"
    : > "$SHIM_ROOT/.agents/overnight/run-1/defrag/latest.json"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=durable-learning
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS  compile-health"* ]]
    [[ "$output" != *"SKIP  compile-health"* ]]
}

@test "Gap 2 runs compile-health when canonical defrag artifact exists" {
    mkdir -p "$SHIM_ROOT/.agents/defrag"
    : > "$SHIM_ROOT/.agents/defrag/latest.json"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=durable-learning
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS  compile-health"* ]]
    [[ "$output" != *"SKIP  compile-health"* ]]
}

@test "Gap 2 SKIPs compile-health even when overnight tree exists but has no latest.json" {
    # Empty overnight tree with no defrag/latest.json under any run dir
    # should still trigger the structural SKIP (the find returns nothing).
    mkdir -p "$SHIM_ROOT/.agents/overnight/run-1"
    run bash "$SHIM_ROOT/scripts/check-three-gap-supergate.sh" --gap=durable-learning
    [ "$status" -eq 0 ]
    [[ "$output" == *"SKIP  compile-health"* ]]
}
