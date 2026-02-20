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
            execution)
                limit=800
                ;;
            judgment|product|session|knowledge|contribute|cross-vendor|utility|team|orchestration|solo)
                limit=600
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

    # --- (d) Examples section required ---
    # User-facing skills: FAIL if missing. Internal skills: WARN if missing.
    USER_FACING="beads bug-hunt codex-team complexity council crank doc evolve handoff implement inbox knowledge plan post-mortem pre-mortem product quickstart release research retro rpi status swarm trace vibe openai-docs oss-docs pr-research pr-plan pr-implement pr-validate pr-prep pr-retro"
    INTERNAL="extract flywheel forge inject provenance ratchet shared standards using-agentops"

    is_user_facing=false
    for uf in $USER_FACING; do
        [ "$skill_name" = "$uf" ] && is_user_facing=true && break
    done

    is_internal=false
    for int in $INTERNAL; do
        [ "$skill_name" = "$int" ] && is_internal=true && break
    done

    has_examples=$(grep -c '^## Examples' "$skill_md" 2>/dev/null || echo 0)
    has_examples=$(echo "$has_examples" | tr -d '[:space:]')
    has_troubleshooting=$(grep -c '^## Troubleshooting' "$skill_md" 2>/dev/null || echo 0)
    has_troubleshooting=$(echo "$has_troubleshooting" | tr -d '[:space:]')

    if $is_user_facing; then
        if [ "$has_examples" -eq 0 ]; then
            fail "$skill_name" "missing '## Examples' section (required for user-facing skills)"
            skill_ok=false
        fi
        if [ "$has_troubleshooting" -eq 0 ]; then
            fail "$skill_name" "missing '## Troubleshooting' section (required for user-facing skills)"
            skill_ok=false
        fi
        # Format validation: Examples should have "User says" pattern
        if [ "$has_examples" -gt 0 ]; then
            user_says_count=$(grep -c '\*\*User says:\*\*' "$skill_md" 2>/dev/null || echo 0)
            user_says_count=$(echo "$user_says_count" | tr -d '[:space:]')
            if [ "$user_says_count" -eq 0 ]; then
                WARNED=$((WARNED + 1))
                WARNINGS="${WARNINGS}  ${YELLOW}⚠${NC} ${skill_name}: Examples section missing '**User says:**' format\n"
            fi
        fi
        # Format validation: Troubleshooting should have table format
        if [ "$has_troubleshooting" -gt 0 ]; then
            has_table=$(grep -c '| Problem |' "$skill_md" 2>/dev/null || echo 0)
            has_table=$(echo "$has_table" | tr -d '[:space:]')
            has_prose_troubleshoot=$(awk '/^## Troubleshooting/ { in_section=1; next } in_section && /^## / { exit } in_section && /^### / { count++ } END { print count+0 }' "$skill_md")
            has_prose_troubleshoot=$(echo "$has_prose_troubleshoot" | tr -d '[:space:]')
            # Accept either table format or prose format (some pre-existing skills use prose)
            if [ "$has_table" -eq 0 ] && [ "$has_prose_troubleshoot" -eq 0 ]; then
                WARNED=$((WARNED + 1))
                WARNINGS="${WARNINGS}  ${YELLOW}⚠${NC} ${skill_name}: Troubleshooting section has no table or structured entries\n"
            fi
        fi
    elif $is_internal; then
        # Internal skills: warn only (shared is excluded from content requirement)
        if [ "$skill_name" != "shared" ]; then
            if [ "$has_examples" -eq 0 ]; then
                WARNED=$((WARNED + 1))
                WARNINGS="${WARNINGS}  ${YELLOW}⚠${NC} ${skill_name}: missing '## Examples' section (recommended for internal skills)\n"
            fi
            if [ "$has_troubleshooting" -eq 0 ]; then
                WARNED=$((WARNED + 1))
                WARNINGS="${WARNINGS}  ${YELLOW}⚠${NC} ${skill_name}: missing '## Troubleshooting' section (recommended for internal skills)\n"
            fi
        fi
    fi

    # --- (e) Word count limit (5000 words) ---
    word_count=$(wc -w < "$skill_md" | tr -d ' ')
    if [ "$word_count" -gt 5000 ]; then
        fail "$skill_name" "${word_count} words exceeds 5000-word limit"
        skill_ok=false
    fi

    # --- (f) Referenced files must exist ---
    # Match patterns like references/foo.md, references/bar-baz.md
    # Also handles cross-skill references like skills/shared/references/foo.md
    ref_paths=$(grep -oE '(skills/[a-z-]+/)?references/[A-Za-z0-9_.-]+(\.[a-z]+)?' "$skill_md" 2>/dev/null || true)
    if [ -n "$ref_paths" ]; then
        while IFS= read -r ref; do
            [ -z "$ref" ] && continue
            if [[ "$ref" == skills/* ]]; then
                # Cross-skill reference — resolve from repo root
                check_path="$REPO_ROOT/$ref"
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

    # --- (g) Verdict schema v2 validation ---
    if [ "$skill_name" = "council" ]; then
        verdict_schema="$skill_dir/schemas/verdict.json"
        if [ -f "$verdict_schema" ]; then
            # Check fix/why/ref fields exist in findings items
            for field in fix why ref; do
                if ! grep -q "\"$field\"" "$verdict_schema" 2>/dev/null; then
                    fail "$skill_name" "verdict.json missing '$field' field in findings items (schema v2)"
                    skill_ok=false
                fi
            done
            # Check schema_version allows value 2
            if ! grep -q '"enum"' "$verdict_schema" 2>/dev/null || ! grep -q '2' "$verdict_schema" 2>/dev/null; then
                fail "$skill_name" "verdict.json schema_version doesn't allow value 2"
                skill_ok=false
            fi
            # Check fix/why/ref are required in schema v2 findings items.
            findings_required=$(python3 -c "
import json, sys
with open('$verdict_schema') as f:
    schema = json.load(f)
items_req = schema.get('properties', {}).get('findings', {}).get('items', {}).get('required', [])
for r in items_req:
    print(r)
" 2>/dev/null || true)
            for field in fix why ref; do
                if ! echo "$findings_required" | grep -qx "$field" 2>/dev/null; then
                    fail "$skill_name" "verdict.json '$field' must be in findings required array (schema v2)"
                    skill_ok=false
                fi
            done
        fi
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
