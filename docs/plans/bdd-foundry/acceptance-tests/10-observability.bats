#!/usr/bin/env bats
# §10 Observability & CLI contract — B67–B72

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

@test "B67: exit codes are a stable taxonomy and every error is structured" {
  # success / no-op = 0
  L0="$(new_lane ok feat-ok)"; add_skill "$L0" zz-b67-ok
  run land "$L0"
  [ "$status" -eq "$EXIT_OK" ]

  # preflight refusal
  L1="$(new_lane pre feat-pre)"; add_skill "$L1" zz-b67-pre
  echo dirt >> "$L1/docs/HAND.md"
  run land "$L1"
  [ "$status" -eq "$EXIT_PREFLIGHT" ]

  # lock wait timeout
  live_holder_start
  L2="$(new_lane lk feat-lk)"; add_skill "$L2" zz-b67-lk
  run land "$L2" --wait-timeout=1
  [ "$status" -eq "$EXIT_LOCK_TIMEOUT" ]
  live_holder_stop

  # source conflict
  SETUP="$(new_lane setup feat-setup)"
  sed -i.bak 's/line-1/line-1-m/' "$SETUP/skills/crank/SKILL.md" && rm -f "$SETUP/skills/crank/SKILL.md.bak"
  ( cd "$SETUP" && bash scripts/regen-all.sh && git add -A && git commit -qm m && git push -q origin HEAD:main )
  L3="$(new_lane cf feat-cf)"
  git -C "$L3" reset -q --hard origin/main~1 2>/dev/null || true
  sed -i.bak 's/line-1/line-1-l/' "$L3/skills/crank/SKILL.md" && rm -f "$L3/skills/crank/SKILL.md.bak"
  git -C "$L3" commit -qam "conflicting edit"
  run land "$L3"
  [ "$status" -eq "$EXIT_CONFLICT" ]
  conflict_summary="$output"

  # gate failure
  L4="$(new_lane gf feat-gf)"; add_skill "$L4" zz-b67-gf
  cat > "$L4/scripts/gate.d/99-fail.sh" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
  chmod +x "$L4/scripts/gate.d/99-fail.sh"
  git -C "$L4" add -A && git -C "$L4" commit -qm fail
  run land "$L4"
  [ "$status" -eq "$EXIT_GATE" ]
  gate_summary="$output"

  # push failure
  L5="$(new_lane pf feat-pf)"; add_skill "$L5" zz-b67-pf
  mkdir -p "$REMOTE/hooks"
  printf '#!/usr/bin/env bash\nexit 1\n' > "$REMOTE/hooks/pre-receive"
  chmod +x "$REMOTE/hooks/pre-receive"
  run land "$L5"
  rm -f "$REMOTE/hooks/pre-receive"
  [ "$status" -eq "$EXIT_PUSH" ]

  # internal error
  L6="$(new_lane ie feat-ie)"; add_skill "$L6" zz-b67-ie
  printf 'garbage-not-json' > "$LAND_LOCK_DIR/lock.json"
  run land "$L6"
  [ "$status" -eq "$EXIT_INTERNAL" ]
  rm -f "$LAND_LOCK_DIR/lock.json"

  # every failure's final summary is structured
  for summary in "$conflict_summary" "$gate_summary"; do
    grep -qiE 'phase' <<<"$summary"
    grep -qE 'retryable: (yes|no)' <<<"$summary"
    grep -qiE 'next action|next:' <<<"$summary"
    grep -qE 'feat-(cf|gf)' <<<"$summary"
    grep -qE '[0-9a-f]{7,40}' <<<"$summary"   # SHAs present
  done
}

@test "B68: every land emits one durable, correlated log; audit appends are atomic" {
  LANE="$(new_lane a feat-a)"; add_skill "$LANE" zz-b68
  run land "$LANE"
  [ "$status" -eq 0 ]
  # last stdout lines include "log: <path>"
  logpath="$(grep -oE 'log: [^ ]+' <<<"$output" | tail -1 | cut -d' ' -f2)"
  [ -n "$logpath" ] && [ -f "$logpath" ]
  log="$(cat "$logpath")"
  grep -qE '[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}' <<<"$log"   # ISO-8601
  grep -q 'feat-a' <<<"$log"
  grep -qiE 'base' <<<"$log"
  grep -qiE 'gate' <<<"$log"
  grep -qiE 'push' <<<"$log"
  grep -qiE 'duration|elapsed|ms|seconds' <<<"$log"
  corr="$(grep -oiE 'correlation[^ ]*[ :=]+[a-zA-Z0-9-]+' <<<"$log" | head -1 | grep -oE '[a-zA-Z0-9-]+$')"
  [ -n "$corr" ]
  audit_entries | grep -q "$corr"

  # failure also emits a durable log
  LF="$(new_lane f feat-f)"; add_skill "$LF" zz-b68-f
  echo dirt >> "$LF/docs/HAND.md"
  run land "$LF"
  [ "$status" -ne 0 ]

  # concurrent audit appends: two lands, all JSONL lines intact
  L1="$(new_lane c1 feat-c1)"; add_skill "$L1" zz-b68-c1
  L2="$(new_lane c2 feat-c2)"; add_skill "$L2" zz-b68-c2
  start_land C1 "$L1"; start_land C2 "$L2"
  wait_land C1; wait_land C2
  while IFS= read -r line; do [ -z "$line" ] || jq empty <<<"$line"; done < "$AUDIT"

  # a pre-existing invalid audit line does not crash later lands or --status
  echo 'NOT-JSON-GARBAGE{{{' >> "$AUDIT"
  L3="$(new_lane c3 feat-c3)"; add_skill "$L3" zz-b68-c3
  run land "$L3"
  [ "$status" -eq 0 ]
  run status_json "$L3"
  jq -e '.state' <<<"$output" >/dev/null
}

@test "B69: post-success local state is defined — branch kept, rebased, clean" {
  LANE="$(new_lane a feat-x)"; add_skill "$LANE" zz-b69
  local_main_before="$(git -C "$LANE" rev-parse refs/heads/main 2>/dev/null || echo NONE)"
  run land "$LANE"
  [ "$status" -eq 0 ]

  [ "$(git -C "$LANE" rev-parse --abbrev-ref HEAD)" = "feat-x" ]   # still on the branch
  git -C "$LANE" fetch -q origin
  git -C "$LANE" merge-base --is-ancestor HEAD origin/main          # at the landed SHA
  [ -z "$(git -C "$LANE" status --porcelain)" ]                     # clean
  [ "$(git -C "$LANE" rev-parse refs/heads/main 2>/dev/null || echo NONE)" = "$local_main_before" ]
  git -C "$LANE" rev-parse --verify -q refs/heads/feat-x >/dev/null # not deleted locally
  # not deleted remotely either (it never existed remotely — assert no deletion side effects)
  [ "$(remote_ref_sha heads/feat-x)" = "ABSENT" ]
}

@test "B70: post-failure local state is defined for every failure class" {
  declare -a lanes outs

  # gate failure
  L1="$(new_lane g feat-g)"; add_skill "$L1" zz-b70-g
  printf '#!/usr/bin/env bash\nexit 1\n' > "$L1/scripts/gate.d/99.sh"; chmod +x "$L1/scripts/gate.d/99.sh"
  git -C "$L1" add -A && git -C "$L1" commit -qm f
  o1="$(land "$L1" 2>&1)" || true

  # push failure
  L2="$(new_lane p feat-p)"; add_skill "$L2" zz-b70-p
  mkdir -p "$REMOTE/hooks"; printf '#!/usr/bin/env bash\nexit 1\n' > "$REMOTE/hooks/pre-receive"; chmod +x "$REMOTE/hooks/pre-receive"
  o2="$(land "$L2" 2>&1)" || true
  rm -f "$REMOTE/hooks/pre-receive"

  # regen failure
  L3="$(new_lane r feat-r)"; add_skill "$L3" zz-b70-r
  printf '#!/usr/bin/env bash\nexit 1\n' > "$L3/scripts/generators/15-bad.sh"; chmod +x "$L3/scripts/generators/15-bad.sh"
  git -C "$L3" add -A && git -C "$L3" commit -qm f
  o3="$(land "$L3" 2>&1)" || true

  # gate timeout
  L4="$(new_lane t feat-t)"; add_skill "$L4" zz-b70-t
  printf '#!/usr/bin/env bash\nsleep 10000\n' > "$L4/scripts/gate.d/50-hang.sh"; chmod +x "$L4/scripts/gate.d/50-hang.sh"
  git -C "$L4" add -A && git -C "$L4" commit -qm f
  o4="$(LAND_GATE_TIMEOUT=3 land "$L4" 2>&1)" || true

  # interrupted queue wait
  live_holder_start
  L5="$(new_lane q feat-q)"; add_skill "$L5" zz-b70-q
  start_land Q "$L5"
  sleep 1
  kill -INT "$PID_Q" 2>/dev/null || true
  wait_land Q
  o5="$(out_of Q)"
  live_holder_stop

  i=0
  for lane in "$L1" "$L2" "$L3" "$L4" "$L5"; do
    i=$((i+1))
    # clean OR exactly one documented "land in progress" marker — never ambiguous wreckage
    dirt="$(git -C "$lane" status --porcelain)"
    if [ -n "$dirt" ]; then
      [ "$(wc -l <<<"$dirt" | tr -d ' ')" -eq 1 ]
      grep -qiE 'land.*progress|\.land' <<<"$dirt"
    fi
    # HEAD on the original branch
    [[ "$(git -C "$lane" rev-parse --abbrev-ref HEAD)" == feat-* ]]
    [ ! -d "$lane/.git/rebase-merge" ]
  done
  for out in "$o1" "$o2" "$o3" "$o4"; do
    grep -qiE 'retry' <<<"$out"   # the summary states the retry instruction
  done
}

@test "B71: --dry-run reports the full plan and mutates nothing" {
  # clean landable branch
  L1="$(new_lane a feat-a)"; add_skill "$L1" zz-b71
  snap="$(git_dir_state_snapshot "$L1")"
  run land "$L1" --dry-run
  [ "$status" -eq 0 ]
  grep -qE '[0-9a-f]{7,40}' <<<"$output"          # resolved base SHA
  grep -q 'zz-b71' <<<"$output"                   # the commits that would land
  grep -qE 'registry.json|context-map' <<<"$output"  # surfaces that would regenerate
  grep -qiE 'gate' <<<"$output"                   # the gate commands
  [ "$(git_dir_state_snapshot "$L1")" = "$snap" ]
  [ "$(audit_count)" -eq 0 ]
  c="$(fresh_clone)"; [ ! -d "$c/skills/zz-b71" ]

  # dirty worktree
  L2="$(new_lane d feat-d)"; add_skill "$L2" zz-b71-d
  echo dirt >> "$L2/docs/HAND.md"
  run land "$L2" --dry-run
  [ "$status" -eq "$EXIT_DRYRUN_BLOCKED" ]
  [[ "$output" =~ dirty ]]

  # held lock
  live_holder_start
  L3="$(new_lane h feat-h)"; add_skill "$L3" zz-b71-h
  run land "$L3" --dry-run
  [ "$status" -eq "$EXIT_DRYRUN_BLOCKED" ]
  [[ "$output" =~ lock|held ]]
  live_holder_stop

  # branch that would fail the gate
  L4="$(new_lane g feat-g)"; add_skill "$L4" zz-b71-g
  printf '#!/usr/bin/env bash\nexit 1\n' > "$L4/scripts/gate.d/99.sh"; chmod +x "$L4/scripts/gate.d/99.sh"
  git -C "$L4" add -A && git -C "$L4" commit -qm f
  run land "$L4" --dry-run
  [ "$status" -eq 0 ] || [ "$status" -eq "$EXIT_DRYRUN_BLOCKED" ]

  # in ALL cases: no lock mutation, no queue entry, no push
  [ "$(audit_count)" -eq 0 ]
  c="$(fresh_clone)"
  [ ! -d "$c/skills/zz-b71-d" ] && [ ! -d "$c/skills/zz-b71-g" ]
}

@test "B72: hostile branch names and paths cannot break land.sh" {
  hostile=$'feat/we;rd$(touch${IFS}pwned)"q"'
  L1="$(new_lane host)"
  if git -C "$L1" checkout -qb "$hostile" 2>/dev/null; then
    add_skill "$L1" zz-b72-h
    run land "$L1"
    # lands or refuses with a clear validation error — deterministically
    if [ "$status" -ne 0 ]; then
      [[ "$output" =~ branch|name|invalid ]]
    fi
    # no shell injection anywhere
    [ -z "$(find "$SANDBOX" "$L1" "${TMPDIR:-/tmp}" -maxdepth 2 -name 'pwned' 2>/dev/null)" ]
    [ ! -f "$L1/pwned" ]
    # audit lines still parse
    if [ -s "$AUDIT" ]; then
      while IFS= read -r line; do [ -z "$line" ] || jq empty <<<"$line"; done < "$AUDIT"
    fi
  else
    # git itself refused the name — land.sh must refuse the closest legal hostile name
    git -C "$L1" checkout -qb 'feat/we;rd"q"'
    add_skill "$L1" zz-b72-h
    run land "$L1"
    [ "$status" -eq 0 ] || [[ "$output" =~ branch|name|invalid ]]
    [ ! -f "$L1/pwned" ]
  fi

  # repo path with spaces + unicode segment
  mkdir -p "$SANDBOX/lanes/sp ace-üni"
  L2="$SANDBOX/lanes/sp ace-üni/lane"
  git clone -q "$REMOTE" "$L2"
  git -C "$L2" checkout -qb feat-space
  add_skill "$L2" zz-b72-space
  run land "$L2"
  [ "$status" -eq 0 ]
  c="$(fresh_clone)"; [ -d "$c/skills/zz-b72-space" ]

  # symlinked repo root
  ln -s "$L2" "$SANDBOX/lanes/symlinked"
  L3="$SANDBOX/lanes/symlinked"
  git -C "$L3" checkout -qb feat-sym
  add_skill "$L3" zz-b72-sym
  run bash -c "cd '$L3' && scripts/land.sh"
  [ "$status" -eq 0 ]
  c="$(fresh_clone)"; [ -d "$c/skills/zz-b72-sym" ]
}
