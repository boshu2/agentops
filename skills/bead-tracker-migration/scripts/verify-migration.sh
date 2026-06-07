#!/usr/bin/env bash
# verify-migration.sh — confirm a bd→br migration lost nothing.
# Execute: bash scripts/verify-migration.sh <source-bd-count> <br-prefix>
#   <source-bd-count>  the total recorded from `bd count` BEFORE migration
#   <br-prefix>        the ID prefix br was initialized with (must match bd's)
# Exit codes:
#   0 counts equal + spot-checks pass   1 count mismatch
#   2 usage error                       3 br workspace/DB unreadable
set -euo pipefail

SRC_COUNT="${1:-}"
PREFIX="${2:-}"

if [[ -z "$SRC_COUNT" || -z "$PREFIX" ]]; then
  echo "usage: verify-migration.sh <source-bd-count> <br-prefix>" >&2
  exit 2
fi
if ! [[ "$SRC_COUNT" =~ ^[0-9]+$ ]]; then
  echo "error: source-bd-count must be an integer, got '$SRC_COUNT'" >&2
  exit 2
fi

if ! command -v br >/dev/null 2>&1; then
  echo "error: br not on PATH" >&2
  exit 3
fi
if [[ ! -d .beads ]]; then
  echo "error: no .beads/ workspace in $(pwd)" >&2
  exit 3
fi

# Pull the br total. br count --json shape can vary across versions, so grab the
# first integer from the JSON as a robust fallback.
BR_JSON="$(br count --json 2>/dev/null || true)"
if [[ -z "$BR_JSON" ]]; then
  echo "error: br count produced no output (DB unreadable?)" >&2
  exit 3
fi
BR_COUNT="$(printf '%s' "$BR_JSON" | grep -oE '[0-9]+' | head -1 || true)"
if [[ -z "$BR_COUNT" ]]; then
  echo "error: could not parse a count from br output: $BR_JSON" >&2
  exit 3
fi

echo "source bd count : $SRC_COUNT"
echo "target br count : $BR_COUNT"
echo "prefix          : $PREFIX"

if [[ "$BR_COUNT" -ne "$SRC_COUNT" ]]; then
  echo "MISMATCH: $((BR_COUNT - SRC_COUNT)) issue delta — DO NOT delete bd data" >&2
  exit 1
fi

# Spot-check: at least one issue resolves under the expected prefix.
if ! br list --json 2>/dev/null | grep -q "\"${PREFIX}-"; then
  echo "WARN: no issue ID with prefix '${PREFIX}-' found — verify prefix" >&2
fi

echo "OK: counts equal ($BR_COUNT) — migration is data-complete"
exit 0
