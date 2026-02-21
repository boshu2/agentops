#!/usr/bin/env bash
# AgentOps Hook: ao ratchet status
# Outputs current ratchet status as JSON at session start.
set -euo pipefail

[ "${AGENTOPS_HOOKS_DISABLED:-0}" = "1" ] && exit 0

if command -v ao >/dev/null 2>&1; then
    ao ratchet status -o json 2>/dev/null || {
        ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
        mkdir -p "$ROOT/.agents/ao" 2>/dev/null
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: ao ratchet status" >> "$ROOT/.agents/ao/hook-errors.log"
    }
fi

exit 0
