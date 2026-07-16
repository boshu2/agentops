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
! grep -Eiq 'AUTO-REDO|ONE-HELPER|HELPER-ESCALATE|ao land|next_action' "$SKILL"

echo "scaffold contract: PASS"
