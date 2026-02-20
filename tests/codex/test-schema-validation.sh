#!/bin/bash
# Test: Schema validation for verdict.json
# Validates verdict.json is well-formed with additionalProperties: false at all levels
# ag-3b7.1
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SCHEMA="$REPO_ROOT/skills/council/schemas/verdict.json"
MEMRL_SCHEMA="$REPO_ROOT/docs/contracts/memrl-policy.schema.json"
MEMRL_PROFILE="$REPO_ROOT/docs/contracts/memrl-policy.profile.example.json"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

passed=0
failed=0

pass() { echo -e "${GREEN}  ✓${NC} $1"; ((passed++)) || true; }
fail() { echo -e "${RED}  ✗${NC} $1"; ((failed++)) || true; }

echo -e "${BLUE}[TEST]${NC} Schema validation: $SCHEMA"

# Test 1: Schema file exists
if [[ -f "$SCHEMA" ]]; then
    pass "Schema file exists"
else
    fail "Schema file not found: $SCHEMA"
    exit 1
fi

# Test 2: Valid JSON
if jq empty "$SCHEMA" 2>/dev/null; then
    pass "Schema is valid JSON"
else
    fail "Schema is not valid JSON"
    exit 1
fi

# Test 3: Root has additionalProperties: false
if jq -e '.additionalProperties == false' "$SCHEMA" > /dev/null 2>&1; then
    pass "Root has additionalProperties: false"
else
    fail "Root missing additionalProperties: false"
fi

# Test 4: findings.items has additionalProperties: false
if jq -e '.properties.findings.items.additionalProperties == false' "$SCHEMA" > /dev/null 2>&1; then
    pass "findings.items has additionalProperties: false"
else
    fail "findings.items missing additionalProperties: false"
fi

# Test 5: Required fields at root
REQUIRED='["verdict","confidence","key_insight","findings","recommendation","schema_version"]'
if jq -e --argjson expected "$REQUIRED" '.required | sort == ($expected | sort)' "$SCHEMA" > /dev/null 2>&1; then
    pass "Root required fields match"
else
    fail "Root required fields mismatch (expected: $REQUIRED)"
fi

# Test 6: verdict enum values
if jq -e '.properties.verdict.enum == ["PASS","WARN","FAIL"]' "$SCHEMA" > /dev/null 2>&1; then
    pass "verdict enum: PASS, WARN, FAIL"
else
    fail "verdict enum mismatch"
fi

# Test 7: confidence enum values
if jq -e '.properties.confidence.enum == ["HIGH","MEDIUM","LOW"]' "$SCHEMA" > /dev/null 2>&1; then
    pass "confidence enum: HIGH, MEDIUM, LOW"
else
    fail "confidence enum mismatch"
fi

# Test 8: findings items required fields
if jq -e '.properties.findings.items.required == ["severity","category","description","location","recommendation","fix","why","ref"]' "$SCHEMA" > /dev/null 2>&1; then
    pass "findings items required: all properties (OpenAI structured output requirement)"
else
    fail "findings items required fields mismatch"
fi

# Test 9: Conforming sample validates structurally
GOOD_SAMPLE='{"verdict":"PASS","confidence":"HIGH","key_insight":"test","findings":[{"severity":"minor","category":"style","description":"test","location":"test.go:1","recommendation":"none","fix":"none","why":"example","ref":"test.go:1"}],"recommendation":"none","schema_version":2}'
if echo "$GOOD_SAMPLE" | jq -e '
  .verdict as $v | .confidence as $c |
  ($v == "PASS" or $v == "WARN" or $v == "FAIL") and
  ($c == "HIGH" or $c == "MEDIUM" or $c == "LOW") and
  (.findings | type == "array") and
  (.findings | all(.severity and .category and .description and .location and .recommendation and (.fix != null) and (.why != null) and (.ref != null))) and
  (.schema_version == 1 or .schema_version == 2)
' > /dev/null 2>&1; then
    pass "Conforming sample passes structural validation"
else
    fail "Conforming sample rejected"
fi

# Test 10: Non-conforming sample (missing required field) detected
BAD_SAMPLE='{"verdict":"PASS","confidence":"HIGH"}'
if echo "$BAD_SAMPLE" | jq -e '
  .key_insight and .findings and .recommendation
' > /dev/null 2>&1; then
    fail "Non-conforming sample should have been rejected (missing fields)"
else
    pass "Non-conforming sample correctly rejected (missing fields)"
fi

# Test 11: MemRL schema exists and is valid JSON
if [[ -f "$MEMRL_SCHEMA" ]] && jq empty "$MEMRL_SCHEMA" 2>/dev/null; then
    pass "MemRL policy schema exists and is valid JSON"
else
    fail "MemRL policy schema missing or invalid JSON: $MEMRL_SCHEMA"
fi

# Test 12: MemRL schema enforces retry|escalate actions
if jq -e '
  .properties.rules.items.properties.action.enum == ["retry","escalate"] and
  .properties.unknown_failure_class_action.enum == ["retry","escalate"] and
  .properties.missing_metadata_action.enum == ["retry","escalate"]
' "$MEMRL_SCHEMA" > /dev/null 2>&1; then
    pass "MemRL schema constrains actions to retry|escalate"
else
    fail "MemRL schema action constraints mismatch"
fi

# Test 13: MemRL profile exists, valid JSON, and shape conforms to schema-required fields
if [[ -f "$MEMRL_PROFILE" ]] && jq -e '
  .schema_version == 1 and
  (.default_mode == "off" or .default_mode == "observe" or .default_mode == "enforce") and
  (.unknown_failure_class_action == "retry" or .unknown_failure_class_action == "escalate") and
  (.missing_metadata_action == "retry" or .missing_metadata_action == "escalate") and
  (.tie_break_rules | type == "array" and length >= 1) and
  (.rules | type == "array" and length >= 1) and
  (.rollback_matrix | type == "array" and length >= 1)
' "$MEMRL_PROFILE" > /dev/null 2>&1; then
    pass "MemRL profile example has required deterministic policy fields"
else
    fail "MemRL profile example missing required fields: $MEMRL_PROFILE"
fi

# Summary
echo ""
echo -e "${BLUE}═══════════════════════════════════════════${NC}"
if [[ $failed -gt 0 ]]; then
    echo -e "${RED}FAILED${NC} - $passed passed, $failed failed"
    exit 1
else
    echo -e "${GREEN}PASSED${NC} - $passed passed, $failed failed"
    exit 0
fi
