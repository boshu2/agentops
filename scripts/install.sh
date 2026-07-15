#!/usr/bin/env bash
# Tombstone: AgentOps 4 removed the 3.x runtime plugin installers.
# Canonical install: one checkout + `ao skills link` (see docs/MIGRATION.md).
set -euo pipefail
cat >&2 <<'MSG'
This AgentOps installer was removed in 4.x.

New installs:
  brew tap boshu2/agentops https://github.com/boshu2/homebrew-agentops
  brew install agentops
  git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
  cd ~/.local/share/agentops && ao skills link

Migration guide: https://github.com/boshu2/agentops/blob/main/docs/MIGRATION.md
MSG
exit 2
