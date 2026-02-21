#!/bin/bash
# Run all skill validation scripts for agentops
# Updated for unified structure (skills/ at repo root)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SKILLS_DIR="$REPO_ROOT/skills"
PASSED=0
FAILED=0
SKIPPED=0
DEP_OK=0
DEP_WARN=0
DEP_WARNINGS_LIST=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "╔════════════════════════════════════════════════════════════╗"
echo "║   AgentOps Skill Validation Test Suite                     ║"
echo "╠════════════════════════════════════════════════════════════╣"
echo "║  Tests: SKILL.md + frontmatter + skill-specific validate   ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Infrastructure/library skills (auto-loaded, not user-invoked)
INFRA_SKILLS=""

is_infra_skill() {
    local skill="$1"
    for infra in $INFRA_SKILLS; do
        if [ "$skill" = "$infra" ]; then
            return 0
        fi
    done
    return 1
}

echo -e "${BLUE}━━━ Skills Directory: $SKILLS_DIR ━━━${NC}"
echo ""

# Validate each skill in skills/
for skill_dir in "$SKILLS_DIR"/*/; do
    [ -d "$skill_dir" ] || continue
    skill_name=$(basename "$skill_dir")
    validate_script="$skill_dir/scripts/validate.sh"

    # Check if it's an infrastructure skill (library)
    if is_infra_skill "$skill_name"; then
        echo -e "  ${BLUE}○ $skill_name (library skill)${NC}"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    # Check SKILL.md exists with frontmatter
    skill_md="$skill_dir/SKILL.md"
    if [[ ! -f "$skill_md" ]]; then
        echo -e "  ${RED}✗ $skill_name (missing SKILL.md)${NC}"
        FAILED=$((FAILED + 1))
        continue
    fi

    if ! head -1 "$skill_md" | grep -q "^---$"; then
        echo -e "  ${RED}✗ $skill_name (no YAML frontmatter)${NC}"
        FAILED=$((FAILED + 1))
        continue
    fi

    if ! grep -q "^name:" "$skill_md"; then
        echo -e "  ${RED}✗ $skill_name (missing 'name' in frontmatter)${NC}"
        FAILED=$((FAILED + 1))
        continue
    fi

    # Check declared dependencies exist (now under metadata: per Anthropic spec)
    dep_list=$(sed -n '/^---$/,/^---$/p' "$skill_md" | \
        awk '/^[[:space:]]*dependencies:/{found=1; next} found && /^[[:space:]]{0,3}[^ -]/{exit} found && /^[[:space:]]*- /{gsub(/^[[:space:]]*- /, ""); print}' | \
        sed 's/#.*//' | tr -d ' ')
    if [ -n "$dep_list" ]; then
        while IFS= read -r dep; do
            [ -z "$dep" ] && continue
            if [ -d "$SKILLS_DIR/$dep" ]; then
                DEP_OK=$((DEP_OK + 1))
            else
                DEP_WARN=$((DEP_WARN + 1))
                DEP_WARNINGS_LIST="${DEP_WARNINGS_LIST}    ${skill_name} -> ${dep}\n"
            fi
        done <<< "$dep_list"
    fi

    # Run skill-specific validation if present
    if [ -f "$validate_script" ]; then
        chmod +x "$validate_script"
        if "$validate_script" > /dev/null 2>&1; then
            echo -e "  ${GREEN}✓ $skill_name${NC}"
            PASSED=$((PASSED + 1))
        else
            echo -e "  ${RED}✗ $skill_name (validate.sh failed)${NC}"
            FAILED=$((FAILED + 1))
        fi
    else
        # No skill-specific tests, basic validation passed
        echo -e "  ${GREEN}✓ $skill_name${NC} ${YELLOW}(no validate.sh)${NC}"
        PASSED=$((PASSED + 1))
    fi
done

# Run lint-skills.sh
echo ""
echo -e "${BLUE}━━━ Skill Lint Checks ━━━${NC}"
echo ""
LINT_SCRIPT="$SCRIPT_DIR/lint-skills.sh"
if [ -f "$LINT_SCRIPT" ]; then
    chmod +x "$LINT_SCRIPT"
    if "$LINT_SCRIPT"; then
        echo -e "  ${GREEN}✓ lint-skills passed${NC}"
    else
        echo -e "  ${RED}✗ lint-skills failed${NC}"
        FAILED=$((FAILED + 1))
    fi
else
    echo -e "  ${YELLOW}⚠ lint-skills.sh not found${NC}"
fi

# Run Claude feature coverage checks
echo ""
echo -e "${BLUE}━━━ Claude Feature Coverage ━━━${NC}"
echo ""
CLAUDE_FEATURE_SCRIPT="$SCRIPT_DIR/test-claude-feature-coverage.sh"
if [ -f "$CLAUDE_FEATURE_SCRIPT" ]; then
    chmod +x "$CLAUDE_FEATURE_SCRIPT"
    if "$CLAUDE_FEATURE_SCRIPT"; then
        echo -e "  ${GREEN}✓ claude feature coverage passed${NC}"
    else
        echo -e "  ${RED}✗ claude feature coverage failed${NC}"
        FAILED=$((FAILED + 1))
    fi
else
    echo -e "  ${YELLOW}⚠ test-claude-feature-coverage.sh not found${NC}"
fi

# Run alias collision detection
echo ""
echo -e "${BLUE}━━━ Alias Collision Detection ━━━${NC}"
echo ""
ALIAS_SCRIPT="$SCRIPT_DIR/check-alias-collisions.sh"
if [ -f "$ALIAS_SCRIPT" ]; then
    chmod +x "$ALIAS_SCRIPT"
    if "$ALIAS_SCRIPT"; then
        echo -e "  ${GREEN}✓ alias collision check passed${NC}"
    else
        echo -e "  ${RED}✗ alias collision check failed${NC}"
        FAILED=$((FAILED + 1))
    fi
else
    echo -e "  ${YELLOW}⚠ check-alias-collisions.sh not found${NC}"
fi

echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                       RESULTS                              ║"
echo "╠════════════════════════════════════════════════════════════╣"
printf "║  ${GREEN}✓${NC} Passed:     %-42s ║\n" "$PASSED skills"
printf "║  ${RED}✗${NC} Failed:     %-42s ║\n" "$FAILED skills"
printf "║  ${BLUE}○${NC} Skipped:    %-42s ║\n" "$SKIPPED (library/no test)"
echo "╠════════════════════════════════════════════════════════════╣"
printf "║  Total Skills: %-40s ║\n" "$((PASSED + FAILED + SKIPPED))"
echo "╠════════════════════════════════════════════════════════════╣"
printf "║  ${GREEN}✓${NC} Deps Found:  %-42s ║\n" "$DEP_OK"
printf "║  ${YELLOW}⚠${NC} Deps Missing: %-42s ║\n" "$DEP_WARN"
echo "╚════════════════════════════════════════════════════════════╝"

if [ $DEP_WARN -gt 0 ]; then
    echo ""
    echo -e "${YELLOW}Dependency warnings (not found locally):${NC}"
    echo -e "$DEP_WARNINGS_LIST"
fi

if [ $FAILED -gt 0 ]; then
    echo ""
    echo -e "${RED}OVERALL: FAIL${NC}"
    exit 1
else
    echo ""
    echo -e "${GREEN}OVERALL: PASS${NC}"
    exit 0
fi
