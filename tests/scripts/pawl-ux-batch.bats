#!/usr/bin/env bats
# pawl UX batch (F10, age-pawl-intent-zhndq.11): friendly usage errors.
#
# `ao pawl route` with missing args must print a clean usage line and exit 2 — NOT leak a raw
# bash `${N:?}` parameter-expansion trace (`scripts/pawl.sh: line NNNN: 1: route needs <bead>`).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl.sh"
}

@test "route with no args -> clean usage line, exit 2 (no bash trace)" {
  run bash "$SCRIPT" route
  [ "$status" -eq 2 ]
  [[ "$output" == "usage: ao pawl route <bead> <packet-file> [pr]" ]]
  [[ "$output" != *"line "* ]]                 # no bash line-number trace
  [[ "$output" != *"route needs"* ]]           # no raw ${N:?} message
}

@test "route with bead but no packet -> clean usage line, exit 2" {
  run bash "$SCRIPT" route somebead
  [ "$status" -eq 2 ]
  [[ "$output" == "usage: ao pawl route <bead> <packet-file> [pr]" ]]
}
