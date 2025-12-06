#!/usr/bin/env bash
#
# Validate commit message format against semantic commit conventions
#
# Usage:
#   validate_commit_message.sh HEAD           # Validate last commit
#   validate_commit_message.sh <commit-hash>  # Validate specific commit
#   validate_commit_message.sh --file <path>  # Validate message file
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Valid commit types
VALID_TYPES="feat|fix|docs|refactor|test|chore|ci|perf|style|revert"

# Validation counters
ERRORS=0
WARNINGS=0

# Get commit message
if [[ "${1:-}" == "--file" ]]; then
    MSG_FILE="${2:-}"
    if [[ ! -f "$MSG_FILE" ]]; then
        echo -e "${RED}❌ Error: File not found: $MSG_FILE${NC}"
        exit 1
    fi
    COMMIT_MSG=$(cat "$MSG_FILE")
else
    COMMIT="${1:-HEAD}"
    COMMIT_MSG=$(git log -1 --pretty=%B "$COMMIT" 2>/dev/null || echo "")
    if [[ -z "$COMMIT_MSG" ]]; then
        echo -e "${RED}❌ Error: Could not read commit message for: $COMMIT${NC}"
        exit 1
    fi
fi

echo "Validating commit message format..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Extract subject line (first non-empty line)
SUBJECT=$(echo "$COMMIT_MSG" | grep -v '^$' | head -1)

echo "Subject: $SUBJECT"
echo ""

# Check 1: Has valid type prefix
if ! echo "$SUBJECT" | grep -qE "^($VALID_TYPES)(\(.*\))?:"; then
    echo -e "${RED}❌ FAIL: Missing valid type prefix${NC}"
    echo "   Must start with one of: feat, fix, docs, refactor, test, chore, ci, perf, style, revert"
    echo "   Example: feat(monitoring): add Prometheus metrics"
    ERRORS=$((ERRORS + 1))
else
    echo -e "${GREEN}✅ PASS: Valid type prefix${NC}"
fi

# Check 2: Has colon and space
if ! echo "$SUBJECT" | grep -qE "^($VALID_TYPES)(\(.*\))?: "; then
    echo -e "${RED}❌ FAIL: Missing colon and space after type/scope${NC}"
    echo "   Format: <type>(<scope>): <subject>"
    echo "   Wrong:  feat(scope)add feature"
    echo "   Right:  feat(scope): add feature"
    ERRORS=$((ERRORS + 1))
else
    echo -e "${GREEN}✅ PASS: Has colon and space${NC}"
fi

# Check 3: Subject not capitalized (after colon)
if echo "$SUBJECT" | grep -qE "^($VALID_TYPES)(\(.*\))?: [A-Z]"; then
    echo -e "${YELLOW}⚠️  WARN: Subject should not be capitalized after colon${NC}"
    echo "   Wrong:  feat: Add feature"
    echo "   Right:  feat: add feature"
    WARNINGS=$((WARNINGS + 1))
else
    echo -e "${GREEN}✅ PASS: Subject not capitalized${NC}"
fi

# Check 4: No period at end of subject
if echo "$SUBJECT" | grep -qE '\.$'; then
    echo -e "${YELLOW}⚠️  WARN: Subject should not end with period${NC}"
    echo "   Wrong:  feat: add feature."
    echo "   Right:  feat: add feature"
    WARNINGS=$((WARNINGS + 1))
else
    echo -e "${GREEN}✅ PASS: No period at end${NC}"
fi

# Check 5: Subject length under 72 characters
SUBJECT_LEN=${#SUBJECT}
if [[ $SUBJECT_LEN -gt 72 ]]; then
    echo -e "${YELLOW}⚠️  WARN: Subject too long ($SUBJECT_LEN chars, should be ≤ 72)${NC}"
    WARNINGS=$((WARNINGS + 1))
else
    echo -e "${GREEN}✅ PASS: Subject length OK ($SUBJECT_LEN chars)${NC}"
fi

# Check 6: Has Context section
if ! echo "$COMMIT_MSG" | grep -qi "^Context:"; then
    echo -e "${YELLOW}⚠️  WARN: Missing 'Context:' section${NC}"
    echo "   AgentOps commits should include Context/Solution/Learning/Impact"
    WARNINGS=$((WARNINGS + 1))
else
    echo -e "${GREEN}✅ PASS: Has Context section${NC}"
fi

# Check 7: Has Solution section
if ! echo "$COMMIT_MSG" | grep -qi "^Solution:"; then
    echo -e "${YELLOW}⚠️  WARN: Missing 'Solution:' section${NC}"
    WARNINGS=$((WARNINGS + 1))
else
    echo -e "${GREEN}✅ PASS: Has Solution section${NC}"
fi

# Check 8: Has Learning section
if ! echo "$COMMIT_MSG" | grep -qi "^Learning:"; then
    echo -e "${YELLOW}⚠️  WARN: Missing 'Learning:' section${NC}"
    WARNINGS=$((WARNINGS + 1))
else
    echo -e "${GREEN}✅ PASS: Has Learning section${NC}"
fi

# Check 9: Has Impact section
if ! echo "$COMMIT_MSG" | grep -qi "^Impact:"; then
    echo -e "${YELLOW}⚠️  WARN: Missing 'Impact:' section${NC}"
    WARNINGS=$((WARNINGS + 1))
else
    echo -e "${GREEN}✅ PASS: Has Impact section${NC}"
fi

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [[ $ERRORS -eq 0 ]] && [[ $WARNINGS -eq 0 ]]; then
    echo -e "${GREEN}✅ Commit message is valid!${NC}"
    exit 0
elif [[ $ERRORS -eq 0 ]]; then
    echo -e "${YELLOW}⚠️  Commit message has $WARNINGS warning(s)${NC}"
    echo "   Consider fixing warnings for better compliance"
    exit 0
else
    echo -e "${RED}❌ Commit message has $ERRORS error(s) and $WARNINGS warning(s)${NC}"
    echo ""
    echo "Quick fix guide:"
    echo "1. Start with valid type: feat, fix, docs, etc."
    echo "2. Add optional scope: feat(monitoring):"
    echo "3. Add space after colon: feat(scope): description"
    echo "4. Use lowercase after colon: feat: add feature"
    echo "5. No period at end: feat: add feature"
    echo "6. Add AgentOps sections: Context/Solution/Learning/Impact"
    echo ""
    echo "Example:"
    echo "  feat(monitoring): add Prometheus metrics"
    echo ""
    echo "  Context: Applications needed observability"
    echo "  Solution: Added /metrics endpoint"
    echo "  Learning: Prometheus requires explicit registration"
    echo "  Impact: Monitoring across 8 production sites"
    exit 1
fi
