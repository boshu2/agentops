#!/usr/bin/env bash
# test-release-e2e-validation.sh - Integration test for ci-local fast release E2E markers
# Usage: bash tests/integration/test-release-e2e-validation.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source shared colors and helpers
source "${REPO_ROOT}/tests/lib/colors.sh"

PASS=0
FAIL=0

pass() {
    green "  PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    red "  FAIL: $1"
    FAIL=$((FAIL + 1))
}

echo "=== Release E2E Validation (fast mode) ==="
echo ""

OUTPUT_FILE="$(mktemp)"
AGENTS_HUB_TMP="$(mktemp -d)"
trap 'rm -f "$OUTPUT_FILE"; rm -rf "$AGENTS_HUB_TMP"' EXIT

log "Running ci-local release gate in fast mode..."
set +e
(cd "$REPO_ROOT" && AGENTS_HUB_OVERRIDE="$AGENTS_HUB_TMP" bash scripts/ci-local-release.sh --fast --jobs 4) >"$OUTPUT_FILE" 2>&1
CI_EXIT=$?
set -e

if [ "$CI_EXIT" -ne 0 ]; then
    fail "ci-local fast mode command exits 0 (got $CI_EXIT)"
    echo "----- ci-local output (tail) -----"
    tail -40 "$OUTPUT_FILE"
    exit 1
fi
pass "ci-local fast mode command exits 0"

check_marker() {
    local marker="$1"
    if grep -Fq "$marker" "$OUTPUT_FILE"; then
        pass "Output contains marker: $marker"
    else
        fail "Output contains marker: $marker"
    fi
}

check_marker "Codex runtime sections"
check_marker "Codex artifact metadata"
check_marker "Hook install smoke (minimal + full)"
check_marker "ao init --hooks + ao rpi smoke"

echo ""
echo "=== Summary ==="
if [ "$FAIL" -gt 0 ]; then
    red "FAILED - $FAIL checks failed"
    exit 1
fi

green "PASSED - $PASS checks passed"
exit 0
