#!/usr/bin/env bats
# age-55qz.11-followup (engagement reliability): cc_send / agy_send share _family_send. A send is
# "engaged" only when the pane actually STARTS PRODUCING OUTPUT (its recent-scrollback cksum changes)
# after a delivery — a reliable, type-agnostic signal. This replaced .11's `atm wait --until=generating`
# gate, which does not detect the Antigravity (agy) CLI and intermittently misses claude too (verified
# live: opus/agy answered while atm-wait timed out). The output-change check catches the observed agy
# failure mode: `atm send --file` reports "delivered":1 but the TUI DROPS the input (empty pane, no
# review) — a re-send re-triggers it. Not-delivered -> respawn + re-send. Pure control-flow.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"; ORIG_PATH="$PATH"; mkdir -p "$TMP/bin"
  RESPAWNS="$TMP/respawns"; : > "$RESPAWNS"
  cat >"$TMP/bin/atm" <<EOF
#!/usr/bin/env bash
[ "\$1" = send ] && printf '%s' "\${ATM_SEND_OUT:-{\"delivered\":1}}"
exit 0
EOF
  chmod +x "$TMP/bin/atm"; export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"   # source-guard returns before dispatch
  SESSION="testsession"; CC_PANE=1; AGY_PANE=3
  respawn_pane() { echo x >> "$RESPAWNS"; return 0; }
  log() { :; }
  sleep() { :; }                          # no real waits in the engagement poll
  PAWL_SEND_ENGAGE_POLLS=2                 # keep the poll short
  ACTFILE="$TMP/act"; : > "$ACTFILE"
  export RESPAWNS ACTFILE
}
teardown() { export PATH="$ORIG_PATH"; rm -rf "$TMP"; }
_nrespawns() { wc -l < "$RESPAWNS" | tr -d ' '; }

# _pane_activity is overridden per-test. ENGAGED = value changes each call (a FILE counter, so it
# persists across the command-substitution subshells _family_send uses); STATIC/dropped = constant.
_engaged_activity() { echo x >> "$ACTFILE"; wc -l < "$ACTFILE" | tr -d ' '; }
_static_activity()  { echo static; }

@test "cc_send: delivered + pane starts producing output -> return 0, no respawn" {
  _pane_activity() { _engaged_activity; }
  ATM_SEND_OUT='{"delivered":1}' run cc_send /tmp/packet
  [ "$status" -eq 0 ]
  [ "$(_nrespawns)" -eq 0 ]
}

@test "agy_send: delivered + pane starts producing output -> return 0, no respawn" {
  _pane_activity() { _engaged_activity; }
  ATM_SEND_OUT='{"delivered":1}' run agy_send /tmp/packet
  [ "$status" -eq 0 ]
  [ "$(_nrespawns)" -eq 0 ]
}

@test "agy_send: delivered but pane STATIC (input dropped) -> re-sends, returns 1, NO respawn on the delivered path" {
  _pane_activity() { _static_activity; }
  ATM_SEND_OUT='{"delivered":1}' run agy_send /tmp/packet
  [ "$status" -eq 1 ]
  [ "$(_nrespawns)" -eq 0 ]   # a dropped-input re-send must NOT respawn (that was the thrash bug)
}

@test "cc_send: NOT delivered -> respawn + re-send, return 1 after 3 tries" {
  _pane_activity() { _static_activity; }
  ATM_SEND_OUT='{"delivered":0}' run cc_send /tmp/packet
  [ "$status" -eq 1 ]
  [ "$(_nrespawns)" -eq 3 ]
}

@test "the unreliable atm-wait engagement helper is gone; the output-change helper is present" {
  # check the SOURCED functions (not comment strings — the comments legitimately explain the removed
  # atm-wait gate, so a grep for 'until=generating'/'_wait_engaged' would false-fail on the docs).
  ! declare -F _wait_engaged >/dev/null   # the unreliable atm-wait engagement gate is gone
  declare -F _pane_activity  >/dev/null   # the reliable output-change engagement signal exists
  declare -F _family_send    >/dev/null   # cc/agy share the one delivery-based sender
}
