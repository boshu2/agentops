#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

for retired in using-atm pre-land-refuters; do
  [ ! -f "skills/$retired/SKILL.md" ] || { echo "retired skill still active: $retired" >&2; exit 1; }
done

active=(skills/ntm/SKILL.md skills/agent-mail/SKILL.md skills/agent-native/SKILL.md skills/automation-shape-routing/SKILL.md skills/swarm/SKILL.md skills/using-gc/SKILL.md)
if rg -n -i '\bATM\b|using-atm|vibing-with-ntm' "${active[@]}"; then
  echo "ATM-era naming remains in canonical orchestration contracts" >&2
  exit 1
fi

rg -q '"ntm", "--robot-help"' cli/internal/adapters/agentworker_ntm/ntm.go
rg -q '"am", "capabilities", "--json"' cli/internal/adapters/agentmail_cli/agentmail.go
rg -q 'single worker pays no coordination tax' skills/agent-native/SKILL.md
rg -q 'external NTM binary' skills/ntm/SKILL.md
rg -q 'CLI self-describes' skills/agent-mail/SKILL.md
echo "orchestration skill boundaries: PASS"
