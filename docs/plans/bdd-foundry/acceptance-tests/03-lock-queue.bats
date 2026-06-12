#!/usr/bin/env bats
# §3 Lock & queue — B26–B34

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

@test "B26: lock acquisition is atomic — exactly one winner at the same instant" {
  # hang-proof barrier: all 10 spin on a sentinel file, released by one touch
  barrier="$SANDBOX/barrier-go"
  for i in $(seq 1 10); do
    lane="$(new_lane "l$i" "feat-l$i")"; add_skill "$lane" "zz-b26-$i"
    ( until [ -f "$barrier" ]; do sleep 0.05; done
      cd "$lane" && scripts/land.sh ) > "$SANDBOX/out/T$i" 2>&1 &
    echo $! >> "$SANDBOX/pids"; eval "PID_T$i=$!"
  done
  sleep 0.5
  touch "$barrier"
  for i in $(seq 1 10); do wait_land "T$i"; done

  # the audit log shows exactly one acquire before any release
  first_two="$(audit_entries | jq -r 'select(.event=="acquire" or .event=="release") | .event' | head -2 | paste -sd' ' -)"
  [ "$first_two" = "acquire release" ]
  # no partial/temp lock file remains
  leftovers="$(find "$LAND_LOCK_DIR" -name '*.tmp' -o -name '*.partial' | wc -l | tr -d ' ')"
  [ "$leftovers" -eq 0 ]
  [ ! -f "$LAND_LOCK_DIR/lock.json" ]
}

@test "B27: unreadable lock storage fails closed before any mutation" {
  # variant 1: truncated JSON
  printf '{"id":"trunc","pid":12,"heart' > "$LAND_LOCK_DIR/lock.json"
  LANE="$(new_lane a feat-a)"; add_skill "$LANE" zz-b27a
  orig="$(git -C "$LANE" rev-parse HEAD)"; before="$(remote_main_sha)"
  run land "$LANE"
  [ "$status" -ne 0 ]
  [[ "$output" =~ lock\ state\ unreadable|lock\ storage ]]
  [ "$(git -C "$LANE" rev-parse HEAD)" = "$orig" ]
  [ "$(remote_main_sha)" = "$before" ]
  # not silently deleted without a corrupt-lock audit entry
  if [ ! -f "$LAND_LOCK_DIR/lock.json" ]; then
    audit_entries | grep -q 'corrupt-lock'
  fi
  rm -f "$LAND_LOCK_DIR/lock.json"

  # variant 2: read-only lock directory
  chmod 555 "$LAND_LOCK_DIR"
  run land "$LANE"
  [ "$status" -ne 0 ]
  [[ "$output" =~ lock\ state\ unreadable|lock\ storage ]]
  chmod 755 "$LAND_LOCK_DIR"
  [ "$(remote_main_sha)" = "$before" ]

  # variant 3: dangling symlink
  ln -s /nonexistent-target "$LAND_LOCK_DIR/lock.json"
  run land "$LANE"
  [ "$status" -ne 0 ]
  [[ "$output" =~ lock\ state\ unreadable|lock\ storage ]]
  [ "$(remote_main_sha)" = "$before" ]
}

@test "B28: lane identity is unique and PID reuse cannot fake liveness" {
  export LAND_STALE_TTL=2
  # two clones, same branch name, both mid-land → distinct ids resolving to worktree paths
  LANE_A="$(new_lane a feat-x)"; add_skill "$LANE_A" zz-b28-a
  LANE_B="$(new_lane b feat-x)"; add_skill "$LANE_B" zz-b28-b
  LAND_TEST_GATE_SLEEP=3 start_land A "$LANE_A"
  wait_until 10 lock_state_is "$LANE_B" held
  start_land B "$LANE_B"
  wait_until 10 queue_len_is "$LANE_A" 1
  sj="$(status_json "$LANE_A")"
  hid="$(jq -r '.holder.id' <<<"$sj")"
  qid="$(jq -r '.queue[0].id // .queue[0]' <<<"$sj")"
  [ "$hid" != "$qid" ]
  grep -q 'lanes/a' <<<"$hid"
  grep -q 'lanes/b' <<<"$qid"
  wait_land A; wait_land B

  # PID-reuse: recorded PID belongs to an unrelated live process (pid 1),
  # heartbeat stale → treated stale via identity+heartbeat, not bare liveness
  fabricate_lock 1 60 "testhost:/dead/worktree:1:111"
  LANE_C="$(new_lane c feat-c)"; add_skill "$LANE_C" zz-b28-c
  run land "$LANE_C"
  [ "$status" -eq 0 ]
  audit_entries | grep -q 'stale-takeover'
}

@test "B29: queue hygiene — no duplicate entries, no ghosts, no starvation" {
  export LAND_WAIT_TIMEOUT=60
  live_holder_start

  # duplicate suppression: same lane enqueues twice concurrently → one entry
  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b29-b
  start_land B1 "$LANE_B"; start_land B2 "$LANE_B"
  wait_until 10 queue_len_is "$LANE_B" 1
  sj="$(status_json "$LANE_B")"
  bcount="$(jq -r '[.queue[] | tostring] | map(select(test("lanes/b"))) | length' <<<"$sj")"
  [ "$bcount" -eq 1 ]

  # ghost removal: SIGKILL a waiter → its entry expires / is removed
  LANE_C="$(new_lane c feat-c)"; add_skill "$LANE_C" zz-b29-c
  start_land C "$LANE_C"
  c_queued() { status_json "$LANE_B" 2>/dev/null | jq -e '[.queue[] | tostring] | map(select(test("lanes/c"))) | length >= 1' >/dev/null 2>&1; }
  wait_until 10 c_queued || true
  kill -9 "$PID_C" 2>/dev/null || true
  # FIFO across arrivals: D waits, then E arrives; D must acquire before E
  LANE_D="$(new_lane d feat-d)"; add_skill "$LANE_D" zz-b29-d
  LANE_E="$(new_lane e feat-e)"; add_skill "$LANE_E" zz-b29-e
  start_land D "$LANE_D"; sleep 0.5; start_land E "$LANE_E"
  live_holder_stop
  wait_land B1; wait_land B2; wait_land D; wait_land E

  acq="$(audit_entries | grep '"acquire"')"
  d_line="$(grep -n 'lanes/d' <<<"$acq" | head -1 | cut -d: -f1)"
  e_line="$(grep -n 'lanes/e' <<<"$acq" | head -1 | cut -d: -f1)"
  [ -n "$d_line" ] && [ -n "$e_line" ] && [ "$d_line" -lt "$e_line" ]
  # C's ghost never blocked anyone: D and E landed
  c="$(fresh_clone)"; [ -d "$c/skills/zz-b29-d" ]; [ -d "$c/skills/zz-b29-e" ]
}

@test "B30: concurrent lands of the same branch — one land, one clean no-op" {
  # variant 1: same worktree, two processes
  LANE="$(new_lane a feat-x)"; add_skill "$LANE" zz-b30
  start_land P1 "$LANE"; start_land P2 "$LANE"
  wait_land P1; wait_land P2
  ok=0
  for t in P1 P2; do
    st_var="ST_$t"
    if [ "${!st_var}" -eq 0 ] && ! grep -q 'already landed\|being landed by' "$SANDBOX/out/$t"; then
      ok=$((ok+1))
    elif grep -qE 'already landed|branch is being landed by' "$SANDBOX/out/$t"; then
      :
    else
      false
    fi
  done
  [ "$ok" -ge 1 ]
  [ "$(remote_patch_ids | sort | uniq -d | wc -l | tr -d ' ')" -eq 0 ]

  # variant 2: two worktrees on the same branch
  LANE_C="$(new_lane c feat-y)"; add_skill "$LANE_C" zz-b30y
  LANE_D="$SANDBOX/lanes/d"; git clone -q "$REMOTE" "$LANE_D"
  git -C "$LANE_D" fetch -q "$LANE_C" feat-y:feat-y; git -C "$LANE_D" checkout -q feat-y
  start_land C "$LANE_C"; start_land D "$LANE_D"
  wait_land C; wait_land D
  c="$(fresh_clone)"; [ -d "$c/skills/zz-b30y" ]
  [ "$(remote_patch_ids | sort | uniq -d | wc -l | tr -d ' ')" -eq 0 ]
}

@test "B31: heartbeat stays fresh through long phases; heartbeat write failure is fail-safe" {
  export LAND_HEARTBEAT_INTERVAL=1
  LANE_A="$(new_lane a feat-a)"; add_skill "$LANE_A" zz-b31
  LAND_TEST_GATE_SLEEP=4 start_land A "$LANE_A"
  wait_until 10 lock_state_is "$LANE_A" held
  # poll every interval through the long gate: age always < 2x interval
  for _ in 1 2 3; do
    sleep 1
    age="$(status_json "$LANE_A" | jq -r '.holder.heartbeat_age_seconds')"
    [ -n "$age" ] && [ "$age" != null ]
    awk -v a="$age" 'BEGIN{exit !(a < 2)}'
  done
  wait_land A
  [ "$ST_A" -eq 0 ]

  # heartbeat write failure while holder alive: either no takeover while live,
  # or holder aborts and releases cleanly — never a takeover racing a live push.
  export LAND_STALE_TTL=2
  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b31-b
  LANE_W="$(new_lane w feat-w)"; add_skill "$LANE_W" zz-b31-w
  LAND_TEST_GATE_SLEEP=5 start_land B "$LANE_B"
  wait_until 10 lock_state_is "$LANE_W" held
  chmod 555 "$LAND_LOCK_DIR" 2>/dev/null || true
  start_land W "$LANE_W"
  wait_land B; wait_land W
  chmod 755 "$LAND_LOCK_DIR"
  c="$(fresh_clone)"
  if [ "$ST_B" -eq 0 ]; then
    [ -d "$c/skills/zz-b31-b" ]
  else
    [ ! -d "$c/skills/zz-b31-b" ]
  fi
  [ "$(remote_patch_ids | sort | uniq -d | wc -l | tr -d ' ')" -eq 0 ]
}

@test "B32: SIGINT/SIGTERM at every phase exit cleanly, no wreckage" {
  export LAND_WAIT_TIMEOUT=60
  for sig in INT TERM; do
    # phase: waiting in queue
    live_holder_start
    LQ="$(new_lane "q$sig" "feat-q$sig")"; add_skill "$LQ" "zz-b32-q$(tr 'A-Z' 'a-z' <<<"$sig")"
    start_land "Q$sig" "$LQ"
    sleep 1
    kill "-$sig" "$(eval echo "\$PID_Q$sig")" 2>/dev/null || true
    wait_land "Q$sig"
    st_var="ST_Q$sig"; [ "${!st_var}" -ne 0 ]
    live_holder_stop
    # no ghost queue entry
    wait_until 10 queue_len_is "$LQ" 0

    # phase: during gate (holding)
    LG="$(new_lane "g$sig" "feat-g$sig")"; add_skill "$LG" "zz-b32-g$(tr 'A-Z' 'a-z' <<<"$sig")"
    before="$(remote_main_sha)"
    LAND_TEST_GATE_SLEEP=10 start_land "G$sig" "$LG"
    wait_until 10 lock_state_is "$LG" held
    kill "-$sig" "$(eval echo "\$PID_G$sig")" 2>/dev/null || true
    wait_land "G$sig"
    st_var="ST_G$sig"; [ "${!st_var}" -ne 0 ]
    [ ! -d "$LG/.git/rebase-merge" ]
    [ "$(remote_main_sha)" = "$before" ]
    # lock released cleanly OR documented marker present
    if ! lock_state_is "$LG" unheld; then
      run land "$LG" --abort
      [ "$status" -eq 0 ]
      lock_state_is "$LG" unheld
    fi
  done
}

@test "B33: a failed holder releases and the queue advances" {
  export LAND_WAIT_TIMEOUT=60
  LANE_A="$(new_lane a feat-a)"
  # A will fail at the gate: plant a violating gate.d check in its branch
  add_skill "$LANE_A" zz-b33-a
  cat > "$LANE_A/scripts/gate.d/99-fail.sh" <<'EOF'
#!/usr/bin/env bash
echo "planted failure"
exit 1
EOF
  chmod +x "$LANE_A/scripts/gate.d/99-fail.sh"
  git -C "$LANE_A" add -A && git -C "$LANE_A" commit -qm "planted gate failure"

  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b33-b
  LANE_C="$(new_lane c feat-c)"; add_skill "$LANE_C" zz-b33-c
  pre_a_main="$(remote_main_sha)"

  LAND_TEST_GATE_SLEEP=2 start_land A "$LANE_A"
  wait_until 10 lock_state_is "$LANE_B" held
  start_land B "$LANE_B"; sleep 0.3; start_land C "$LANE_C"
  wait_land A; wait_land B; wait_land C
  [ "$ST_A" -ne 0 ]; [ "$ST_B" -eq 0 ]; [ "$ST_C" -eq 0 ]

  # A pushed nothing; B's recorded base SHA is the pre-A main tip
  grep -q "$pre_a_main" "$SANDBOX/out/B"
  acq="$(audit_entries | grep '"acquire"')"
  b_line="$(grep -n 'lanes/b' <<<"$acq" | head -1 | cut -d: -f1)"
  c_line="$(grep -n 'lanes/c' <<<"$acq" | head -1 | cut -d: -f1)"
  [ "$b_line" -lt "$c_line" ]
  c="$(fresh_clone)"
  [ ! -d "$c/skills/zz-b33-a" ]; [ -d "$c/skills/zz-b33-b" ]; [ -d "$c/skills/zz-b33-c" ]
}

@test "B34: the status contract is total — every lock state has pinned JSON" {
  LANE="$(new_lane a feat-x)"

  # unheld
  run status_json "$LANE"
  [ "$status" -eq 0 ]
  jq -e '.state=="unheld" and .holder==null and (.queue|type=="array")' <<<"$output"

  # held-live
  live_holder_start
  run status_json "$LANE"
  [ "$status" -eq 0 ]
  jq -e '.state=="held" and .holder.id and .holder.pid and (.holder.heartbeat_age_seconds!=null)' <<<"$output"
  live_holder_stop

  # held-stale (dead PID + old heartbeat)
  ( sleep 0.1 ) & dp=$!; wait "$dp" 2>/dev/null || true
  fabricate_lock "$dp" 9999
  run status_json "$LANE"
  [ "$status" -eq 0 ]
  jq -e '.state=="stale"' <<<"$output"
  rm -f "$LAND_LOCK_DIR/lock.json"

  # corrupt lock file
  printf 'not json at all' > "$LAND_LOCK_DIR/lock.json"
  run status_json "$LANE"
  [ "$status" -ne 0 ]
  jq -e '.state=="corrupt"' <<<"$output"
  rm -f "$LAND_LOCK_DIR/lock.json"

  # unreadable lock directory
  chmod 000 "$LAND_LOCK_DIR"
  run status_json "$LANE"
  st=$status
  chmod 755 "$LAND_LOCK_DIR"
  [ "$st" -ne 0 ]
  jq -e '.state=="unreadable"' <<<"$output"

  # no --status invocation mutated lock or queue state
  [ "$(audit_count acquire)" = 0 ]
}
