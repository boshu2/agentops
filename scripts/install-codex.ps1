# Tombstone: AgentOps 3.3 removed the legacy Codex plugin installer.
# Supported installs: npx skills add, runtime plugins, or one checkout +
# `ao skills link` (see docs/MIGRATION.md).
$ErrorActionPreference = "Stop"
Write-Error @"
This AgentOps installer was removed in 3.3.

Install the skills (pick one):
  npx skills@latest add boshu2/agentops --all -g       # universal, all coding agents
  claude plugin install agentops@agentops-marketplace  # Claude Code managed bundle
  codex plugin add agentops@agentops-marketplace       # Codex managed bundle

Source-tracked install (edit skills / contribute):
  git clone https://github.com/boshu2/agentops.git `$HOME\.local\share\agentops
  cd `$HOME\.local\share\agentops
  ao skills link

Or install the CLI: irm https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-ao.ps1 | iex

Migration guide: https://github.com/boshu2/agentops/blob/main/docs/MIGRATION.md
"@
exit 2
