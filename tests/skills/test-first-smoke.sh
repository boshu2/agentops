#!/usr/bin/env bash
# Smoke test for --test-first flow structural patterns
# Validates that all skill files contain required structural elements
# for the spec-first TDD pipeline (SPEC WAVE → TEST WAVE → GREEN mode).
#
# Usage: ./tests/skills/test-first-smoke.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

total=0
passed=0
failed=0

pass() { echo -e "${GREEN}  PASS${NC} $1"; ((total++)) || true; ((passed++)) || true; }
fail() { echo -e "${RED}  FAIL${NC} $1"; ((total++)) || true; ((failed++)) || true; }

cd "$REPO_ROOT"

# =============================================================================
# 1. Contract Template checks
# =============================================================================
echo -e "${BLUE}[TEST-FIRST]${NC} Contract template (skills/crank/references/contract-template.md)"

CONTRACT="skills/crank/references/contract-template.md"

if [[ -f "$CONTRACT" ]]; then
    pass "File exists"
else
    fail "File missing: $CONTRACT"
    # All subsequent contract checks will fail; skip to next section
    echo -e "${BLUE}[TEST-FIRST]${NC} Skipping remaining contract checks (file missing)"
fi

if [[ -f "$CONTRACT" ]]; then
    # 1a: All 7 required ## headings
    for heading in "Problem" "Inputs" "Outputs" "Invariants" "Failure Modes" "Out of Scope" "Test Cases"; do
        if grep -qE "^## ${heading}$" "$CONTRACT"; then
            pass "Contract has ## $heading heading"
        else
            fail "Contract missing ## $heading heading"
        fi
    done

    # 1b: Contract Granularity section
    if grep -qE "^## Contract Granularity$" "$CONTRACT"; then
        pass "Contract has ## Contract Granularity section"
    else
        fail "Contract missing ## Contract Granularity section"
    fi

    # 1c: YAML frontmatter with framework field
    if head -20 "$CONTRACT" | grep -q '```yaml' && grep -q 'framework:' "$CONTRACT"; then
        pass "Contract has YAML frontmatter with framework field"
    else
        fail "Contract missing YAML frontmatter with framework field"
    fi

    # 1d: Minimum 30 lines (structural check, not a stub)
    line_count=$(wc -l < "$CONTRACT" | tr -d ' ')
    if [[ "$line_count" -ge 30 ]]; then
        pass "Contract has $line_count lines (>= 30 minimum)"
    else
        fail "Contract has only $line_count lines (< 30 minimum)"
    fi
fi

# =============================================================================
# 2. Crank SKILL.md checks
# =============================================================================
echo -e "${BLUE}[TEST-FIRST]${NC} Crank SKILL.md (skills/crank/SKILL.md)"

CRANK="skills/crank/SKILL.md"
# Billboard refactor (#275) thinned crank SKILL.md and moved step detail into
# references/. Content-presence checks scan the skill as a unit: SKILL.md + references/.
CRANK_REFS="skills/crank/references/"

if [[ ! -f "$CRANK" ]]; then
    fail "File missing: $CRANK"
else
    # 2a: --test-first in a table row (pipe-delimited, not just prose mention)
    if grep -qE '^\|.*--test-first.*\|' "$CRANK"; then
        pass "--test-first appears in a table row (flag table)"
    else
        fail "--test-first not found in any table row"
    fi

    # 2b: SPEC WAVE (Step 3b) documented in the crank skill unit (SKILL.md + references/)
    # Billboard refactor moved step detail into references/wave-dispatch.md.
    if grep -rqsE 'Step 3b: SPEC WAVE' "$CRANK" "$CRANK_REFS"; then
        pass "Step 3b: SPEC WAVE documented in crank skill"
    else
        fail "Step 3b: SPEC WAVE missing from crank skill (SKILL.md + references/)"
    fi

    # 2c: TEST WAVE (Step 3c) documented in the crank skill unit
    if grep -rqsE 'Step 3c: TEST WAVE' "$CRANK" "$CRANK_REFS"; then
        pass "Step 3c: TEST WAVE documented in crank skill"
    else
        fail "Step 3c: TEST WAVE missing from crank skill (SKILL.md + references/)"
    fi

    # 2d: Category-based skip logic (spec-eligible or docs/chore pattern), skill-wide
    if grep -rqsE 'spec-eligible|spec.eligible' "$CRANK" "$CRANK_REFS" \
        && grep -rqsE 'docs.*chore|chore.*docs' "$CRANK" "$CRANK_REFS"; then
        pass "Category-based skip logic present in crank skill (spec-eligible + docs/chore)"
    else
        fail "Category-based skip logic missing from crank skill (need spec-eligible AND docs/chore)"
    fi

    # 2e: Backward compat — Step 4 (standard wave execution) documented skill-wide
    if grep -rqsE 'Step 4:' "$CRANK" "$CRANK_REFS"; then
        pass "Backward compat: Step 4 documented in crank skill"
    else
        fail "Backward compat: Step 4 missing from crank skill (standard wave execution removed)"
    fi

    # 2f: Backward compat — Step 0 (Load Knowledge Context) documented skill-wide
    if grep -rqsE 'Step 0:' "$CRANK" "$CRANK_REFS"; then
        pass "Backward compat: Step 0 documented in crank skill (Load Knowledge Context)"
    else
        fail "Backward compat: Step 0 missing from crank skill (Load Knowledge Context removed)"
    fi
fi

# =============================================================================
# 3. Wave-patterns.md checks
# =============================================================================
echo -e "${BLUE}[TEST-FIRST]${NC} Wave patterns (skills/crank/references/wave-patterns.md)"

WAVES="skills/crank/references/wave-patterns.md"

if [[ ! -f "$WAVES" ]]; then
    fail "File missing: $WAVES"
else
    # 3a: Spec-First Wave Model section
    if grep -qE '^## Spec-First Wave Model' "$WAVES"; then
        pass "Spec-First Wave Model section exists"
    else
        fail "Spec-First Wave Model section missing"
    fi

    # 3b: RED gate documented
    if grep -qE 'RED (confirmation|gate)|RED Confirmation Gate' "$WAVES"; then
        pass "RED gate documented"
    else
        fail "RED gate not documented"
    fi

    # 3c: GREEN gate documented
    if grep -qE 'GREEN (confirmation|gate)|GREEN Confirmation Gate' "$WAVES"; then
        pass "GREEN gate documented"
    else
        fail "GREEN gate not documented"
    fi

    # 3d: Category-based skip documented
    if grep -qE 'Category.Based Skip|category.based skip' "$WAVES"; then
        pass "Category-based skip documented"
    else
        fail "Category-based skip not documented"
    fi
fi

# =============================================================================
# 4. Pre-mortem gate hook checks
# =============================================================================
echo -e "${BLUE}[TEST-FIRST]${NC} Pre-mortem gate hook (hooks/pre-mortem-gate.sh)"

GATE="hooks/pre-mortem-gate.sh"

if [[ ! -f "$GATE" ]]; then
    fail "File missing: $GATE"
else
    # 4a: File is executable
    if [[ -x "$GATE" ]]; then
        pass "pre-mortem-gate.sh is executable"
    else
        fail "pre-mortem-gate.sh is not executable"
    fi

    # 4b: Kill switch env var
    if grep -q 'AGENTOPS_SKIP_PRE_MORTEM_GATE' "$GATE"; then
        pass "Kill switch (AGENTOPS_SKIP_PRE_MORTEM_GATE) present"
    else
        fail "Kill switch (AGENTOPS_SKIP_PRE_MORTEM_GATE) missing"
    fi

    # 4c: Worker exemption
    if grep -q 'AGENTOPS_WORKER' "$GATE"; then
        pass "Worker exemption (AGENTOPS_WORKER) present"
    else
        fail "Worker exemption (AGENTOPS_WORKER) missing"
    fi

    # 4d: Bypass flag
    if grep -q '\-\-skip-pre-mortem' "$GATE"; then
        pass "Bypass flag (--skip-pre-mortem) present"
    else
        fail "Bypass flag (--skip-pre-mortem) missing"
    fi

    # 4e: Reads stdin
    if grep -qE 'INPUT=\$\(cat\)' "$GATE"; then
        pass "Reads stdin (INPUT=\$(cat))"
    else
        fail "Stdin reading pattern missing"
    fi

    # 4f: jq parsing with fallback
    if grep -q 'command -v jq' "$GATE" && grep -q 'Fallback' "$GATE"; then
        pass "JSON parsing with jq + fallback"
    else
        fail "JSON parsing (jq + fallback) missing"
    fi

    # 4g: Exit 2 for blocking
    if grep -q 'exit 2' "$GATE"; then
        pass "Exit code 2 for blocking"
    else
        fail "Exit code 2 (blocking) missing"
    fi

    # 4h: Fail-open pattern (exit 0 on missing deps)
    fail_open_count=$(grep -c 'exit 0' "$GATE")
    if [[ "$fail_open_count" -ge 3 ]]; then
        pass "Fail-open pattern ($fail_open_count exit 0 paths)"
    else
        fail "Insufficient fail-open paths (found $fail_open_count, need >= 3)"
    fi

    # 4i: Registered in hooks.json
    if grep -q 'pre-mortem-gate.sh' hooks/hooks.json 2>/dev/null; then
        pass "Registered in hooks.json"
    else
        fail "Not registered in hooks.json"
    fi

    # 4j: Skill tool matcher in hooks.json
    if grep -q '"matcher": "Skill"' hooks/hooks.json 2>/dev/null; then
        pass "Skill tool matcher in hooks.json"
    else
        fail "Skill tool matcher missing in hooks.json"
    fi
fi

# =============================================================================
# 5. Implement SKILL.md checks
# =============================================================================
echo -e "${BLUE}[TEST-FIRST]${NC} Implement SKILL.md (skills/implement/SKILL.md)"

IMPL="skills/implement/SKILL.md"

if [[ ! -f "$IMPL" ]]; then
    fail "File missing: $IMPL"
else
    # 5a: ### GREEN Mode section heading
    if grep -qE '^### GREEN Mode' "$IMPL"; then
        pass "### GREEN Mode section heading exists"
    else
        fail "### GREEN Mode section heading missing"
    fi

    # 5b: Test immutability rule documented in the implement skill unit
    # (SKILL.md + references/). Billboard refactor moved GREEN-mode detail into
    # references/green-mode.md; the rule is also stated inline in SKILL.md.
    if grep -rqsiE 'do(es)? NOT modify test|MUST NOT modify (existing )?test|tests are immutable|test files (provided|immutable)|failing tests \(immutable\)' \
        "$IMPL" "skills/implement/references/"; then
        pass "Test immutability rule documented in implement skill"
    else
        fail "Test immutability rule missing from implement skill (SKILL.md + references/)"
    fi
fi

# =============================================================================
# 6. Passthrough wiring checks (rpi + evolve)
# =============================================================================
echo -e "${BLUE}[TEST-FIRST]${NC} Passthrough wiring (skills/rpi/SKILL.md, skills/evolve/SKILL.md)"

RPI="skills/rpi/SKILL.md"
EVOLVE="skills/evolve/SKILL.md"

if [[ ! -f "$RPI" ]]; then
    fail "File missing: $RPI"
else
    # 6a: --test-first in rpi flag table with default on
    if grep -qE '^\|.*--test-first.*\|.*on.*\|' "$RPI"; then
        pass "/rpi documents --test-first in flag table with default on"
    else
        fail "/rpi missing --test-first default-on entry in flag table"
    fi

    # 6b: --no-test-first explicit opt-out path in rpi flag table
    if grep -qE '^\|.*--no-test-first.*\|' "$RPI"; then
        pass "/rpi documents --no-test-first explicit opt-out in flag table"
    else
        fail "/rpi missing --no-test-first opt-out entry in flag table"
    fi

    # 6c: --test-first in rpi_state
    if grep -q 'test_first' "$RPI"; then
        pass "/rpi tracks test_first in state"
    else
        fail "/rpi missing test_first in state object"
    fi
fi

if [[ ! -f "$EVOLVE" ]]; then
    fail "File missing: $EVOLVE"
else
    # 6d: --test-first in evolve flag table
    if grep -qE '^\|.*--test-first.*\|' "$EVOLVE"; then
        pass "/evolve documents --test-first in flag table"
    else
        fail "/evolve missing --test-first in flag table"
    fi

    # 6e: --test-first in evolve_state
    if grep -q 'test_first' "$EVOLVE"; then
        pass "/evolve tracks test_first in state"
    else
        fail "/evolve missing test_first in state object"
    fi
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
echo -e "${BLUE}=============================================${NC}"

if [[ $failed -gt 0 ]]; then
    echo -e "${RED}FAILED${NC} - $passed/$total passed, $failed failed"
    exit 1
else
    echo -e "${GREEN}PASSED${NC} - All $total checks passed"
    exit 0
fi
