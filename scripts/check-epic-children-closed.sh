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
# Dependencies: ao, jq (stubbed via PATH in the hermetic bats suite).
#
# Tracker-agnostic (age-5w8fd): all bead reads route through `ao beads exec`,
# which resolves bd vs br and the ledger automatically. Child enumeration:
#   ao beads exec children <epic> --json
# br synthesizes plain child ids one per line (extra flags ignored); bd
# forwards verbatim to `bd children <epic> --json`, a JSON array of issue
# objects. Both shapes are handled below. Per-child status uses
#   ao beads exec show <child> --json
# whose envelope ao normalizes to the canonical (br) shape for both trackers.

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

command -v ao >/dev/null 2>&1 || die "ao CLI not on PATH"
command -v jq >/dev/null 2>&1 || die "jq not on PATH"

# Enumerate child ids via the tracker-agnostic entry point. A tracker error is
# a hard stop (exit 4) — a gate that cannot see the children must not pass.
children_raw="$(ao beads exec children "$EPIC" --json)" \
  || die "children query failed for $EPIC (ao beads exec children)"

# Two shapes reach us: bd emits a JSON array of issue objects; br emits plain
# child ids one per line. Detect JSON by the first non-space character.
first_char="$(printf '%s' "$children_raw" | tr -d '[:space:]' | head -c1)"
if [[ "$first_char" == "[" || "$first_char" == "{" ]]; then
  child_ids="$(printf '%s' "$children_raw" | jq -r '
    (if type=="array" then . else (.issues // []) end)
    | .[]
    | (.id // empty)
  ' 2>/dev/null)" || die "could not parse children JSON for $EPIC"
else
  child_ids="$(printf '%s\n' "$children_raw" | awk 'NF{print $1}')"
fi

if [[ -z "$child_ids" ]]; then
  echo "OK: epic $EPIC has no open children (no children found)" >&2
  exit 0
fi

offenders=0
while IFS= read -r child; do
  [[ -n "$child" ]] || continue
  # A per-child status read failure is fail-closed as an offender (exit 1),
  # not exit 4: one unreadable child must block the epic close, not abort the
  # report of every other offender. Only the enumeration error above is 4.
  status="$(ao beads exec show "$child" --json 2>/dev/null | jq -r '.[0].status // .status // empty' 2>/dev/null)"
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
