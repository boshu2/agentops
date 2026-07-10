#!/usr/bin/env bats
# age-l3xj (D5): route identifier containment. A route bead id is interpolated into
# evidence/state paths ($EVID_DIR/${bead}-*.txt, ${bead}.packet.md); traversal, separators,
# control characters, flag-shaped or over-long ids must be REJECTED before any file write,
# so every evidence/state path stays beneath its configured root. Pure/mockable — no live
# substrate; cmd_route must die on the id BEFORE touching the lock or evidence dirs.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"; ORIG_PATH="$PATH"; mkdir -p "$TMP/bin"
  for b in atm tmux; do printf '#!/usr/bin/env bash\nexit 0\n' > "$TMP/bin/$b"; chmod +x "$TMP/bin/$b"; done
  export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"   # source-guard returns before dispatch
  EVID_DIR="$TMP/evidence"
  ROUTE_LOCK="$TMP/route.lock"
  ROOT="$TMP"; STATE_DIR="pawl"
  log() { :; }
  # a real packet file so only the id can be at fault
  PKT="$TMP/packet.md"; echo "packet" > "$PKT"
}
teardown() { export PATH="$ORIG_PATH"; rm -rf "$TMP"; }

_assert_rejected_before_write() {
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalid bead id"* ]]
  # no evidence dir/file, no lock: rejected BEFORE any write
  [ ! -e "$EVID_DIR" ]
  [ ! -e "$ROUTE_LOCK" ]
}

@test "route id: path traversal '../pwn' is rejected before any file write" {
  run cmd_route "../pwn" "$PKT"
  _assert_rejected_before_write
}

@test "route id: separator 'a/b' is rejected" {
  run cmd_route "a/b" "$PKT"
  _assert_rejected_before_write
}

@test "route id: absolute '/etc/cron.d/x' is rejected" {
  run cmd_route "/etc/cron.d/x" "$PKT"
  _assert_rejected_before_write
}

@test "route id: whitespace 'a b' is rejected" {
  run cmd_route "a b" "$PKT"
  _assert_rejected_before_write
}

@test "route id: control character is rejected" {
  run cmd_route "$(printf 'a\tb')" "$PKT"
  _assert_rejected_before_write
}

@test "route id: leading '-' (flag-shaped) is rejected" {
  run cmd_route "-rf" "$PKT"
  _assert_rejected_before_write
}

@test "route id: leading '.' (dotfile / '..') is rejected" {
  run cmd_route ".." "$PKT"
  _assert_rejected_before_write
  run cmd_route ".hidden" "$PKT"
  _assert_rejected_before_write
}

@test "route id: over-long (65 chars) is rejected" {
  local id; id="$(printf 'a%.0s' $(seq 1 65))"
  run cmd_route "$id" "$PKT"
  _assert_rejected_before_write
}

@test "route id: shell metacharacters are rejected" {
  run cmd_route 'a$(touch pwn)' "$PKT"
  _assert_rejected_before_write
  run cmd_route 'a;b' "$PKT"
  _assert_rejected_before_write
}

@test "route id: a normal bead id passes validation (fails LATER on no-session, not on the id)" {
  session_exists() { return 1; }
  run cmd_route "age-l3xj.2" "$PKT"
  [ "$status" -ne 0 ]
  [[ "$output" != *"invalid bead id"* ]]
  [[ "$output" == *"no standing session"* ]]
}
