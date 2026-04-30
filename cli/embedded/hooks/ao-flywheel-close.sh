#!/usr/bin/env bash
# AgentOps Hook: ao flywheel close-loop
# Closes the flywheel loop at stop.
set -euo pipefail

# TEMP: disabled until pend-* triple-ID amplification is verifiably fixed.
# Active incident 2026-04-30: this hook fired on every Stop event and ingested
# the stale .agents/knowledge/pending/ queue, producing 700+ pend-*-pend-*-pend-*
# files in .agents/learnings/ during a single session. The pool-side dedup fix
# (PR #163, commits 4af82384 + f6fce986) did not stop this driver path.
# See: .agents/learnings/2026-04-30-pend-pollution-actively-growing-during-session.md
exit 0

[ "${AGENTOPS_HOOKS_DISABLED:-0}" = "1" ] && exit 0

if command -v ao >/dev/null 2>&1; then
    ao flywheel close-loop --quiet >/dev/null 2>/dev/null || {
        ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
        mkdir -p "$ROOT/.agents/ao" 2>/dev/null
        echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: ao flywheel close-loop" >> "$ROOT/.agents/ao/hook-errors.log"
    }
fi

exit 0
