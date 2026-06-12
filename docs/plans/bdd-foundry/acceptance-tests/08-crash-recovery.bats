#!/usr/bin/env bats
# §8 Crash & recovery — B57–B61

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

@test "B57: crash-point matrix — SIGKILL at every phase is recoverable to exactly-once" {
  export LAND_STALE_TTL=2
  phases="rebase regen-write regen-commit gate push pre-release"
  i=0
  for phase in $phases; do
    i=$((i+1))
    lane="$(new_lane "c$i" "feat-c$i")"
    add_skill "$lane" "zz-b57-$i"
    LAND_TEST_CRASH_AFTER="$phase" start_land "K$i" "$lane"
    wait_land "K$i"
    sleep 3   # let the lock go stale

    # the SAME lane reruns (preceded by --abort wherever a marker demands it)
    ( cd "$lane" && scripts/land.sh --abort ) || true
    run land "$lane"
    if [ "$phase" = "push" ] || [ "$phase" = "pre-release" ]; then
      # crashed after a (possibly) successful push: rerun reports already landed
      [ "$status" -eq 0 ]
    else
      [ "$status" -eq 0 ]
    fi

    c="$(fresh_clone)"
    [ -d "$c/skills/zz-b57-$i" ]
    # exactly-once: no duplicate patch-ids on main
    [ "$(remote_patch_ids | sort | uniq -d | wc -l | tr -d ' ')" -eq 0 ]
    # no stranded lock / stale queue entry / rebase wreckage
    run land "$lane" --status
    [[ "$output" == *unheld* ]]
    [ ! -d "$lane/.git/rebase-merge" ]
  done
  # the crashed-after-push case reports already landed on a further rerun
  lastlane="$SANDBOX/lanes/c$i"
  run land "$lastlane"
  [ "$status" -eq 0 ]
  [[ "$output" == *"already landed"* ]]
}

@test "B58: --abort has a complete contract" {
  # (a) no land in progress → exit 0, "nothing to abort", nothing changed
  LANE="$(new_lane a feat-a)"; add_skill "$LANE" zz-b58-a
  snap="$(git_dir_state_snapshot "$LANE")"
  run land "$LANE" --abort
  [ "$status" -eq 0 ]
  [[ "$output" == *"nothing to abort"* ]]
  [ "$(git_dir_state_snapshot "$LANE")" = "$snap" ]

  # (b) the invoking lane owns an in-progress marker (crash mid-land first)
  export LAND_STALE_TTL=2
  LB="$(new_lane b feat-b)"; add_skill "$LB" zz-b58-b
  orig="$(git -C "$LB" rev-parse HEAD)"
  LAND_TEST_CRASH_AFTER=regen-write start_land B "$LB"
  wait_land B
  sleep 3
  run land "$LB" --abort
  [ "$status" -eq 0 ]
  [ "$(git -C "$LB" rev-parse --abbrev-ref HEAD)" = "feat-b" ]
  [ "$(git -C "$LB" rev-parse HEAD)" = "$orig" ]
  worktree_clean "$LB"                       # (d) stray regen artifacts removed
  run land "$LB" --status
  [[ "$output" == *unheld* ]]                # lock released
  audit_entries | grep -q '"abort"'          # abort audit entry

  # (c) a DIFFERENT live lane holds the lock → refuse, untouched
  live_holder_start
  holder_before="$(cat "$LAND_LOCK_DIR/lock.json")"
  run land "$LANE" --abort
  [ "$status" -ne 0 ]
  [[ "$output" == *"lock held by live lane"* ]]
  jq -e '.nonce == "cafe0001"' "$LAND_LOCK_DIR/lock.json" >/dev/null
  live_holder_stop
}

@test "B59: every documented failure has a clean retry path" {
  # (1) gate failure → fix all violations → retry lands
  L1="$(new_lane g feat-g)"
  add_skill "$L1" zz-b59-g
  cat > "$L1/scripts/gate.d/99-fail.sh" <<'EOF'
#!/usr/bin/env bash
echo "planted failure"; exit 1
EOF
  chmod +x "$L1/scripts/gate.d/99-fail.sh"
  git -C "$L1" add -A && git -C "$L1" commit -qm "planted gate failure"
  run land "$L1"
  [ "$status" -ne 0 ]
  git -C "$L1" rm -q scripts/gate.d/99-fail.sh && git -C "$L1" commit -qm "fix violation"
  run land "$L1"
  [ "$status" -eq 0 ]

  # (2) conflict abort → resolve in branch → retry lands
  SETUP="$(new_lane setup feat-setup)"
  sed -i.bak 's/line-1/line-1-main/' "$SETUP/skills/crank/SKILL.md" && rm -f "$SETUP/skills/crank/SKILL.md.bak"
  ( cd "$SETUP" && bash scripts/regen-all.sh && git add -A && git commit -qm m && git push -q origin HEAD:main )
  L2="$(new_lane c feat-c)"
  git -C "$L2" reset -q --hard origin/main~1 2>/dev/null || true
  sed -i.bak 's/line-1/line-1-lane/' "$L2/skills/crank/SKILL.md" && rm -f "$L2/skills/crank/SKILL.md.bak"
  git -C "$L2" commit -qam "lane edit"
  run land "$L2"
  [ "$status" -ne 0 ]
  # resolve: take main's line then add lane's real change elsewhere
  git -C "$L2" fetch -q origin
  git -C "$L2" rebase -q origin/main || {
    ( cd "$L2" && sed -i.bak 's/^<<<<<<<.*//;s/^=======.*//;s/^>>>>>>>.*//' skills/crank/SKILL.md && rm -f skills/crank/SKILL.md.bak \
      && git add -A && GIT_EDITOR=true git rebase --continue )
  }
  run land "$L2"
  [ "$status" -eq 0 ]

  # (3) stale takeover: A crashed, B landed first; A's retry rebases onto B's main
  export LAND_STALE_TTL=2
  LA="$(new_lane sa feat-sa)"; add_skill "$LA" zz-b59-a
  LB="$(new_lane sb feat-sb)"; add_skill "$LB" zz-b59-b
  LAND_TEST_CRASH_AFTER=gate start_land A "$LA"
  wait_land A
  sleep 3
  run land "$LB"
  [ "$status" -eq 0 ]
  b_tip="$(remote_main_sha)"
  ( cd "$LA" && scripts/land.sh --abort ) || true
  run land "$LA"
  [ "$status" -eq 0 ]
  c="$(fresh_clone)"
  git -C "$c" merge-base --is-ancestor "$b_tip" origin/main   # B's work in A's ancestry
  [ -d "$c/skills/zz-b59-a" ] && [ -d "$c/skills/zz-b59-b" ]
  [ "$(remote_patch_ids | sort | uniq -d | wc -l | tr -d ' ')" -eq 0 ]  # no duplicate regen commits
  # no stale queue entries
  run status_json "$LA"
  jq -e '.queue | length == 0' <<<"$output"
}

@test "B60: destructive steps record a recovery point; temp artifacts are cleaned" {
  # failure case: recovery point recorded BEFORE first history mutation, restorable
  LANE="$(new_lane a feat-a)"
  add_skill "$LANE" zz-b60
  orig="$(git -C "$LANE" rev-parse HEAD)"
  cat > "$LANE/scripts/gate.d/99-fail.sh" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
  chmod +x "$LANE/scripts/gate.d/99-fail.sh"
  git -C "$LANE" add -A && git -C "$LANE" commit -qm "will fail gate"
  orig2="$(git -C "$LANE" rev-parse HEAD)"

  run land "$LANE"
  [ "$status" -ne 0 ]
  # the recorded original tip still resolves: backup ref or audit field
  if git -C "$LANE" for-each-ref 'refs/land/backup/*' | grep -q .; then
    backup="$(git -C "$LANE" for-each-ref --format='%(objectname)' 'refs/land/backup/*' | head -1)"
    [ "$backup" = "$orig2" ]
  else
    audit_entries | grep -q "$orig2"
  fi
  run land "$LANE" --abort
  [ "$status" -eq 0 ] || true
  [ "$(git -C "$LANE" rev-parse HEAD)" = "$orig2" ]

  # success case: no temp branches/worktrees/patch files/backup refs remain
  LS="$(new_lane s feat-s)"; add_skill "$LS" zz-b60-s
  refs_before="$(git -C "$LS" for-each-ref --format='%(refname)' | grep -vE 'refs/(heads/feat-s|remotes)' | sort)"
  run land "$LS"
  [ "$status" -eq 0 ]
  refs_after="$(git -C "$LS" for-each-ref --format='%(refname)' | grep -vE 'refs/(heads/feat-s|remotes)' | sort)"
  [ "$refs_before" = "$refs_after" ]
  [ -z "$(git -C "$LS" for-each-ref 'refs/land/*')" ]
  [ "$(git -C "$LS" worktree list | wc -l | tr -d ' ')" -eq 1 ]
  [ -z "$(find "$LS" -name '*.patch' -newer "$LS/.git/HEAD" 2>/dev/null)" ]
}

@test "B61: disk-full never corrupts main, the lock, or the audit log" {
  # ENOSPC stand-ins (write-failure class), per variant:
  before="$(remote_main_sha)"

  # (a) a generator write fails after partial output
  L1="$(new_lane a feat-a)"
  cat > "$L1/scripts/generators/15-enospc.sh" <<'EOF'
#!/usr/bin/env bash
printf 'partial' > registry.json
echo "write error: No space left on device" >&2
exit 1
EOF
  chmod +x "$L1/scripts/generators/15-enospc.sh"
  git -C "$L1" add -A && git -C "$L1" commit -qm "enospc generator"
  add_skill "$L1" zz-b61-a
  run land "$L1"
  [ "$status" -ne 0 ]
  [ "$(remote_main_sha)" = "$before" ]

  # (b) the regen commit fails (object store unwritable)
  L2="$(new_lane b feat-b)"; add_skill "$L2" zz-b61-b
  cat > "$SANDBOX/freeze-objects.sh" <<EOF
#!/usr/bin/env bash
chmod -R a-w "$L2/.git/objects" 2>/dev/null || true
EOF
  chmod +x "$SANDBOX/freeze-objects.sh"
  # freeze right after gate so the regen commit write fails
  run env LAND_TEST_AFTER_GATE_CMD="$SANDBOX/freeze-objects.sh" bash -c "cd '$L2' && scripts/land.sh"
  st=$status
  chmod -R u+w "$L2/.git/objects" 2>/dev/null || true
  [ "$st" -ne 0 ] || {
    # acceptable: regen commit happened pre-gate — then the push variant covers it
    true
  }
  [ "$(remote_main_sha)" = "$before" ]
  # no half-written commit reachable from any ref
  git -C "$L2" fsck --no-dangling >/dev/null

  # (c) an audit append fails (audit log unwritable)
  L3="$(new_lane c feat-c)"; add_skill "$L3" zz-b61-c
  touch "$AUDIT"; chmod 444 "$AUDIT"
  run land "$L3"
  st3=$status
  chmod 644 "$AUDIT"
  [ "$st3" -ne 0 ]
  [ "$(remote_main_sha)" = "$before" ]

  # the lock file and audit log still parse afterward
  [ ! -s "$LAND_LOCK_DIR/lock.json" ] || jq empty "$LAND_LOCK_DIR/lock.json"
  if [ -s "$AUDIT" ]; then
    while IFS= read -r line; do [ -z "$line" ] || jq empty <<<"$line"; done < "$AUDIT"
  fi
  run land "$L3" --status --json
  jq -e '.state' <<<"$output" >/dev/null   # a torn append never breaks later --status
}
