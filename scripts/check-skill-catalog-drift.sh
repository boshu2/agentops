#!/usr/bin/env bash
# check-skill-catalog-drift.sh — CI gate that fails if skills/catalog.json
# is out of sync with skills/*/SKILL.md frontmatter.
#
# Thin wrapper over generate-skill-catalog.sh --check so the workflow has a
# stable, named entry point and any future drift checks (catalog vs schema,
# catalog vs codex parity) can be added here without changing the workflow.
#
# Exit codes:
#   0 — catalog up-to-date
#   1 — drift detected
#   2 — wrapper or upstream tool error

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
GEN="$ROOT/scripts/generate-skill-catalog.sh"

if [ ! -x "$GEN" ]; then
  echo "check-skill-catalog-drift: $GEN missing or not executable" >&2
  exit 2
fi

"$GEN" --check
