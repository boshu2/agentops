#!/usr/bin/env bash
# Run all team-runner tests
# Test harness: intentionally -uo (not -euo) to accumulate TOTAL_FAIL
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$REPO_ROOT" || exit 1

TOTAL_FAIL=0

run_test() {
    local test_file="$1"
    local test_name
    test_name=$(basename "$test_file" .sh)
    echo ""
    echo "━━━ ${test_name} ━━━"
    if bash "$test_file"; then
        echo ">>> ${test_name}: ALL PASSED"
    else
        echo ">>> ${test_name}: SOME FAILED"
        TOTAL_FAIL=$((TOTAL_FAIL + 1))
    fi
}

echo "=== Team Runner Test Suite ==="

run_test "$SCRIPT_DIR/test-schemas.sh"
run_test "$SCRIPT_DIR/test-watch-stream.sh"
run_test "$SCRIPT_DIR/test-watch-claude-stream.sh"
run_test "$SCRIPT_DIR/test-runner-dry-run.sh"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [[ $TOTAL_FAIL -eq 0 ]]; then
    echo "ALL TEST SUITES PASSED"
    exit 0
else
    echo "FAILURES: $TOTAL_FAIL test suite(s) failed"
    exit 1
fi
