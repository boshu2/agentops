#!/bin/bash
# Test: Codex sandbox mode (-s read-only)
# Proves -s read-only is accepted by CLI and -o output capture works
# ag-3b7.3
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CODEX_MODEL="${CODEX_MODEL:-gpt-5.3-codex}"
OUTPUT_RO="/tmp/codex-sandbox-ro-$$.md"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

passed=0
failed=0
skipped=0

pass() { echo -e "${GREEN}  ✓${NC} $1"; ((passed++)) || true; }
fail() { echo -e "${RED}  ✗${NC} $1"; ((failed++)) || true; }
skip() { echo -e "${YELLOW}  ⊘${NC} $1"; ((skipped++)) || true; }

cleanup() {
    rm -f "$OUTPUT_RO"
}
trap cleanup EXIT

echo -e "${BLUE}[TEST]${NC} Codex sandbox mode (-s read-only)"

# Pre-flight: Codex CLI available?
if ! command -v codex > /dev/null 2>&1; then
    skip "Codex CLI not found — skipping all tests"
    echo -e "${YELLOW}SKIPPED${NC} - Codex CLI not available"
    exit 0
fi
pass "Codex CLI found"

# Test 1: -s read-only accepted by CLI with -o output capture (retry once on transient failure)
CODEX_OK=false
for attempt in 1 2; do
    echo -e "${BLUE}  Running codex exec -s read-only with -o (attempt $attempt, up to 120s)...${NC}"
    if timeout 120 codex exec -s read-only -m "$CODEX_MODEL" -C "$REPO_ROOT" \
        -o "$OUTPUT_RO" \
        "List the files in the skills/ directory and summarize what you see" \
        > /dev/null 2>&1; then
        CODEX_OK=true
        break
    fi
    [[ $attempt -eq 1 ]] && echo -e "${YELLOW}  Retrying (Codex MCP startup can be slow)...${NC}"
done
if $CODEX_OK; then
    pass "codex exec -s read-only with -o succeeded (exit 0)"
else
    fail "codex exec -s read-only with -o failed after 2 attempts"
fi

# Test 2: -o output was captured despite read-only sandbox
if [[ -s "$OUTPUT_RO" ]]; then
    pass "-o output captured in read-only mode (CLI-level capture works)"
else
    fail "-o output missing or empty — CLI-level capture may not work with -s read-only"
fi

# Test 3: Output content is non-trivial (not just an error message)
RO_SIZE=$(wc -c < "$OUTPUT_RO" 2>/dev/null | tr -d ' ')
if [[ "${RO_SIZE:-0}" -gt 50 ]]; then
    pass "Read-only output has substantive content (${RO_SIZE} bytes)"
else
    fail "Read-only output suspiciously small (${RO_SIZE} bytes)"
fi

# Summary
echo ""
echo -e "${BLUE}═══════════════════════════════════════════${NC}"
if [[ $failed -gt 0 ]]; then
    echo -e "${RED}FAILED${NC} - $passed passed, $failed failed, $skipped skipped"
    exit 1
else
    echo -e "${GREEN}PASSED${NC} - $passed passed, $failed failed, $skipped skipped"
    exit 0
fi
