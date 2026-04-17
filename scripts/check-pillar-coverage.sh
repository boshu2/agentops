#!/usr/bin/env bash
# check-pillar-coverage.sh — Verify GOALS.yaml has goals for all 4 pillars.
#
# Post-GOALS.md migration (see CLAUDE.md §Agent Goals): GOALS.yaml is the
# legacy YAML format. When it is absent but GOALS.md is present, skip with
# exit 0 because the markdown narrative does not carry machine-readable
# per-goal `pillar` fields; no downstream gate depends on this script today.
set -uo pipefail

GOALS_FILE="${1:-GOALS.yaml}"

if [ ! -f "$GOALS_FILE" ]; then
  if [ -f "GOALS.md" ]; then
    echo "SKIP: GOALS.yaml not found; repo uses GOALS.md (post-migration). Pillar coverage is documented in the narrative, not machine-checkable here." >&2
    exit 0
  fi
  echo "ERROR: GOALS.yaml not found at $GOALS_FILE" >&2
  exit 1
fi

pillars=$(yq '.goals[].pillar' "$GOALS_FILE" | sort -u)

for p in knowledge-compounding validated-acceleration goal-driven-automation zero-friction-workflow; do
  if ! echo "$pillars" | grep -q "$p"; then
    echo "MISSING pillar: $p" >&2
    exit 1
  fi
done
