#!/bin/bash
# Run all Codex integration tests
# Usage: ./tests/codex/run-all.sh
# ag-3b7.5
# Test harness: intentionally -uo (not -euo) to accumulate passed/failed/skipped
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

total=0
passed=0
failed=0
skipped=0

echo -e "${BLUE}════════════════════════════════════════════${NC}"
echo -e "${BLUE} Codex Integration Tests${NC}"
echo -e "${BLUE}════════════════════════════════════════════${NC}"
echo ""

for test_script in "$SCRIPT_DIR"/test-*.sh; do
    [[ ! -f "$test_script" ]] && continue
    test_name=$(basename "$test_script" .sh)
    ((total++)) || true

    echo -e "${BLUE}── $test_name ──${NC}"
    test_output=""
    if test_output=$(bash "$test_script" 2>&1); then
        printf '%s\n' "$test_output"
        if printf '%s\n' "$test_output" | grep -q "SKIPPED"; then
            ((skipped++)) || true
        else
            ((passed++)) || true
        fi
    else
        printf '%s\n' "$test_output"
        if printf '%s\n' "$test_output" | grep -q "SKIPPED"; then
            ((skipped++)) || true
        else
            ((failed++)) || true
        fi
    fi
    echo ""
done

# Summary
echo -e "${BLUE}════════════════════════════════════════════${NC}"
echo -e "${BLUE} Summary: $total tests${NC}"
echo -e "  ${GREEN}Passed:${NC}  $passed"
echo -e "  ${RED}Failed:${NC}  $failed"
echo -e "  ${YELLOW}Skipped:${NC} $skipped"
echo -e "${BLUE}════════════════════════════════════════════${NC}"

if [[ $failed -gt 0 ]]; then
    echo -e "${RED}OVERALL: FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}OVERALL: PASSED${NC}"
    exit 0
fi
