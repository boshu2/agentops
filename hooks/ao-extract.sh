#!/usr/bin/env bash
# AgentOps Hook: ao extract
# Runs knowledge extraction pipeline at session start.
# practices: [wiki-knowledge-surface, pragmatic-programmer]
set -euo pipefail

[ "${AGENTOPS_HOOKS_DISABLED:-0}" = "1" ] && exit 0

if command -v ao >/dev/null 2>&1; then
    ao forge transcript --last-session --quiet 2>/dev/null || {
        ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
        mkdir -p "$ROOT/.agents/ao" 2>/dev/null
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: ao forge transcript --last-session --quiet" >> "$ROOT/.agents/ao/hook-errors.log"
    }
fi

exit 0
