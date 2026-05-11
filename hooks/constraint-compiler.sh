#!/usr/bin/env bash
# AgentOps Hook Helper: constraint-compiler
# Legacy wrapper that routes tagged learnings through hooks/finding-compiler.sh.
# practices: [design-by-contract, code-complete]
#
# Usage: bash hooks/constraint-compiler.sh <learning-path>
set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: constraint-compiler.sh <learning-path>" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
FINDING_COMPILER="${SCRIPT_DIR}/finding-compiler.sh"
LEARNING_PATH="$1"
shift || true

if [ ! -f "$LEARNING_PATH" ]; then
    echo "ERROR: Learning file not found: $LEARNING_PATH" >&2
    exit 1
fi

if [ ! -x "$FINDING_COMPILER" ]; then
    echo "ERROR: finding-compiler.sh not executable: $FINDING_COMPILER" >&2
    exit 1
fi

echo "Routing legacy constraint compilation through finding-compiler.sh" >&2
exec bash "$FINDING_COMPILER" "$LEARNING_PATH" "$@"
