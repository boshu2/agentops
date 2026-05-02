#!/usr/bin/env bash
# AgentOps Hook Helper: ao inject
# Legacy/manual wrapper for explicit knowledge injection. Runtime manifests keep
# SessionStart lean and use factory/JIT retrieval instead of calling this by default.
set -euo pipefail

[ "${AGENTOPS_HOOKS_DISABLED:-0}" = "1" ] && exit 0

if command -v ao >/dev/null 2>&1; then
    ao inject --apply-decay --format markdown --max-tokens 1000 2>/dev/null || {
        ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
        if [ -z "${AO_AGENTS_DIR:-}" ] && [ -f "$ROOT/lib/ao-paths.sh" ]; then
            eval "$(bash "$ROOT/lib/ao-paths.sh" 2>/dev/null)" 2>/dev/null || true
        fi
        AO_DIR="${AO_AGENTS_DIR:-$ROOT/.agents}/ao"
        mkdir -p "$AO_DIR" 2>/dev/null
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: ao inject" >> "$AO_DIR/hook-errors.log"
    }
fi

exit 0
