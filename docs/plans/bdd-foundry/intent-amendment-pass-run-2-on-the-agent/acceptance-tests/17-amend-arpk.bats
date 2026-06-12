#!/usr/bin/env bats
# §E Cutover: ag-arpk disposition — B87.
# Live tracker reads from the MAIN checkout (never a worktree); fail-closed.

setup() {
  load helpers2
}

@test "B87: ag-arpk is dispositioned — explicit chosen path, named residual, machine-readable state agrees" {
  body="$(br_show ag-arpk)"
  [ -n "$body" ]

  # no longer an untouched open P1: EITHER deferred-with-reason OR superseded
  printf '%s\n' "$body" | grep -Eiq 'defer|supersed'

  # the disposition names the exact residual: cross-host (Mac ↔ bushido)
  # landing serialization is NOT provided by land.sh's host-local lock
  printf '%s\n' "$body" | grep -Eiq 'cross-host|host-local'
  printf '%s\n' "$body" | grep -Eiq 'land\.sh'

  # merge queue remains the named serializer option for that gap, and the
  # residual-handling choice is stated (a reader can tell what is protected)
  printf '%s\n' "$body" | grep -Eiq 'merge.?queue'

  # machine-readable state AGREES with the prose: br ready (run from the main
  # checkout, unlimited) no longer surfaces ag-arpk as unclaimed active work
  # — kept-planned must be blocked by a dependency edge, hence also not ready
  ready="$(br_ready_all)"
  ! printf '%s\n' "$ready" | grep -q 'ag-arpk'

  # if the chosen path is "keep merge-queue planned", the sequencing edge or
  # deferral must be visible on the bead itself (status, label, or dependency
  # on the land.sh epic) — an OPEN bead with no such marker fails
  if printf '%s\n' "$body" | grep -q 'OPEN'; then
    printf '%s\n' "$body" | grep -Eiq 'deferred|blocked|Blockers:|depends|after the land\.sh'
  fi
}
