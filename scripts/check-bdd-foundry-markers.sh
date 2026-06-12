#!/usr/bin/env bash
# check-bdd-foundry-markers.sh — bdd-foundry marker/enforcement floor check (ag-wi9w1).
#
# $1 = candidate file (default: the repo canonical .claude/workflows/bdd-foundry.js,
# so the gate can run it argless). Fails non-zero NAMING the missing marker on the
# first floor violated. Floors are exactly the S4 set of the
# canonicalize-bdd-foundry-workflow plan (spec C4).
#
# Ordering note (deviation from the spec table's row order, required by X2): the
# X2 mutation fixtures strip whole lines, which breaks JS syntax — running
# `node --check` first would surface a SyntaxError instead of naming the missing
# marker. Marker/enforcement greps therefore run BEFORE the syntax check; the
# floor SET is unchanged.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
candidate="${1:-$repo_root/.claude/workflows/bdd-foundry.js}"
if [ ! -f "$candidate" ]; then
  echo "FAIL: candidate file not found: $candidate"
  exit 1
fi

count() { grep -c "$1" "$candidate" || true; }

floor() { # $1=actual $2=floor $3=failure message naming the marker
  if [ "$1" -lt "$2" ]; then
    echo "FAIL: $3 (found $1, floor $2) in $candidate"
    exit 1
  fi
}

floor "$(count 'DRIFT_SCHEMA')" 2 "DRIFT_SCHEMA marker below floor (v4 drift-guard lost)"
floor "$(count 'beads\.json')" 3 "beads.json plumbing below floor (v3 file plumbing lost)"
floor "$(count 'DIR-MISAIM')" 2 "DIR-MISAIM marker below floor (v5 preflight lost)"
floor "$(count 'pre-run-N base snapshot')" 1 "'pre-run-N base snapshot' marker missing (v5 base snapshot lost)"

throw_window="$(grep -A6 -E "includes\('DIR-MISAIM'\)|startsWith\('DIR-MISAIM'\)" "$candidate" | grep -c 'throw' || true)"
floor "$throw_window" 1 "DIR-MISAIM preflight no longer throws (enforcement shape lost)"

guard_chain="$(grep -Fc 'cycleFree && uncovered.length === 0 && driftOk' "$candidate" || true)"
floor "$guard_chain" 2 "tracker-write guard chain 'cycleFree && uncovered.length === 0 && driftOk' below floor"

floor "$(count 'gap_dispositions')" 2 "gap_dispositions schema requirement below floor"

if ! command -v node >/dev/null 2>&1; then
  echo "FAIL: node not found — cannot run node --check on $candidate (admission gates fail closed)"
  exit 1
fi
if ! node --check "$candidate"; then
  echo "FAIL: node --check failed for $candidate"
  exit 1
fi

echo "OK: all bdd-foundry marker/enforcement floors pass for $candidate"
