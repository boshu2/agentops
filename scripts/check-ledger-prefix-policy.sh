#!/usr/bin/env bash
# check-ledger-prefix-policy.sh - warn when the private br ledger contains
# bead IDs outside the AgentOps `ag-` prefix.
#
# The br cache import/rebuild path is operational via `br sync`. This guard
# only inspects `_beads/issues.jsonl`; it does not run a rebuild and does not
# evict legacy migrated rows. The existing foreign-prefix migration residue is
# deferred to a separate cleanup bead.
#
# Posture: WARN-FIRST / non-blocking. A contaminated ledger exits 0 after
# reporting every foreign ID and a count per foreign prefix. This lets the gate
# observe the current ledger without hard-failing while historical rows remain.
#
# Exit codes:
#   0 = clean, contaminated-but-warned, or local-only ledger absent
#   1 = script dependency or JSONL parse failure
#   2 = bad invocation

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
LEDGER="${LEDGER_PREFIX_POLICY_LEDGER:-$REPO_ROOT/_beads/issues.jsonl}"
EXPECTED_PREFIX="ag-"

if [ "$#" -gt 0 ]; then
  case "$1" in
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "check-ledger-prefix-policy: unknown argument: $1" >&2
      exit 2
      ;;
  esac
fi

if [ ! -f "$LEDGER" ]; then
  echo "check-ledger-prefix-policy: SKIP - ledger missing at $LEDGER"
  echo "  _beads is local-only/gitignored in this public repo; run br sync to import the cache when the private ledger is present."
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "check-ledger-prefix-policy: FAIL - jq is required to inspect $LEDGER" >&2
  exit 1
fi

if ! IDS="$(jq -r 'select(type == "object" and (.id? | type == "string")) | .id' "$LEDGER")"; then
  echo "check-ledger-prefix-policy: FAIL - could not parse JSONL at $LEDGER" >&2
  exit 1
fi

TOTAL_IDS=0
FOREIGN_IDS=()
FOREIGN_PREFIXES=()

while IFS= read -r id; do
  [ -n "$id" ] || continue
  TOTAL_IDS=$((TOTAL_IDS + 1))

  if [[ "$id" == "$EXPECTED_PREFIX"* ]]; then
    continue
  fi

  FOREIGN_IDS+=("$id")
  if [[ "$id" == *-* ]]; then
    FOREIGN_PREFIXES+=("${id%%-*}-")
  else
    FOREIGN_PREFIXES+=("<no-hyphen>")
  fi
done <<< "$IDS"

FOREIGN_COUNT="${#FOREIGN_IDS[@]}"

if [ "$FOREIGN_COUNT" -eq 0 ]; then
  echo "check-ledger-prefix-policy: PASS - all $TOTAL_IDS bead id(s) use $EXPECTED_PREFIX prefix"
  echo "  path: $LEDGER"
  exit 0
fi

echo "check-ledger-prefix-policy: WARN - $FOREIGN_COUNT foreign-prefix bead id(s) in $LEDGER"
echo "  expected prefix: $EXPECTED_PREFIX"
echo "  posture: warn-first/non-blocking until the deferred migration-residue eviction bead lands"
echo "foreign ids:"
printf '  %s\n' "${FOREIGN_IDS[@]}"
echo "foreign prefix counts:"
printf '%s\n' "${FOREIGN_PREFIXES[@]}" | sort | uniq -c | awk '{ printf "  %s %s\n", $2, $1 }'
exit 0
