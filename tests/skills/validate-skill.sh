#!/bin/bash
# Generic skill validation framework
# Usage: validate-skill.sh <skill-name-or-path> [skills-base-dir]
#
# Validates:
# 1. SKILL.md exists and has valid frontmatter
# 2. All declared dependencies (skills:) exist
# 3. All referenced files exist
# 4. Runs skill-specific validate.sh if present
#
# Arguments:
#   <skill-name-or-path>  Either a skill name (looks in SKILLS_DIR) or full path to skill dir
#   [skills-base-dir]     Optional base directory for dependency resolution

# Test harness: intentionally -uo (not -euo) to accumulate ERRORS/CHECKS counters
set -uo pipefail

SKILL="${1:-}"
# Default skills dir, can be overridden by second arg or if first arg is a path
SKILLS_DIR="${2:-${HOME}/.claude/skills}"
ERRORS=0
CHECKS=0
WARNINGS=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

usage() {
    echo "Usage: $0 <skill-name-or-path> [skills-base-dir]"
    echo "       $0 --all [skills-base-dir]"
    echo ""
    echo "Arguments:"
    echo "  <skill-name-or-path>  Skill name or full path to skill directory"
    echo "  [skills-base-dir]     Base directory for dependency resolution (default: ~/.claude/skills)"
    echo ""
    echo "Options:"
    echo "  --all         Validate all skills in SKILLS_DIR"
    exit 1
}

check() {
    local desc="$1"
    local result="$2"  # 0 = pass, non-zero = fail

    CHECKS=$((CHECKS + 1))
    if [ "$result" -eq 0 ]; then
        echo -e "  ${GREEN}✓${NC} $desc"
        return 0
    else
        echo -e "  ${RED}✗${NC} $desc"
        ERRORS=$((ERRORS + 1))
        return 1
    fi
}

warn() {
    local desc="$1"
    echo -e "  ${YELLOW}⚠${NC} $desc"
    WARNINGS=$((WARNINGS + 1))
}

# Extract frontmatter field from SKILL.md
# Usage: get_frontmatter "field" "skill-dir"
get_frontmatter() {
    local field="$1"
    local skill_dir="$2"
    local skill_md="$skill_dir/SKILL.md"

    if [ ! -f "$skill_md" ]; then
        return 1
    fi

    # Extract YAML frontmatter between --- markers
    # Then extract the field value
    sed -n '/^---$/,/^---$/p' "$skill_md" | \
        grep "^${field}:" | \
        sed "s/^${field}:[[:space:]]*//" | \
        tr -d '"' | tr -d "'"
}

# Extract skills array from frontmatter
# Returns newline-separated list of skill names
get_skill_dependencies() {
    local skill_dir="$1"
    local skill_md="$skill_dir/SKILL.md"

    if [ ! -f "$skill_md" ]; then
        return 1
    fi

    # Extract YAML frontmatter and find skills: section
    # Handle both inline array and multi-line list formats
    sed -n '/^---$/,/^---$/p' "$skill_md" | \
        awk '/^skills:/{found=1; next} found && /^[^ -]/{exit} found && /^  - /{print substr($0, 5)}' | \
        tr -d ' '
}

# Extract dependencies array from frontmatter
# Returns newline-separated list of dependency names (strips comments)
get_declared_dependencies() {
    local skill_dir="$1"
    local skill_md="$skill_dir/SKILL.md"

    if [ ! -f "$skill_md" ]; then
        return 1
    fi

    # Extract YAML frontmatter, find dependencies: section (under metadata:)
    # Handle multi-line list format, strip inline comments
    sed -n '/^---$/,/^---$/p' "$skill_md" | \
        awk '/^[[:space:]]*dependencies:/{found=1; next} found && /^[[:space:]]{0,3}[^ -]/{exit} found && /^[[:space:]]*- /{gsub(/^[[:space:]]*- /, ""); print}' | \
        sed 's/#.*//' | \
        tr -d ' '
}

# Validate a single skill
# Args: skill_name_or_path [dep_base_dir]
# If skill_name_or_path is a directory, use it directly
# Otherwise, look in SKILLS_DIR
validate_skill() {
    local skill_input="$1"
    local dep_base="${2:-$SKILLS_DIR}"
    local skill_dir
    local skill_name

    # Determine if input is a path or a name
    if [ -d "$skill_input" ]; then
        skill_dir="$skill_input"
        skill_name=$(basename "$skill_dir")
    else
        skill_name="$skill_input"
        skill_dir="$SKILLS_DIR/$skill_name"
    fi

    local local_errors=0
    local local_checks=0

    echo "━━━ Validating: $skill_name ━━━"

    # Test 1: SKILL.md exists
    if [ -f "$skill_dir/SKILL.md" ]; then
        echo -e "  ${GREEN}✓${NC} SKILL.md exists"
        local_checks=$((local_checks + 1))
    else
        echo -e "  ${RED}✗${NC} SKILL.md exists"
        local_errors=$((local_errors + 1))
        local_checks=$((local_checks + 1))
        echo ""
        echo "  Result: FAIL ($local_errors errors)"
        ERRORS=$((ERRORS + local_errors))
        CHECKS=$((CHECKS + local_checks))
        return 1
    fi

    # Test 2: Frontmatter has required fields
    local name
    name=$(get_frontmatter "name" "$skill_dir")
    if [ -n "$name" ]; then
        echo -e "  ${GREEN}✓${NC} Frontmatter: name field present ($name)"
        local_checks=$((local_checks + 1))
    else
        echo -e "  ${RED}✗${NC} Frontmatter: name field present"
        local_errors=$((local_errors + 1))
        local_checks=$((local_checks + 1))
    fi

    local description
    description=$(get_frontmatter "description" "$skill_dir")
    if [ -n "$description" ]; then
        echo -e "  ${GREEN}✓${NC} Frontmatter: description field present"
        local_checks=$((local_checks + 1))
    else
        echo -e "  ${YELLOW}⚠${NC} Frontmatter: description field missing"
        WARNINGS=$((WARNINGS + 1))
    fi

    # Test 3: Check declared skill dependencies exist
    # Note: For agentops plugins, dependencies might be in different plugin kits
    # Skip dependency check for plugin-based skills (dependency resolution is complex)
    local deps
    deps=$(get_skill_dependencies "$skill_dir")
    if [ -n "$deps" ]; then
        while IFS= read -r dep; do
            # Check both in dep_base and in SKILLS_DIR
            if [ -d "$dep_base/$dep" ] || [ -d "$SKILLS_DIR/$dep" ]; then
                echo -e "  ${GREEN}✓${NC} Dependency: $dep exists"
                local_checks=$((local_checks + 1))
            else
                # For plugin skills, dependencies might be in other plugin kits
                # Warn instead of fail
                echo -e "  ${YELLOW}⚠${NC} Dependency: $dep (not found locally, may be in another plugin)"
                WARNINGS=$((WARNINGS + 1))
            fi
        done <<< "$deps"
    fi

    # Test 3b: Check declared dependencies (dependencies: field) exist
    local declared_deps
    declared_deps=$(get_declared_dependencies "$skill_dir")
    if [ -n "$declared_deps" ]; then
        while IFS= read -r dep; do
            [ -z "$dep" ] && continue
            if [ -d "$dep_base/$dep" ] || [ -d "$SKILLS_DIR/$dep" ]; then
                echo -e "  ${GREEN}✓${NC} Dependency: $dep exists"
                local_checks=$((local_checks + 1))
            else
                echo -e "  ${YELLOW}⚠${NC} Dependency: $dep not found (may be external or not yet created)"
                WARNINGS=$((WARNINGS + 1))
            fi
        done <<< "$declared_deps"
    fi

    # Test 4: Check references directory if local references are mentioned in SKILL.md
    # Only check for LOCAL references (references/foo.md), not cross-skill refs (kit/skills/x/references/)
    # Local refs start with backtick or quote followed by "references/"
    if grep -qE '[\`\"\047]references/' "$skill_dir/SKILL.md" 2>/dev/null; then
        if [ -d "$skill_dir/references" ]; then
            echo -e "  ${GREEN}✓${NC} References directory exists"
            local_checks=$((local_checks + 1))

            # Check specific referenced files (only local refs without path prefix)
            local refs
            refs=$(grep -oE '[\`\"\047]references/[a-zA-Z0-9_-]+\.md' "$skill_dir/SKILL.md" | sed 's/^[\`\"\047]//' | sort -u)
            if [ -n "$refs" ]; then
                while IFS= read -r ref; do
                    if [ -f "$skill_dir/$ref" ]; then
                        echo -e "  ${GREEN}✓${NC} Reference: $ref exists"
                        local_checks=$((local_checks + 1))
                    else
                        echo -e "  ${RED}✗${NC} Reference: $ref exists"
                        local_errors=$((local_errors + 1))
                        local_checks=$((local_checks + 1))
                    fi
                done <<< "$refs"
            fi
        else
            echo -e "  ${RED}✗${NC} References directory exists (referenced in SKILL.md)"
            local_errors=$((local_errors + 1))
            local_checks=$((local_checks + 1))
        fi
    fi

    # Test 4b: Skill-specific flag allowlist / contract validation (repo-local)
    #
    # These checks enforce that documented flag values stay in sync with their canonical reference docs,
    # preventing silent drift where invalid values appear to "work" but are ignored/defaulted.
    local skill_md
    skill_md="$skill_dir/SKILL.md"

    if [[ "$skill_name" == "council" ]]; then
        local techniques_ref profiles_ref
        techniques_ref="$skill_dir/references/brainstorm-techniques.md"
        profiles_ref="$skill_dir/references/model-profiles.md"

        # Extract allowlisted technique slugs from the canonical table in brainstorm-techniques.md
        local allowed_techniques
        if [[ -f "$techniques_ref" ]]; then
            allowed_techniques="$(grep -E '^\|[[:space:]]*`[a-zA-Z0-9_-]+`[[:space:]]*\|' "$techniques_ref" 2>/dev/null | sed -E 's/^\|[[:space:]]*`([^`]+)`.*/\1/' | sort -u)"
            if [[ -n "${allowed_techniques:-}" ]]; then
                echo -e "  ${GREEN}✓${NC} Council: extracted technique allowlist from references"
                local_checks=$((local_checks + 1))
            else
                echo -e "  ${RED}✗${NC} Council: extracted technique allowlist from references"
                local_errors=$((local_errors + 1))
                local_checks=$((local_checks + 1))
            fi
        else
            echo -e "  ${RED}✗${NC} Council: references/brainstorm-techniques.md exists"
            local_errors=$((local_errors + 1))
            local_checks=$((local_checks + 1))
        fi

        # Extract allowlisted profile slugs from model-profiles.md (first column backticks)
        local allowed_profiles
        if [[ -f "$profiles_ref" ]]; then
            allowed_profiles="$(grep -E '^\|[[:space:]]*`[a-zA-Z0-9_-]+`[[:space:]]*\|' "$profiles_ref" 2>/dev/null | sed -E 's/^\|[[:space:]]*`([^`]+)`.*/\1/' | sort -u)"
            if [[ -n "${allowed_profiles:-}" ]]; then
                echo -e "  ${GREEN}✓${NC} Council: extracted profile allowlist from references"
                local_checks=$((local_checks + 1))
            else
                echo -e "  ${RED}✗${NC} Council: extracted profile allowlist from references"
                local_errors=$((local_errors + 1))
                local_checks=$((local_checks + 1))
            fi
        else
            echo -e "  ${RED}✗${NC} Council: references/model-profiles.md exists"
            local_errors=$((local_errors + 1))
            local_checks=$((local_checks + 1))
        fi

        # Verify council SKILL.md documents allowlisted technique names in the --technique row
        local technique_row documented_techniques
        technique_row="$(grep -F '| `--technique=<name>` |' "$skill_md" 2>/dev/null | head -n 1 || true)"
        if [[ -n "${technique_row:-}" ]]; then
            documented_techniques="$(echo "$technique_row" | sed -n 's/.*technique (\([^)]*\)).*/\1/p' | tr ',' $'\n' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//' | grep -E '^[a-zA-Z0-9_-]+$' | sort -u)"
            if [[ -n "${documented_techniques:-}" ]] && [[ -n "${allowed_techniques:-}" ]] && diff -u <(echo "$allowed_techniques") <(echo "$documented_techniques") >/dev/null 2>&1; then
                echo -e "  ${GREEN}✓${NC} Council: --technique allowlist matches references"
                local_checks=$((local_checks + 1))
            else
                echo -e "  ${RED}✗${NC} Council: --technique allowlist matches references"
                [[ -n "${allowed_techniques:-}" ]] && echo "    expected:" && echo "$allowed_techniques" | sed 's/^/      - /'
                [[ -n "${documented_techniques:-}" ]] && echo "    documented:" && echo "$documented_techniques" | sed 's/^/      - /'
                local_errors=$((local_errors + 1))
                local_checks=$((local_checks + 1))
            fi
        else
            echo -e "  ${RED}✗${NC} Council: documents --technique=<name> flag row"
            local_errors=$((local_errors + 1))
            local_checks=$((local_checks + 1))
        fi

        # Verify council SKILL.md documents allowlisted profile names in the --profile row
        local profile_row documented_profiles
        profile_row="$(grep -F '| `--profile=<name>` |' "$skill_md" 2>/dev/null | head -n 1 || true)"
        if [[ -n "${profile_row:-}" ]]; then
            documented_profiles="$(echo "$profile_row" | sed -n 's/.*profile (\([^)]*\)).*/\1/p' | tr ',' $'\n' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//' | grep -E '^[a-zA-Z0-9_-]+$' | sort -u)"
            if [[ -n "${documented_profiles:-}" ]] && [[ -n "${allowed_profiles:-}" ]] && diff -u <(echo "$allowed_profiles") <(echo "$documented_profiles") >/dev/null 2>&1; then
                echo -e "  ${GREEN}✓${NC} Council: --profile allowlist matches references"
                local_checks=$((local_checks + 1))
            else
                echo -e "  ${RED}✗${NC} Council: --profile allowlist matches references"
                [[ -n "${allowed_profiles:-}" ]] && echo "    expected:" && echo "$allowed_profiles" | sed 's/^/      - /'
                [[ -n "${documented_profiles:-}" ]] && echo "    documented:" && echo "$documented_profiles" | sed 's/^/      - /'
                local_errors=$((local_errors + 1))
                local_checks=$((local_checks + 1))
            fi
        else
            echo -e "  ${RED}✗${NC} Council: documents --profile=<name> flag row"
            local_errors=$((local_errors + 1))
            local_checks=$((local_checks + 1))
        fi
    fi

    if [[ "$skill_name" == "crank" ]]; then
        local commit_ref
        commit_ref="$skill_dir/references/commit-strategies.md"

        local per_task_row
        per_task_row="$(grep -F '| `--per-task-commits` |' "$skill_md" 2>/dev/null | head -n 1 || true)"
        if [[ -n "${per_task_row:-}" ]] && echo "$per_task_row" | grep -q 'references/commit-strategies.md'; then
            echo -e "  ${GREEN}✓${NC} Crank: --per-task-commits flag row references commit-strategies.md"
            local_checks=$((local_checks + 1))
        else
            echo -e "  ${RED}✗${NC} Crank: --per-task-commits flag row references commit-strategies.md"
            local_errors=$((local_errors + 1))
            local_checks=$((local_checks + 1))
        fi

        if [[ -f "$commit_ref" ]] && grep -q '^## wave-batch' "$commit_ref" && grep -q '^## per-task' "$commit_ref" && grep -q 'wave-batch-fallback' "$commit_ref"; then
            echo -e "  ${GREEN}✓${NC} Crank: commit strategy contract strings present"
            local_checks=$((local_checks + 1))
        else
            echo -e "  ${RED}✗${NC} Crank: commit strategy contract strings present"
            local_errors=$((local_errors + 1))
            local_checks=$((local_checks + 1))
        fi
    fi

    # Test 5: Run skill-specific validate.sh if present
    local validate_script="$skill_dir/scripts/validate.sh"
    if [ -f "$validate_script" ]; then
        echo ""
        echo "  Running skill-specific tests..."
        chmod +x "$validate_script"
        if "$validate_script"; then
            echo -e "  ${GREEN}✓${NC} Skill-specific validation passed"
            local_checks=$((local_checks + 1))
        else
            echo -e "  ${RED}✗${NC} Skill-specific validation failed"
            local_errors=$((local_errors + 1))
            local_checks=$((local_checks + 1))
        fi
    fi

    echo ""
    if [ $local_errors -gt 0 ]; then
        echo -e "  Result: ${RED}FAIL${NC} ($local_errors errors, $local_checks checks)"
    else
        echo -e "  Result: ${GREEN}PASS${NC} ($local_checks checks)"
    fi

    ERRORS=$((ERRORS + local_errors))
    CHECKS=$((CHECKS + local_checks))

    return $local_errors
}

# Main
if [ -z "$SKILL" ]; then
    usage
fi

if [ "$SKILL" = "--all" ]; then
    echo "╔════════════════════════════════════════════╗"
    echo "║   Generic Skill Validation Framework       ║"
    echo "╚════════════════════════════════════════════╝"
    echo ""

    PASSED=0
    FAILED=0

    for skill_dir in "$SKILLS_DIR"/*/; do
        skill_name=$(basename "$skill_dir")
        # Skip leading-underscore scaffolding (e.g. skills/_fixtures/) — planted
        # test fixtures, not real skills.
        case "$skill_name" in _*) continue ;; esac
        if validate_skill "$skill_name"; then
            PASSED=$((PASSED + 1))
        else
            FAILED=$((FAILED + 1))
        fi
        echo ""
    done

    echo "╔════════════════════════════════════════════╗"
    echo "║               SUMMARY                      ║"
    echo "╠════════════════════════════════════════════╣"
    printf "║  Skills Passed:  %-24s ║\n" "$PASSED"
    printf "║  Skills Failed:  %-24s ║\n" "$FAILED"
    printf "║  Total Checks:   %-24s ║\n" "$CHECKS"
    printf "║  Total Errors:   %-24s ║\n" "$ERRORS"
    printf "║  Warnings:       %-24s ║\n" "$WARNINGS"
    echo "╚════════════════════════════════════════════╝"

    if [ $FAILED -gt 0 ]; then
        echo ""
        echo "OVERALL: FAIL"
        exit 1
    else
        echo ""
        echo "OVERALL: PASS"
        exit 0
    fi
else
    # Validate single skill (can be path or name)
    # If it's a directory path, use directly; otherwise look in SKILLS_DIR
    if [ -d "$SKILL" ]; then
        # Direct path provided
        if validate_skill "$SKILL" "$SKILLS_DIR"; then
            exit 0
        else
            exit 1
        fi
    elif [ -d "$SKILLS_DIR/$SKILL" ]; then
        # Skill name provided, found in SKILLS_DIR
        if validate_skill "$SKILL" "$SKILLS_DIR"; then
            exit 0
        else
            exit 1
        fi
    else
        echo "Error: Skill '$SKILL' not found (checked: $SKILL, $SKILLS_DIR/$SKILL)"
        exit 1
    fi
fi
