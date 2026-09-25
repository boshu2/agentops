#!/usr/bin/env bash

# shellcheck source=scripts/lib/preamble.sh
# shellcheck disable=SC1007,SC1091
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"
cd "$REPO_ROOT" || exit 1

for retired in using-atm pre-land-refuters automation-shape-routing swarm crank; do
  [ ! -f "skills/$retired/SKILL.md" ] || { echo "retired skill still active: $retired" >&2; exit 1; }
done

active=(skills/agent-native/SKILL.md skills/orchestrate/SKILL.md skills/implement/SKILL.md skills/using-gc/SKILL.md)
if rg -n -i '\bATM\b|using-atm|vibing-with-ntm' "${active[@]}"; then
  echo "ATM-era naming remains in canonical orchestration contracts" >&2
  exit 1
fi

echo "orchestration skill boundaries: PASS"
