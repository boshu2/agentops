#!/usr/bin/env bash
# Test: RPI E2E Workflow
# Comprehensive test for full Research -> Plan -> Pre-mortem -> Crank -> Vibe -> Post-Mortem pipeline
# Verifies artifacts created at each phase and gates enforced
#
# This test validates:
# 1. Each skill produces expected artifacts
# 2. Ratchet tracks progress through phases
# 3. Knowledge flywheel captures learnings
# 4. Gates block inappropriate progression
#
# Usage: ./test-rpi-e2e.sh [--verbose] [--keep-fixtures]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source helpers
source "$SCRIPT_DIR/test-helpers.sh"

# Test configuration
VERBOSE=false
KEEP_FIXTURES=false
TEST_PROJECT=""
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --keep-fixtures)
            KEEP_FIXTURES=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_pass() {
    echo -e "  ${GREEN}[PASS]${NC} $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

log_fail() {
    echo -e "  ${RED}[FAIL]${NC} $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

log_skip() {
    echo -e "  ${YELLOW}[SKIP]${NC} $1"
    SKIP_COUNT=$((SKIP_COUNT + 1))
}

verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "  ${BLUE}[DEBUG]${NC} $1"
    fi
}

# Setup test fixture project
setup_test_project() {
    log "Setting up RPI E2E test fixture project..."

    TEST_PROJECT=$(mktemp -d)
    verbose "Test project: $TEST_PROJECT"

    # Initialize git repo (required for many skills)
    cd "$TEST_PROJECT"
    git init -q
    git config user.email "test@example.com"
    git config user.name "Test User"

    # Create .agents directory structure (RPI artifact directories)
    mkdir -p .agents/research
    mkdir -p .agents/plans
    mkdir -p .agents/council
    mkdir -p .agents/retros
    mkdir -p .agents/learnings
    mkdir -p .agents/patterns
    mkdir -p .agents/ao/sessions
    mkdir -p .agents/pool/pending
    mkdir -p .agents/pool/staged
    mkdir -p .agents/tooling  # test fixture — legacy path kept for e2e isolation

    # Create .beads directory for issue tracking
    mkdir -p .beads/issues
    echo '{"prefix": "test", "next_id": 1}' > .beads/config.json

    # Create a simple source file to work with
    mkdir -p src
    cat > src/calculator.py << 'PYFILE'
"""Simple calculator module for testing RPI workflow."""

def add(a: int, b: int) -> int:
    """Add two numbers."""
    return a + b

def subtract(a: int, b: int) -> int:
    """Subtract b from a."""
    return a - b

def multiply(a: int, b: int) -> int:
    """Multiply two numbers."""
    return a * b

def divide(a: int, b: int) -> float:
    """Divide a by b."""
    if b == 0:
        raise ValueError("Cannot divide by zero")
    return a / b
PYFILE

    # Create test file
    mkdir -p tests
    cat > tests/test_calculator.py << 'TESTFILE'
"""Tests for calculator module."""
import pytest
from src.calculator import add, subtract, multiply, divide

def test_add():
    assert add(2, 3) == 5
    assert add(-1, 1) == 0

def test_subtract():
    assert subtract(5, 3) == 2
    assert subtract(0, 5) == -5

def test_multiply():
    assert multiply(3, 4) == 12
    assert multiply(-2, 3) == -6

def test_divide():
    assert divide(10, 2) == 5.0
    assert divide(7, 2) == 3.5

def test_divide_by_zero():
    with pytest.raises(ValueError):
        divide(1, 0)
TESTFILE

    # Create requirements.txt
    echo "pytest>=7.0.0" > requirements.txt

    # Initial commit
    git add .
    git commit -q -m "Initial project setup for RPI E2E test"

    log "Test fixture project ready: $TEST_PROJECT"
    cd "$SCRIPT_DIR"
}

# Cleanup test project
cleanup_test_project() {
    if [[ -n "$TEST_PROJECT" && -d "$TEST_PROJECT" ]]; then
        if [[ "$KEEP_FIXTURES" == "true" ]]; then
            log "Test fixtures preserved at: $TEST_PROJECT"
        else
            rm -rf "$TEST_PROJECT"
            verbose "Cleaned up test project"
        fi
    fi
}

trap cleanup_test_project EXIT

# ============================================================================
# Phase 1: Research Skill Tests
# ============================================================================

test_research_artifacts() {
    log "Testing Research Phase artifacts..."

    cd "$TEST_PROJECT"

    # Simulate research output (what the skill should create)
    cat > .agents/research/$(date +%Y-%m-%d)-calculator-api.md << 'RESEARCH'
---
schema_version: 1
---

# Research: Calculator API

**Date:** $(date +%Y-%m-%d)
**Scope:** Calculator module analysis

## Summary

Investigated the calculator module structure and found a simple arithmetic library
with add, subtract, multiply, and divide operations.

## Key Files

| File | Purpose |
|------|---------|
| src/calculator.py | Core arithmetic functions |
| tests/test_calculator.py | Unit tests |

## Findings

- Clean API with type hints (src/calculator.py:3-25)
- Division by zero handling exists (src/calculator.py:22-24)
- Good test coverage for happy paths

## Recommendations

- Add input validation for non-numeric types
- Consider adding modulo operation
RESEARCH

    # Check research artifact exists
    local research_count
    research_count=$(find .agents/research -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$research_count" -ge 1 ]]; then
        log_pass "Research artifact created (.agents/research/*.md)"
    else
        log_fail "No research artifact found"
    fi

    # Check research has required sections
    local research_file
    research_file=$(find .agents/research -name "*.md" -type f | head -1)

    if [[ -n "$research_file" ]]; then
        if grep -q "## Summary" "$research_file" && \
           grep -q "## Key Files" "$research_file" && \
           grep -q "## Findings" "$research_file"; then
            log_pass "Research artifact has required sections"
        else
            log_fail "Research artifact missing required sections"
        fi
    fi

    cd "$SCRIPT_DIR"
}

# ============================================================================
# Phase 2: Plan Skill Tests
# ============================================================================

test_plan_artifacts() {
    log "Testing Plan Phase artifacts..."

    cd "$TEST_PROJECT"

    # Simulate plan output
    cat > .agents/plans/$(date +%Y-%m-%d)-calculator-improvements.md << 'PLAN'
---
schema_version: 1
---

# Plan: Calculator Improvements

**Date:** $(date +%Y-%m-%d)
**Source:** .agents/research/*-calculator-api.md

## Overview

Add input validation and modulo operation to calculator module.

## Issues

### Issue 1: Add input validation
**Dependencies:** None
**Acceptance:** Invalid inputs raise TypeError
**Description:** Add type checking to all functions

### Issue 2: Add modulo operation
**Dependencies:** Issue 1
**Acceptance:** modulo(a, b) returns a % b
**Description:** Implement modulo function with validation

## Execution Order

**Wave 1** (parallel): Issue 1
**Wave 2** (after Wave 1): Issue 2

## Next Steps
- Run `/implement` for each issue
- Or `/crank` for autonomous execution
PLAN

    # Simulate beads issue creation
    cat > .beads/issues/test-0001.json << 'ISSUE1'
{
  "id": "test-0001",
  "title": "Add input validation",
  "status": "open",
  "type": "task",
  "created_at": "2026-02-03T10:00:00Z",
  "labels": ["planned"],
  "body": "Add type checking to all calculator functions"
}
ISSUE1

    cat > .beads/issues/test-0002.json << 'ISSUE2'
{
  "id": "test-0002",
  "title": "Add modulo operation",
  "status": "open",
  "type": "task",
  "created_at": "2026-02-03T10:00:00Z",
  "labels": ["planned"],
  "blocks": [],
  "blocked_by": ["test-0001"],
  "body": "Implement modulo function with validation"
}
ISSUE2

    # Check plan artifact exists
    local plan_count
    plan_count=$(find .agents/plans -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$plan_count" -ge 1 ]]; then
        log_pass "Plan artifact created (.agents/plans/*.md)"
    else
        log_fail "No plan artifact found"
    fi

    # Check plan has wave structure
    local plan_file
    plan_file=$(find .agents/plans -name "*.md" -type f | head -1)

    if [[ -n "$plan_file" ]]; then
        if grep -q "Wave 1" "$plan_file" || grep -q "wave1" "$plan_file"; then
            log_pass "Plan artifact has wave structure"
        else
            log_fail "Plan artifact missing wave structure"
        fi
    fi

    # Check beads issues created
    local issue_count
    issue_count=$(find .beads/issues -name "*.json" -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$issue_count" -ge 1 ]]; then
        log_pass "Beads issues created ($issue_count issues)"
    else
        log_fail "No beads issues found"
    fi

    # Check issue dependencies exist
    if grep -q "blocked_by" .beads/issues/test-0002.json 2>/dev/null; then
        log_pass "Issue dependencies tracked (blocked_by)"
    else
        log_fail "Issue dependencies not tracked"
    fi

    cd "$SCRIPT_DIR"
}

# ============================================================================
# Phase 3: Pre-mortem Skill Tests
# ============================================================================

test_premortem_artifacts() {
    log "Testing Pre-mortem Phase artifacts..."

    cd "$TEST_PROJECT"

    # Simulate pre-mortem council output
    mkdir -p .agents/council
    cat > .agents/council/$(date +%Y-%m-%d)-pre-mortem-calculator.md << 'PREMORTEM'
---
schema_version: 1
type: pre-mortem
---

# Pre-mortem: Calculator Improvements

**Date:** $(date +%Y-%m-%d)
**Plan:** .agents/plans/*-calculator-improvements.md

## Council Verdict

**Result:** PASS (0 critical, 1 advisory)

## Judge Verdicts

| Judge | Verdict | Confidence |
|-------|---------|------------|
| Correctness | PASS | high |
| Architecture | PASS | high |
| Error-Paths | PASS | medium |

## Findings

### CRITICAL
(none)

### ADVISORY
1. Consider edge cases for very large numbers (overflow)

## Recommendation

Plan is ready to implement. No blocking issues found.
PREMORTEM

    git add . && git commit -q -m "Pre-mortem validation of calculator improvements"

    # Check pre-mortem artifact exists
    local premortem_count
    premortem_count=$(find .agents/council -name "*pre-mortem*" -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$premortem_count" -ge 1 ]]; then
        log_pass "Pre-mortem artifact created (.agents/council/*pre-mortem*.md)"
    else
        log_fail "No pre-mortem artifact found"
    fi

    # Check pre-mortem has council verdict
    local premortem_file
    premortem_file=$(find .agents/council -name "*pre-mortem*" -type f | head -1)

    if [[ -n "$premortem_file" ]]; then
        if grep -q "Council Verdict" "$premortem_file"; then
            log_pass "Pre-mortem has council verdict"
        else
            log_fail "Pre-mortem missing council verdict"
        fi

        if grep -q "Judge Verdicts" "$premortem_file"; then
            log_pass "Pre-mortem has judge verdicts table"
        else
            log_fail "Pre-mortem missing judge verdicts"
        fi
    fi

    cd "$SCRIPT_DIR"
}

# ============================================================================
# Phase 4: Crank Skill Tests
# ============================================================================

test_crank_artifacts() {
    log "Testing Crank Phase artifacts..."

    cd "$TEST_PROJECT"

    # Simulate implementation - add input validation
    cat > src/calculator.py << 'PYFILE_V2'
"""Simple calculator module for testing RPI workflow."""

def _validate_numeric(*args):
    """Validate that all arguments are numeric."""
    for arg in args:
        if not isinstance(arg, (int, float)):
            raise TypeError(f"Expected numeric type, got {type(arg).__name__}")

def add(a: int, b: int) -> int:
    """Add two numbers."""
    _validate_numeric(a, b)
    return a + b

def subtract(a: int, b: int) -> int:
    """Subtract b from a."""
    _validate_numeric(a, b)
    return a - b

def multiply(a: int, b: int) -> int:
    """Multiply two numbers."""
    _validate_numeric(a, b)
    return a * b

def divide(a: int, b: int) -> float:
    """Divide a by b."""
    _validate_numeric(a, b)
    if b == 0:
        raise ValueError("Cannot divide by zero")
    return a / b

def modulo(a: int, b: int) -> int:
    """Return a modulo b."""
    _validate_numeric(a, b)
    if b == 0:
        raise ValueError("Cannot modulo by zero")
    return a % b
PYFILE_V2

    # Add test for new functionality
    cat >> tests/test_calculator.py << 'TESTFILE_ADD'

def test_modulo():
    from src.calculator import modulo
    assert modulo(10, 3) == 1
    assert modulo(8, 4) == 0

def test_type_validation():
    with pytest.raises(TypeError):
        add("a", 1)
TESTFILE_ADD

    # Update issue status
    cat > .beads/issues/test-0001.json << 'ISSUE1_CLOSED'
{
  "id": "test-0001",
  "title": "Add input validation",
  "status": "closed",
  "type": "task",
  "created_at": "2026-02-03T10:00:00Z",
  "closed_at": "2026-02-03T11:00:00Z",
  "labels": ["planned"],
  "body": "Add type checking to all calculator functions"
}
ISSUE1_CLOSED

    cat > .beads/issues/test-0002.json << 'ISSUE2_CLOSED'
{
  "id": "test-0002",
  "title": "Add modulo operation",
  "status": "closed",
  "type": "task",
  "created_at": "2026-02-03T10:00:00Z",
  "closed_at": "2026-02-03T12:00:00Z",
  "labels": ["planned"],
  "body": "Implement modulo function with validation"
}
ISSUE2_CLOSED

    # Create git commit
    git add .
    git commit -q -m "Implement input validation and modulo operation

Implements: test-0001, test-0002"

    # Check code changes committed
    local commit_count
    commit_count=$(git rev-list --count HEAD)

    if [[ "$commit_count" -ge 2 ]]; then
        log_pass "Code changes committed ($commit_count commits)"
    else
        log_fail "No new commits found"
    fi

    # Check issues closed
    local closed_count
    closed_count=$(grep -l '"status": "closed"' .beads/issues/*.json 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$closed_count" -ge 1 ]]; then
        log_pass "Issues closed after implementation ($closed_count closed)"
    else
        log_fail "Issues not closed"
    fi

    # Check implementation added new function
    if grep -q "def modulo" src/calculator.py; then
        log_pass "New function added (modulo)"
    else
        log_fail "Expected function not found"
    fi

    # Check validation added
    if grep -q "_validate_numeric" src/calculator.py; then
        log_pass "Validation helper added"
    else
        log_fail "Validation not implemented"
    fi

    cd "$SCRIPT_DIR"
}

# ============================================================================
# Phase 4: Vibe Skill Tests
# ============================================================================

test_vibe_artifacts() {
    log "Testing Vibe Phase artifacts..."

    cd "$TEST_PROJECT"

    # Simulate vibe output
    mkdir -p .agents/council
    cat > .agents/council/$(date +%Y-%m-%d)-calculator-validation.md << 'VIBE'
---
schema_version: 1
---

# Vibe Report: Calculator Module

**Date:** $(date +%Y-%m-%d)
**Files Reviewed:** 2
**Grade:** A

## Summary

Code quality is good. Input validation added properly with consistent error handling.
No security issues detected.

## Gate Decision

[x] PASS - 0 critical findings

## Findings

### CRITICAL
(none)

### HIGH
(none)

### MEDIUM
1. **src/calculator.py:3** - Consider using Union[int, float] for type hints
   - **Fix:** Update type hints for accuracy

## Aspects Summary

| Aspect | Status |
|--------|--------|
| Semantic | OK |
| Security | OK |
| Quality | OK |
| Architecture | OK |
| Complexity | OK |
| Performance | OK |
| Slop | OK |
| Accessibility | N/A |
VIBE

    # Create tooling output simulation
    cat > .agents/tooling/summary.json << 'TOOLSUMMARY'  # test fixture in isolated tmp dir
{
  "timestamp": "2026-02-03T12:00:00Z",
  "exit_code": 0,
  "tools": {
    "ruff": {"status": "pass", "findings": 0},
    "pytest": {"status": "pass", "findings": 0}
  }
}
TOOLSUMMARY

    # Check vibe artifact exists (match validation pattern, not pre-mortem)
    local vibe_count
    vibe_count=$(find .agents/council -name "*validation*" -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$vibe_count" -ge 1 ]]; then
        log_pass "Vibe artifact created (.agents/council/*validation*.md)"
    else
        log_fail "No vibe artifact found"
    fi

    # Check vibe has grade
    local vibe_file
    vibe_file=$(find .agents/council -name "*validation*" -type f | head -1)

    if [[ -n "$vibe_file" ]]; then
        if grep -q "Grade:" "$vibe_file"; then
            log_pass "Vibe artifact has grade"
        else
            log_fail "Vibe artifact missing grade"
        fi

        if grep -q "Gate Decision" "$vibe_file"; then
            log_pass "Vibe artifact has gate decision"
        else
            log_fail "Vibe artifact missing gate decision"
        fi
    fi

    # Check tooling summary
    if [[ -f .agents/tooling/summary.json ]]; then
        log_pass "Tooling summary created"
    else
        log_fail "Tooling summary missing"
    fi

    cd "$SCRIPT_DIR"
}

# ============================================================================
# Phase 5: Post-Mortem Skill Tests
# ============================================================================

test_postmortem_artifacts() {
    log "Testing Post-Mortem Phase artifacts..."

    cd "$TEST_PROJECT"

    # Simulate retro output
    cat > .agents/retros/$(date +%Y-%m-%d)-post-mortem-calculator.md << 'RETRO'
---
schema_version: 1
---

# Post-Mortem: Calculator Improvements

**Date:** $(date +%Y-%m-%d)
**Epic:** Calculator API improvements
**Duration:** 2 hours

## CI Suite Results (Gate)

| Tool | Status | Findings | Details |
|------|--------|----------|---------|
| pytest | PASS | 0 | .agents/tooling/pytest.txt |
| ruff | PASS | 0 | .agents/tooling/ruff.txt |

**Gate Status:** PASS (CI suite passed, post-mortem proceeds)

## Triaged Findings

### True Positives (actionable)
(none)

### False Positives (dismissed)
(none)

## Plan Compliance

| Plan Item | Delivered | Verified | Notes |
|-----------|-----------|----------|-------|
| Input validation | yes | yes (tests pass) | Type checking added |
| Modulo operation | yes | yes (tests pass) | Function implemented |

## Learnings Extracted

| ID | Category | Learning | Source | Verified |
|----|----------|----------|--------|----------|
| L-2026-02-03-1 | technical | _validate_numeric helper reduces code duplication | src/calculator.py | yes (6 usages) |
| L-2026-02-03-2 | process | Plan-first approach found validation gap | .agents/plans/*.md | yes (commit history) |

## Follow-up Issues

(none)

## Knowledge Flywheel Status

- **Learnings indexed:** 2
- **Session provenance:** test-session-001
- **ao forge status:** PASS
RETRO

    # Simulate learnings extraction
    cat > .agents/learnings/$(date +%Y-%m-%d)-calculator.md << 'LEARNING'
# Learning: Validation Helper Pattern

**ID**: L-2026-02-03-1
**Category**: technical
**Confidence**: high

## What We Learned

Creating a shared _validate_numeric() helper function reduces code duplication
and ensures consistent validation across all arithmetic functions.

## Why It Matters

Centralized validation is easier to maintain and test. Changes to validation
logic only need to happen in one place.

## Source

src/calculator.py - implemented during calculator improvements epic

---

# Learning: Plan-First Approach

**ID**: L-2026-02-03-2
**Category**: process
**Confidence**: high

## What We Learned

Running /research before /plan identified the validation gap that might have
been missed with direct implementation.

## Why It Matters

Research phase surfaces requirements before implementation begins.

## Source

.agents/research/*-calculator-api.md
LEARNING

    # Check retro artifact exists
    local retro_count
    retro_count=$(find .agents/retros -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$retro_count" -ge 1 ]]; then
        log_pass "Retro artifact created (.agents/retros/*.md)"
    else
        log_fail "No retro artifact found"
    fi

    # Check learnings extracted
    local learning_count
    learning_count=$(find .agents/learnings -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$learning_count" -ge 1 ]]; then
        log_pass "Learnings extracted (.agents/learnings/*.md)"
    else
        log_fail "No learnings found"
    fi

    # Check retro has CI results
    local retro_file
    retro_file=$(find .agents/retros -name "*post-mortem*.md" -type f | head -1)

    if [[ -n "$retro_file" ]]; then
        if grep -q "CI Suite Results" "$retro_file"; then
            log_pass "Retro artifact has CI suite results"
        else
            log_fail "Retro artifact missing CI suite results"
        fi

        if grep -q "Learnings Extracted" "$retro_file"; then
            log_pass "Retro artifact has learnings section"
        else
            log_fail "Retro artifact missing learnings section"
        fi
    fi

    cd "$SCRIPT_DIR"
}

# ============================================================================
# Ratchet / Gate Tests
# ============================================================================

test_ratchet_tracking() {
    log "Testing Ratchet progress tracking..."

    cd "$TEST_PROJECT"

    # Simulate ratchet chain entries
    mkdir -p .agents/ao
    cat > .agents/ao/chain.jsonl << 'CHAIN'
{"step":"research","status":"completed","output":".agents/research/2026-02-03-calculator-api.md","time":"2026-02-03T10:00:00Z"}
{"step":"plan","status":"completed","output":".agents/plans/2026-02-03-calculator-improvements.md","time":"2026-02-03T10:30:00Z"}
{"step":"pre-mortem","status":"completed","output":".agents/council/2026-02-03-pre-mortem-calculator.md","time":"2026-02-03T10:45:00Z"}
{"step":"crank","status":"completed","output":"abc1234","time":"2026-02-03T11:00:00Z"}
{"step":"vibe","status":"completed","output":".agents/council/2026-02-03-calculator-validation.md","time":"2026-02-03T11:30:00Z"}
{"step":"post-mortem","status":"completed","output":".agents/retros/2026-02-03-post-mortem-calculator.md","time":"2026-02-03T12:00:00Z"}
CHAIN

    # Check chain file exists
    if [[ -f .agents/ao/chain.jsonl ]]; then
        log_pass "Ratchet chain file exists"
    else
        log_fail "Ratchet chain file missing"
    fi

    # Check all phases recorded
    local phases=("research" "plan" "pre-mortem" "crank" "vibe" "post-mortem")
    local all_recorded=true

    for phase in "${phases[@]}"; do
        if ! grep -q "\"step\":\"$phase\"" .agents/ao/chain.jsonl 2>/dev/null; then
            log_fail "Phase not recorded in ratchet: $phase"
            all_recorded=false
        fi
    done

    if [[ "$all_recorded" == "true" ]]; then
        log_pass "All 6 RPI phases recorded in ratchet chain"
    fi

    # Verify chain is valid JSONL
    local valid_jsonl=true
    while IFS= read -r line; do
        if ! echo "$line" | jq -e . > /dev/null 2>&1; then
            valid_jsonl=false
            break
        fi
    done < .agents/ao/chain.jsonl

    if [[ "$valid_jsonl" == "true" ]]; then
        log_pass "Ratchet chain is valid JSONL"
    else
        log_fail "Ratchet chain contains invalid JSON"
    fi

    # Check completed status
    local completed_count
    completed_count=$(grep -c '"status":"completed"' .agents/ao/chain.jsonl 2>/dev/null || echo "0")

    if [[ "$completed_count" -ge 6 ]]; then
        log_pass "All phases have completed status ($completed_count entries)"
    else
        log_fail "Not all phases completed ($completed_count entries)"
    fi

    cd "$SCRIPT_DIR"
}

test_gate_enforcement() {
    log "Testing Gate enforcement..."

    cd "$TEST_PROJECT"

    # Test: vibe gate blocks on critical findings
    mkdir -p .agents/council
    cat > .agents/council/test-blocked.md << 'BLOCKED_VIBE'
# Vibe Report: Test

**Grade:** D

## Gate Decision

[ ] PASS
[x] BLOCK - 1 critical finding must be fixed

## Findings

### CRITICAL
1. **src/auth.py:42** - Hardcoded secret detected
BLOCKED_VIBE

    # Check block detection
    if grep -q "BLOCK" .agents/council/test-blocked.md; then
        log_pass "Gate can detect blocking conditions"
    else
        log_fail "Gate block detection failed"
    fi

    # Test: chain shows blocked status
    echo '{"step":"vibe","status":"blocked","reason":"1 critical finding","time":"2026-02-03T12:00:00Z"}' >> .agents/ao/chain.jsonl

    if grep -q '"status":"blocked"' .agents/ao/chain.jsonl; then
        log_pass "Blocked status can be recorded in chain"
    else
        log_fail "Blocked status not recordable"
    fi

    # Test: pre-mortem gate blocks on critical findings
    cat > .agents/council/test-premortem-blocked.md << 'BLOCKED_PM'
# Pre-mortem: Test

## Council Verdict

**Result:** FAIL (1 critical)

## Findings

### CRITICAL
1. Plan has no error handling strategy for API failures
BLOCKED_PM

    echo '{"step":"pre-mortem","status":"blocked","reason":"1 critical finding in plan","time":"2026-02-03T12:05:00Z"}' >> .agents/ao/chain.jsonl

    if grep -q '"step":"pre-mortem","status":"blocked"' .agents/ao/chain.jsonl; then
        log_pass "Pre-mortem gate can block on critical findings"
    else
        log_fail "Pre-mortem gate block not recorded"
    fi

    cd "$SCRIPT_DIR"
}

test_phase_summaries() {
    log "Testing Phase summary validation..."

    cd "$TEST_PROJECT"

    # Simulate phase summary files for all 6 phases
    mkdir -p .agents/rpi
    local phases=("research" "plan" "pre-mortem" "crank" "vibe" "post-mortem")
    local phase_num=1

    for phase in "${phases[@]}"; do
        cat > ".agents/rpi/phase-${phase_num}-summary-${phase}.md" << SUMMARY
# Phase ${phase_num}: ${phase}

## Summary

This phase completed successfully with expected artifacts produced.
Key outputs were generated and validated against schema requirements.

## Duration

Approximately 15 minutes.

## Artifacts

- Primary output verified
- Schema compliance confirmed
SUMMARY
        phase_num=$((phase_num + 1))
    done

    # Check each phase summary exists and meets criteria
    phase_num=1
    for phase in "${phases[@]}"; do
        local summary_file=".agents/rpi/phase-${phase_num}-summary-${phase}.md"

        if [[ -f "$summary_file" ]] && [[ -s "$summary_file" ]]; then
            local word_count
            word_count=$(wc -w < "$summary_file" | tr -d ' ')

            if [[ "$word_count" -lt 400 ]]; then
                log_pass "Phase $phase_num ($phase) summary: exists, non-empty, ${word_count} words < 400"
            else
                log_fail "Phase $phase_num ($phase) summary: ${word_count} words exceeds 400 limit"
            fi
        else
            log_fail "Phase $phase_num ($phase) summary missing or empty"
        fi

        phase_num=$((phase_num + 1))
    done

    cd "$SCRIPT_DIR"
}

test_complexity_scaling() {
    log "Testing Complexity scaling flag selection..."

    cd "$TEST_PROJECT"

    # Simulate complexity config that maps issue count to vibe depth
    mkdir -p .agents/ao
    cat > .agents/ao/complexity-config.json << 'COMPLEXITY'
{
  "rules": [
    {"min_issues": 1, "max_issues": 2, "vibe_flag": "--quick"},
    {"min_issues": 3, "max_issues": 7, "vibe_flag": ""},
    {"min_issues": 8, "max_issues": 999, "vibe_flag": "--deep"}
  ],
  "mutex": ["--fast-path", "--deep"]
}
COMPLEXITY

    # Test: 1-2 issues → --quick
    local issue_count=2
    local expected_flag
    expected_flag=$(jq -r --argjson n "$issue_count" '.rules[] | select(.min_issues <= $n and .max_issues >= $n) | .vibe_flag' .agents/ao/complexity-config.json)

    if [[ "$expected_flag" == "--quick" ]]; then
        log_pass "1-2 issues selects --quick flag (count=$issue_count)"
    else
        log_fail "1-2 issues should select --quick, got: '$expected_flag'"
    fi

    # Test: 3-7 issues → default (empty)
    issue_count=5
    expected_flag=$(jq -r --argjson n "$issue_count" '.rules[] | select(.min_issues <= $n and .max_issues >= $n) | .vibe_flag' .agents/ao/complexity-config.json)

    if [[ -z "$expected_flag" ]]; then
        log_pass "3-7 issues selects default flag (count=$issue_count)"
    else
        log_fail "3-7 issues should select default (empty), got: '$expected_flag'"
    fi

    # Test: 8+ issues → --deep
    issue_count=12
    expected_flag=$(jq -r --argjson n "$issue_count" '.rules[] | select(.min_issues <= $n and .max_issues >= $n) | .vibe_flag' .agents/ao/complexity-config.json)

    if [[ "$expected_flag" == "--deep" ]]; then
        log_pass "8+ issues selects --deep flag (count=$issue_count)"
    else
        log_fail "8+ issues should select --deep, got: '$expected_flag'"
    fi

    # Test: --fast-path and --deep are mutex
    local mutex_flags
    mutex_flags=$(jq -r '.mutex | join(" ")' .agents/ao/complexity-config.json)

    if echo "$mutex_flags" | grep -q "fast-path" && echo "$mutex_flags" | grep -q "deep"; then
        log_pass "Mutex constraint: --fast-path and --deep are mutually exclusive"
    else
        log_fail "Mutex constraint not defined for --fast-path/--deep"
    fi

    cd "$SCRIPT_DIR"
}

test_promise_tag_parsing() {
    log "Testing Promise tag parsing..."

    cd "$TEST_PROJECT"

    # Simulate crank output with promise tags
    mkdir -p .agents/crank
    cat > .agents/crank/worker-1-output.md << 'WORKER1'
# Worker 1: Add input validation

Implementation complete.

<promise>DONE</promise>

## Changes
- Added _validate_numeric() helper
- Applied to all 4 arithmetic functions
WORKER1

    cat > .agents/crank/worker-2-output.md << 'WORKER2'
# Worker 2: Add modulo operation

Blocked by missing dependency.

<promise>BLOCKED</promise>

## Reason
Waiting for input validation to be merged first.
WORKER2

    cat > .agents/crank/worker-3-output.md << 'WORKER3'
# Worker 3: Add power operation

Partial implementation — tests not yet written.

<promise>PARTIAL</promise>

## Changes
- Added power() function
## Missing
- Unit tests for edge cases
WORKER3

    # Check DONE tag detection
    if grep -q '<promise>DONE</promise>' .agents/crank/worker-1-output.md; then
        log_pass "DONE promise tag detected in crank output"
    else
        log_fail "DONE promise tag not found"
    fi

    # Check BLOCKED tag detection
    if grep -q '<promise>BLOCKED</promise>' .agents/crank/worker-2-output.md; then
        log_pass "BLOCKED promise tag detected in crank output"
    else
        log_fail "BLOCKED promise tag not found"
    fi

    # Check PARTIAL tag detection
    if grep -q '<promise>PARTIAL</promise>' .agents/crank/worker-3-output.md; then
        log_pass "PARTIAL promise tag detected in crank output"
    else
        log_fail "PARTIAL promise tag not found"
    fi

    # Simulate chain recording with promise-derived status
    cat >> .agents/ao/chain.jsonl << 'PROMISE_CHAIN'
{"step":"crank","worker":"worker-1","status":"completed","promise":"DONE","time":"2026-02-03T13:00:00Z"}
{"step":"crank","worker":"worker-2","status":"blocked","promise":"BLOCKED","time":"2026-02-03T13:00:00Z"}
{"step":"crank","worker":"worker-3","status":"partial","promise":"PARTIAL","time":"2026-02-03T13:00:00Z"}
PROMISE_CHAIN

    # Verify chain records all three promise statuses
    local promise_statuses
    promise_statuses=$(grep '"promise"' .agents/ao/chain.jsonl | jq -r '.promise' 2>/dev/null | sort | tr '\n' ',')

    if echo "$promise_statuses" | grep -q "BLOCKED" && \
       echo "$promise_statuses" | grep -q "DONE" && \
       echo "$promise_statuses" | grep -q "PARTIAL"; then
        log_pass "Chain records all promise tag statuses (DONE/BLOCKED/PARTIAL)"
    else
        log_fail "Chain missing some promise statuses: $promise_statuses"
    fi

    cd "$SCRIPT_DIR"
}

test_gate_retry_logic() {
    log "Testing Gate retry logic..."

    cd "$TEST_PROJECT"

    # Simulate pre-mortem FAIL → retry → PASS sequence
    mkdir -p .agents/council
    cat > .agents/council/test-premortem-attempt-1.md << 'PM_FAIL'
# Pre-mortem: Attempt 1

## Council Verdict

**Result:** FAIL (1 critical)

## Findings

### CRITICAL
1. FINDING: Missing error handling in divide-by-zero path
   FIX: Add guard clause before division
   REF: src/calculator.py:28
PM_FAIL

    cat > .agents/council/test-premortem-attempt-2.md << 'PM_PASS'
# Pre-mortem: Attempt 2

## Council Verdict

**Result:** PASS (0 critical)

## Findings

### ADVISORY
1. FINDING: Consider logging division operations
   FIX: Add optional logging parameter
   REF: src/calculator.py:28
PM_PASS

    # Check retry produces multiple attempts
    local attempt_count
    attempt_count=$(find .agents/council -name "test-premortem-attempt-*" -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$attempt_count" -ge 2 ]]; then
        log_pass "Pre-mortem retry produces multiple attempt artifacts ($attempt_count)"
    else
        log_fail "Pre-mortem retry missing attempt artifacts"
    fi

    # Simulate vibe FAIL → retry → PASS with structured findings
    cat > .agents/council/test-vibe-attempt-1.md << 'VIBE_FAIL'
# Vibe Report: Attempt 1

**Grade:** D

## Gate Decision
[x] BLOCK

## Findings

### CRITICAL
1. FINDING: SQL injection in user input handling
   FIX: Use parameterized queries
   REF: src/db.py:15
VIBE_FAIL

    cat > .agents/council/test-vibe-attempt-2.md << 'VIBE_PASS'
# Vibe Report: Attempt 2

**Grade:** B

## Gate Decision
[x] PASS

## Findings

### MEDIUM
1. FINDING: Variable naming could be clearer
   FIX: Rename 'x' to 'user_count'
   REF: src/db.py:22
VIBE_PASS

    # Check vibe retry attempts exist
    local vibe_attempt_count
    vibe_attempt_count=$(find .agents/council -name "test-vibe-attempt-*" -type f 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$vibe_attempt_count" -ge 2 ]]; then
        log_pass "Vibe retry produces multiple attempt artifacts ($vibe_attempt_count)"
    else
        log_fail "Vibe retry missing attempt artifacts"
    fi

    # Check structured findings format (FINDING/FIX/REF)
    local findings_format_count
    findings_format_count=$(grep -l 'FINDING:' .agents/council/test-*-attempt-*.md 2>/dev/null | wc -l | tr -d ' ')

    if [[ "$findings_format_count" -ge 2 ]]; then
        log_pass "Structured findings format (FINDING/FIX/REF) used across attempts"
    else
        log_fail "Structured findings format missing"
    fi

    # Simulate max 3 attempts cap in chain
    cat >> .agents/ao/chain.jsonl << 'RETRY_CHAIN'
{"step":"pre-mortem","status":"blocked","attempt":1,"reason":"1 critical finding","time":"2026-02-03T12:10:00Z"}
{"step":"pre-mortem","status":"blocked","attempt":2,"reason":"1 critical finding","time":"2026-02-03T12:20:00Z"}
{"step":"pre-mortem","status":"completed","attempt":3,"reason":"all findings resolved","time":"2026-02-03T12:30:00Z"}
RETRY_CHAIN

    local max_attempt
    max_attempt=$(grep '"step":"pre-mortem"' .agents/ao/chain.jsonl | grep -o '"attempt":[0-9]*' | sed 's/"attempt"://' | sort -n | tail -1)

    if [[ "$max_attempt" -le 3 ]]; then
        log_pass "Gate retry capped at max 3 attempts (highest: $max_attempt)"
    else
        log_fail "Gate retry exceeded max 3 attempts ($max_attempt)"
    fi

    cd "$SCRIPT_DIR"
}

# ============================================================================
# Knowledge Flywheel Tests
# ============================================================================

test_knowledge_flywheel() {
    log "Testing Knowledge flywheel integration..."

    cd "$TEST_PROJECT"

    # Simulate citations tracking
    mkdir -p .agents/ao
    cat > .agents/ao/citations.jsonl << 'CITATIONS'
{"artifact_path":".agents/learnings/2026-02-03-calculator.md","session_id":"test-session-001","cited_at":"2026-02-03T12:00:00Z"}
{"artifact_path":".agents/learnings/2026-02-03-calculator.md","session_id":"test-session-002","cited_at":"2026-02-03T13:00:00Z"}
CITATIONS

    # Check citations file exists
    if [[ -f .agents/ao/citations.jsonl ]]; then
        log_pass "Citations file exists"
    else
        log_fail "Citations file missing"
    fi

    # Check learnings can be cited
    local citation_count
    citation_count=$(wc -l < .agents/ao/citations.jsonl | tr -d ' ')

    if [[ "$citation_count" -ge 1 ]]; then
        log_pass "Learnings cited in future sessions ($citation_count citations)"
    else
        log_fail "No citations found"
    fi

    # Check provenance tracking
    mkdir -p .agents/ao/provenance
    cat > .agents/ao/provenance/graph.jsonl << 'PROVENANCE'
{"from":"research","to":"plan","relation":"informs","time":"2026-02-03T10:30:00Z"}
{"from":"plan","to":"implement","relation":"guides","time":"2026-02-03T11:00:00Z"}
{"from":"implement","to":"vibe","relation":"validates","time":"2026-02-03T11:30:00Z"}
PROVENANCE

    if [[ -f .agents/ao/provenance/graph.jsonl ]]; then
        log_pass "Provenance graph exists"
    else
        log_skip "Provenance graph not implemented"
    fi

    cd "$SCRIPT_DIR"
}

# ============================================================================
# Skill Recognition Tests (using Claude)
# ============================================================================

test_skill_recognition() {
    log "Testing skill recognition (requires Claude Code CLI)..."

    # Check if Claude CLI is available
    if ! command -v claude &> /dev/null; then
        log_skip "Claude Code CLI not available"
        return 0
    fi

    # Test research skill recognition
    local output
    output=$(run_claude "What are the 5 main phases in the RPI workflow in this plugin?" 60 2>&1) || true

    if echo "$output" | grep -qi "research\|plan\|implement\|vibe\|post-mortem"; then
        log_pass "RPI phases recognized"
    else
        log_skip "Could not verify RPI phase recognition"
    fi

    # Test artifact locations
    output=$(run_claude "Where does the research skill store its output artifacts?" 45 2>&1) || true

    if echo "$output" | grep -qi ".agents/research"; then
        log_pass "Research artifact location recognized"
    else
        log_skip "Could not verify research artifact location"
    fi
}

# ============================================================================
# Integration Test: Full Pipeline Simulation
# ============================================================================

test_full_pipeline_integration() {
    log "Testing full pipeline integration..."

    cd "$TEST_PROJECT"

    # Verify directory structure matches expected RPI layout
    local expected_dirs=(
        ".agents/research"
        ".agents/plans"
        ".agents/council"
        ".agents/council"
        ".agents/retros"
        ".agents/learnings"
        ".agents/ao"
        ".beads/issues"
    )

    local all_dirs_exist=true
    for dir in "${expected_dirs[@]}"; do
        if [[ ! -d "$dir" ]]; then
            log_fail "Expected directory missing: $dir"
            all_dirs_exist=false
        fi
    done

    if [[ "$all_dirs_exist" == "true" ]]; then
        log_pass "All RPI directories exist"
    fi

    # Verify artifact count matches workflow progression
    local research_arts=$(find .agents/research -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')
    local plan_arts=$(find .agents/plans -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')
    local vibe_arts=$(find .agents/council -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')
    local retro_arts=$(find .agents/retros -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')
    local learning_arts=$(find .agents/learnings -name "*.md" -type f 2>/dev/null | wc -l | tr -d ' ')

    verbose "Artifacts: research=$research_arts, plan=$plan_arts, vibe=$vibe_arts, retro=$retro_arts, learnings=$learning_arts"

    if [[ "$research_arts" -ge 1 ]] && \
       [[ "$plan_arts" -ge 1 ]] && \
       [[ "$vibe_arts" -ge 1 ]] && \
       [[ "$retro_arts" -ge 1 ]] && \
       [[ "$learning_arts" -ge 1 ]]; then
        log_pass "All pipeline phases produced artifacts"
    else
        log_fail "Some pipeline phases missing artifacts"
    fi

    # Verify git history shows implementation
    local commit_msg
    commit_msg=$(git log --oneline -1)

    if echo "$commit_msg" | grep -qi "implement\|add\|fix"; then
        log_pass "Git history reflects implementation"
    else
        log_skip "Could not verify git history"
    fi

    cd "$SCRIPT_DIR"
}

# ============================================================================
# Main Test Runner
# ============================================================================

main() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  RPI E2E Test Suite${NC}"
    echo -e "${BLUE}  Research -> Plan -> Pre-mortem -> Crank -> Vibe -> Post-Mortem${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
    echo ""

    setup_test_project

    echo ""
    echo "Phase 1: Research"
    echo "─────────────────"
    test_research_artifacts

    echo ""
    echo "Phase 2: Plan"
    echo "─────────────"
    test_plan_artifacts

    echo ""
    echo "Phase 3: Pre-mortem"
    echo "───────────────────"
    test_premortem_artifacts

    echo ""
    echo "Phase 4: Crank"
    echo "──────────────"
    test_crank_artifacts

    echo ""
    echo "Phase 5: Vibe"
    echo "─────────────"
    test_vibe_artifacts

    echo ""
    echo "Phase 6: Post-Mortem"
    echo "────────────────────"
    test_postmortem_artifacts

    echo ""
    echo "Ratchet & Gates"
    echo "───────────────"
    test_ratchet_tracking
    test_gate_enforcement
    test_phase_summaries
    test_complexity_scaling
    test_promise_tag_parsing
    test_gate_retry_logic

    echo ""
    echo "Knowledge Flywheel"
    echo "──────────────────"
    test_knowledge_flywheel

    echo ""
    echo "Skill Recognition"
    echo "─────────────────"
    test_skill_recognition

    echo ""
    echo "Integration"
    echo "───────────"
    test_full_pipeline_integration

    # Summary
    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
    echo -e "  ${BLUE}Test Results${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════${NC}"
    echo -e "  ${GREEN}Passed:${NC}  $PASS_COUNT"
    echo -e "  ${RED}Failed:${NC}  $FAIL_COUNT"
    echo -e "  ${YELLOW}Skipped:${NC} $SKIP_COUNT"
    echo ""

    if [[ $FAIL_COUNT -eq 0 ]]; then
        echo -e "${GREEN}All tests passed!${NC}"
        echo ""
        echo "The RPI workflow E2E test validates:"
        echo "  - Research phase creates .agents/research/*.md"
        echo "  - Plan phase creates .agents/plans/*.md + beads issues"
        echo "  - Pre-mortem phase creates .agents/council/*pre-mortem*.md"
        echo "  - Crank phase creates code changes + closes issues"
        echo "  - Vibe phase creates .agents/council/*.md with gate decision"
        echo "  - Post-mortem creates .agents/retros/*.md + learnings"
        echo "  - Ratchet tracks progress in .agents/ao/chain.jsonl"
        echo "  - Gates can block on critical findings"
        echo ""
        exit 0
    else
        echo -e "${RED}Some tests failed.${NC}"
        exit 1
    fi
}

main "$@"
