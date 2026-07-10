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
  # age-l3xj: session state is now a SESSION-scoped shared file (SESSION_JSON), not repo-local.
  # Point it at a temp path the tests control; helpers derive it per SESSION when re-set below.
  SESSION_JSON="$TMP/session.json"
}
# Re-point SESSION_JSON to a per-session temp file (mirrors the real session-slug scoping).
_seed_session_json_path() { SESSION_JSON="$TMP/session-$(_pawl_lease_slug "$SESSION").json"; }

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

# --- age-l3xj (D5): crash-safe state persistence ---

@test "_write_session_json: atomic — no .tmp litter, file parses as JSON after write" {
  _seed_cc_agy_session
  run bash -c "ls \"$(dirname "$SESSION_JSON")\" | grep -c '\.tmp' || true"
  [ "$output" = "0" ]
  python3 -c "import json; json.load(open('$SESSION_JSON'))"
}

@test "_touch_route_ts: atomic — no .tmp litter, file parses as JSON after touch" {
  _seed_cc_agy_session
  _touch_route_ts
  run bash -c "ls \"$(dirname "$SESSION_JSON")\" | grep -c '\.tmp' || true"
  [ "$output" = "0" ]
  python3 -c "import json; json.load(open('$SESSION_JSON'))"
}

@test "load_session: a truncated/partial session.json falls back to the documented default, never garbage" {
  _seed_cc_agy_session
  # Simulate a torn write from a pre-atomic writer / external truncation.
  head -c 17 "$SESSION_JSON" > "$SESSION_JSON.trunc"
  mv "$SESSION_JSON.trunc" "$SESSION_JSON"
  ENABLED=""; TIER=""
  load_session
  [ "$ENABLED" = "cc cod agy" ]   # explicit legacy fallback — not a partial parse
  [ "$TIER" = "multi" ]
}

@test "_session_idle: a truncated session.json yields the -1 sentinel, not a crash" {
  _seed_cc_agy_session
  head -c 17 "$SESSION_JSON" > t && mv t "$SESSION_JSON"
  run _session_idle
  [ "$status" -eq 0 ]
  [ "$output" = "-1" ]
}

# age-l3xj (refuter rounds 18+21): session state is SESSION-scoped (a per-session file), so session
# B never reads or rewrites session A's state. Distinct sessions => distinct files => no
# contamination; a route to session B with no B-state falls to the documented default.
@test "load_session for session B does NOT pick up session A's state (distinct files)" {
  SESSION="session-A"; _seed_session_json_path; _set_panes_from_enabled cc agy; _write_session_json
  SESSION="session-B"; _seed_session_json_path                               # B's own (absent) file
  ENABLED=""; TIER=""; CC_PANE=""; COD_PANE=""; AGY_PANE=""
  load_session
  [ "$ENABLED" = "cc cod agy" ]   # the documented default — NOT A's {cc,agy} metadata
}

@test "_session_idle returns -1 for session B when only session A has state (reap no-op)" {
  SESSION="session-A"; _seed_session_json_path; _set_panes_from_enabled cc agy; _write_session_json
  SESSION="session-B"; _seed_session_json_path
  run _session_idle
  [ "$output" = "-1" ]
}

@test "_touch_route_ts for session B does NOT rewrite session A's file" {
  SESSION="session-A"; _seed_session_json_path; _set_panes_from_enabled cc agy; _write_session_json
  local a_file="$SESSION_JSON"; local before; before="$(cat "$a_file")"
  SESSION="session-B"; _seed_session_json_path
  _touch_route_ts
  [ "$(cat "$a_file")" = "$before" ]   # A's file untouched by B's touch
}

# Round 21: cross-repo route to a SHARED session reads the SAME session-scoped state `up` wrote —
# the real {cc,agy} layout — instead of defaulting to cc cod agy and mislabeling the AGY pane.
@test "cross-repo: two repos, one shared session → route reads the SAME layout up wrote" {
  SESSION="shared--pawl-service"; _seed_session_json_path
  # Repo A runs up: writes the session-scoped layout {cc,agy}.
  ROOT="$TMP/repoA"; _set_panes_from_enabled cc agy; _write_session_json
  # Repo B routes to the SAME session (different ROOT) — same SESSION_JSON, so same layout.
  ROOT="$TMP/repoB"; ENABLED=""; TIER=""; CC_PANE=""; COD_PANE=""; AGY_PANE=""
  load_session
  [ "$ENABLED" = "cc agy" ]      # B sees A's real layout, NOT the cc cod agy default
  [ "$AGY_PANE" = "2" ]          # AGY correctly at pane 2 (not mislabeled as codex)
  [ -z "$COD_PANE" ]
}

@test "matching session still loads + touches normally (no false rejection)" {
  _seed_cc_agy_session
  load_session
  [ "$ENABLED" = "cc agy" ]       # same SESSION => metadata IS used
  _touch_route_ts
  run _session_idle
  [ "$output" -ge 0 ]             # and the idle clock works
}
