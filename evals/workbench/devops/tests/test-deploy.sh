#!/usr/bin/env bash
set -euo pipefail

# Tests for deploy.sh
# Uses temp directories for isolation; cleans up on exit.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY="${SCRIPT_DIR}/../scripts/deploy.sh"
passed=0
failed=0

cleanup_dirs=()
make_tmpdir() {
    local d
    d="$(mktemp -d)"
    cleanup_dirs+=("$d")
    echo "$d"
}
cleanup() {
    for d in "${cleanup_dirs[@]}"; do
        rm -rf "$d"
    done
}
trap cleanup EXIT

fail() { echo "FAIL: $1"; failed=$((failed + 1)); }
pass() { echo "PASS: $1"; passed=$((passed + 1)); }

# --- Test 1: Successful deploy ---
test_successful_deploy() {
    local tmpdir logdir config
    tmpdir="$(make_tmpdir)"
    logdir="${tmpdir}/logs"
    config="${tmpdir}/deploy.yaml"
    cat > "$config" <<EOF
app_name: test-app
version: "2.0.0"
target: staging
health_url: http://localhost:9999/health
log_dir: ${logdir}
EOF
    local output
    if output=$(DEPLOY_CONFIG="$config" bash "$DEPLOY" 2>&1); then
        if echo "$output" | grep -q "Deployment successful"; then
            if [[ -d "$logdir" ]] && ls "$logdir"/deploy-*.log &>/dev/null; then
                pass "successful deploy"
                return
            fi
            fail "successful deploy - no log file created"
            return
        fi
        fail "successful deploy - missing success message"
        return
    fi
    fail "successful deploy - script exited non-zero"
}

# --- Test 2: Missing config file ---
test_missing_config() {
    local output rc=0
    output=$(DEPLOY_CONFIG="/nonexistent/path.yaml" bash "$DEPLOY" 2>&1) || rc=$?
    if [[ $rc -ne 0 ]] && echo "$output" | grep -qi "config file not found"; then
        pass "missing config file"
    else
        fail "missing config file (rc=$rc)"
    fi
}

# --- Test 3: Invalid config (missing required fields) ---
test_invalid_config() {
    local tmpdir config
    tmpdir="$(make_tmpdir)"
    config="${tmpdir}/bad.yaml"
    cat > "$config" <<EOF
app_name: test-app
EOF
    local output rc=0
    output=$(DEPLOY_CONFIG="$config" bash "$DEPLOY" 2>&1) || rc=$?
    if [[ $rc -ne 0 ]] && echo "$output" | grep -qi "missing required"; then
        pass "invalid config (missing fields)"
    else
        fail "invalid config (missing fields) (rc=$rc, output: $output)"
    fi
}

# --- Test 4: Invalid target value ---
test_invalid_target() {
    local tmpdir config
    tmpdir="$(make_tmpdir)"
    config="${tmpdir}/bad-target.yaml"
    cat > "$config" <<EOF
app_name: test-app
version: "1.0.0"
target: mars
log_dir: ${tmpdir}/logs
EOF
    local output rc=0
    output=$(DEPLOY_CONFIG="$config" bash "$DEPLOY" 2>&1) || rc=$?
    if [[ $rc -ne 0 ]] && echo "$output" | grep -qi "invalid target"; then
        pass "invalid target value"
    else
        fail "invalid target value (rc=$rc)"
    fi
}

# --- Test 5: Simulated deploy failure ---
test_deploy_failure() {
    local tmpdir config
    tmpdir="$(make_tmpdir)"
    config="${tmpdir}/deploy.yaml"
    cat > "$config" <<EOF
app_name: test-app
version: "1.0.0"
target: staging
log_dir: ${tmpdir}/logs
EOF
    local output rc=0
    output=$(SIMULATE_DEPLOY_FAILURE=1 DEPLOY_CONFIG="$config" bash "$DEPLOY" 2>&1) || rc=$?
    if [[ $rc -ne 0 ]] && echo "$output" | grep -qi "failed"; then
        pass "simulated deploy failure"
    else
        fail "simulated deploy failure (rc=$rc)"
    fi
}

# --- Run ---
test_successful_deploy
test_missing_config
test_invalid_config
test_invalid_target
test_deploy_failure

echo ""
echo "deploy tests: ${passed} passed, ${failed} failed"
[[ $failed -eq 0 ]]
