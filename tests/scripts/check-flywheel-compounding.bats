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

@test "SKIP with σ=0 AND ρ=0 AND citations_this_period=0 (dormant precondition)" {
    # f-2026-04-30-002 path: fully dormant corpus has no signal to evaluate,
    # so the gate returns exit 77 (autotools skip) and the goals runner
    # classifies as `skip`, not `fail`. Locks the new SKIP-by-precondition
    # contract introduced alongside SkipExitCode in the goals runner.
    write_fake_ao '{"escape_velocity_compounding":false,"sigma":0,"rho":0,"sigma_rho":0,"delta":0.003,"metrics":{"citations_this_period":0,"total_artifacts":0,"learnings_created":0}}'
    run env AO_BIN="$FAKE_AO" bash "$SCRIPT"
    [ "$status" -eq 77 ]
    [[ "$output" == *"SKIP"* ]]
    [[ "$output" == *"σ=0 ρ=0"* ]]
    [[ "$output" == *"dormant"* ]]
    [[ "$output" == *"f-2026-04-30-002"* ]]
    [[ "$output" == *"exit 77"* ]]
    # SKIP path must NOT mention the high-confidence remediation (different cause)
    [[ "$output" != *"applied|reference"* ]]
}

@test "SKIP with σ=0 ρ=0 + dormant payload still surfaces metrics + period" {
    # Even under SKIP, operators should see at-a-glance what the empty
    # corpus state looks like (artifact counts, period). Locks both the
    # exit-77 contract and the diagnostic content.
    write_fake_ao '{"escape_velocity_compounding":false,"sigma":0,"rho":0,"sigma_rho":0,"delta":0.003,"golden_signals":{"trend_verdict":"stagnant","concentration_verdict":"dormant","overall_verdict":"accumulating"},"metrics":{"citations_this_period":0,"total_artifacts":47,"learnings_created":65,"period_start":"2026-04-23T00:00:00Z","period_end":"2026-04-30T00:00:00Z"}}'
    run env AO_BIN="$FAKE_AO" bash "$SCRIPT"
    [ "$status" -eq 77 ]
    [[ "$output" == *"SKIP"* ]]
    [[ "$output" == *"citations_this_period=0"* ]]
    [[ "$output" == *"total_artifacts=47"* ]]
    [[ "$output" == *"learnings_created=65"* ]]
    [[ "$output" == *"period=[2026-04-23T00:00:00Z .. 2026-04-30T00:00:00Z]"* ]]
    [[ "$output" == *"f-2026-04-30-002"* ]]
}

@test "SKIP precondition can be disabled via FLYWHEEL_SKIP_DORMANT=0" {
    # Dev override: when an operator wants to see the FAIL diagnostic for
    # debugging, the SKIP path is opt-out via FLYWHEEL_SKIP_DORMANT=0.
    # Falls back to the inconsistent-index FAIL hint (citations=0 ON the
    # dormant payload after the override; index-inconsistent path).
    write_fake_ao '{"escape_velocity_compounding":false,"sigma":0,"rho":0,"sigma_rho":0,"delta":0.003,"metrics":{"citations_this_period":0,"total_artifacts":0,"learnings_created":0}}'
    run env AO_BIN="$FAKE_AO" FLYWHEEL_SKIP_DORMANT=0 bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"σ=0 ρ=0"* ]]
}

@test "FAIL with σ=0 AND ρ=0 BUT citations_this_period>0 (index-inconsistent)" {
    # When σ and ρ are zero but citations exist in the period, the index
    # is inconsistent — that's a real fail, not dormancy. Stays exit 1
    # with the index-inconsistent hint.
    write_fake_ao '{"escape_velocity_compounding":false,"sigma":0,"rho":0,"sigma_rho":0,"delta":0.003,"metrics":{"citations_this_period":12,"total_artifacts":47,"learnings_created":65}}'
    run env AO_BIN="$FAKE_AO" bash "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"citation index inconsistent"* ]]
    [[ "$output" == *"ao flywheel reindex"* ]]
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
