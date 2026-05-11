#!/usr/bin/env bash
# AgentOps Hook: ao forge transcript
# Queues transcript forging at session end.
# practices: [wiki-knowledge-surface, pragmatic-programmer]
set -euo pipefail

[ "${AGENTOPS_HOOKS_DISABLED:-0}" = "1" ] && exit 0

if command -v ao >/dev/null 2>&1; then
    ao forge transcript --last-session --queue --quiet 2>/dev/null || {
        ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
        if [ -z "${AO_AGENTS_DIR:-}" ] && [ -f "$ROOT/lib/ao-paths.sh" ]; then
            eval "$(bash "$ROOT/lib/ao-paths.sh" 2>/dev/null)" 2>/dev/null || true
        fi
        AO_DIR="${AO_AGENTS_DIR:-$ROOT/.agents}/ao"
        mkdir -p "$AO_DIR" 2>/dev/null
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: ao forge transcript" >> "$AO_DIR/hook-errors.log"
    }
fi

exit 0
