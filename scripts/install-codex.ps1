# Tombstone: AgentOps 3.3 removed the legacy Codex plugin installer.
# Canonical install: one checkout + `ao skills link` (see docs/MIGRATION.md).
$ErrorActionPreference = "Stop"
Write-Error @"
This AgentOps installer was removed in 3.3.

New installs:
  git clone https://github.com/boshu2/agentops.git `$HOME\.local\share\agentops
  cd `$HOME\.local\share\agentops
  ao skills link

Or install the CLI: irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-ao.ps1 | iex

Migration guide: https://github.com/boshu2/agentops/blob/main/docs/MIGRATION.md
"@
exit 2
