#!/usr/bin/env bash
# test-quality-signals.sh - Verify quality-signals.sh hook signal detection
#
# Tests:
#   a. Normal prompt — no signal written
#   b. Repeated prompt — repeated_prompt signal detected
#   c. Correction pattern — correction signal detected
#   d. Kill switch — no output when disabled
#
# Usage: ./tests/hooks/test-quality-signals.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
HOOK="$REPO_ROOT/hooks/quality-signals.sh"

PASS=0
FAIL=0

red()   { printf '\033[0;31m%s\033[0m\n' "$1"; }
green() { printf '\033[0;32m%s\033[0m\n' "$1"; }

pass() {
    green "  PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    red "  FAIL: $1"
    FAIL=$((FAIL + 1))
}

# Pre-flight
if [ ! -f "$HOOK" ]; then
    red "ERROR: Hook not found at $HOOK"
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    red "ERROR: jq is required"
    exit 1
fi

# Setup isolated temp repo (hook uses git rev-parse --show-toplevel)
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

setup_test_repo() {
    local dir="$1"
    git -C "$dir" init -q
    mkdir -p "$dir/.agents/ao" "$dir/.agents/signals"
}

# ============================================================
# Test A: Normal prompt — no signal
# ============================================================
echo "Test A: Normal prompt produces no signal"
TEST_A="$TMPDIR/test-a"
mkdir -p "$TEST_A"
setup_test_repo "$TEST_A"

export CLAUDE_SESSION_ID="test-a"
unset AGENTOPS_HOOKS_DISABLED 2>/dev/null || true
unset AGENTOPS_QUALITY_SIGNALS_DISABLED 2>/dev/null || true

echo '{"prompt":"implement the feature"}' | (cd "$TEST_A" && bash "$HOOK")

SIGNAL_LOG="$TEST_A/.agents/signals/session-quality.jsonl"
if [ ! -f "$SIGNAL_LOG" ]; then
    pass "No signal log created for normal prompt"
else
    if [ ! -s "$SIGNAL_LOG" ]; then
        pass "Signal log exists but is empty for normal prompt"
    else
        fail "Signal log should be empty for normal prompt, got: $(cat "$SIGNAL_LOG")"
    fi
fi

# ============================================================
# Test B: Repeated prompt — detection
# ============================================================
echo "Test B: Repeated prompt triggers repeated_prompt signal"
TEST_B="$TMPDIR/test-b"
mkdir -p "$TEST_B"
setup_test_repo "$TEST_B"

export CLAUDE_SESSION_ID="test-b"

# First submission seeds .last-prompt
echo '{"prompt":"do the thing"}' | (cd "$TEST_B" && bash "$HOOK")

# Second identical submission should trigger repeated_prompt
echo '{"prompt":"do the thing"}' | (cd "$TEST_B" && bash "$HOOK")

SIGNAL_LOG="$TEST_B/.agents/signals/session-quality.jsonl"
if [ -f "$SIGNAL_LOG" ] && grep -q '"repeated_prompt"' "$SIGNAL_LOG"; then
    pass "repeated_prompt signal detected"
else
    fail "Expected repeated_prompt signal in $SIGNAL_LOG"
    [ -f "$SIGNAL_LOG" ] && cat "$SIGNAL_LOG"
fi

# ============================================================
# Test C: Correction pattern — detection
# ============================================================
echo "Test C: Correction pattern triggers correction signal"
TEST_C="$TMPDIR/test-c"
mkdir -p "$TEST_C"
setup_test_repo "$TEST_C"

export CLAUDE_SESSION_ID="test-c"

echo '{"prompt":"no that'\''s wrong"}' | (cd "$TEST_C" && bash "$HOOK")

SIGNAL_LOG="$TEST_C/.agents/signals/session-quality.jsonl"
if [ -f "$SIGNAL_LOG" ] && grep -q '"correction"' "$SIGNAL_LOG"; then
    pass "correction signal detected for 'no that's wrong'"
else
    fail "Expected correction signal in $SIGNAL_LOG"
    [ -f "$SIGNAL_LOG" ] && cat "$SIGNAL_LOG"
fi

# Additional correction patterns
for pattern in "wrong approach" "stop doing that" "revert the changes" "undo that" "incorrect result" "not what I asked"; do
    TEST_CX="$TMPDIR/test-c-$(echo "$pattern" | tr ' ' '-')"
    mkdir -p "$TEST_CX"
    setup_test_repo "$TEST_CX"

    echo "{\"prompt\":\"$pattern\"}" | (cd "$TEST_CX" && bash "$HOOK")

    SIGNAL_LOG="$TEST_CX/.agents/signals/session-quality.jsonl"
    if [ -f "$SIGNAL_LOG" ] && grep -q '"correction"' "$SIGNAL_LOG"; then
        pass "correction signal detected for '$pattern'"
    else
        fail "Expected correction signal for '$pattern'"
        [ -f "$SIGNAL_LOG" ] && cat "$SIGNAL_LOG"
    fi
done

# ============================================================
# Test D: Kill switch — disabled
# ============================================================
echo "Test D: Kill switch prevents output"
TEST_D="$TMPDIR/test-d"
mkdir -p "$TEST_D"
setup_test_repo "$TEST_D"

export CLAUDE_SESSION_ID="test-d"
export AGENTOPS_QUALITY_SIGNALS_DISABLED=1

echo '{"prompt":"no that'\''s wrong"}' | (cd "$TEST_D" && bash "$HOOK")

SIGNAL_LOG="$TEST_D/.agents/signals/session-quality.jsonl"
if [ ! -f "$SIGNAL_LOG" ]; then
    pass "No signal log when kill switch is active"
elif [ ! -s "$SIGNAL_LOG" ]; then
    pass "Signal log empty when kill switch is active"
else
    fail "Expected no output with kill switch, got: $(cat "$SIGNAL_LOG")"
fi

unset AGENTOPS_QUALITY_SIGNALS_DISABLED

# Also test AGENTOPS_HOOKS_DISABLED
TEST_D2="$TMPDIR/test-d2"
mkdir -p "$TEST_D2"
setup_test_repo "$TEST_D2"

export AGENTOPS_HOOKS_DISABLED=1

echo '{"prompt":"no that'\''s wrong"}' | (cd "$TEST_D2" && bash "$HOOK")

SIGNAL_LOG="$TEST_D2/.agents/signals/session-quality.jsonl"
if [ ! -f "$SIGNAL_LOG" ]; then
    pass "No signal log when global hook kill switch is active"
elif [ ! -s "$SIGNAL_LOG" ]; then
    pass "Signal log empty when global hook kill switch is active"
else
    fail "Expected no output with global kill switch, got: $(cat "$SIGNAL_LOG")"
fi

unset AGENTOPS_HOOKS_DISABLED

# ============================================================
# Summary
# ============================================================
echo ""
echo "Results: $PASS passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
    red "FAILED"
    exit 1
fi
green "ALL PASSED"
exit 0
