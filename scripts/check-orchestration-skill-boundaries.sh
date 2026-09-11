#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

for retired in using-atm pre-land-refuters automation-shape-routing swarm crank; do
  [ ! -f "skills/$retired/SKILL.md" ] || { echo "retired skill still active: $retired" >&2; exit 1; }
done

active=(skills/ntm/SKILL.md skills/agent-mail/SKILL.md skills/agent-native/SKILL.md skills/implement/SKILL.md skills/using-gc/SKILL.md)
if rg -n -i '\bATM\b|using-atm|vibing-with-ntm' "${active[@]}"; then
  echo "ATM-era naming remains in canonical orchestration contracts" >&2
  exit 1
fi

echo "orchestration skill boundaries: PASS"
