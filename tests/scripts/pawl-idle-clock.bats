#!/usr/bin/env bats
# pawl.sh idle clock (F13, age-pawl-intent-zhndq.14): the route idle-clock must be bumped
# portably (jq — a route-path hard dep — not python3-only), and `reap` must FAIL SAFE on a
# clock it cannot write rather than tear down an actively-serving session on a frozen timestamp.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl.sh"
  TMP="$(mktemp -d)"
  export PAWL_SESSION="idle-clock-test"
  export PAWL_SESSION_JSON="$TMP/session.json"
  # A session-JSON matching PAWL_SESSION, with a deliberately OLD last_route_ts.
  printf '{"session":"idle-clock-test","families":"cc cod","tier":"multi","up_ts":1000,"last_route_ts":1000}\n' \
    > "$PAWL_SESSION_JSON"
}
teardown() { rm -rf "$TMP"; }

# _touch_route_ts bumps last_route_ts (via jq) to ~now, and _session_idle reads it back small.
@test "idle clock: _touch_route_ts bumps last_route_ts (jq), _session_idle reads it near-zero" {
  run bash -c '
    export PAWL_SESSION PAWL_SESSION_JSON
    source "$1"
    _touch_route_ts
    idle="$(_session_idle)"
    # the clock was just bumped to now, so idle must be small and NON-negative (not the frozen 1000)
    printf "idle=%s\n" "$idle"
    [ "$idle" -ge 0 ] && [ "$idle" -lt 60 ]
  ' _ "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"idle="* ]]
  # The persisted file must still be parseable by the compact grep reader (jq -c form).
  grep -qE '"last_route_ts":[0-9]+' "$PAWL_SESSION_JSON"
}

# _clock_writable is true when jq is on PATH (the normal case), false when neither jq nor python3 is.
@test "idle clock: _clock_writable true with jq present, false when jq+python3 both absent" {
  run bash -c 'export PAWL_SESSION PAWL_SESSION_JSON; source "$1"; _clock_writable && echo WRITABLE' _ "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"WRITABLE"* ]]

  # With an empty PATH (no jq, no python3), _clock_writable must be false — the reap fail-safe trigger.
  run bash -c 'export PAWL_SESSION PAWL_SESSION_JSON; source "$1"; PATH="/nonexistent"; _clock_writable && echo WRITABLE || echo NOT-WRITABLE' _ "$SCRIPT"
  [[ "$output" == *"NOT-WRITABLE"* ]]
}
