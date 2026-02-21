#!/usr/bin/env bash
# AgentOps Hook: ao session-outcome
# Records session outcome at session end.
set -euo pipefail

[ "${AGENTOPS_HOOKS_DISABLED:-0}" = "1" ] && exit 0

if command -v ao >/dev/null 2>&1; then
    ao session-outcome 2>/dev/null || {
        ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
        mkdir -p "$ROOT/.agents/ao" 2>/dev/null
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: ao session-outcome" >> "$ROOT/.agents/ao/hook-errors.log"
    }
fi

exit 0
