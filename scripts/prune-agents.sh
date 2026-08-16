#!/usr/bin/env bash
# prune-agents.sh — compatibility wrapper over `ao session prune-agents`
#
# Usage:
#   ./scripts/prune-agents.sh              # Dry run (default)
#   ./scripts/prune-agents.sh --execute    # Apply retention deletions
#   ./scripts/prune-agents.sh --quiet      # Summary only
#
# Retention policy and every filesystem mutation live in the Go CLI. This
# wrapper only resolves the checkout root and matching ao binary, then forwards
# arguments. AGENTOPS_AO_BIN is the explicit binary seam used by CI and tests.

set -euo pipefail

# Anchor to this checkout (or the fixture override) before invoking ao, so a
# caller's current directory cannot select another repository's .agents tree.
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/repo-root.sh"
repo_root="$(resolve_repo_root)"

ao_bin="${AGENTOPS_AO_BIN:-}"
if [[ -z "$ao_bin" && -x "$repo_root/cli/bin/ao" ]]; then
  ao_bin="$repo_root/cli/bin/ao"
fi
if [[ -z "$ao_bin" ]]; then
  ao_bin="$(command -v ao 2>/dev/null || true)"
fi
if [[ -z "$ao_bin" || ! -x "$ao_bin" ]]; then
  echo "prune-agents: ao binary not found; build cli/bin/ao or set AGENTOPS_AO_BIN" >&2
  exit 1
fi

cd "$repo_root"
exec "$ao_bin" session prune-agents "$@"
