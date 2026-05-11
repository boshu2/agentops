#!/usr/bin/env bash
# AgentOps Hook: ao feedback-loop
# Runs feedback loop analysis for the current session at session end.
# practices: [dora-metrics, sre, lean-startup]
set -euo pipefail

[ "${AGENTOPS_HOOKS_DISABLED:-0}" = "1" ] && exit 0

if command -v ao >/dev/null 2>&1; then
    ao feedback-loop --session "${CLAUDE_SESSION_ID:-}" 2>/dev/null || {
        ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
        if [ -z "${AO_AGENTS_DIR:-}" ] && [ -f "$ROOT/lib/ao-paths.sh" ]; then
            eval "$(bash "$ROOT/lib/ao-paths.sh" 2>/dev/null)" 2>/dev/null || true
        fi
        AO_DIR="${AO_AGENTS_DIR:-$ROOT/.agents}/ao"
        mkdir -p "$AO_DIR" 2>/dev/null
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: ao feedback-loop" >> "$AO_DIR/hook-errors.log"
    }
fi

exit 0
