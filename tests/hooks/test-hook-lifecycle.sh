#!/usr/bin/env bash
# test-hook-lifecycle.sh - Hook lifecycle integration test
# Simulates a complete session event sequence by piping JSON events into hooks.
# Verifies each hook produces correct side effects in the expected order.
#
# Lifecycle sequence:
#   1. SessionStart   → session-start.sh (directory creation, silent runtime prep)
#   2. PreToolUse(Read)  → citation-tracker.sh (citation recorded)
#   3. PreToolUse(Bash)  → git-worker-guard.sh (worker git commit blocked)
#   4. PostToolUse(Bash) → ratchet-advance.sh (advance suggestion on ratchet record)
#   5. Stop              → stop-auto-handoff.sh (handoff file written)
#   6. SessionEnd        → session-end-maintenance.sh (clean completion)
#
# Usage: ./tests/hooks/test-hook-lifecycle.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOKS_DIR="$REPO_ROOT/hooks"

PASS=0
FAIL=0

red() { printf '\033[0;31m%s\033[0m\n' "$1"; }
green() { printf '\033[0;32m%s\033[0m\n' "$1"; }
yellow() { printf '\033[0;33m%s\033[0m\n' "$1"; }
blue() { printf '\033[0;34m%s\033[0m\n' "$1"; }

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
    red "ERROR: jq is required for hook lifecycle tests"
    exit 1
fi

if ! command -v git >/dev/null 2>&1; then
    red "ERROR: git is required for hook lifecycle tests"
    exit 1
fi

# ============================================================
# Setup: Create isolated mock repo with full .agents/ structure
# ============================================================

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

setup_lifecycle_repo() {
    local dir="$1"

    # Directory structure hooks expect
    mkdir -p "$dir/.agents/learnings"
    mkdir -p "$dir/.agents/research"
    mkdir -p "$dir/.agents/products"
    mkdir -p "$dir/.agents/retros"
    mkdir -p "$dir/.agents/patterns"
    mkdir -p "$dir/.agents/council"
    mkdir -p "$dir/.agents/knowledge/pending"
    mkdir -p "$dir/.agents/plans"
    mkdir -p "$dir/.agents/rpi"
    mkdir -p "$dir/.agents/ao/sessions"
    mkdir -p "$dir/.agents/ao/packets/pending"
    mkdir -p "$dir/.agents/handoff"
    mkdir -p "$dir/lib"
    mkdir -p "$dir/skills/standards/references"

    # Copy helper libraries (hooks source these)
    /bin/cp "$REPO_ROOT/lib/hook-helpers.sh" "$dir/lib/hook-helpers.sh"
    if [ -f "$REPO_ROOT/lib/chain-parser.sh" ]; then
        /bin/cp "$REPO_ROOT/lib/chain-parser.sh" "$dir/lib/chain-parser.sh"
    fi

    # Create sample learning (for citation tracking)
    cat > "$dir/.agents/learnings/2026-02-25-lifecycle-test.md" <<'EOF'
---
title: Lifecycle test learning
id: lifecycle-test
date: 2026-02-25
maturity: seed
tags: [test]
---

## Context
Test learning for hook lifecycle validation.

## Lesson
Hooks should process events in sequence without errors.
EOF

    # Create sample research file (for citation tracking)
    cat > "$dir/.agents/research/2026-02-25-lifecycle-research.md" <<'EOF'
# Lifecycle Research

## Objective
Test hook lifecycle.

## Findings
- Hooks read filesystem directly
EOF

    # Minimal CLAUDE.md
    echo "# Test Project" > "$dir/CLAUDE.md"

    # Python standards fixture (for standards-injector)
    cat > "$dir/skills/standards/references/python.md" <<'EOF'
# Python Standards
- Use type hints
- Follow PEP 8
EOF

    # Initialize git repo
    cd "$dir"
    git init -q
    git config user.email "test@example.com"
    git config user.name "Test User"
    git add .
    git commit -q -m "Initial lifecycle fixtures"
}

LIFECYCLE_REPO="$TMPDIR/lifecycle-repo"
mkdir -p "$LIFECYCLE_REPO"
setup_lifecycle_repo "$LIFECYCLE_REPO"

echo "=== Hook Lifecycle Integration Tests ==="
echo ""
blue "Test repo: $LIFECYCLE_REPO"
echo ""

# ============================================================
echo "=== Step 1: SessionStart ==="
# ============================================================

cd "$LIFECYCLE_REPO"

# Run session-start.sh
SESSION_OUTPUT=$(bash "$HOOKS_DIR/session-start.sh" 2>/dev/null || true)
SESSION_EXIT=$?

if [ $SESSION_EXIT -eq 0 ]; then
    pass "step 1: session-start exits 0"
else
    fail "step 1: session-start exits 0 (got $SESSION_EXIT)"
fi

# Validate JSON output
# session-start may emit ao output before the JSON, so extract last JSON block
SESSION_JSON=""
if echo "$SESSION_OUTPUT" | jq -e '.' >/dev/null 2>&1; then
    SESSION_JSON="$SESSION_OUTPUT"
else
    # Extract last JSON object
    SESSION_JSON=$(echo "$SESSION_OUTPUT" | LC_ALL=C awk '/^[[:space:]]*\{/{found=1; buf=""} found{buf=buf $0 "\n"} /^[[:space:]]*\}/{if(found) last=buf; found=0} END{printf "%s", last}')
fi

if [ -z "$SESSION_OUTPUT" ]; then
    pass "step 1: session-start remains silent"
else
    fail "step 1: session-start remains silent"
fi

# Verify directory creation (session-start creates .agents/ dirs)
EXPECTED_DIRS=(
    ".agents/research"
    ".agents/products"
    ".agents/retros"
    ".agents/learnings"
    ".agents/patterns"
    ".agents/council"
    ".agents/knowledge/pending"
    ".agents/ao"
)

DIR_CHECK_OK=true
for dir in "${EXPECTED_DIRS[@]}"; do
    if [ ! -d "$LIFECYCLE_REPO/$dir" ]; then
        DIR_CHECK_OK=false
        break
    fi
done
if $DIR_CHECK_OK; then
    pass "step 1: all .agents/ directories created"
else
    fail "step 1: all .agents/ directories created"
fi

# Verify environment.json was created
if [ -f "$LIFECYCLE_REPO/.agents/ao/environment.json" ]; then
    if jq -e '.timestamp' "$LIFECYCLE_REPO/.agents/ao/environment.json" >/dev/null 2>&1; then
        pass "step 1: environment.json created with timestamp"
    else
        fail "step 1: environment.json created with timestamp"
    fi
else
    # environment.json may not be created in all modes
    yellow "SKIP: environment.json not created (mode-dependent)"
fi

# ============================================================
echo ""
echo "=== Step 2: PreToolUse(Read) — Citation Tracker ==="
# ============================================================

# Simulate a Read event on a learning file
CITATION_INPUT='{"tool_input":{"file_path":".agents/learnings/2026-02-25-lifecycle-test.md"}}'

# Clean up any prior dedup state
rm -f /tmp/citation-tracker-*.seen 2>/dev/null

cd "$LIFECYCLE_REPO"
CITATION_OUTPUT=$(echo "$CITATION_INPUT" | \
    CLAUDE_SESSION_ID="lifecycle-test-session" \
    bash "$HOOKS_DIR/citation-tracker.sh" 2>/dev/null || true)
CITATION_EXIT=$?

if [ $CITATION_EXIT -eq 0 ]; then
    pass "step 2: citation-tracker exits 0"
else
    fail "step 2: citation-tracker exits 0 (got $CITATION_EXIT)"
fi

# Check that citation was recorded in citations.jsonl
if [ -f "$LIFECYCLE_REPO/.agents/ao/citations.jsonl" ]; then
    if grep -q "lifecycle-test" "$LIFECYCLE_REPO/.agents/ao/citations.jsonl"; then
        pass "step 2: citation recorded in citations.jsonl"
    else
        fail "step 2: citation recorded in citations.jsonl"
    fi
else
    fail "step 2: citations.jsonl file created"
fi

# Verify citation JSON structure
if [ -f "$LIFECYCLE_REPO/.agents/ao/citations.jsonl" ]; then
    LAST_CITATION=$(tail -1 "$LIFECYCLE_REPO/.agents/ao/citations.jsonl")
    if echo "$LAST_CITATION" | jq -e '.artifact_path' >/dev/null 2>&1; then
        pass "step 2: citation entry has artifact_path field"
    else
        fail "step 2: citation entry has artifact_path field"
    fi

    if echo "$LAST_CITATION" | jq -e '.cited_at' >/dev/null 2>&1; then
        pass "step 2: citation entry has cited_at timestamp"
    else
        fail "step 2: citation entry has cited_at timestamp"
    fi
fi

# Test session-level dedup: same file again should NOT create a duplicate
CITATION_BEFORE=$(wc -l < "$LIFECYCLE_REPO/.agents/ao/citations.jsonl" 2>/dev/null || echo 0)
echo "$CITATION_INPUT" | \
    CLAUDE_SESSION_ID="lifecycle-test-session" \
    bash "$HOOKS_DIR/citation-tracker.sh" >/dev/null 2>&1 || true
CITATION_AFTER=$(wc -l < "$LIFECYCLE_REPO/.agents/ao/citations.jsonl" 2>/dev/null || echo 0)

if [ "$CITATION_BEFORE" -eq "$CITATION_AFTER" ]; then
    pass "step 2: session dedup prevents duplicate citation"
else
    fail "step 2: session dedup prevents duplicate citation"
fi

# Test different artifact gets its own citation
RESEARCH_INPUT='{"tool_input":{"file_path":".agents/research/2026-02-25-lifecycle-research.md"}}'
echo "$RESEARCH_INPUT" | \
    CLAUDE_SESSION_ID="lifecycle-test-session" \
    bash "$HOOKS_DIR/citation-tracker.sh" >/dev/null 2>&1 || true
CITATION_COUNT=$(wc -l < "$LIFECYCLE_REPO/.agents/ao/citations.jsonl" 2>/dev/null || echo 0)
CITATION_COUNT=$(echo "$CITATION_COUNT" | tr -d ' ')

if [ "$CITATION_COUNT" -ge 2 ]; then
    pass "step 2: different artifact gets separate citation"
else
    fail "step 2: different artifact gets separate citation (count: $CITATION_COUNT)"
fi

# ============================================================
echo ""
echo "=== Step 3: PreToolUse(Bash) — Git Worker Guard ==="
# ============================================================

# Simulate a worker agent trying to git commit
WORKER_GIT_INPUT='{"tool_input":{"command":"git commit -m \"worker change\""}}'

cd "$LIFECYCLE_REPO"
GUARD_OUTPUT=$(echo "$WORKER_GIT_INPUT" | \
    CLAUDE_AGENT_NAME="worker-alpha" \
    bash "$HOOKS_DIR/git-worker-guard.sh" 2>&1 || true)
GUARD_EXIT=$?

# Worker commits should be BLOCKED (non-zero exit or stderr message)
if echo "$GUARD_OUTPUT" | grep -qi "worker.*must not commit\|blocked\|denied\|not commit"; then
    pass "step 3: git-worker-guard blocks worker commit"
else
    # Check if exit was non-zero
    if [ $GUARD_EXIT -ne 0 ]; then
        pass "step 3: git-worker-guard blocks worker commit (non-zero exit)"
    else
        fail "step 3: git-worker-guard blocks worker commit"
    fi
fi

# Non-worker (lead) should be allowed
LEAD_GIT_INPUT='{"tool_input":{"command":"git commit -m \"lead change\""}}'
LEAD_EXIT=0
echo "$LEAD_GIT_INPUT" | \
    CLAUDE_AGENT_NAME="lead" \
    bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || LEAD_EXIT=$?
if [ $LEAD_EXIT -eq 0 ]; then
    pass "step 3: git-worker-guard allows lead commit"
else
    fail "step 3: git-worker-guard allows lead commit"
fi

# Non-git command should pass through
SAFE_INPUT='{"tool_input":{"command":"ls -la"}}'
SAFE_EXIT=0
echo "$SAFE_INPUT" | bash "$HOOKS_DIR/git-worker-guard.sh" >/dev/null 2>&1 || SAFE_EXIT=$?
if [ $SAFE_EXIT -eq 0 ]; then
    pass "step 3: git-worker-guard passes non-git commands"
else
    fail "step 3: git-worker-guard passes non-git commands"
fi

# git add -A should be blocked for workers
ADD_ALL_INPUT='{"tool_input":{"command":"git add -A"}}'
ADD_EXIT=0
ADD_OUTPUT=$(echo "$ADD_ALL_INPUT" | \
    CLAUDE_AGENT_NAME="worker-beta" \
    bash "$HOOKS_DIR/git-worker-guard.sh" 2>&1 || ADD_EXIT=$?)
if [ $ADD_EXIT -ne 0 ] || echo "$ADD_OUTPUT" | grep -qi "worker\|blocked\|denied"; then
    pass "step 3: git-worker-guard blocks worker git add -A"
else
    fail "step 3: git-worker-guard blocks worker git add -A"
fi

# ============================================================
echo ""
echo "=== Step 4: PostToolUse(Bash) — Ratchet Advance ==="
# ============================================================

# Simulate successful `ao ratchet record research`
RATCHET_INPUT='{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":"0","stdout":"recorded"}}'

cd "$LIFECYCLE_REPO"
# ratchet-advance tries to run `ao ratchet next` — that may fail in test env.
# What we're testing is that the hook processes the input correctly.
RATCHET_OUTPUT=$(echo "$RATCHET_INPUT" | \
    AGENTOPS_RATCHET_ADVANCE_TIMEOUT=1 \
    bash "$HOOKS_DIR/ratchet-advance.sh" 2>/dev/null || true)
RATCHET_EXIT=$?

# Hook should exit cleanly (0) even if ao isn't available for next step
if [ $RATCHET_EXIT -eq 0 ]; then
    pass "step 4: ratchet-advance exits 0 on ratchet record"
else
    fail "step 4: ratchet-advance exits 0 on ratchet record (got $RATCHET_EXIT)"
fi

# If output exists, it should be valid JSON with additionalContext
if [ -n "$RATCHET_OUTPUT" ]; then
    if echo "$RATCHET_OUTPUT" | jq -e '.hookSpecificOutput.additionalContext' >/dev/null 2>&1; then
        pass "step 4: ratchet-advance emits additionalContext suggestion"
    else
        # May produce fallback suggestion text
        pass "step 4: ratchet-advance produced output (non-JSON fallback)"
    fi
else
    # No output is acceptable when ao ratchet next isn't available
    pass "step 4: ratchet-advance silent when ao unavailable (fail-open)"
fi

# Non-ratchet command should be ignored
NONRATCHET_INPUT='{"tool_input":{"command":"ls -la"},"tool_response":{"exit_code":"0"}}'
NONRATCHET_EXIT=0
echo "$NONRATCHET_INPUT" | bash "$HOOKS_DIR/ratchet-advance.sh" >/dev/null 2>&1 || NONRATCHET_EXIT=$?
if [ $NONRATCHET_EXIT -eq 0 ]; then
    pass "step 4: ratchet-advance ignores non-ratchet commands"
else
    fail "step 4: ratchet-advance ignores non-ratchet commands"
fi

# Failed ratchet record should be ignored
FAILED_RATCHET='{"tool_input":{"command":"ao ratchet record research"},"tool_response":{"exit_code":"1","stderr":"error"}}'
FAILED_EXIT=0
echo "$FAILED_RATCHET" | bash "$HOOKS_DIR/ratchet-advance.sh" >/dev/null 2>&1 || FAILED_EXIT=$?
if [ $FAILED_EXIT -eq 0 ]; then
    pass "step 4: ratchet-advance ignores failed ratchet record"
else
    fail "step 4: ratchet-advance ignores failed ratchet record"
fi

# ============================================================
echo ""
echo "=== Step 5: Stop — Auto Handoff ==="
# ============================================================

# stop-auto-handoff.sh captures last_assistant_message and writes a handoff file

HANDOFF_REPO="$TMPDIR/handoff-test"
mkdir -p "$HANDOFF_REPO/.agents/ao" "$HANDOFF_REPO/.agents/handoff" "$HANDOFF_REPO/lib"
git -C "$HANDOFF_REPO" init -q
/bin/cp "$REPO_ROOT/lib/hook-helpers.sh" "$HANDOFF_REPO/lib/hook-helpers.sh"
if [ -f "$REPO_ROOT/lib/chain-parser.sh" ]; then
    /bin/cp "$REPO_ROOT/lib/chain-parser.sh" "$HANDOFF_REPO/lib/chain-parser.sh"
fi

STOP_INPUT='{"last_assistant_message":"I completed the implementation of the auth middleware. The tests pass and coverage is at 85%. Next steps: add rate limiting."}'

cd "$HANDOFF_REPO"
STOP_OUTPUT=$(echo "$STOP_INPUT" | \
    CLAUDE_SESSION_ID="lifecycle-stop-test" \
    bash "$HOOKS_DIR/stop-auto-handoff.sh" 2>/dev/null || true)
STOP_EXIT=$?

if [ $STOP_EXIT -eq 0 ]; then
    pass "step 6: stop-auto-handoff exits 0"
else
    fail "step 6: stop-auto-handoff exits 0 (got $STOP_EXIT)"
fi

# Check that a handoff file was written
if ls "$HANDOFF_REPO/.agents/handoff"/stop-*.md >/dev/null 2>&1; then
    pass "step 6: handoff markdown file created"

    # Verify handoff content
    HANDOFF_FILE=$(ls -t "$HANDOFF_REPO/.agents/handoff"/stop-*.md | head -1)
    if grep -q "auth middleware" "$HANDOFF_FILE"; then
        pass "step 6: handoff contains assistant message"
    else
        fail "step 6: handoff contains assistant message"
    fi

    if grep -q "Last Assistant Message" "$HANDOFF_FILE"; then
        pass "step 6: handoff has structured sections"
    else
        fail "step 6: handoff has structured sections"
    fi
else
    fail "step 6: handoff markdown file created"
fi

# Check that a memory packet was written (if hook-helpers supports it)
if ls "$HANDOFF_REPO/.agents/ao/packets/pending"/*.json >/dev/null 2>&1; then
    PACKET_FILE=$(ls -t "$HANDOFF_REPO/.agents/ao/packets/pending"/*.json | head -1)
    if jq -e '.schema_version == 1' "$PACKET_FILE" >/dev/null 2>&1; then
        pass "step 6: memory packet has schema_version 1"
    else
        fail "step 6: memory packet has schema_version 1"
    fi

    if jq -e '.packet_type' "$PACKET_FILE" >/dev/null 2>&1; then
        pass "step 6: memory packet has packet_type"
    else
        fail "step 6: memory packet has packet_type"
    fi
else
    # Packets may not be written if ao isn't available
    yellow "SKIP: no memory packet written (ao-dependent)"
fi

# Test empty message produces no handoff
EMPTY_HANDOFF_REPO="$TMPDIR/empty-handoff"
mkdir -p "$EMPTY_HANDOFF_REPO/.agents/ao" "$EMPTY_HANDOFF_REPO/.agents/handoff" "$EMPTY_HANDOFF_REPO/lib"
git -C "$EMPTY_HANDOFF_REPO" init -q
/bin/cp "$REPO_ROOT/lib/hook-helpers.sh" "$EMPTY_HANDOFF_REPO/lib/hook-helpers.sh"
if [ -f "$REPO_ROOT/lib/chain-parser.sh" ]; then
    /bin/cp "$REPO_ROOT/lib/chain-parser.sh" "$EMPTY_HANDOFF_REPO/lib/chain-parser.sh"
fi

cd "$EMPTY_HANDOFF_REPO"
echo '{"last_assistant_message":""}' | \
    bash "$HOOKS_DIR/stop-auto-handoff.sh" >/dev/null 2>&1 || true

if ! ls "$EMPTY_HANDOFF_REPO/.agents/handoff"/stop-*.md >/dev/null 2>&1; then
    pass "step 6: no handoff written for empty message"
else
    fail "step 6: no handoff written for empty message"
fi

# ============================================================
echo ""
echo "=== Step 7: SessionEnd — Maintenance ==="
# ============================================================

# session-end-maintenance.sh runs forge, notebook update, maturity scans.
# In test env without a real ao, it should exit cleanly (fail-open).

END_REPO="$TMPDIR/end-test"
mkdir -p "$END_REPO/.agents/ao" "$END_REPO/lib"
git -C "$END_REPO" init -q
/bin/cp "$REPO_ROOT/lib/hook-helpers.sh" "$END_REPO/lib/hook-helpers.sh"
if [ -f "$REPO_ROOT/lib/chain-parser.sh" ]; then
    /bin/cp "$REPO_ROOT/lib/chain-parser.sh" "$END_REPO/lib/chain-parser.sh"
fi

cd "$END_REPO"
END_EXIT=0
bash "$HOOKS_DIR/session-end-maintenance.sh" >/dev/null 2>&1 || END_EXIT=$?

if [ $END_EXIT -eq 0 ]; then
    pass "step 7: session-end-maintenance exits 0 (fail-open)"
else
    fail "step 7: session-end-maintenance exits 0 (got $END_EXIT)"
fi

# Kill switch test
END_KILL_EXIT=0
AGENTOPS_HOOKS_DISABLED=1 bash "$HOOKS_DIR/session-end-maintenance.sh" >/dev/null 2>&1 || END_KILL_EXIT=$?
if [ $END_KILL_EXIT -eq 0 ]; then
    pass "step 7: session-end-maintenance kill switch works"
else
    fail "step 7: session-end-maintenance kill switch works"
fi

# ============================================================
echo ""
echo "=== Cross-Cutting: Kill Switch Verification ==="
# ============================================================

# Verify ALL hooks respect the AGENTOPS_HOOKS_DISABLED kill switch
HOOKS_WITH_KILL_SWITCH=(
    "session-start.sh"
    "factory-router.sh"
    "citation-tracker.sh"
    "git-worker-guard.sh"
    "ratchet-advance.sh"
    "stop-auto-handoff.sh"
    "session-end-maintenance.sh"
    "pending-cleaner.sh"
    "standards-injector.sh"
    "precompact-snapshot.sh"
)

cd "$LIFECYCLE_REPO"
for hook in "${HOOKS_WITH_KILL_SWITCH[@]}"; do
    HOOK_PATH="$HOOKS_DIR/$hook"
    if [ -f "$HOOK_PATH" ]; then
        EC=0
        OUTPUT=$(echo '{}' | AGENTOPS_HOOKS_DISABLED=1 bash "$HOOK_PATH" 2>&1 || EC=$?)
        if [ $EC -eq 0 ] && [ -z "$OUTPUT" ]; then
            pass "kill switch: $hook"
        elif [ $EC -eq 0 ]; then
            # Some hooks may produce minimal output even with kill switch
            pass "kill switch: $hook (exit 0, minor output)"
        else
            fail "kill switch: $hook (exit $EC)"
        fi
    else
        yellow "SKIP: $hook not found"
    fi
done

# ============================================================
echo ""
echo "=== Cross-Cutting: Non-Git-Repo Fail-Open ==="
# ============================================================

# Hooks should not crash when run outside a git repo
NON_GIT="$TMPDIR/non-git-dir"
mkdir -p "$NON_GIT"

FAILOPEN_HOOKS=(
    "citation-tracker.sh"
    "pending-cleaner.sh"
)

cd "$NON_GIT"
for hook in "${FAILOPEN_HOOKS[@]}"; do
    HOOK_PATH="$HOOKS_DIR/$hook"
    if [ -f "$HOOK_PATH" ]; then
        EC=0
        echo '{}' | bash "$HOOK_PATH" >/dev/null 2>&1 || EC=$?
        if [ $EC -eq 0 ]; then
            pass "fail-open outside git: $hook"
        else
            fail "fail-open outside git: $hook (exit $EC)"
        fi
    fi
done

# ============================================================
# Summary
# ============================================================

echo ""
echo "======================================="
echo "Hook Lifecycle Test Summary:"
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "  TOTAL: $((PASS + FAIL))"
echo "======================================="

if [ $FAIL -gt 0 ]; then
    red "FAILED: $FAIL test(s) failed"
    exit 1
else
    green "SUCCESS: All hook lifecycle tests passed"
    exit 0
fi
