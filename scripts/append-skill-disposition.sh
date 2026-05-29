#!/usr/bin/env bash
# append-skill-disposition.sh — ensure a new skill has a row in
# docs/contracts/skill-dispositions.yaml (ag-cw2y item-1 scaffold-half).
#
# Idempotent: appends a placeholder row only if the skill has none, so a
# newly-scaffolded skill is one-shot-green against heal.sh Check 12
# (MISSING_DISPOSITION). The placeholder uses a real bounded context so
# check-bounded-contexts-drift.sh passes; the author refines domain /
# hexagonal_role / disposition / rationale during content fill (mirrors the
# "manual content fill required" contract of the SKILL.md skeleton).
#
# Usage: append-skill-disposition.sh <skill-name> [repo-root]
set -euo pipefail

SKILL="${1:?usage: append-skill-disposition.sh <skill-name> [repo-root]}"
REPO_ROOT="${2:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
FILE="$REPO_ROOT/docs/contracts/skill-dispositions.yaml"

if [[ ! -f "$FILE" ]]; then
  echo "append-skill-disposition: no dispositions file at $FILE" >&2
  exit 1
fi

if grep -qE "^[[:space:]]*-[[:space:]]+skill:[[:space:]]+${SKILL}[[:space:]]*$" "$FILE"; then
  echo "append-skill-disposition: '$SKILL' already present — no-op"
  exit 0
fi

cat >> "$FILE" <<EOF
  - skill:          $SKILL
    domain:         "BC4 Factory"
    hexagonal_role: supporting
    disposition:    keep
    rationale:      "TODO: refine BC/hexagonal_role/disposition/rationale for $SKILL"
EOF
echo "append-skill-disposition: added placeholder row for '$SKILL' (refine before finalizing)"
