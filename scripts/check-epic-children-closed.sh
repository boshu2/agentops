#!/usr/bin/env bash
#
# check-epic-children-closed.sh — the no-epic-close-with-open-child GATE.
#
# Enumerates the children of an epic (parent-child dependents) and FAILS if any
# child is still open or in_progress. This is the committed, tested form of the
# advisory "never close an epic with an open child" prose from the crank/evolve
# skills; reconcile-pr.sh --epic calls it before closing an epic.
#
# Usage:
#   scripts/check-epic-children-closed.sh <epic-id>
#
# Exit codes (documented contract — tests assert these exactly):
#   0  all children are closed (or the epic has no children)
#   1  at least one child is open/in_progress (each offending child printed)
#   4  usage / missing-dependency / bad-input error
#
# Dependencies: bd, jq (stubbed via PATH in the hermetic bats suite).
#
# Child enumeration uses the canonical bd dependents query:
#   bd dep list <epic> --direction=up -t parent-child --json
# We pull the child id from whichever id-bearing field the record carries
# (id / from_id / issue_id / dependent_id) so the gate survives bd schema drift.

set -uo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/check-epic-children-closed.sh <epic-id>

Fails (exit 1) if any parent-child child of <epic-id> is still open or
in_progress, naming each offender. Exits 0 when every child is closed.

Exit codes: 0 all-closed · 1 open-child(ren) · 4 usage.
USAGE
}

die() { echo "ERROR: $*" >&2; exit 4; }

EPIC=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --*)       usage; die "unknown flag: $1" ;;
    *)         [[ -z "$EPIC" ]] || { usage; die "only one epic-id allowed"; }; EPIC="$1"; shift ;;
  esac
done

[[ -n "$EPIC" ]] || { usage; die "need an epic-id"; }

command -v bd >/dev/null 2>&1 || die "bd CLI not on PATH"
command -v jq >/dev/null 2>&1 || die "jq not on PATH"

# Enumerate child ids (parent-child dependents of the epic).
children_json="$(bd dep list "$EPIC" --direction=up -t parent-child --json 2>/dev/null || true)"

# Guard: if the query errored or returned a non-array, treat as no children
# only when it's clearly empty; otherwise surface the error.
if echo "$children_json" | jq -e 'type=="object" and has("error")' >/dev/null 2>&1; then
  die "bd dep list failed: $(echo "$children_json" | jq -r '.error')"
fi

# Pull the child id from whichever id-bearing field exists.
child_ids="$(echo "$children_json" | jq -r '
  (if type=="array" then . else [] end)
  | .[]
  | (.id // .from_id // .issue_id // .dependent_id // empty)
' 2>/dev/null || true)"

if [[ -z "$child_ids" ]]; then
  echo "OK: epic $EPIC has no open children (no children found)" >&2
  exit 0
fi

offenders=0
while IFS= read -r child; do
  [[ -n "$child" ]] || continue
  status="$(bd show "$child" --json 2>/dev/null | jq -r '.[0].status // .status // empty' 2>/dev/null)"
  case "$status" in
    open|in_progress)
      echo "OPEN-CHILD: $child status=$status" >&2
      offenders=$((offenders+1))
      ;;
    "")
      echo "WARN: could not read status for child $child (treating as offender)" >&2
      offenders=$((offenders+1))
      ;;
    *)
      : # closed / done / cancelled — fine
      ;;
  esac
done <<< "$child_ids"

if [[ "$offenders" -gt 0 ]]; then
  echo "EPIC-GATE FAIL: $EPIC has $offenders open/in_progress child(ren)" >&2
  exit 1
fi

echo "OK: all children of $EPIC are closed" >&2
exit 0
