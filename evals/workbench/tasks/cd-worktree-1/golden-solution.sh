#!/usr/bin/env bash
set -euo pipefail
bead="${1:?Usage: check-worktree-per-bead.sh <bead-id> <shell-script>}"
file="${2:?}"
if grep -Eq "git[[:space:]]+worktree[[:space:]]+add[^#\n]*${bead}" "$file" || grep -Eq "agentops-wt-${bead}" "$file"; then
  exit 0
fi
echo "missing per-bead worktree for $bead"
exit 1
