#!/usr/bin/env bats
# session.json write -> idle-touch -> reload round-trip (age-nomq). The cross-family review
# caught that `_touch_route_ts` rewrote session.json via python's default json formatter
# ("key": "val", spaced) while the grep readers (load_session, _session_idle) required the
# COMPACT format (_write_session_json's printf). After the FIRST route the file became
# unparseable: a {cc,agy} session silently reverted to the 3-family default (agy mis-indexed)
# and the idle clock stuck at -1 (reaper dead). These tests round-trip a REAL persisted file
# through _touch_route_ts and assert the readers still parse it (fixture fidelity: exercise the
# actual writer + the actual idle-rewriter, not a hand-built compact-only fixture).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"
  TMP="$(mktemp -d)"
  ROOT="$TMP"                 # redirect state writes/reads into the temp dir
  STATE_DIR="pawl"
  mkdir -p "$TMP/pawl"
}

teardown() { rm -rf "$TMP"; }

# Build a session via the REAL writer for a {cc, agy} (no codex) host: agy must land at pane 2.
_seed_cc_agy_session() {
  _set_panes_from_enabled cc agy
  _write_session_json
}

@test "round-trip: {cc,agy} session reloads identically AFTER _touch_route_ts (no format-drift revert)" {
  _seed_cc_agy_session
  # First load (as `route` does) — correct.
  load_session
  [ "$ENABLED" = "cc agy" ]
  [ "$TIER" = "multi" ]
  [ "$CC_PANE" = "1" ]
  [ "$AGY_PANE" = "2" ]
  [ -z "$COD_PANE" ]

  # The idle-clock rewrite that previously corrupted the format.
  _touch_route_ts

  # Reload (as the NEXT route / health / reap does) — must be IDENTICAL, not the 3-family default.
  ENABLED=""; TIER=""; CC_PANE=""; COD_PANE=""; AGY_PANE=""
  load_session
  [ "$ENABLED" = "cc agy" ]      # NOT the "cc cod agy" fallback
  [ "$TIER" = "multi" ]
  [ "$CC_PANE" = "1" ]
  [ "$AGY_PANE" = "2" ]          # NOT mis-indexed to 3
  [ -z "$COD_PANE" ]             # codex stays absent (no invented pane)
}

@test "round-trip: single-family (fresh) session does NOT reload as multi after a touch" {
  _set_panes_from_enabled cc
  _write_session_json
  _touch_route_ts
  ENABLED=""; TIER=""
  load_session
  [ "$ENABLED" = "cc" ]
  [ "$TIER" = "fresh" ]          # NOT "multi" (the 3-family default would lie about cross-family)
}

@test "_session_idle: still parseable AFTER _touch_route_ts (idle reaper not silently dead)" {
  _seed_cc_agy_session
  _touch_route_ts
  run _session_idle
  [ "$status" -eq 0 ]
  # A freshly-touched session is idle ~0s, and crucially NOT the -1 'unparseable' sentinel.
  [ "$output" -ge 0 ]
}

@test "_session_idle: no session file -> -1 sentinel (reap no-op)" {
  run _session_idle
  [ "$output" = "-1" ]
}
