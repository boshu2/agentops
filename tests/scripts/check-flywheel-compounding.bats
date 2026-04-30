#!/usr/bin/env bats
#
# Tests for scripts/check-flywheel-compounding.sh — verifies the gate's hint
# branches (σ=0 ρ=0 dormant vs ρ=0-only vs generic insufficient-influence)
# stay distinct so operators see the right remediation per failure mode.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-flywheel-compounding.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_AO="$TMP_DIR/ao"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# write_fake_ao OUTPUT_JSON
# Creates a fake `ao` shim at $FAKE_AO that prints the given JSON when called
# with `flywheel status --json`. Lets the test target the gate's branching
# logic without depending on real corpus state.
write_fake_ao() {
    local payload="$1"
    cat > "$FAKE_AO" <<EOF
#!/usr/bin/env bash
if [[ "\$1" == "flywheel" && "\$2" == "status" && "\$3" == "--json" ]]; then
    cat <<'JSON'
$payload
JSON
    exit 0
fi
exit 1
EOF
    chmod +x "$FAKE_AO"
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "PASS branch: escape_velocity_compounding=true exits 0" {
    write_fake_ao '{"escape_velocity_compounding":true,"sigma":0.5,"rho":0.4,"sigma_rho":0.2,"delta":0.001}'
    run env AO_BIN="$FAKE_AO" bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"compounding"* ]]
}

@test "FAIL with σ=0 AND ρ=0 emits dormant-corpus hint" {
    write_fake_ao '{"escape_velocity_compounding":false,"sigma":0,"rho":0,"sigma_rho":0,"delta":0.003}'
    run env AO_BIN="$FAKE_AO" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"σ=0 ρ=0"* ]]
    [[ "$output" == *"dormant"* ]]
    # σ=0 ρ=0 hint must NOT mention only the high-confidence remediation
    [[ "$output" != *"applied|reference"* ]]
    # σ=0 ρ=0 must surface multi-session-bound diagnostic (per the
    # 2026-04-30 quarantine-strengthening cycle).
    [[ "$output" == *"multi-session-bound"* ]]
}

@test "FAIL with σ=0 AND ρ=0 surfaces verdict + period block when payload provides them" {
    write_fake_ao '{"escape_velocity_compounding":false,"sigma":0,"rho":0,"sigma_rho":0,"delta":0.003,"golden_signals":{"trend_verdict":"stagnant","concentration_verdict":"dormant","overall_verdict":"accumulating"},"metrics":{"citations_this_period":0,"total_artifacts":47,"learnings_created":65,"period_start":"2026-04-23T00:00:00Z","period_end":"2026-04-30T00:00:00Z"}}'
    run env AO_BIN="$FAKE_AO" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"trend_verdict=stagnant"* ]]
    [[ "$output" == *"concentration_verdict=dormant"* ]]
    [[ "$output" == *"overall_verdict=accumulating"* ]]
    [[ "$output" == *"citations_this_period=0"* ]]
    [[ "$output" == *"total_artifacts=47"* ]]
    [[ "$output" == *"period=[2026-04-23T00:00:00Z .. 2026-04-30T00:00:00Z]"* ]]
    [[ "$output" == *"f-2026-04-30-002.md"* ]]
}

@test "FAIL with ρ=0 only (σ>0) emits high-confidence-citation hint" {
    write_fake_ao '{"escape_velocity_compounding":false,"sigma":0.5,"rho":0,"sigma_rho":0,"delta":0.003}'
    run env AO_BIN="$FAKE_AO" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"ρ=0"* ]]
    [[ "$output" == *"applied|reference"* ]]
    # The σ=0 branch must NOT fire when σ>0
    [[ "$output" != *"dormant"* ]]
}

@test "FAIL with σρ ≤ δ/100 (both nonzero) emits generic hint" {
    write_fake_ao '{"escape_velocity_compounding":false,"sigma":0.001,"rho":0.001,"sigma_rho":0.000001,"delta":0.003}'
    run env AO_BIN="$FAKE_AO" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"insufficient evidence-backed influence"* ]]
    [[ "$output" != *"dormant"* ]]
    [[ "$output" != *"applied|reference"* ]]
}

@test "ao subprocess failure exits 1 with clear error" {
    cat > "$FAKE_AO" <<'EOF'
#!/usr/bin/env bash
exit 2
EOF
    chmod +x "$FAKE_AO"
    run env AO_BIN="$FAKE_AO" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL: ao flywheel status --json failed"* ]]
}
