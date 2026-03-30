#!/usr/bin/env bash
# Test: Cross-runtime skill validation — ao inject standalone
# Validates ao inject produces well-formed output in both JSON and text modes
# Promoted from _quarantine/codex/ — standalone, no Codex/Claude runtime needed
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

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

echo -e "${BLUE}[TEST]${NC} Cross-runtime skill validation (ao inject standalone)"

# Pre-flight: ao binary available?
AO_BIN=""
if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    AO_BIN="$REPO_ROOT/cli/bin/ao"
elif command -v ao > /dev/null 2>&1; then
    AO_BIN="$(command -v ao)"
else
    # Try to build ao for test
    BUILD_TMP="/tmp/ao-test-$$"
    if (cd "$REPO_ROOT/cli" && go build -o "$BUILD_TMP" ./cmd/ao 2>/dev/null); then
        AO_BIN="$BUILD_TMP"
    else
        skip "ao binary not available and build failed — skipping"
        echo -e "${YELLOW}SKIPPED${NC} - ao binary not available"
        exit 0
    fi
fi
pass "ao binary available: $AO_BIN"

# Test 1: ao inject --format json produces valid JSON with expected top-level keys
echo -e "${BLUE}  [1/5] Testing ao inject --format json output structure...${NC}"
JSON_RESULT=$("$AO_BIN" inject --format json --no-cite 2>/dev/null) || {
    fail "ao inject --format json failed (exit $?)"
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════${NC}"
    echo -e "${RED}FAILED${NC} - $passed passed, $failed failed, $skipped skipped"
    exit 1
}

if echo "$JSON_RESULT" | jq empty 2>/dev/null; then
    pass "ao inject --format json returned valid JSON"
else
    fail "ao inject --format json did not return valid JSON"
    echo "  Output: $(echo "$JSON_RESULT" | head -3)" >&2
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════${NC}"
    echo -e "${RED}FAILED${NC} - $passed passed, $failed failed, $skipped skipped"
    exit 1
fi

# Test 2: JSON output has required structural keys (timestamp always present)
if echo "$JSON_RESULT" | jq -e '.timestamp' > /dev/null 2>&1; then
    pass "JSON output contains 'timestamp' key"
else
    fail "JSON output missing 'timestamp' key"
fi

# Test 3: JSON learnings field is an array (may be empty if no local learnings)
LEARNINGS_TYPE=$(echo "$JSON_RESULT" | jq -r '.learnings | type' 2>/dev/null || echo "missing")
if [[ "$LEARNINGS_TYPE" == "array" ]]; then
    LEARNINGS_COUNT=$(echo "$JSON_RESULT" | jq '.learnings | length' 2>/dev/null)
    pass "JSON output has 'learnings' array ($LEARNINGS_COUNT items)"
elif [[ "$LEARNINGS_TYPE" == "missing" ]]; then
    # No learnings key is valid when no knowledge found
    pass "JSON output has no learnings (empty knowledge base is valid)"
else
    fail "JSON output 'learnings' is not an array (type: $LEARNINGS_TYPE)"
fi

# Test 4: ao inject default (text) mode produces non-empty output
echo -e "${BLUE}  [4/5] Testing ao inject default (text) output...${NC}"
TEXT_RESULT=$("$AO_BIN" inject --no-cite 2>/dev/null) || {
    fail "ao inject (text mode) failed (exit $?)"
    TEXT_RESULT=""
}

if [[ -n "$TEXT_RESULT" ]] && [[ ${#TEXT_RESULT} -gt 10 ]]; then
    pass "ao inject text mode produced output (${#TEXT_RESULT} chars)"
else
    fail "ao inject text mode produced empty or trivial output"
fi

# Test 5: ao inject with non-matching query returns gracefully (no crash)
echo -e "${BLUE}  [5/5] Testing ao inject with non-matching query...${NC}"
NO_MATCH_RESULT=$("$AO_BIN" inject "zzz-nonexistent-topic-xyz-9999" --format json --no-cite 2>/dev/null)
NO_MATCH_EXIT=$?

if [[ $NO_MATCH_EXIT -eq 0 ]]; then
    pass "ao inject handles non-matching query gracefully (exit 0)"
    # Verify output is still valid JSON even with no matches
    if echo "$NO_MATCH_RESULT" | jq empty 2>/dev/null; then
        pass "Non-matching query still returns valid JSON"
    else
        fail "Non-matching query returned invalid JSON"
    fi
else
    fail "ao inject crashed on non-matching query (exit $NO_MATCH_EXIT)"
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
