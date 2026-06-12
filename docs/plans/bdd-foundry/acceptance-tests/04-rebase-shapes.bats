#!/usr/bin/env bats
# §4 Rebase & branch shapes — B35–B41

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

advance_main() { # skill-name → push one out-of-band commit to the remote
  local d; d="$(mktemp -d "$SANDBOX/adv.XXXX")"
  git clone -q "$REMOTE" "$d/c"
  ( cd "$d/c" \
    && mkdir -p "skills/$1" && printf -- '---\nname: %s\n---\n' "$1" > "skills/$1/SKILL.md" \
    && bash scripts/regen-all.sh && git add -A && git commit -qm "advance: $1" \
    && git push -q origin main )
}

@test "B35: rebase base comes from a post-acquisition fetch; fetch failure fails closed" {
  export LAND_WAIT_TIMEOUT=60
  live_holder_start
  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b35
  start_land B "$LANE_B"
  sleep 1
  advance_main zz-adv-1; advance_main zz-adv-2; advance_main zz-adv-3
  final_tip="$(remote_main_sha)"
  live_holder_stop
  wait_land B
  [ "$ST_B" -eq 0 ]
  # recorded base SHA equals the remote tip at acquisition time (post-advances)
  grep -q "$final_tip" "$SANDBOX/out/B"

  # fetch failure fails closed
  LANE_C="$(new_lane c feat-c)"; add_skill "$LANE_C" zz-b35c
  orig="$(git -C "$LANE_C" rev-parse HEAD)"
  mv "$REMOTE" "$REMOTE.away"
  run land "$LANE_C"
  st=$status; out="$output"
  mv "$REMOTE.away" "$REMOTE"
  [ "$st" -ne 0 ]
  [[ "$out" == *"cannot determine base"* ]]
  [ "$(git -C "$LANE_C" rev-parse HEAD)" = "$orig" ]
  run land "$LANE_C" --status
  [[ "$output" == *unheld* ]]
  c="$(fresh_clone)"; [ ! -d "$c/skills/zz-b35c" ]
}

@test "B36: nothing-to-land shapes exit fast and mutate nothing" {
  before="$(remote_main_sha)"

  # 0 ahead / 0 behind
  L0="$(new_lane z0 feat-z0)"
  t0=$(date +%s); run land "$L0"; t1=$(date +%s)
  [ "$status" -eq 0 ]; [[ "$output" == *"nothing to land"* ]]; [ $((t1-t0)) -lt 10 ]
  [ "$(remote_main_sha)" = "$before" ]

  # 0 ahead / N behind
  advance_main zz-b36-adv
  LB="$(new_lane zb feat-zb)"   # cloned BEFORE advance? clone now then rewind:
  git -C "$LB" reset -q --hard "$before"
  run land "$LB"
  [ "$status" -eq 0 ]; [[ "$output" == *"nothing to land"* ]]
  [ "$(remote_main_sha)" != "$before" ]   # advance still there, untouched

  # landable branch + diverged LOCAL main: origin/main is authoritative,
  # local main never mutated
  LD="$(new_lane zd feat-zd)"; add_skill "$LD" zz-b36-d
  git -C "$LD" branch -f main HEAD   # local main diverges from origin/main
  local_main_before="$(git -C "$LD" rev-parse refs/heads/main)"
  run land "$LD"
  [ "$status" -eq 0 ]
  [ "$(git -C "$LD" rev-parse refs/heads/main)" = "$local_main_before" ]
  c="$(fresh_clone)"; [ -d "$c/skills/zz-b36-d" ]
}

@test "B37: already-landed is patch-aware; partial lands are completed, not duplicated" {
  # cherry-picked commit: same patch-id, different SHA
  LP="$(new_lane p feat-p)"; add_skill "$LP" zz-b37-p
  d="$(mktemp -d "$SANDBOX/cp.XXXX")"; git clone -q "$REMOTE" "$d/c"
  git -C "$d/c" fetch -q "$LP" feat-p
  git -C "$d/c" cherry-pick -q FETCH_HEAD
  ( cd "$d/c" && bash scripts/regen-all.sh && git add -A && git commit -q --amend --no-edit && git push -q origin main )
  before="$(remote_main_sha)"
  run land "$LP"
  [ "$status" -eq 0 ]; [[ "$output" == *"already landed"* ]]
  [ "$(remote_main_sha)" = "$before" ]

  # partial: 2 commits, first's patch already on main → exactly the missing one lands
  LQ="$(new_lane q feat-q)"
  add_skill "$LQ" zz-b37-q1
  add_skill "$LQ" zz-b37-q2
  d2="$(mktemp -d "$SANDBOX/cp2.XXXX")"; git clone -q "$REMOTE" "$d2/c"
  git -C "$d2/c" fetch -q "$LQ" feat-q
  git -C "$d2/c" cherry-pick -q FETCH_HEAD~1
  ( cd "$d2/c" && git push -q origin main )
  run land "$LQ"
  [ "$status" -eq 0 ]
  c="$(fresh_clone)"
  [ -d "$c/skills/zz-b37-q1" ] && [ -d "$c/skills/zz-b37-q2" ]
  [ "$(remote_patch_ids | sort | uniq -d | wc -l | tr -d ' ')" -eq 0 ]
}

@test "B38: branch shapes that would denormalize main are handled explicitly" {
  # merge commit
  LM="$(new_lane m feat-m)"
  add_skill "$LM" zz-b38-m1
  git -C "$LM" checkout -qb side main 2>/dev/null || git -C "$LM" checkout -qb side origin/main
  add_skill "$LM" zz-b38-m2
  git -C "$LM" checkout -q feat-m
  git -C "$LM" merge -q --no-ff -m "merge side" side
  run land "$LM"
  if [ "$status" -eq 0 ]; then
    c="$(fresh_clone)"
    [ -z "$(git -C "$c" log --merges --format=%H origin/main)" ]   # flattened
  else
    [[ "$output" == *"merge commits not supported"* ]]
  fi

  # empty commit
  LE="$(new_lane e feat-e)"
  git -C "$LE" commit -q --allow-empty -m "empty"
  run land "$LE"
  c="$(fresh_clone)"
  [ -z "$(git -C "$c" log --merges --format=%H origin/main)" ]

  # revert commit
  LR="$(new_lane r feat-r)"
  add_skill "$LR" zz-b38-r
  git -C "$LR" revert -q --no-edit HEAD
  run land "$LR"
  c="$(fresh_clone)"
  [ -z "$(git -C "$c" log --merges --format=%H origin/main)" ]   # strictly linear
}

@test "B39: a lane that becomes conflicted only after its predecessor lands" {
  export LAND_WAIT_TIMEOUT=60
  LANE_A="$(new_lane a feat-a)"
  sed -i.bak 's/line-2/line-2-A/' "$LANE_A/skills/crank/SKILL.md" && rm -f "$LANE_A/skills/crank/SKILL.md.bak"
  git -C "$LANE_A" commit -qam "A edit"
  LANE_B="$(new_lane b feat-b)"
  sed -i.bak 's/line-2/line-2-B/' "$LANE_B/skills/crank/SKILL.md" && rm -f "$LANE_B/skills/crank/SKILL.md.bak"
  git -C "$LANE_B" commit -qam "B edit"
  b_orig="$(git -C "$LANE_B" rev-parse HEAD)"
  LANE_C="$(new_lane c feat-c)"; add_skill "$LANE_C" zz-b39-c

  LAND_TEST_GATE_SLEEP=2 start_land A "$LANE_A"
  wait_until 10 lock_state_is "$LANE_B" held
  start_land B "$LANE_B"; sleep 0.3; start_land C "$LANE_C"
  wait_land A; wait_land B; wait_land C

  [ "$ST_A" -eq 0 ]
  [ "$ST_B" -ne 0 ]   # B aborts cleanly with B15 invariants
  [ ! -d "$LANE_B/.git/rebase-merge" ]
  worktree_clean "$LANE_B"
  [ "$(git -C "$LANE_B" rev-parse HEAD)" = "$b_orig" ]
  [ "$ST_C" -eq 0 ]   # B's failure never wedges the queue
  c="$(fresh_clone)"; [ -d "$c/skills/zz-b39-c" ]
  run land "$LANE_C" --status
  [[ "$output" == *unheld* ]]
}

@test "B40: conflicts are classified — modify/delete, rename/rename, binary" {
  # seed extra files for the fixtures
  SETUP="$(new_lane setup feat-setup)"
  echo "to-delete" > "$SETUP/docs/MODDEL.md"
  echo "to-rename" > "$SETUP/docs/RENAME.md"
  printf '\x00\x01\x02\x03BINARY' > "$SETUP/docs/blob.bin"
  ( cd "$SETUP" && git add -A && git commit -qm "seed extra files" && git push -q origin HEAD:main )

  # modify/delete: branch edits a file main deleted
  D1="$(mktemp -d "$SANDBOX/m.XXXX")"; git clone -q "$REMOTE" "$D1/c"
  ( cd "$D1/c" && git rm -q docs/MODDEL.md && git commit -qm "delete on main" && git push -q origin main )
  L1="$(new_lane md feat-md)"
  git -C "$L1" reset -q --hard origin/main~1 2>/dev/null || true
  echo "edited" >> "$L1/docs/MODDEL.md"; git -C "$L1" commit -qam "edit deleted file"
  run land "$L1"
  [ "$status" -ne 0 ]
  [[ "$output" == *"docs/MODDEL.md"* ]] && [[ "$output" == *"modify/delete"* ]]
  [ ! -d "$L1/.git/rebase-merge" ]; worktree_clean "$L1"

  # rename/rename
  D2="$(mktemp -d "$SANDBOX/r.XXXX")"; git clone -q "$REMOTE" "$D2/c"
  ( cd "$D2/c" && git mv docs/RENAME.md docs/RENAME-main.md && git commit -qm "rename on main" && git push -q origin main )
  L2="$(new_lane rr feat-rr)"
  git -C "$L2" reset -q --hard origin/main~1
  git -C "$L2" mv docs/RENAME.md docs/RENAME-lane.md; git -C "$L2" commit -qm "rename on lane"
  run land "$L2"
  [ "$status" -ne 0 ]
  [[ "$output" == *"rename/rename"* ]]
  worktree_clean "$L2"

  # binary
  D3="$(mktemp -d "$SANDBOX/b.XXXX")"; git clone -q "$REMOTE" "$D3/c"
  ( cd "$D3/c" && printf '\x00\x09MAIN' > docs/blob.bin && git commit -qam "binary on main" && git push -q origin main )
  L3="$(new_lane bin feat-bin)"
  git -C "$L3" reset -q --hard origin/main~1
  printf '\x00\x08LANE' > "$L3/docs/blob.bin"; git -C "$L3" commit -qam "binary on lane"
  run land "$L3"
  [ "$status" -ne 0 ]
  [[ "$output" == *"docs/blob.bin"* ]] && [[ "$output" == *"binary"* ]]
  worktree_clean "$L3"
}

@test "B41: land.sh is provably non-interactive and leaves git config untouched" {
  LANE="$(new_lane a feat-x)"; add_skill "$LANE" zz-b41
  # hostile config + prompting hook + tripwire editor
  git -C "$LANE" config rerere.enabled true
  git -C "$LANE" config rebase.autoStash true
  mkdir -p "$LANE/.git/hooks"
  cat > "$LANE/.git/hooks/commit-msg" <<'EOF'
#!/usr/bin/env bash
if [ -t 0 ]; then read -r -p "interactive? " _; fi
exit 0
EOF
  chmod +x "$LANE/.git/hooks/commit-msg"
  cat > "$SANDBOX/editor-tripwire" <<EOF
#!/usr/bin/env bash
touch "$SANDBOX/editor-invoked"
exit 1
EOF
  chmod +x "$SANDBOX/editor-tripwire"

  cfg_before="$(git -C "$LANE" config --list | sort)"
  gcfg_before="$(cat "$GIT_CONFIG_GLOBAL")"

  # success variant — detached from TTY, stdin /dev/null
  run env GIT_EDITOR="$SANDBOX/editor-tripwire" bash -c \
    "cd '$LANE' && scripts/land.sh < /dev/null"
  [ "$status" -eq 0 ]
  [ ! -f "$SANDBOX/editor-invoked" ]

  # autoStash neutralized: dirty tree still refused despite rebase.autoStash
  LD="$(new_lane d feat-d)"; add_skill "$LD" zz-b41d
  git -C "$LD" config rebase.autoStash true
  echo dirt >> "$LD/docs/HAND.md"
  run env GIT_EDITOR="$SANDBOX/editor-tripwire" bash -c \
    "cd '$LD' && scripts/land.sh < /dev/null"
  [ "$status" -ne 0 ]; [[ "$output" =~ working\ tree\ dirty ]]

  # conflict-abort variant: rerere may not silently auto-resolve
  SETUP="$(new_lane setup feat-setup)"
  sed -i.bak 's/line-3/line-3-main/' "$SETUP/skills/crank/SKILL.md" && rm -f "$SETUP/skills/crank/SKILL.md.bak"
  ( cd "$SETUP" && bash scripts/regen-all.sh && git add -A && git commit -qm m && git push -q origin HEAD:main )
  LC="$(new_lane c feat-c)"
  git -C "$LC" config rerere.enabled true
  sed -i.bak 's/line-3/line-3-lane/' "$LC/skills/crank/SKILL.md" && rm -f "$LC/skills/crank/SKILL.md.bak"
  git -C "$LC" commit -qam "lane edit"
  run env GIT_EDITOR="$SANDBOX/editor-tripwire" bash -c \
    "cd '$LC' && scripts/land.sh < /dev/null"
  [ "$status" -ne 0 ]
  [[ "$output" == *"skills/crank/SKILL.md"* ]]
  [ ! -f "$SANDBOX/editor-invoked" ]

  # config bit-identical before/after (repo + global)
  [ "$(git -C "$LANE" config --list | sort)" = "$cfg_before" ]
  [ "$(cat "$GIT_CONFIG_GLOBAL")" = "$gcfg_before" ]
}
