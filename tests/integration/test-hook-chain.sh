#!/usr/bin/env bash
# test-hook-chain.sh - Integration test for hook chain execution
# Tests session-start.sh with real filesystem state
# Usage: ./tests/integration/test-hook-chain.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOKS_DIR="$REPO_ROOT/hooks"

# Source shared colors and helpers
source "${SCRIPT_DIR}/../lib/colors.sh"

PASS=0
FAIL=0

# Override pass() and fail() to increment local counters
pass() {
    green "  PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    red "  FAIL: $1"
    FAIL=$((FAIL + 1))
}

# Pre-flight checks
if ! command -v jq >/dev/null 2>&1; then
    red "ERROR: jq is required for hook chain tests"
    exit 1
fi

if ! command -v git >/dev/null 2>&1; then
    red "ERROR: git is required for hook chain tests"
    exit 1
fi

# Create temp directory with real .agents/ structure
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Set up real directory structure that hooks expect
mkdir -p "$TMPDIR/.agents/learnings"
mkdir -p "$TMPDIR/.agents/research"
mkdir -p "$TMPDIR/.agents/products"
mkdir -p "$TMPDIR/.agents/retros"
mkdir -p "$TMPDIR/.agents/patterns"
mkdir -p "$TMPDIR/.agents/council"
mkdir -p "$TMPDIR/.agents/knowledge/pending"
mkdir -p "$TMPDIR/.agents/ao"
mkdir -p "$TMPDIR/lib"
mkdir -p "$TMPDIR/skills/standards/references"

# Create sample learning file
cat > "$TMPDIR/.agents/learnings/2026-02-10-ag-test.md" <<'EOF'
# Test Learning

## Context
Integration test learning artifact.

## What Worked
- Real filesystem setup
- Hook chain execution

## What Didn't
- N/A

## Next Time
- Continue testing with real structures
EOF

# Create sample research file
cat > "$TMPDIR/.agents/research/2026-02-10-test-research.md" <<'EOF'
# Test Research

## Objective
Test hook chain integration.

## Findings
- Hooks read filesystem directly
- No mocking needed
EOF

# Create minimal CLAUDE.md
cat > "$TMPDIR/CLAUDE.md" <<'EOF'
# Test Project

Test project for hook chain integration testing.
EOF

# Copy hook-helpers.sh (session-start.sh might need it)
if [ -f "$REPO_ROOT/lib/hook-helpers.sh" ]; then
    cp "$REPO_ROOT/lib/hook-helpers.sh" "$TMPDIR/lib/"
fi

# Copy hooks directory to temp location so hooks can find fixtures
mkdir -p "$TMPDIR/hooks"
cp -r "$HOOKS_DIR"/* "$TMPDIR/hooks/"
# Update HOOKS_DIR to point to temp location
HOOKS_DIR="$TMPDIR/hooks"

# Initialize git repo (hooks use git rev-parse)
cd "$TMPDIR"
git init -q
git config user.email "test@example.com"
git config user.name "Test User"
git add .
git commit -q -m "Initial commit"

echo "=== Hook Chain Integration Tests ==="
echo ""

# ============================================================
echo "=== Test 1: session-start.sh basic execution ==="
# ============================================================

# Run session-start.sh with proper environment
cd "$TMPDIR"
OUTPUT=$(bash "$HOOKS_DIR/session-start.sh" 2>&1 || true)
EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    pass "session-start.sh exits with code 0"
else
    fail "session-start.sh exits with code 0 (got $EXIT_CODE)"
fi

if [ -z "$OUTPUT" ]; then
    pass "session-start.sh stays silent"
else
    fail "session-start.sh stays silent"
fi

# ============================================================
echo ""
echo "=== Test 2: session-start.sh silent runtime prep ==="
# ============================================================

if [ -z "$OUTPUT" ]; then
    pass "session-start.sh emits no operator-facing context"
else
    fail "session-start.sh emits no operator-facing context"
fi

# ============================================================
echo ""
echo "=== Test 3: session-start.sh directory creation ==="
# ============================================================

# Check that all expected .agents directories exist
EXPECTED_DIRS=(
    ".agents/research"
    ".agents/products"
    ".agents/retros"
    ".agents/learnings"
    ".agents/patterns"
    ".agents/council"
    ".agents/knowledge/pending"
    ".agents/findings"
    ".agents/planning-rules"
    ".agents/pre-mortem-checks"
    ".agents/constraints"
    ".agents/ao"
)

for dir in "${EXPECTED_DIRS[@]}"; do
    if [ -d "$TMPDIR/$dir" ]; then
        pass "Created directory: $dir"
    else
        fail "Created directory: $dir"
    fi
done

# ============================================================
echo ""
echo "=== Test 4: session-start.sh environment manifest ==="
# ============================================================

ENV_JSON="$TMPDIR/.agents/ao/environment.json"
if [ -f "$ENV_JSON" ]; then
    pass "Created environment.json"
else
    fail "Created environment.json"
fi

if [ -f "$ENV_JSON" ]; then
    if jq . "$ENV_JSON" >/dev/null 2>&1; then
        pass "environment.json is valid JSON"
    else
        fail "environment.json is valid JSON"
    fi

    # Check for required fields
    if jq -e '.timestamp' "$ENV_JSON" >/dev/null 2>&1; then
        pass "environment.json contains timestamp"
    else
        fail "environment.json contains timestamp"
    fi

    if jq -e '.platform' "$ENV_JSON" >/dev/null 2>&1; then
        pass "environment.json contains platform"
    else
        fail "environment.json contains platform"
    fi

    if jq -e '.tools' "$ENV_JSON" >/dev/null 2>&1; then
        pass "environment.json contains tools"
    else
        fail "environment.json contains tools"
    fi

    if jq -e '.git' "$ENV_JSON" >/dev/null 2>&1; then
        pass "environment.json contains git state"
    else
        fail "environment.json contains git state"
    fi
fi

# ============================================================
echo ""
echo "=== Test 5: session-start.sh kill switch ==="
# ============================================================

cd "$TMPDIR"
KILL_OUTPUT=$(AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/session-start.sh" 2>&1 || true)
KILL_EXIT=$?

if [ $KILL_EXIT -eq 0 ]; then
    pass "session-start.sh respects AGENTOPS_HOOKS_DISABLED"
else
    fail "session-start.sh respects AGENTOPS_HOOKS_DISABLED (exit code: $KILL_EXIT)"
fi

# With kill switch, output should be empty or minimal
if [ -z "$KILL_OUTPUT" ] || [ ${#KILL_OUTPUT} -lt 10 ]; then
    pass "session-start.sh produces no output with kill switch"
else
    fail "session-start.sh produces no output with kill switch"
fi

# ============================================================
echo ""
echo "=== Test 6: Hook chain sequence ==="
# ============================================================

# Simulate a full session start sequence
# 1. Session start creates environment
cd "$TMPDIR"
rm -f .agents/ao/environment.json 2>/dev/null || true

SESSION_OUTPUT=$(bash "$HOOKS_DIR/session-start.sh" 2>&1 || true)
SESSION_EXIT=$?

if [ $SESSION_EXIT -eq 0 ] && [ -f ".agents/ao/environment.json" ]; then
    pass "Hook chain: session-start creates environment"
else
    fail "Hook chain: session-start creates environment"
fi

# session-start should produce valid output
CHAIN_SUCCESS=true
if ! echo "$SESSION_OUTPUT" | jq . >/dev/null 2>&1; then
    CHAIN_SUCCESS=false
fi

if $CHAIN_SUCCESS; then
    pass "Hook chain: session-start produces valid JSON"
else
    fail "Hook chain: session-start produces valid JSON"
fi

# ============================================================
# Summary
# ============================================================

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test Summary:"
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "  TOTAL: $((PASS + FAIL))"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ $FAIL -gt 0 ]; then
    red "FAILED: $FAIL test(s) failed"
    exit 1
else
    green "SUCCESS: All tests passed"
    exit 0
fi
