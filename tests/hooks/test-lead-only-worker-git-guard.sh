#!/usr/bin/env bash
# Smoke test for lead-only-worker-git-guard.sh hook
# Validates: script exists, syntax, kill switches, block + pass paths.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source shared colors/helpers if available; fall back to plain output.
if [[ -f "${SCRIPT_DIR}/../lib/colors.sh" ]]; then
    # shellcheck disable=SC1091
    source "${SCRIPT_DIR}/../lib/colors.sh"
else
    RED=""; GREEN=""; NC=""
    log() { echo "$@"; }
    pass() { echo "  PASS  $1"; }
fi

errors=0
fail() { echo "  FAIL  $1"; errors=$((errors + 1)); }

HOOK="$REPO_ROOT/hooks/lead-only-worker-git-guard.sh"

log "Testing lead-only-worker-git-guard.sh..."

# ---- Structural checks ------------------------------------------------------
[[ -f "$HOOK" ]] && pass "Script exists" || { fail "missing $HOOK"; exit 1; }
bash -n "$HOOK" 2>/dev/null && pass "Valid bash syntax" || fail "syntax error"
grep -q "AGENTOPS_HOOKS_DISABLED" "$HOOK" && pass "Has global kill switch" \
    || fail "missing AGENTOPS_HOOKS_DISABLED"
grep -q "AGENTOPS_LEAD_ONLY_GUARD_DISABLED" "$HOOK" && pass "Has local kill switch" \
    || fail "missing AGENTOPS_LEAD_ONLY_GUARD_DISABLED"

# ---- Behavioral checks ------------------------------------------------------
# Run the hook in a clean cwd so the .agents/swarm-role check doesn't fire.
TMPDIR_TEST=$(mktemp -d)
trap 'rm -rf "$TMPDIR_TEST"' EXIT
cd "$TMPDIR_TEST"

run_hook() {
    # $1 = JSON stdin payload, remaining args = env assignments
    local payload="$1"; shift
    env -i HOME="$HOME" PATH="$PATH" PWD="$TMPDIR_TEST" "$@" \
        bash "$HOOK" <<<"$payload"
}

assert_blocked() {
    local label="$1"; local payload="$2"; shift 2
    local out rc
    out=$(run_hook "$payload" "$@" 2>&1)
    rc=$?
    if [[ $rc -eq 2 ]]; then
        pass "BLOCK: $label"
    else
        fail "expected block (exit 2) for $label, got rc=$rc out=$out"
    fi
}

assert_allowed() {
    local label="$1"; local payload="$2"; shift 2
    local out rc
    out=$(run_hook "$payload" "$@" 2>&1)
    rc=$?
    if [[ $rc -eq 0 ]]; then
        pass "ALLOW: $label"
    else
        fail "expected allow (exit 0) for $label, got rc=$rc out=$out"
    fi
}

# Worker context: destructive verbs MUST block.
assert_blocked "worker git commit" \
    '{"tool_input":{"command":"git commit -m test"}}' AGENTOPS_SWARM_ROLE=worker
assert_blocked "worker git push" \
    '{"tool_input":{"command":"git push origin main"}}' AGENTOPS_SWARM_ROLE=worker
assert_blocked "worker git reset --hard" \
    '{"tool_input":{"command":"git reset --hard HEAD~1"}}' AGENTOPS_SWARM_ROLE=worker
assert_blocked "worker git rebase" \
    '{"tool_input":{"command":"git rebase main"}}' AGENTOPS_SWARM_ROLE=worker
assert_blocked "worker git merge" \
    '{"tool_input":{"command":"git merge feature"}}' AGENTOPS_SWARM_ROLE=worker
assert_blocked "worker git cherry-pick" \
    '{"tool_input":{"command":"git cherry-pick abc123"}}' AGENTOPS_SWARM_ROLE=worker
assert_blocked "worker git branch -D" \
    '{"tool_input":{"command":"git branch -D feature"}}' AGENTOPS_SWARM_ROLE=worker
assert_blocked "worker git checkout -B" \
    '{"tool_input":{"command":"git checkout -B newbranch"}}' AGENTOPS_SWARM_ROLE=worker
assert_blocked "worker git worktree remove" \
    '{"tool_input":{"command":"git worktree remove /tmp/wt"}}' AGENTOPS_SWARM_ROLE=worker
assert_blocked "worker chained commit" \
    '{"tool_input":{"command":"echo hi && git commit -m x"}}' AGENTOPS_SWARM_ROLE=worker

# Worker context: non-destructive git MUST be allowed.
assert_allowed "worker git status" \
    '{"tool_input":{"command":"git status"}}' AGENTOPS_SWARM_ROLE=worker
assert_allowed "worker git diff" \
    '{"tool_input":{"command":"git diff"}}' AGENTOPS_SWARM_ROLE=worker
assert_allowed "worker git log" \
    '{"tool_input":{"command":"git log --oneline -5"}}' AGENTOPS_SWARM_ROLE=worker
assert_allowed "worker git show" \
    '{"tool_input":{"command":"git show HEAD"}}' AGENTOPS_SWARM_ROLE=worker
assert_allowed "worker git blame" \
    '{"tool_input":{"command":"git blame README.md"}}' AGENTOPS_SWARM_ROLE=worker
assert_allowed "worker git fetch" \
    '{"tool_input":{"command":"git fetch origin"}}' AGENTOPS_SWARM_ROLE=worker
assert_allowed "worker non-git command" \
    '{"tool_input":{"command":"ls -la"}}' AGENTOPS_SWARM_ROLE=worker
assert_allowed "worker git branch -d (safe delete)" \
    '{"tool_input":{"command":"git branch -d old"}}' AGENTOPS_SWARM_ROLE=worker
assert_allowed "worker git checkout -b (safe new)" \
    '{"tool_input":{"command":"git checkout -b feat"}}' AGENTOPS_SWARM_ROLE=worker

# Lead context: destructive commands MUST be allowed (no role signal).
assert_allowed "lead git commit" \
    '{"tool_input":{"command":"git commit -m test"}}'
assert_allowed "lead git push" \
    '{"tool_input":{"command":"git push origin main"}}'
assert_allowed "lead git reset --hard" \
    '{"tool_input":{"command":"git reset --hard HEAD~1"}}'

# Kill switches.
assert_allowed "global kill switch" \
    '{"tool_input":{"command":"git commit -m x"}}' \
    AGENTOPS_SWARM_ROLE=worker AGENTOPS_HOOKS_DISABLED=1
assert_allowed "local kill switch" \
    '{"tool_input":{"command":"git commit -m x"}}' \
    AGENTOPS_SWARM_ROLE=worker AGENTOPS_LEAD_ONLY_GUARD_DISABLED=1

# Legacy AGENTOPS_ROLE signal still works.
assert_blocked "legacy AGENTOPS_ROLE=worker" \
    '{"tool_input":{"command":"git commit -m x"}}' AGENTOPS_ROLE=worker

# CLAUDE_AGENT_NAME=worker-* signal works.
assert_blocked "native team worker name" \
    '{"tool_input":{"command":"git push"}}' CLAUDE_AGENT_NAME=worker-1

if [[ $errors -eq 0 ]]; then
    echo "ALL TESTS PASSED"
    exit 0
else
    echo "FAILED: $errors test(s)"
    exit 1
fi
