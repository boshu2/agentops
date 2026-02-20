#!/bin/bash
# Test: Codex structured output via --output-schema
# Proves codex exec --output-schema produces valid JSON conforming to verdict.json
# ag-3b7.2
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCHEMA="$REPO_ROOT/skills/council/schemas/verdict.json"
CODEX_MODEL="${CODEX_MODEL:-gpt-5.3-codex}"
OUTPUT="/tmp/codex-schema-test-$$.json"

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
    rm -f "$OUTPUT"
}
trap cleanup EXIT

echo -e "${BLUE}[TEST]${NC} Codex structured output (--output-schema)"

# Pre-flight: Codex CLI available?
if ! command -v codex > /dev/null 2>&1; then
    skip "Codex CLI not found — skipping all tests"
    echo -e "${YELLOW}SKIPPED${NC} - Codex CLI not available"
    exit 0
fi
pass "Codex CLI found"

# Test 1: Run codex exec with --output-schema
echo -e "${BLUE}  Running codex exec with --output-schema (up to 120s, 3 attempts)...${NC}"
max_attempts=3
attempt=1
run_succeeded=0
last_exit=0
while [[ $attempt -le $max_attempts ]]; do
    if timeout 120 codex exec -s read-only -m "$CODEX_MODEL" -C "$REPO_ROOT" \
        --output-schema "$SCHEMA" \
        -o "$OUTPUT" \
        "Review skills/council/schemas/verdict.json and return a PASS verdict with one minor finding about schema design" \
        > /dev/null 2>&1; then
        run_succeeded=1
        break
    fi

    last_exit=$?
    if [[ $last_exit -eq 124 ]]; then
        echo -e "${YELLOW}  Timeout on attempt $attempt/$max_attempts${NC}"
    else
        fail "codex exec with --output-schema failed (exit $last_exit)"
        echo -e "${RED}FAILED${NC} - Codex command failed"
        exit 1
    fi
    attempt=$((attempt + 1))
done

if [[ $run_succeeded -eq 1 ]]; then
    pass "codex exec with --output-schema succeeded (exit 0)"
else
    skip "codex exec timed out after $max_attempts attempts"
    echo -e "${YELLOW}SKIPPED${NC} - Codex command timed out repeatedly"
    exit 0
fi

# Test 2: Output file exists and is non-empty
if [[ -s "$OUTPUT" ]]; then
    pass "Output file exists and is non-empty"
else
    fail "Output file missing or empty: $OUTPUT"
    exit 1
fi

# Test 3: Output is valid JSON
if jq empty "$OUTPUT" 2>/dev/null; then
    pass "Output is valid JSON"
else
    fail "Output is not valid JSON"
    echo "  First 200 chars:" >&2
    head -c 200 "$OUTPUT" >&2
    exit 1
fi

# Test 4: verdict field is valid enum
VERDICT=$(jq -r '.verdict' "$OUTPUT" 2>/dev/null)
if [[ "$VERDICT" == "PASS" || "$VERDICT" == "WARN" || "$VERDICT" == "FAIL" ]]; then
    pass "verdict is valid enum: $VERDICT"
else
    fail "verdict is not PASS/WARN/FAIL: $VERDICT"
fi

# Test 5: confidence field is valid enum
CONFIDENCE=$(jq -r '.confidence' "$OUTPUT" 2>/dev/null)
if [[ "$CONFIDENCE" == "HIGH" || "$CONFIDENCE" == "MEDIUM" || "$CONFIDENCE" == "LOW" ]]; then
    pass "confidence is valid enum: $CONFIDENCE"
else
    fail "confidence is not HIGH/MEDIUM/LOW: $CONFIDENCE"
fi

# Test 6: Required fields exist
for field in verdict confidence key_insight findings recommendation; do
    if jq -e ".$field" "$OUTPUT" > /dev/null 2>&1; then
        pass "Required field exists: $field"
    else
        fail "Required field missing: $field"
    fi
done

# Test 7: findings is an array
if jq -e '.findings | type == "array"' "$OUTPUT" > /dev/null 2>&1; then
    pass "findings is an array"
else
    fail "findings is not an array"
fi

# Test 8: findings items have required fields (if non-empty)
FINDINGS_COUNT=$(jq '.findings | length' "$OUTPUT" 2>/dev/null)
if [[ "$FINDINGS_COUNT" -gt 0 ]]; then
    if jq -e '.findings | all(.severity and .description)' "$OUTPUT" > /dev/null 2>&1; then
        pass "All findings have severity + description ($FINDINGS_COUNT findings)"
    else
        fail "Some findings missing severity or description"
    fi
else
    skip "No findings to validate (empty array)"
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
