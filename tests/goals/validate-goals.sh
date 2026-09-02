#!/usr/bin/env bash
# Validate GOALS.md (or legacy GOALS.yaml) schema and fitness function integrity
#
# Usage:
#   bash tests/goals/validate-goals.sh              # validate the repo's GOALS.md
#   bash tests/goals/validate-goals.sh <goals-file> # validate an explicit file
#
# The path argument exists so the negative fixtures under tests/goals/fixtures/
# can be run through the real validator. Executable references inside a goals
# file always resolve against the repository root, never against the fixture's
# own directory: a fixture row naming a missing script must fail.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

errors=0

pass() { echo -e "${GREEN}  ✓${NC} $1"; }
fail() { echo -e "${RED}  ✗${NC} $1"; errors=$((errors + 1)); }

GOALS_ARG="${1:-}"

# Detect goals file format
if [[ -n "$GOALS_ARG" ]]; then
    if [[ ! -f "$GOALS_ARG" ]]; then
        fail "No goals file at $GOALS_ARG"
        exit 1
    fi
    GOALS_FILE="$GOALS_ARG"
    case "$GOALS_FILE" in
        *.yaml | *.yml) GOALS_FORMAT="yaml" ;;
        *) GOALS_FORMAT="md" ;;
    esac
    echo "Validating $GOALS_FILE..."
elif [[ -f "$REPO_ROOT/GOALS.md" ]]; then
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
    pass "Goals file exists ($GOALS_FILE)"

    # 1. Has mission (first non-heading, non-empty line after # Goals)
    if head -5 "$GOALS_FILE" | grep -qv '^#\|^$'; then
        pass "Has mission statement"
    else
        fail "Missing mission statement"
    fi

    # The 2026-08-25 rewrite replaced the North Stars / numbered-directives /
    # Steer shape with prose fitness properties plus one executable Gates
    # table. The assertions below are invariants of THAT shape: the table is
    # the only machine-consumed part of this file (`ao goals measure` runs it),
    # so it is what gets guarded — it must exist, be non-empty, and every row
    # must point at something that is really in the tree.

    # 2. Has Gates section with table
    if grep -q '^## Gates' "$GOALS_FILE"; then
        pass "Has Gates section"
    else
        fail "Missing Gates section"
    fi

    # 3. Gate table has entries (lines starting with |, excluding header/separator)
    gate_count=$(grep -cE '^\| [a-z]' "$GOALS_FILE" || true)
    if [[ $gate_count -gt 0 ]]; then
        pass "Found $gate_count gates"
    else
        fail "No gate entries found in table"
    fi

    # 4. Gate weights are in range 1-10
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

    # 5. No duplicate gate IDs
    # `|| true` on the producer only: with pipefail a zero-row table would kill
    # the script mid-run and swallow the remaining assertions. The zero-row
    # case is already reported as a failure by check 3 above.
    dup_count=$({ grep -E '^\| [a-z]' "$GOALS_FILE" || true; } | awk -F'|' '{print $2}' | sed 's/^ *//;s/ *$//' | sort | uniq -d | wc -l | tr -d ' ')
    if [[ $dup_count -eq 0 ]]; then
        pass "No duplicate gate IDs"
    else
        fail "Found $dup_count duplicate gate IDs"
    fi

    # 6. Every repository path a gate row cites really exists. This is the
    #    rot guard and the only new invariant: a Check cell that runs a
    #    deleted scripts/check-*.sh reads as executable and measures as a
    #    permanent failure. Rows whose Check is a self-contained command (for
    #    example `cd cli && go test ./...`) cite no path and are left alone —
    #    the check never demands that a bare gate id be registered anywhere.
    missing_paths=""
    while IFS= read -r line; do
        safe_line="${line//\\|/__AGENTOPS_ESCAPED_PIPE__}"
        gate_id=$(echo "$safe_line" | awk -F'|' '{gsub(/^ *| *$/, "", $2); print $2}')
        check_cell=$(echo "$safe_line" | awk -F'|' '{print $3}')
        while IFS= read -r ref; do
            [[ -n "$ref" ]] || continue
            # A cited path is a token under scripts/ or one ending in .sh.
            case "$ref" in
                scripts/* | *.sh) ;;
                *) continue ;;
            esac
            if [[ ! -e "$REPO_ROOT/$ref" ]]; then
                missing_paths+=" ${gate_id}->${ref}"
            fi
        done < <(echo "$check_cell" | tr -c 'A-Za-z0-9_./-' '\n' || true)
    done < <(grep -E '^\| [a-z]' "$GOALS_FILE" || true)

    if [[ -z "$missing_paths" ]]; then
        pass "Every repository path cited by a gate row exists"
    else
        fail "Gate rows cite missing paths:$missing_paths"
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

    goal_count=$(grep -c '^[[:space:]]*- id:' "$GOALS_FILE" || true)

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
