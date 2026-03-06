#!/bin/bash
# Validate pr-research skill
set -euo pipefail

# Determine SKILL_DIR relative to this script (works in plugins or ~/.claude)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

ERRORS=0
CHECKS=0

check() {
    local desc="$1"
    local cmd="$2"
    local expected="$3"

    CHECKS=$((CHECKS + 1))
    if eval "$cmd" 2>/dev/null | grep -qi "$expected"; then
        echo "✓ $desc"
    else
        echo "✗ $desc"
        echo "  Command: $cmd"
        echo "  Expected to find: $expected"
        ERRORS=$((ERRORS + 1))
    fi
}

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

echo "=== PR Research Skill Validation ==="
echo ""


# Verify git is available
check "git binary exists" "which git" "git"

# Verify dependent skill exists
check_exists "Beads skill exists" "$SKILL_DIR/../beads/SKILL.md"

# Verify pr-research workflow patterns in SKILL.md
check_pattern "SKILL.md has upstream exploration" "$SKILL_DIR/SKILL.md" "[Uu]pstream"
check_pattern "SKILL.md has contribution guidelines" "$SKILL_DIR/SKILL.md" "[Cc]ontribution.*[Gg]uide"
check_pattern "SKILL.md has research output" "$SKILL_DIR/SKILL.md" "research/|\.agents/"

echo ""
echo "=== Results ==="
echo "Checks: $CHECKS"
echo "Errors: $ERRORS"

if [ $ERRORS -gt 0 ]; then
    echo ""
    echo "FAIL: PR-research skill validation failed"
    exit 1
else
    echo ""
    echo "PASS: PR-research skill validation passed"
    exit 0
fi
