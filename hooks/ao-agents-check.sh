#!/usr/bin/env bash
# AgentOps Hook: .agents/ file count warning
# Warns when .agents/ directory exceeds 500 files.
set -euo pipefail

AGENTS_DIR="$(git rev-parse --show-toplevel 2>/dev/null || echo .)/.agents"
FCOUNT=$(find "$AGENTS_DIR" -type f 2>/dev/null | wc -l | tr -d ' ')
if [ "$FCOUNT" -gt 500 ]; then
    echo "Warning: .agents/ has $FCOUNT files. Run: scripts/prune-agents.sh --execute"
fi

exit 0
