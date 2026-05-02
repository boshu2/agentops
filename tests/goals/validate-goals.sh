#!/usr/bin/env bash
# Validate GOALS.md (or legacy GOALS.yaml) schema and fitness function integrity
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

errors=0

pass() { echo -e "${GREEN}  ✓${NC} $1"; }
fail() { echo -e "${RED}  ✗${NC} $1"; errors=$((errors + 1)); }

# Detect goals file format
if [[ -f "$REPO_ROOT/GOALS.md" ]]; then
    GOALS_FILE="$REPO_ROOT/GOALS.md"
    GOALS_FORMAT="md"
    echo "Validating GOALS.md..."
elif [[ -f "$REPO_ROOT/GOALS.yaml" ]]; then
    GOALS_FILE="$REPO_ROOT/GOALS.yaml"
    GOALS_FORMAT="yaml"
    echo "Validating GOALS.yaml..."
else
    fail "No GOALS.md or GOALS.yaml found at $REPO_ROOT"
    exit 1
fi

if [[ "$GOALS_FORMAT" == "md" ]]; then
    pass "GOALS.md exists"

    # 1. Has mission (first non-heading, non-empty line after # Goals)
    if head -5 "$GOALS_FILE" | grep -qv '^#\|^$'; then
        pass "Has mission statement"
    else
        fail "Missing mission statement"
    fi

    # 2. Has North Stars section
    if grep -q '^## North Stars' "$GOALS_FILE"; then
        pass "Has North Stars section"
    else
        fail "Missing North Stars section"
    fi

    # 3. Has Directives section with numbered headings
    directive_count=$(grep -c '^### [0-9]' "$GOALS_FILE" || true)
    if [[ $directive_count -gt 0 ]]; then
        pass "Found $directive_count directives"
    else
        fail "No directives found (expected ### N. headings)"
    fi

    # 4. Each directive has a Steer line
    steer_count=$(grep -c '^\*\*Steer:\*\*' "$GOALS_FILE" || true)
    if [[ $steer_count -ge $directive_count ]]; then
        pass "All directives have Steer field"
    else
        fail "Some directives missing Steer ($steer_count of $directive_count)"
    fi

    # 5. Has Gates section with table
    if grep -q '^## Gates' "$GOALS_FILE"; then
        pass "Has Gates section"
    else
        fail "Missing Gates section"
    fi

    # 6. Gate table has entries (lines starting with |, excluding header/separator)
    gate_count=$(grep -cE '^\| [a-z]' "$GOALS_FILE" || true)
    if [[ $gate_count -gt 0 ]]; then
        pass "Found $gate_count gates"
    else
        fail "No gate entries found in table"
    fi

    # 7. Gate weights are in range 1-10
    # Parse weight from second-to-last column (pipes in Check column break naive awk)
    bad_weights=0
    while IFS= read -r line; do
        # Split on markdown table pipes after hiding escaped pipes in commands.
        safe_line="${line//\\|/__AGENTOPS_ESCAPED_PIPE__}"
        w=$(echo "$safe_line" | awk -F'|' '{gsub(/ /, "", $4); print $4}')
        if [[ -n "$w" ]] && { ! [[ "$w" =~ ^[0-9]+$ ]] || [[ "$w" -lt 1 ]] || [[ "$w" -gt 10 ]]; }; then
            bad_weights=$((bad_weights + 1))
        fi
    done < <(grep -E '^\| [a-z]' "$GOALS_FILE")

    if [[ $bad_weights -eq 0 ]]; then
        pass "All gate weights in range 1-10"
    else
        fail "$bad_weights gate weights out of range"
    fi

    # 8. No duplicate gate IDs
    dup_count=$(grep -E '^\| [a-z]' "$GOALS_FILE" | awk -F'|' '{print $2}' | sed 's/^ *//;s/ *$//' | sort | uniq -d | wc -l | tr -d ' ')
    if [[ $dup_count -eq 0 ]]; then
        pass "No duplicate gate IDs"
    else
        fail "Found $dup_count duplicate gate IDs"
    fi

else
    # Legacy GOALS.yaml validation
    pass "GOALS.yaml exists"

    if python3 -c "import yaml; yaml.safe_load(open('$GOALS_FILE'))" 2>/dev/null; then
        pass "Valid YAML syntax"
    else
        fail "Invalid YAML syntax"
        exit 1
    fi

    if grep -q '^version:' "$GOALS_FILE"; then
        pass "Has version field"
    else
        fail "Missing version field"
    fi

    if grep -q '^mission:' "$GOALS_FILE"; then
        pass "Has mission field"
    else
        fail "Missing mission field"
    fi

    goal_count=0
    while IFS= read -r id; do
        goal_count=$((goal_count + 1))
    done < <(grep '^\s*- id:' "$GOALS_FILE" | sed 's/.*id:\s*//' | tr -d '"' | tr -d "'")

    if [[ $goal_count -gt 0 ]]; then
        pass "Found $goal_count goals"
    else
        fail "No goals found"
    fi

    desc_count=$(grep -c '^\s*description:' "$GOALS_FILE" || true)
    check_count=$(grep -c '^\s*check:' "$GOALS_FILE" || true)
    weight_count=$(grep -c '^\s*weight:' "$GOALS_FILE" || true)

    if [[ $desc_count -ge $goal_count ]]; then
        pass "All goals have description field"
    else
        fail "Some goals missing description ($desc_count of $goal_count)"
    fi

    if [[ $check_count -ge $goal_count ]]; then
        pass "All goals have check field"
    else
        fail "Some goals missing check ($check_count of $goal_count)"
    fi

    if [[ $weight_count -ge $goal_count ]]; then
        pass "All goals have weight field"
    else
        fail "Some goals missing weight ($weight_count of $goal_count)"
    fi

    dup_count=$(grep '^\s*- id:' "$GOALS_FILE" | sed 's/.*id:\s*//' | tr -d '"' | tr -d "'" | sort | uniq -d | wc -l | tr -d ' ')
    if [[ $dup_count -eq 0 ]]; then
        pass "No duplicate goal IDs"
    else
        fail "Found $dup_count duplicate goal IDs"
    fi

    bad_weights=0
    while IFS= read -r w; do
        w=$(echo "$w" | tr -d ' ')
        if ! [[ "$w" =~ ^[0-9]+$ ]] || [[ "$w" -lt 1 ]] || [[ "$w" -gt 10 ]]; then
            bad_weights=$((bad_weights + 1))
        fi
    done < <(grep '^\s*weight:' "$GOALS_FILE" | sed 's/.*weight:\s*//' | tr -d '"')

    if [[ $bad_weights -eq 0 ]]; then
        pass "All weights in range 1-10"
    else
        fail "$bad_weights weights out of range"
    fi
fi

echo ""
if [[ $errors -eq 0 ]]; then
    echo -e "${GREEN}Goals validation passed${NC}"
    exit 0
else
    echo -e "${RED}Goals validation failed ($errors errors)${NC}"
    exit 1
fi
