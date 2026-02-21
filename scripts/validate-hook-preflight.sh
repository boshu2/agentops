#!/bin/bash
# validate-hook-preflight.sh
# Enforce hook safety checklist before shipping.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

errors=0

pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; errors=$((errors + 1)); }
section() { echo -e "${BLUE}$1${NC}"; }

search_q() {
    local pattern="$1"
    local file="$2"
    if command -v rg >/dev/null 2>&1; then
        rg -n "$pattern" "$file" >/dev/null 2>&1
    else
        grep -nE "$pattern" "$file" >/dev/null 2>&1
    fi
}

cd "$REPO_ROOT"

HOOK_FILES=(
  "hooks/session-start.sh"
  "hooks/session-end-maintenance.sh"
  "hooks/precompact-snapshot.sh"
  "hooks/pending-cleaner.sh"
  "hooks/task-validation-gate.sh"
  "hooks/dangerous-git-guard.sh"
  "hooks/git-worker-guard.sh"
  "hooks/pre-mortem-gate.sh"
  "hooks/push-gate.sh"
  "hooks/context-guard.sh"
  "hooks/prompt-nudge.sh"
  "hooks/ratchet-advance.sh"
  "hooks/standards-injector.sh"
  "hooks/stop-team-guard.sh"
)

section "Hook preflight checks"

# 1) File presence
for file in "${HOOK_FILES[@]}"; do
    if [[ -f "$file" ]]; then
        pass "$file exists"
    else
        fail "$file missing"
    fi
done

if [[ -f "lib/hook-helpers.sh" ]]; then
    pass "lib/hook-helpers.sh exists"
else
    fail "lib/hook-helpers.sh missing"
fi

if [[ -f "lib/chain-parser.sh" ]]; then
    pass "lib/chain-parser.sh exists"
else
    fail "lib/chain-parser.sh missing"
fi

# 2) Kill switches
for file in "${HOOK_FILES[@]}"; do
    if search_q "AGENTOPS_HOOKS_DISABLED" "$file"; then
        pass "$file has global kill switch"
    else
        fail "$file missing AGENTOPS_HOOKS_DISABLED"
    fi
done

if search_q "AGENTOPS_SESSION_START_DISABLED" hooks/session-start.sh; then
    pass "session-start has hook-specific kill switch"
else
    fail "session-start missing AGENTOPS_SESSION_START_DISABLED"
fi

if search_q "AGENTOPS_PRECOMPACT_DISABLED" hooks/precompact-snapshot.sh; then
    pass "precompact has hook-specific kill switch"
else
    fail "precompact missing AGENTOPS_PRECOMPACT_DISABLED"
fi

if search_q "AGENTOPS_PENDING_CLEANER_DISABLED" hooks/pending-cleaner.sh; then
    pass "pending-cleaner has hook-specific kill switch"
else
    fail "pending-cleaner missing AGENTOPS_PENDING_CLEANER_DISABLED"
fi

if search_q "AGENTOPS_TASK_VALIDATION_DISABLED" hooks/task-validation-gate.sh; then
    pass "task-validation-gate has hook-specific kill switch"
else
    fail "task-validation-gate missing AGENTOPS_TASK_VALIDATION_DISABLED"
fi

if search_q "AGENTOPS_SKIP_PRE_MORTEM_GATE" hooks/pre-mortem-gate.sh; then
    pass "pre-mortem-gate has hook-specific kill switch"
else
    fail "pre-mortem-gate missing AGENTOPS_SKIP_PRE_MORTEM_GATE"
fi

if search_q "AGENTOPS_AUTOCHAIN" hooks/ratchet-advance.sh; then
    pass "ratchet-advance has hook-specific kill switch"
else
    fail "ratchet-advance missing AGENTOPS_AUTOCHAIN"
fi

if search_q "AGENTOPS_CONTEXT_GUARD_DISABLED" hooks/context-guard.sh; then
    pass "context-guard has hook-specific kill switch"
else
    fail "context-guard missing AGENTOPS_CONTEXT_GUARD_DISABLED"
fi

# 3) Path rooting checks
if search_q "ROOT=.*git rev-parse --show-toplevel" hooks/session-start.sh \
    && search_q '\$ROOT/\.agents' hooks/session-start.sh; then
    pass "session-start is repo-rooted for .agents paths"
else
    fail "session-start missing repo-rooted .agents path handling"
fi

if search_q "resolve_repo_path\(\)" hooks/task-validation-gate.sh \
    && search_q "path escapes repo root" hooks/task-validation-gate.sh; then
    pass "task-validation-gate enforces repo-rooted validation paths"
else
    fail "task-validation-gate missing repo-root path enforcement"
fi

# 4) Unsafe eval checks
unsafe=0
for file in "${HOOK_FILES[@]}"; do
    if search_q '(^|[[:space:];])eval([[:space:](]|$)' "$file"; then
        fail "$file contains unsafe eval usage"
        unsafe=1
    fi
    if grep -E '(^|[^\\$()])`[^`]+`' "$file" | grep -vE '^\s*#' >/dev/null 2>&1; then
        fail "$file contains backtick command substitution"
        unsafe=1
    fi
done

if [[ "$unsafe" -eq 0 ]]; then
    pass "no unsafe eval/backtick usage detected"
fi

# 5) Basic telemetry checks
if search_q "HOOK_FAIL:" hooks/session-start.sh \
    && search_q "hook-errors\\.log" hooks/session-start.sh; then
    pass "session-start telemetry present"
else
    fail "session-start telemetry missing"
fi

if search_q "precompact-snapshot:" hooks/precompact-snapshot.sh \
    && search_q "hook-errors\\.log" hooks/precompact-snapshot.sh; then
    pass "precompact telemetry present"
else
    fail "precompact telemetry missing"
fi

if search_q "pending-cleaner:" hooks/pending-cleaner.sh \
    && search_q "ALERT|AUTOCLEAR" hooks/pending-cleaner.sh; then
    pass "pending-cleaner stale alert telemetry present"
else
    fail "pending-cleaner telemetry missing"
fi

if search_q "task-validation-gate:" hooks/task-validation-gate.sh \
    && search_q "hook-errors\\.log" hooks/task-validation-gate.sh; then
    pass "task-validation telemetry present"
else
    fail "task-validation telemetry missing"
fi

section "Hook manifest guardrails"

if [[ -f "hooks/hooks.json" ]]; then
    pass "hooks/hooks.json exists"
else
    fail "hooks/hooks.json missing"
fi

if command -v jq >/dev/null 2>&1; then
    missing_timeout="$(
        jq -r '
          .hooks
          | to_entries[]
          | .key as $event
          | (.value[]?.hooks[]? // empty)
          | select(.type == "command")
          | select(.command | test("(^|[ ;{])ao "))
          | select((.timeout // 0) <= 0)
          | "\($event): \(.command)"
        ' hooks/hooks.json
    )"
    if [[ -n "$missing_timeout" ]]; then
        fail "hooks/hooks.json has ao commands without timeout"
        echo "$missing_timeout" | sed 's/^/  - /'
    else
        pass "all ao hook commands have explicit timeout"
    fi

    inline_missing_guard="$(
        jq -r '
          .hooks
          | to_entries[]
          | .key as $event
          | (.value[]?.hooks[]? // empty)
          | select(.type == "command")
          | select(.command | contains("command -v ao"))
          | select((.command | contains("AGENTOPS_HOOKS_DISABLED")) | not)
          | "\($event): \(.command)"
        ' hooks/hooks.json
    )"
    if [[ -n "$inline_missing_guard" ]]; then
        fail "inline ao hook commands missing AGENTOPS_HOOKS_DISABLED guard"
        echo "$inline_missing_guard" | sed 's/^/  - /'
    else
        pass "inline ao hook commands include AGENTOPS_HOOKS_DISABLED guard"
    fi

    if jq -e '
      .hooks.SessionEnd[]?.hooks[]?
      | select(.command | contains("ao batch-feedback"))
      | select(.command | contains("--max-sessions"))
      | select(.command | contains("--max-runtime"))
      | select((.timeout // 0) > 0)
    ' hooks/hooks.json >/dev/null 2>&1; then
        pass "batch-feedback hook is bounded (inline)"
    elif jq -e '
      .hooks.SessionEnd[]?.hooks[]?
      | select(.command | contains("session-end-maintenance.sh"))
      | select((.timeout // 0) > 0)
    ' hooks/hooks.json >/dev/null 2>&1; then
        pass "session-end-maintenance hook is installed with timeout"
    else
        fail "session-end heavy maintenance guardrails missing"
    fi
else
    fail "jq is required for hooks/hooks.json guardrail checks"
fi

if [[ -f "hooks/session-end-maintenance.sh" ]]; then
    if search_q "session-end-heavy\\.lock" hooks/session-end-maintenance.sh \
        && search_q "flock" hooks/session-end-maintenance.sh; then
        pass "session-end-maintenance uses cross-process lock"
    else
        fail "session-end-maintenance missing cross-process lock guardrail"
    fi
fi

echo ""
if [[ "$errors" -gt 0 ]]; then
    echo -e "${RED}Hook preflight FAILED (${errors} issues)${NC}"
    exit 1
fi

echo -e "${GREEN}Hook preflight PASSED${NC}"
exit 0
