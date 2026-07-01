#!/usr/bin/env bats
# age-55qz.11-followup: cc_send / agy_send are DELIVERY-based (both share _family_send). The original
# .11 gated them on `atm wait --until=generating`, but that primitive does NOT reliably detect a pane
# generating (verified live 2026-07-01 — the opus pane answered "ack" and the agy pane answered
# "pong" to a trivial task while `atm wait` timed out the whole window; `--type gemini` did not even
# match the Antigravity pane). A gate on it produced FALSE "not engaged" that respawn-thrashed the
# pane before it could review — the flakiness this fixes. These lock the reliable behavior: deliver +
# retry-on-no-delivery, and NEVER call `atm wait`. Pure control-flow (mock atm + respawn_pane).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"; ORIG_PATH="$PATH"; mkdir -p "$TMP/bin"
  RESPAWNS="$TMP/respawns"; : > "$RESPAWNS"
  WAITCALLS="$TMP/waitcalls"; : > "$WAITCALLS"
  # atm mock: `send` echoes $ATM_SEND_OUT; `wait` RECORDS a call (the send path must never invoke it).
  cat >"$TMP/bin/atm" <<EOF
#!/usr/bin/env bash
case "\$1" in
  send) printf '%s' "\${ATM_SEND_OUT:-{\"delivered\":1}}" ;;
  wait) echo called >> "$WAITCALLS" ;;
  *)    : ;;
esac
exit 0
EOF
  chmod +x "$TMP/bin/atm"; export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"   # source-guard returns before dispatch
  SESSION="testsession"; CC_PANE=1; AGY_PANE=3
  respawn_pane() { echo x >> "$RESPAWNS"; return 0; }
  log() { :; }
  export RESPAWNS WAITCALLS
}
teardown() { export PATH="$ORIG_PATH"; rm -rf "$TMP"; }

_nrespawns() { wc -l < "$RESPAWNS"  | tr -d ' '; }
_nwaits()    { wc -l < "$WAITCALLS" | tr -d ' '; }

@test "cc_send: delivered -> return 0, no respawn, and NEVER calls atm wait (anti-thrash)" {
  ATM_SEND_OUT='{"delivered":1}' run cc_send /tmp/packet
  [ "$status" -eq 0 ]
  [ "$(_nrespawns)" -eq 0 ]
  [ "$(_nwaits)" -eq 0 ]
}

@test "agy_send: delivered -> return 0, no respawn, and NEVER calls atm wait (anti-thrash)" {
  ATM_SEND_OUT='{"delivered":1}' run agy_send /tmp/packet
  [ "$status" -eq 0 ]
  [ "$(_nrespawns)" -eq 0 ]
  [ "$(_nwaits)" -eq 0 ]
}

@test "cc_send: NOT delivered -> return 1 after 3 respawn tries (delivery gate holds)" {
  ATM_SEND_OUT='{"delivered":0}' run cc_send /tmp/packet
  [ "$status" -eq 1 ]
  [ "$(_nrespawns)" -eq 3 ]
}

@test "agy_send: NOT delivered -> return 1 after 3 respawn tries" {
  ATM_SEND_OUT='{"delivered":0}' run agy_send /tmp/packet
  [ "$status" -eq 1 ]
  [ "$(_nrespawns)" -eq 3 ]
}

@test "the unreliable atm-wait engagement gate is gone from the whole send path" {
  ATM_SEND_OUT='{"delivered":1}' run cc_send /tmp/packet
  ATM_SEND_OUT='{"delivered":1}' run agy_send /tmp/packet
  ATM_SEND_OUT='{"delivered":0}' run cc_send /tmp/packet   # even the retry/respawn path
  [ "$(_nwaits)" -eq 0 ]
  # and the helper/config it depended on are removed
  ! grep -q '_wait_engaged'   "$REPO_ROOT/scripts/pawl.sh"
  ! grep -q 'PAWL_ENGAGE_WAIT' "$REPO_ROOT/scripts/pawl.sh"
}
