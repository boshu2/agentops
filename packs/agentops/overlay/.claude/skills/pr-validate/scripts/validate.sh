#!/bin/bash
# Validate pr-validate skill
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

ERRORS=0
CHECKS=0

check_pattern() {
    local desc="$1"
    local file="$2"
    local pattern="$3"

    CHECKS=$((CHECKS + 1))
    if grep -qiE "$pattern" "$file" 2>/dev/null; then
        echo "✓ $desc"
    else
        echo "✗ $desc (pattern '$pattern' not found in $file)"
        ERRORS=$((ERRORS + 1))
    fi
}

echo "=== PR Validate Skill Validation ==="
echo ""

check_pattern "SKILL.md checks upstream alignment" "$SKILL_DIR/SKILL.md" "[Uu]pstream[[:space:]]+[Aa]lignment"
check_pattern "SKILL.md checks isolation" "$SKILL_DIR/SKILL.md" "[Ii]solation[[:space:]]+[Cc]heck"
check_pattern "SKILL.md checks scope creep" "$SKILL_DIR/SKILL.md" "[Ss]cope[[:space:]]+[Cc]heck|scope creep"
check_pattern "SKILL.md has Examples section" "$SKILL_DIR/SKILL.md" "^## Examples"
check_pattern "SKILL.md has Troubleshooting section" "$SKILL_DIR/SKILL.md" "^## Troubleshooting"

echo ""
echo "=== Results ==="
echo "Checks: $CHECKS"
echo "Errors: $ERRORS"

if [ $ERRORS -gt 0 ]; then
    echo ""
    echo "FAIL: pr-validate skill validation failed"
    exit 1
else
    echo ""
    echo "PASS: pr-validate skill validation passed"
    exit 0
fi
