#!/bin/bash
# Lint all skills for quality standards
# Checks: tier frontmatter, line count limits, references/ directory, broken references
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SKILLS_DIR="$REPO_ROOT/skills"

CHECKED=0
PASSED=0
FAILED=0
WARNED=0
FAILURES=""
WARNINGS=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

fail() {
    local skill="$1"
    local reason="$2"
    FAILED=$((FAILED + 1))
    FAILURES="${FAILURES}  ${RED}✗${NC} ${skill}: ${reason}\n"
}

echo -e "${BLUE}━━━ Skill Lint ━━━${NC}"
echo ""

for skill_dir in "$SKILLS_DIR"/*/; do
    [ -d "$skill_dir" ] || continue
    skill_name=$(basename "$skill_dir")
    skill_md="$skill_dir/SKILL.md"

    # Skip if no SKILL.md (other tests catch that)
    [ -f "$skill_md" ] || continue

    # Runtime compatibility pointers intentionally contain only frontmatter and
    # one canonical invocation. The redirect gate owns their format.
    grep -Eq '^implementation:[[:space:]]+false([[:space:]]|$)' "$skill_md" && continue

    CHECKED=$((CHECKED + 1))
    skill_ok=true

    # --- (a) tier: in frontmatter ---
    # Extract frontmatter (between first two --- lines)
    frontmatter=$(sed -n '1,/^---$/p' "$skill_md" | tail -n +2)
    # The above gets from line 1 to first ---; we need between first and second ---
    frontmatter=$(awk 'BEGIN{n=0} /^---$/{n++; if(n==2) exit; next} n==1{print}' "$skill_md")

    tier=""
    # tier is now under metadata: per Anthropic skills spec
    if echo "$frontmatter" | grep -q '^[[:space:]]*tier:'; then
        tier=$(echo "$frontmatter" | grep '^[[:space:]]*tier:' | head -1 | sed 's/^[[:space:]]*tier:[[:space:]]*//' | tr -d '\r')
    else
        fail "$skill_name" "missing 'tier:' in YAML frontmatter (under metadata:)"
        skill_ok=false
    fi

    # --- (b) Line count limits ---
    line_count=$(wc -l < "$skill_md" | tr -d ' ')

    if [ -n "$tier" ]; then
        limit=600
        case "$tier" in
            library|meta)
                limit=250
                ;;
            background)
                limit=300
                ;;
            execution|experimental)
                limit=800
                ;;
            judgment|product|session|knowledge|contribute|cross-vendor|orchestration)
                limit=1050
                ;;
        esac
        if [ "$line_count" -gt "$limit" ]; then
            fail "$skill_name" "tier=$tier, ${line_count} lines exceeds ${limit}-line limit"
            skill_ok=false
        fi
    fi

    # --- (c) >300 lines should have references/ (warning, not failure) ---
    if [ "$line_count" -gt 300 ] && [ ! -d "$skill_dir/references" ]; then
        WARNED=$((WARNED + 1))
        WARNINGS="${WARNINGS}  ${YELLOW}⚠${NC} ${skill_name}: ${line_count} lines but no references/ directory (consider splitting)\n"
    fi

    # --- (d) Word count limit ---
    word_count=$(wc -w < "$skill_md" | tr -d ' ')
    if [ "$word_count" -gt 5500 ]; then
        fail "$skill_name" "${word_count} words exceeds 5500-word limit"
        skill_ok=false
    fi

    # --- (e) Referenced files must exist ---
    # Match patterns like references/foo.md, references/bar-baz.md
    # Also handles cross-skill references:
    #   - skills/<name>/references/foo.md (repo-absolute)
    #   - ../shared/references/foo.md (relative to skill dir)
    #   - ../<sibling-skill>/references/foo.md (relative cross-skill)
    ref_paths=$(grep -oE '(\.\./[a-z-]+/|skills/[a-z-]+/)?references/[A-Za-z0-9_.-]+(\.[a-z]+)?' "$skill_md" 2>/dev/null || true)
    if [ -n "$ref_paths" ]; then
        while IFS= read -r ref; do
            [ -z "$ref" ] && continue
            if [[ "$ref" == skills/* ]]; then
                # Cross-skill reference — resolve from repo root
                check_path="$REPO_ROOT/$ref"
            elif [[ "$ref" == ../shared/* ]]; then
                # Relative shared reference — resolve via skill dir parent
                check_path="$skill_dir/$ref"
            else
                # Local reference — resolve from skill directory
                check_path="$skill_dir/$ref"
            fi
            if [ ! -f "$check_path" ]; then
                fail "$skill_name" "referenced file '$ref' does not exist"
                skill_ok=false
            fi
        done <<< "$ref_paths"
    fi

    if $skill_ok; then
        PASSED=$((PASSED + 1))
        echo -e "  ${GREEN}✓${NC} $skill_name (tier=$tier, ${line_count} lines)"
    else
        echo -e "  ${RED}✗${NC} $skill_name"
    fi
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "$CHECKED skills checked, ${GREEN}$PASSED passed${NC}, ${YELLOW}$WARNED warnings${NC}, ${RED}$FAILED failed${NC}"

if [ $WARNED -gt 0 ]; then
    echo ""
    echo -e "${YELLOW}Warnings:${NC}"
    echo -e "$WARNINGS"
fi

if [ $FAILED -gt 0 ]; then
    echo ""
    echo -e "${RED}Failures:${NC}"
    echo -e "$FAILURES"
    exit 1
else
    echo ""
    echo -e "${GREEN}All skills pass lint checks.${NC}"
    exit 0
fi
