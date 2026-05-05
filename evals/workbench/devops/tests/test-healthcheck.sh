#!/usr/bin/env bash
set -euo pipefail

# Tests for healthcheck.sh
# Spins up a temporary python HTTP server for positive tests.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HEALTHCHECK="${SCRIPT_DIR}/../scripts/healthcheck.sh"
passed=0
failed=0
SERVER_PID=""

cleanup() {
    if [[ -n "$SERVER_PID" ]]; then
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

fail() { echo "FAIL: $1"; failed=$((failed + 1)); }
pass() { echo "PASS: $1"; passed=$((passed + 1)); }

# Find an open port
find_port() {
    python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()"
}

PORT="$(find_port)"
SERVE_DIR="$(mktemp -d)"

# Start a minimal HTTP server
python3 -m http.server "$PORT" --directory "$SERVE_DIR" &>/dev/null &
SERVER_PID=$!
# Wait for it to be ready
for _i in 1 2 3 4 5; do
    if curl -s -o /dev/null "http://localhost:${PORT}/" 2>/dev/null; then
        break
    fi
    sleep 0.3
done

# --- Test 1: Healthy URL ---
test_healthy_url() {
    local output rc=0
    output=$(MAX_RETRIES=1 bash "$HEALTHCHECK" "http://localhost:${PORT}/" 2>&1) || rc=$?
    if [[ $rc -eq 0 ]] && echo "$output" | grep -q '"status":"healthy"'; then
        pass "healthy URL"
    else
        fail "healthy URL (rc=$rc, output: $output)"
    fi
}

# --- Test 2: Unhealthy URL (nothing listening) ---
test_unhealthy_url() {
    local dead_port output rc=0
    dead_port="$(find_port)"
    output=$(MAX_RETRIES=1 BACKOFF_SECS=0 bash "$HEALTHCHECK" "http://localhost:${dead_port}/nope" 2>&1) || rc=$?
    if [[ $rc -ne 0 ]] && echo "$output" | grep -q '"status":"unhealthy"'; then
        pass "unhealthy URL"
    else
        fail "unhealthy URL (rc=$rc, output: $output)"
    fi
}

# --- Test 3: Retry behavior (attempts > 1 on failure) ---
test_retry_behavior() {
    local dead_port output rc=0
    dead_port="$(find_port)"
    output=$(MAX_RETRIES=3 BACKOFF_SECS=0 bash "$HEALTHCHECK" "http://localhost:${dead_port}/nope" 2>&1) || rc=$?
    local attempts
    attempts=$(echo "$output" | grep -oE '"attempts":[0-9]+' | grep -oE '[0-9]+')
    if [[ "${attempts:-0}" -eq 3 ]]; then
        pass "retry behavior (3 attempts)"
    else
        fail "retry behavior (expected 3 attempts, got ${attempts:-?})"
    fi
}

# --- Test 4: JSON output format ---
test_json_output() {
    local output rc=0
    output=$(MAX_RETRIES=1 bash "$HEALTHCHECK" "http://localhost:${PORT}/" 2>&1) || rc=$?
    # Validate JSON structure has all required fields
    local valid=true
    for field in status url attempts latency_ms; do
        if ! echo "$output" | grep -q "\"${field}\""; then
            valid=false
        fi
    done
    if $valid; then
        pass "JSON output format"
    else
        fail "JSON output format (output: $output)"
    fi
}

# --- Test 5: No URL provided ---
test_no_url() {
    local output rc=0
    output=$(HEALTHCHECK_CONFIG="/nonexistent" bash "$HEALTHCHECK" 2>&1) || rc=$?
    if [[ $rc -ne 0 ]] && echo "$output" | grep -q '"error"'; then
        pass "no URL provided"
    else
        fail "no URL provided (rc=$rc)"
    fi
}

# --- Run ---
test_healthy_url
test_unhealthy_url
test_retry_behavior
test_json_output
test_no_url

rm -rf "$SERVE_DIR"

echo ""
echo "healthcheck tests: ${passed} passed, ${failed} failed"
[[ $failed -eq 0 ]]
