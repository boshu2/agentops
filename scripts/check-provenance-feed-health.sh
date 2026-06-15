#!/usr/bin/env bash
# check-provenance-feed-health.sh — assert the provenance sensor stays fed.
#
# Anti-regression gate: fails if the provenance ledger has stopped growing
# beyond the genesis row (the emitters silently died). Keeps milestone-1
# "fed" an enforced outcome, not a hope.
#
# Posture: WARN-ONLY by default (exit 0 with a warning). Pass --strict
# (or AGENTOPS_PROVENANCE_FEED_STRICT=1) to exit non-zero when the
# ledger has only the genesis row (dead sensor).
#
# Exit codes:
#   0 = ledger has real edges (healthy), or warn-only with dead sensor
#   1 = --strict and ledger has only genesis (dead sensor)
#   2 = script error (missing ledger file, bad invocation)

set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
LEDGER="${PROVENANCE_LEDGER:-$REPO_ROOT/docs/provenance/ledger.jsonl}"
STRICT="${AGENTOPS_PROVENANCE_FEED_STRICT:-0}"

while [ $# -gt 0 ]; do
  case "$1" in
    --strict) STRICT=1 ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "check-provenance-feed-health: unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

if [ ! -f "$LEDGER" ]; then
  echo "check-provenance-feed-health: FAIL — ledger missing at $LEDGER"
  exit 2
fi

ROW_COUNT=0
while IFS= read -r line; do
  [ -z "$line" ] && continue
  ROW_COUNT=$((ROW_COUNT + 1))
done < "$LEDGER"

if [ "$ROW_COUNT" -gt 1 ]; then
  echo "check-provenance-feed-health: PASS ($ROW_COUNT edges in ledger)"
  exit 0
fi

if [ "$STRICT" = "1" ]; then
  echo "check-provenance-feed-health: FAIL — ledger has only ${ROW_COUNT} row(s) (dead sensor)"
  echo "  path: $LEDGER"
  echo "  fix:  provenance emitters are not firing; check ao provenance emit-landed"
  exit 1
fi

echo "check-provenance-feed-health: WARN — ledger has only ${ROW_COUNT} row(s) (dead sensor)"
echo "  path: $LEDGER"
exit 0
