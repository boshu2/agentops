#!/bin/bash
# Validate oss-docs skill
set -euo pipefail

# Determine SKILL_DIR relative to this script (works in plugins or ~/.claude)
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

check_exists() {
    local desc="$1"
    local path="$2"

    CHECKS=$((CHECKS + 1))
    if [ -e "$path" ]; then
        echo "✓ $desc"
    else
        echo "✗ $desc ($path not found)"
        ERRORS=$((ERRORS + 1))
    fi
}

echo "=== OSS-Docs Skill Validation ==="
echo ""


# Verify dependent skill exists
check_exists "Standards skill exists" "$SKILL_DIR/../standards/SKILL.md"

# Verify oss-docs workflow patterns in SKILL.md
check_pattern "SKILL.md has README documentation" "$SKILL_DIR/SKILL.md" "README"
check_pattern "SKILL.md has CONTRIBUTING documentation" "$SKILL_DIR/SKILL.md" "CONTRIBUTING"
check_pattern "SKILL.md has open source patterns" "$SKILL_DIR/SKILL.md" "[Oo]pen [Ss]ource|OSS"
check_pattern "SKILL.md mentions AGENTS.md" "$SKILL_DIR/SKILL.md" "AGENTS.md"

echo ""
echo "=== Results ==="
echo "Checks: $CHECKS"
echo "Errors: $ERRORS"

if [ $ERRORS -gt 0 ]; then
    echo ""
    echo "FAIL: OSS-docs skill validation failed"
    exit 1
else
    echo ""
    echo "PASS: OSS-docs skill validation passed"
    exit 0
fi
