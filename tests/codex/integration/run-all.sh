#!/usr/bin/env bash
# Run all three live Codex CLI primitive probes, retaining every child's evidence.
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
passed=0
failed=0
if ! command -v codex >/dev/null 2>&1; then
    echo "Codex live probes: 0 passed, 0 failed, 3 unavailable (CLI absent)"
    exit 77
fi
for test_name in codex-review sandbox-mode structured-output; do
    echo "=== Codex $test_name ==="
    if bash "$SCRIPT_DIR/test-$test_name.sh"; then
        passed=$((passed + 1))
    else
        status=$?
        echo "FAIL: $test_name exited $status"
        failed=$((failed + 1))
    fi
done
echo "Codex live probes: $passed passed, $failed failed, 0 unavailable"
[[ "$failed" -eq 0 ]] || exit 1
