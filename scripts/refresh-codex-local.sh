#!/usr/bin/env bash
set -euo pipefail

# refresh-codex-local.sh — thin compatibility wrapper.
# AgentOps 3.3 uses source links, not a Codex plugin cache refresh.
# Prefer: ao skills link

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

if ! command -v ao >/dev/null 2>&1; then
  if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
    AO="$REPO_ROOT/cli/bin/ao"
  else
    echo "ao not found on PATH. Build with: cd cli && make build" >&2
    echo "Then run: ao skills link" >&2
    exit 1
  fi
else
  AO="$(command -v ao)"
fi

echo "Linking AgentOps skills into detected runtimes (ao skills link)..."
exec "$AO" skills link "$@"
