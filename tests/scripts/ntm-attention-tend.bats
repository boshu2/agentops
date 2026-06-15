#!/usr/bin/env bats
# Acceptance for scripts/ntm-attention-tend.sh (ag-56cru).
# Proves the wake bridge is event-triggered, bounded, and — critically — leaves
# NO orphan watcher (Navi #955: a bounded `ntm --robot-attention` probe can fork
# a watcher child that survives if only the parent is killed).
#
# Strategy: NTM_BIN points at a mock `ntm` that, on --robot-attention, optionally
# forks a long-lived watcher (recording its pid) before printing canned JSON. The
# bridge must reap that watcher's process group. Activity/send are mocked too.

setup() {
  TESTDIR="$(mktemp -d)"
  export MOCK_SENDS_LOG="$TESTDIR/sends.log"
  export MOCK_WATCHER_PIDFILE="$TESTDIR/watcher.pid"
  export MOCK_ATTENTION_JSON="$TESTDIR/attn.json"
  export MOCK_ACTIVITY_JSON="$TESTDIR/act.json"
  export MOCK_FORK_WATCHER=0
  : > "$MOCK_SENDS_LOG"
  : > "$MOCK_ATTENTION_JSON"
  : > "$MOCK_ACTIVITY_JSON"

  MOCK_NTM="$TESTDIR/ntm"
  cat > "$MOCK_NTM" <<'EOF'
#!/usr/bin/env bash
mode=""
for a in "$@"; do
  case "$a" in
    --robot-attention) mode=attention ;;
    --robot-activity=*) mode=activity ;;
    --robot-send=*) mode=send ;;
  esac
done
case "$mode" in
  attention)
    [ "${MOCK_HANG:-0}" = "1" ] && sleep 30
    if [ "${MOCK_FORK_WATCHER:-0}" = "1" ]; then
      sleep 300 &
      echo $! > "$MOCK_WATCHER_PIDFILE"
    fi
    cat "$MOCK_ATTENTION_JSON" 2>/dev/null
    ;;
  activity) cat "$MOCK_ACTIVITY_JSON" 2>/dev/null ;;
  send) echo "$*" >> "$MOCK_SENDS_LOG" ;;
esac
exit 0
EOF
  chmod +x "$MOCK_NTM"
  export NTM_BIN="$MOCK_NTM"
  BRIDGE="${BATS_TEST_DIRNAME}/../../scripts/ntm-attention-tend.sh"
}

teardown() {
  if [ -f "$MOCK_WATCHER_PIDFILE" ]; then
    kill "$(cat "$MOCK_WATCHER_PIDFILE" 2>/dev/null)" 2>/dev/null || true
  fi
  rm -rf "$TESTDIR"
}

attention_action_required() {
  printf '%s\n' '{"trigger_event":{"actionability":"action_required","summary":"24 urgent unread","details":{"agents":["mossylantern"]}}}' > "$MOCK_ATTENTION_JSON"
}
attention_info_only() {
  printf '%s\n' '{"trigger_event":{"actionability":"info","summary":"fyi","details":{"agents":["mossylantern"]}}}' > "$MOCK_ATTENTION_JSON"
}
activity_idle() {
  printf '%s\n' '{"agents":[{"name":"mossylantern","pane":"1","state":"idle"}]}' > "$MOCK_ACTIVITY_JSON"
}
activity_busy() {
  printf '%s\n' '{"agents":[{"name":"mossylantern","pane":"1","state":"busy"}]}' > "$MOCK_ACTIVITY_JSON"
}

@test "wakes an idle agent that has action_required attention" {
  attention_action_required; activity_idle
  run bash "$BRIDGE" mysession
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$MOCK_SENDS_LOG")" -eq 1 ]
  grep -q -- '--robot-send=mysession' "$MOCK_SENDS_LOG"
  grep -q -- '--pane=1' "$MOCK_SENDS_LOG"
}

@test "sends nothing when attention is not action_required" {
  attention_info_only; activity_idle
  run bash "$BRIDGE" mysession
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$MOCK_SENDS_LOG")" -eq 0 ]
}

@test "sends nothing on empty attention output (bounded, exit 0)" {
  : > "$MOCK_ATTENTION_JSON"; activity_idle
  run bash "$BRIDGE" mysession
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$MOCK_SENDS_LOG")" -eq 0 ]
}

@test "sends nothing on malformed attention JSON (bounded, exit 0)" {
  printf '%s\n' 'not json {{{' > "$MOCK_ATTENTION_JSON"; activity_idle
  run bash "$BRIDGE" mysession
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$MOCK_SENDS_LOG")" -eq 0 ]
}

@test "does not wake a busy pane" {
  attention_action_required; activity_busy
  run bash "$BRIDGE" mysession
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$MOCK_SENDS_LOG")" -eq 0 ]
}

@test "does not wake an idle agent that is NOT in the action_required set" {
  printf '%s\n' '{"trigger_event":{"actionability":"action_required","summary":"x","details":{"agents":["EmeraldJaguar"]}}}' > "$MOCK_ATTENTION_JSON"
  activity_idle  # idle agent is mossylantern, not affected
  run bash "$BRIDGE" mysession
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$MOCK_SENDS_LOG")" -eq 0 ]
}

@test "--dry-run sends nothing but reports the intended wake" {
  attention_action_required; activity_idle
  run bash "$BRIDGE" --dry-run mysession
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$MOCK_SENDS_LOG")" -eq 0 ]
  [[ "$output" == *"DRY-RUN would wake"* ]]
}

@test "usage error (exit 2) when no session given" {
  run bash "$BRIDGE"
  [ "$status" -eq 2 ]
}

# --- the core no-orphan proof (Navi #955) ------------------------------------
# The probe is launched as a session leader (setsid / python3 / perl os.setsid)
# on EVERY platform, so a watcher the probe forks shares its PGID and is reaped.
# This runs on macOS too (python3/perl), not just the Linux floor — skipping it
# on macOS would hollow the acceptance.
@test "no orphan watcher survives the bounded probe" {
  command -v setsid >/dev/null 2>&1 || command -v python3 >/dev/null 2>&1 || command -v perl >/dev/null 2>&1 || skip "no session-leader tool (setsid/python3/perl)"
  attention_action_required; activity_idle
  export MOCK_FORK_WATCHER=1
  run bash "$BRIDGE" mysession
  [ "$status" -eq 0 ]
  [ -f "$MOCK_WATCHER_PIDFILE" ]
  wpid="$(cat "$MOCK_WATCHER_PIDFILE")"
  [ -n "$wpid" ]
  dead=0
  for _ in 1 2 3 4 5 6 7 8; do
    if ! kill -0 "$wpid" 2>/dev/null; then dead=1; break; fi
    sleep 0.5
  done
  [ "$dead" -eq 1 ]   # the forked watcher must be reaped, not orphaned
}

@test "bounded: a hanging probe does not hang the bridge" {
  attention_action_required; activity_idle
  export MOCK_HANG=1 NTM_PROBE_TIMEOUT=1
  run bash "$BRIDGE" mysession
  [ "$status" -eq 0 ]   # bridge returns despite a hung probe (empty attn -> nothing to do)
  [ "$(wc -l < "$MOCK_SENDS_LOG")" -eq 0 ]
}

# --- static non-daemon / safe-reap guarantees --------------------------------
@test "fallback fails closed (no unisolated run)" {
  grep -Eq 'cannot guarantee no-orphan' "$BRIDGE"   # explicit fail-closed message
  grep -Eq 'refusing' "$BRIDGE"
}

@test "interrupted exit group-reaps the in-flight probe" {
  grep -q 'CURRENT_PROBE_PGID' "$BRIDGE"            # cleanup reaps the probe group on INT/TERM
}

@test "probe wait is bounded (no bare wait that can hang forever)" {
  grep -q 'NTM_PROBE_TIMEOUT' "$BRIDGE"             # deadline-bounded poll
  grep -q 'kill -KILL -- -' "$BRIDGE"               # escalate to KILL so reap is bounded
}
@test "cleanup is non-self-killing and the reap is guarded" {
  run grep -q 'pkill' "$BRIDGE"
  [ "$status" -ne 0 ]                                   # never pkill (can match unrelated procs)
  grep -q 'jobs -pr' "$BRIDGE"                          # cleanup reaps only our own jobs
  grep -q 'trap - EXIT INT TERM' "$BRIDGE"              # cleanup clears traps (no recursion)
  grep -q 'kill -TERM -- -' "$BRIDGE"                   # per-probe process-group reap
  grep -q 'self_pgid' "$BRIDGE"                         # reap guarded against our own group
}

@test "script is non-daemon by construction" {
  grep -q 'trap cleanup EXIT' "$BRIDGE"
  run grep -Eq 'while[[:space:]]+(true|:)' "$BRIDGE"
  [ "$status" -ne 0 ]                                   # no infinite/busy loop
  run grep -Eq '^[[:space:]]*set -m' "$BRIDGE"
  [ "$status" -ne 0 ]                                   # never job-control set -m (unsafe reap)
}
