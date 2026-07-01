#!/usr/bin/env bats
# age-55qz.11: cc_send / agy_send must gate on ENGAGEMENT (the pane reached `generating`), not just
# on `"delivered":1` (keystrokes typed). On no-engagement they respawn to a fresh context + retry up
# to 3x (mirroring cod_send). These tests mock `atm` (send + wait) and respawn_pane so the control
# flow is exercised deterministically — no live tri-model substrate required. All pure control-flow.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"; ORIG_PATH="$PATH"; mkdir -p "$TMP/bin"
  RESPAWNS="$TMP/respawns"; : > "$RESPAWNS"
  # atm mock: `atm send` echoes $ATM_SEND_OUT; `atm wait` exits $ATM_WAIT_RC; anything else exits 0.
  cat >"$TMP/bin/atm" <<EOF
#!/usr/bin/env bash
case "\$1" in
  send) printf '%s' "\${ATM_SEND_OUT:-{\"delivered\":1}}" ;;
  wait) exit "\${ATM_WAIT_RC:-0}" ;;
  *)    exit 0 ;;
esac
EOF
  chmod +x "$TMP/bin/atm"; export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"   # source-guard returns before dispatch
  SESSION="testsession"; CC_PANE=1; AGY_PANE=3
  # neuter respawn (would spawn a real pane) and count invocations via a file (run is a subshell).
  respawn_pane() { echo x >> "$RESPAWNS"; return 0; }
  log() { :; }
  export RESPAWNS
}
teardown() { export PATH="$ORIG_PATH"; rm -rf "$TMP"; }

@test "cc_send: delivered + engaged -> return 0, no respawn" {
  ATM_SEND_OUT='{"delivered":1}' ATM_WAIT_RC=0 run cc_send /tmp/packet
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$RESPAWNS" | tr -d ' ')" -eq 0 ]
}

@test "cc_send: delivered but NEVER engages -> return 1 after 3 respawn tries" {
  ATM_SEND_OUT='{"delivered":1}' ATM_WAIT_RC=1 run cc_send /tmp/packet
  [ "$status" -eq 1 ]
  [ "$(wc -l < "$RESPAWNS" | tr -d ' ')" -eq 3 ]
}

@test "cc_send: NOT delivered -> return 1 after 3 respawn tries (delivery gate still holds)" {
  ATM_SEND_OUT='{"delivered":0}' ATM_WAIT_RC=0 run cc_send /tmp/packet
  [ "$status" -eq 1 ]
  [ "$(wc -l < "$RESPAWNS" | tr -d ' ')" -eq 3 ]
}

@test "agy_send: delivered + engaged -> return 0, no respawn" {
  ATM_SEND_OUT='{"delivered":1}' ATM_WAIT_RC=0 run agy_send /tmp/packet
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$RESPAWNS" | tr -d ' ')" -eq 0 ]
}

@test "agy_send: delivered but never engages -> return 1 after 3 respawn tries" {
  ATM_SEND_OUT='{"delivered":1}' ATM_WAIT_RC=1 run agy_send /tmp/packet
  [ "$status" -eq 1 ]
  [ "$(wc -l < "$RESPAWNS" | tr -d ' ')" -eq 3 ]
}

@test "_wait_engaged: passes through atm wait rc (0 engaged, non-zero not)" {
  ATM_WAIT_RC=0 run _wait_engaged 1 claude
  [ "$status" -eq 0 ]
  ATM_WAIT_RC=7 run _wait_engaged 1 claude
  [ "$status" -ne 0 ]
}
