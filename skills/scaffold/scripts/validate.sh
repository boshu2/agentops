#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"

[[ -s "$SKILL" ]]
grep -q '^name: scaffold$' "$SKILL"
grep -q '^## Contract$' "$SKILL"
grep -q '^## Evidence$' "$SKILL"
grep -Fq 'The caller owns version control, revision, and delivery.' "$SKILL"
if grep -Eiq 'AUTO-REDO|ONE-HELPER|HELPER-ESCALATE|ao land|next_action' "$SKILL"; then
  echo 'scaffold contract contains retired lifecycle vocabulary' >&2
  exit 1
fi

# References must not re-grant the Git-mutation or continuation authority the
# contract denies ("does not schedule RPI, create work ownership, mutate Git,
# or decide what happens next"). This sweep covers references/** because a grant
# hidden in a template file passes unseen when only SKILL.md is scanned.
if [[ -d "$SKILL_DIR/references" ]] \
  && grep -rEiq 'initial commit|git commit|git push|next steps:' "$SKILL_DIR/references"; then
  echo 'scaffold references confer Git-commit or continuation authority the contract denies' >&2
  exit 1
fi

echo "scaffold contract: PASS"
