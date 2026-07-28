#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"

[[ -s "$SKILL" ]]
grep -q '^name: scaffold$' "$SKILL"
grep -q '^  effects: \[\]$' "$SKILL"
grep -q '^## Contract$' "$SKILL"
grep -q '^## Evidence$' "$SKILL"
grep -Fq 'The caller owns version control, revision, and delivery.' "$SKILL"
if grep -Eiq 'AUTO-REDO|ONE-HELPER|HELPER-ESCALATE|ao land|next_action' "$SKILL"; then
  echo 'scaffold contract contains retired lifecycle vocabulary' >&2
  exit 1
fi

echo "scaffold contract: PASS"
