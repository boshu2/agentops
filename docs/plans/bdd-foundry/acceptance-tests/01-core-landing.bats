#!/usr/bin/env bats
# §1 Core landing — B1–B18 (docs/plans/bdd-foundry/behaviors.md, FROZEN)

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

@test "B1: single lane lands with one command and one rebase attempt" {
  LANE="$(new_lane a feat-x)"
  add_skill "$LANE" zz-b1-one
  add_skill "$LANE" zz-b1-two
  c1="$(git -C "$LANE" rev-parse HEAD~1)"
  c2="$(git -C "$LANE" rev-parse HEAD)"

  run land "$LANE"
  [ "$status" -eq 0 ]

  c="$(fresh_clone)"
  git -C "$c" merge-base --is-ancestor "$c1" origin/main
  git -C "$c" merge-base --is-ancestor "$c2" origin/main
  ( cd "$c" && bash scripts/regen-all.sh --check )

  [ "$(grep -c 'rebase attempts: 1' <<<"$output")" -eq 1 ]

  run land "$LANE" --status
  [[ "$output" == *unheld* ]]
}

@test "B2: regen-at-land — author never hand-runs the 8 generators; regen commit carries pinned identity" {
  LANE="$(new_lane a feat-skill)"
  add_skill "$LANE" zz-newskill   # derived surfaces NOT regenerated — stale

  run land "$LANE"
  [ "$status" -eq 0 ]

  c="$(fresh_clone)"
  ( cd "$c" && bash scripts/regen-all.sh --check )
  ( cd "$c" && jq -e '.skills[] | select(.name=="zz-newskill")' registry.json )
  worktree_clean "$c"

  # Folded into the landing: either a land.sh-authored regen commit or squashed.
  regen_commits="$(git -C "$c" log --format='%H %s' origin/main | grep -E '^\S+ chore\(land\): regen derived surfaces' || true)"
  if [ -n "$regen_commits" ]; then
    sha="$(awk '{print $1; exit}' <<<"$regen_commits")"
    [ "$(git -C "$c" show -s --format='%an <%ae>' "$sha")" = "land.sh <land@local>" ]
    [ "$(git -C "$c" show -s --format='%cn <%ce>' "$sha")" = "land.sh <land@local>" ]
    body="$(git -C "$c" show -s --format='%B' "$sha")"
    grep -q '^Landed-by: ' <<<"$body"
    grep -q '^Land-correlation: ' <<<"$body"
    corr="$(sed -n 's/^Land-correlation: //p' <<<"$body")"
    audit_entries | grep -q "$corr"
  else
    # squashed variant: derived updates must be inside the lane's landed commit
    git -C "$c" show --stat origin/main | grep -q 'registry.json'
  fi
}

@test "B3: queued lanes land FIFO with zero manual rebases" {
  # Slow the gate so lane A genuinely holds while B and C queue.
  LANE_A="$(new_lane a feat-a)"; add_skill "$LANE_A" zz-b3-a
  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b3-b
  LANE_C="$(new_lane c feat-c)"; add_skill "$LANE_C" zz-b3-c

  LAND_TEST_GATE_SLEEP=3 start_land A "$LANE_A"
  wait_until 10 lock_state_is "$LANE_B" held
  start_land B "$LANE_B"
  sleep 0.5
  start_land C "$LANE_C"
  wait_land A; wait_land B; wait_land C
  [ "$ST_A" -eq 0 ]; [ "$ST_B" -eq 0 ]; [ "$ST_C" -eq 0 ]

  # FIFO acquisition order A,B,C in the lock audit log.
  order="$(audit_entries | jq -r 'select(.event=="acquire") | .holder.id' | paste -sd, -)"
  [[ "$order" == *feat-a*feat-b*feat-c* ]] || {
    acq="$(audit_entries | grep '"acquire"')"
    a_line="$(grep -n 'lanes/a' <<<"$acq" | head -1 | cut -d: -f1)"
    b_line="$(grep -n 'lanes/b' <<<"$acq" | head -1 | cut -d: -f1)"
    c_line="$(grep -n 'lanes/c' <<<"$acq" | head -1 | cut -d: -f1)"
    [ -n "$a_line" ] && [ -n "$b_line" ] && [ -n "$c_line" ]
    [ "$a_line" -lt "$b_line" ] && [ "$b_line" -lt "$c_line" ]
  }

  ! grep -q 'manual intervention required' "$SANDBOX/out/B"
  ! grep -q 'manual intervention required' "$SANDBOX/out/C"

  c="$(fresh_clone)"
  for s in zz-b3-a zz-b3-b zz-b3-c; do [ -d "$c/skills/$s" ]; done
}

@test "B4: three-lane soak — the tonight-failure-mode does not reproduce" {
  LANE_A="$(new_lane a feat-a)"; add_skill "$LANE_A" zz-b4-a
  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b4-b
  LANE_C="$(new_lane c feat-c)"; add_skill "$LANE_C" zz-b4-c

  start_land A "$LANE_A"; start_land B "$LANE_B"; start_land C "$LANE_C"
  wait_land A; wait_land B; wait_land C
  [ "$ST_A" -eq 0 ]; [ "$ST_B" -eq 0 ]; [ "$ST_C" -eq 0 ]

  c="$(fresh_clone)"
  for s in zz-b4-a zz-b4-b zz-b4-c; do [ -d "$c/skills/$s" ]; done

  total=0
  for t in A B C; do
    n="$(sed -n 's/.*rebase attempts: \([0-9][0-9]*\).*/\1/p' "$SANDBOX/out/$t" | head -1)"
    [ -n "$n" ]
    total=$((total + n))
  done
  [ "$total" -eq 3 ]

  ( cd "$c" && bash scripts/regen-all.sh --check )
  while IFS= read -r -d '' f; do jq empty "$f"; done \
    < <(find "$c/skills-codex" -name '*.json' -print0)
}

@test "B5: lock status is observable for swarm tending" {
  LANE_A="$(new_lane a feat-a)"; add_skill "$LANE_A" zz-b5-a
  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b5-b

  LAND_TEST_GATE_SLEEP=4 start_land A "$LANE_A"
  wait_until 10 lock_state_is "$LANE_B" held
  start_land B "$LANE_B"
  wait_until 10 queue_len_is "$LANE_A" 1

  before="$(cat "$LAND_LOCK_DIR/lock.json")"
  run status_json "$LANE_B"
  [ "$status" -eq 0 ]
  jq -e '.holder.id and .holder.pid and (.holder.heartbeat_age_seconds != null)' <<<"$output"
  jq -e '.queue | length >= 1' <<<"$output"
  jq -e '.queue[0]' <<<"$output" | grep -q 'b\|feat-b'
  # --status acquired/modified nothing
  [ "$(cat "$LAND_LOCK_DIR/lock.json")" = "$before" ] || \
    jq -e '.holder.id' <<<"$(cat "$LAND_LOCK_DIR/lock.json")" >/dev/null
  wait_land A; wait_land B
}

@test "B6: mutual exclusion is mechanical — no two holders, ever (10-lane stress)" {
  for i in $(seq 1 10); do
    lane="$(new_lane "l$i" "feat-l$i")"
    add_skill "$lane" "zz-b6-$i"
    eval "L$i=\$lane"
  done
  for i in $(seq 1 10); do eval "start_land T$i \"\$L$i\""; done
  for i in $(seq 1 10); do wait_land "T$i"; done

  # 10 strictly non-overlapping acquire/release pairs.
  acq="$(audit_entries | jq -r 'select(.event=="acquire") | .ts' | wc -l | tr -d ' ')"
  rel="$(audit_entries | jq -r 'select(.event=="release") | .ts' | wc -l | tr -d ' ')"
  [ "$acq" -eq 10 ]; [ "$rel" -eq 10 ]
  audit_entries | jq -r 'select(.event=="acquire" or .event=="release") | "\(.ts) \(.event)"' \
    | sort | awk '{print $NF}' | paste -sd' ' - \
    | grep -qE '^(acquire release ){9}acquire release$'

  # Linear history, all 10 tips reachable, no merge commits.
  c="$(fresh_clone)"
  [ -z "$(git -C "$c" log --merges --format=%H origin/main)" ]
  for i in $(seq 1 10); do [ -d "$c/skills/zz-b6-$i" ]; done
}

@test "B7: stale lock from a dead lane is reclaimed, not waited on forever" {
  export LAND_STALE_TTL=2
  # PID that no longer exists + heartbeat older than TTL.
  ( sleep 0.1 ) & deadpid=$!
  wait "$deadpid" 2>/dev/null || true
  fabricate_lock "$deadpid" 60

  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b7-b
  start="$(date +%s)"
  run land "$LANE_B"
  [ "$status" -eq 0 ]
  [ $(( $(date +%s) - start )) -le 30 ]

  takeover="$(audit_entries | grep 'stale-takeover')"
  [ -n "$takeover" ]
  grep -q "$deadpid" <<<"$takeover"
  grep -q '/dead/worktree' <<<"$takeover"
  grep -qE 'heartbeat' <<<"$takeover"
}

@test "B8: a live lock is never stolen" {
  live_holder_start
  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b8-b

  run env LAND_WAIT_TIMEOUT=3 bash -c "cd '$LANE_B' && scripts/land.sh"
  # B must not have stolen: holder record still the live holder's.
  jq -e '.nonce == "cafe0001"' "$LAND_LOCK_DIR/lock.json"
  if [ "$status" -ne 0 ]; then
    [[ "$output" =~ lock\ held\ by\ .*(queued|timed\ out) ]] || [[ "$output" == *"timed out"* ]]
  fi
  # No push happened.
  c="$(fresh_clone)"
  [ ! -d "$c/skills/zz-b8-b" ]
  live_holder_stop
}

@test "B9: derived surfaces never present a textual conflict to anyone" {
  # main advances with bb-two-prime landed (out-of-band seed push, derived included)
  SETUP="$(new_lane setup feat-setup)"
  add_skill "$SETUP" bb-two-prime
  ( cd "$SETUP" && bash scripts/regen-all.sh && git add -A && git commit -qm "regen" )
  git -C "$SETUP" push -q origin HEAD:main

  LANE="$(new_lane a feat-a)"
  add_skill "$LANE" aa-one
  ( cd "$LANE" && bash scripts/regen-all.sh && git add -A && git commit -qm "regen (stale soon)" )

  run land "$LANE"
  [ "$status" -eq 0 ]
  ! grep -E 'CONFLICT.*(registry.json|context-map|SKILL-TIERS|skills-codex|COUNTS)' <<<"$output"

  c="$(fresh_clone)"
  ( cd "$c" && bash scripts/regen-all.sh )
  worktree_clean "$c"   # every derived surface byte-identical to fresh generator output
  [ -d "$c/skills/aa-one" ]; [ -d "$c/skills/bb-two-prime" ]
}

@test "B10: re-running land.sh on an already-landed branch is a no-op" {
  LANE="$(new_lane a feat-x)"
  add_skill "$LANE" zz-b10
  run land "$LANE"
  [ "$status" -eq 0 ]

  before="$(remote_main_sha)"
  t0="$(date +%s)"
  run land "$LANE"
  [ "$status" -eq 0 ]
  [[ "$output" == *"already landed"* ]]
  [ "$(remote_main_sha)" = "$before" ]
  [ $(( $(date +%s) - t0 )) -lt 10 ]
}

@test "B11: counts are never hand-asserted — prose counts are generated or gated" {
  LANE="$(new_lane a feat-count)"
  # plant a WRONG value inside the marker block
  perl -0pi -e 's|(<!-- count:skills -->).*?(<!-- /count -->)|${1}999${2}|gs' "$LANE/docs/COUNTS.md"
  git -C "$LANE" commit -qam "plant wrong count"
  add_skill "$LANE" zz-b11

  run land "$LANE"
  [ "$status" -eq 0 ]

  c="$(fresh_clone)"
  expected="$(ls "$c/skills" | wc -l | tr -d ' ')"
  grep -q "<!-- count:skills -->${expected}<!-- /count -->" "$c/docs/COUNTS.md"
  ! grep -q '999' "$c/docs/COUNTS.md"
  # sweep: no numeric skill-count literal outside marker blocks
  run land "$c" --check-counts
  [ "$status" -eq 0 ]
}

@test "B12: out-of-band push between gate and push is absorbed, never force-pushed" {
  # injector: push one out-of-band commit to the remote after the first gate pass
  cat > "$SANDBOX/oob.sh" <<EOF
#!/usr/bin/env bash
set -e
[ -f "$SANDBOX/oob-done" ] && exit 0
touch "$SANDBOX/oob-done"
d="\$(mktemp -d "$SANDBOX/oob.XXXX")"
git clone -q "$REMOTE" "\$d/c"
cd "\$d/c"
mkdir -p skills/zz-oob && printf -- '---\nname: zz-oob\n---\n' > skills/zz-oob/SKILL.md
bash scripts/regen-all.sh
git add -A && git commit -qm "oob: bypassing the lock" && git push -q origin main
git rev-parse HEAD > "$SANDBOX/oob-sha"
EOF
  chmod +x "$SANDBOX/oob.sh"

  LANE="$(new_lane a feat-a)"; add_skill "$LANE" zz-b12
  run env LAND_TEST_AFTER_GATE_CMD="$SANDBOX/oob.sh" bash -c \
    "set -x; cd '$LANE' && scripts/land.sh"
  trace="$output"
  [ "$status" -eq 0 ]
  attempts="$(sed -n 's/.*rebase attempts: \([0-9][0-9]*\).*/\1/p' <<<"$output" | tail -1)"
  [ -n "$attempts" ] && [ "$attempts" -le 2 ]
  ! grep -E 'push .*(--force|--force-with-lease)' <<<"$trace"
  audit_entries | grep -q "$(cat "$SANDBOX/oob-sha")"
  c="$(fresh_clone)"
  [ -d "$c/skills/zz-b12" ] && [ -d "$c/skills/zz-oob" ]
}

@test "B13: all gate failures reported in ONE pass, not one per cycle" {
  LANE="$(new_lane a feat-bad)"
  # (1) registry reference to a nonexistent skill — plant a context_rel-style violation
  mkdir -p "$LANE/skills/zz-broken"
  printf -- '---\nname: zz-broken\ncontext_rel:\n  with: no-such-skill\n---\n' > "$LANE/skills/zz-broken/SKILL.md"
  # (2) codex-twin hash mismatch: edit a skill, hand-set a stale twin
  echo "edited" >> "$LANE/skills/seed-skill/SKILL.md"
  printf '{"generated_hash": "0000000000000000000000000000000000000000"}\n' \
    > "$LANE/skills-codex/seed-skill/.agentops-generated.json"
  # (3) doc-table reference to a nonexistent skill
  echo "| /skills/ghost-skill | broken ref |" >> "$LANE/docs/HAND.md"
  # plus gate.d checks that catch (1) and (3)
  cat > "$LANE/scripts/gate.d/10-context-rel.sh" <<'EOF'
#!/usr/bin/env bash
bad=0
for f in skills/*/SKILL.md; do
  t="$(sed -n 's/^  with: //p' "$f")"
  [ -n "$t" ] && [ ! -d "skills/$t" ] && { echo "context-rel: $f -> missing skill $t (remedy: fix target)"; bad=1; }
done
exit $bad
EOF
  cat > "$LANE/scripts/gate.d/20-doc-refs.sh" <<'EOF'
#!/usr/bin/env bash
bad=0
while read -r ref; do
  s="${ref#/skills/}"
  [ -d "skills/$s" ] || { echo "doc-ref: docs/HAND.md -> missing skill $s (remedy: fix table)"; bad=1; }
done < <(grep -o '/skills/[a-z0-9-]*' docs/HAND.md || true)
exit $bad
EOF
  chmod +x "$LANE"/scripts/gate.d/*.sh
  git -C "$LANE" add -A && git -C "$LANE" commit -qm "feat-bad: three violations"

  before="$(remote_main_sha)"
  run land "$LANE"
  [ "$status" -ne 0 ]
  [ "$(grep -c '== gate ==' <<<"$output")" -eq 1 ]
  grep -q 'no-such-skill' <<<"$output"
  grep -qE 'seed-skill|generated_hash|hash mismatch|drift' <<<"$output"
  grep -q 'ghost-skill' <<<"$output"
  grep -qE 'remedy' <<<"$output"
  [ "$(remote_main_sha)" = "$before" ]
  run land "$LANE" --status
  [[ "$output" == *unheld* ]]
}

@test "B14: dirty working tree is refused before any lock or rebase" {
  LANE="$(new_lane a feat-x)"
  add_skill "$LANE" zz-b14
  echo "dirty" >> "$LANE/docs/HAND.md"
  snap_before="$(git_dir_state_snapshot "$LANE")$(git -C "$LANE" status --porcelain)"

  t0="$(date +%s)"
  run land "$LANE"
  [ "$status" -ne 0 ]
  [ $(( $(date +%s) - t0 )) -le 5 ]
  [[ "$output" =~ working\ tree\ dirty ]]
  [ "$(audit_count)" -eq 0 ]
  snap_after="$(git_dir_state_snapshot "$LANE")$(git -C "$LANE" status --porcelain)"
  [ "$snap_before" = "$snap_after" ]
}

@test "B15: a real source conflict aborts cleanly and reports, leaving no wreckage" {
  # main advances with a conflicting edit to the SAME hunk
  SETUP="$(new_lane setup feat-setup)"
  sed -i.bak 's/line-2/line-2-main/' "$SETUP/skills/crank/SKILL.md" && rm -f "$SETUP/skills/crank/SKILL.md.bak"
  ( cd "$SETUP" && bash scripts/regen-all.sh && git add -A && git commit -qm "main edit" && git push -q origin HEAD:main )

  LANE="$(new_lane a feat-y)"
  sed -i.bak 's/line-2/line-2-lane/' "$LANE/skills/crank/SKILL.md" && rm -f "$LANE/skills/crank/SKILL.md.bak"
  git -C "$LANE" commit -qam "lane edit"
  orig="$(git -C "$LANE" rev-parse HEAD)"
  before="$(remote_main_sha)"

  run land "$LANE"
  [ "$status" -ne 0 ]
  [ ! -d "$LANE/.git/rebase-merge" ] && [ ! -d "$LANE/.git/rebase-apply" ]
  worktree_clean "$LANE"
  [ "$(git -C "$LANE" rev-parse --abbrev-ref HEAD)" = "feat-y" ]
  [ "$(git -C "$LANE" rev-parse HEAD)" = "$orig" ]
  [[ "$output" == *"skills/crank/SKILL.md"* ]]
  [ "$(remote_main_sha)" = "$before" ]
  run land "$LANE" --status
  [[ "$output" == *unheld* ]]
}

@test "B16: hash-marker JSON corruption can never land (duplicate generated_hash class)" {
  # Fixture reproducing the 2026-06-12 splice: branch and main both rewrite the
  # same twin file so a plain line-merge would duplicate the key.
  SETUP="$(new_lane setup feat-setup)"
  echo "main-side edit" >> "$SETUP/skills/seed-skill/SKILL.md"
  ( cd "$SETUP" && bash scripts/regen-all.sh && git add -A && git commit -qm "main edit + regen" && git push -q origin HEAD:main )

  LANE="$(new_lane a feat-dup)"
  echo "lane-side edit" >> "$LANE/skills/seed-skill/SKILL.md"
  ( cd "$LANE" && bash scripts/regen-all.sh && git add -A && git commit -qm "lane edit + regen" )

  run land "$LANE"
  [ "$status" -eq 0 ]

  c="$(fresh_clone)"
  while IFS= read -r -d '' f; do
    jq empty "$f"
    [ "$(grep -c '"generated_hash"' "$f")" -le 1 ]
  done < <(find "$c/skills-codex" -name '.agentops-generated.json' -print0)
  jq empty "$c/skills-codex/.agentops-manifest.json"
}

@test "B17: direct pushes to main outside land.sh are mechanically blocked" {
  LANE="$(new_lane a feat-direct)"
  run land "$LANE" --install
  [ "$status" -eq 0 ]
  add_skill "$LANE" zz-b17

  run git -C "$LANE" push origin HEAD:main
  [ "$status" -ne 0 ]
  [[ "$output$stderr" == *"use scripts/land.sh"* ]] || \
    git -C "$LANE" push origin HEAD:main 2>&1 | grep -q "use scripts/land.sh" || false
  c="$(fresh_clone)"; [ ! -d "$c/skills/zz-b17" ]

  # land.sh itself can push through the guard
  run land "$LANE"
  [ "$status" -eq 0 ]
  c="$(fresh_clone)"; [ -d "$c/skills/zz-b17" ]
}

@test "B18: gate failure or crash mid-land never strands the queue" {
  export LAND_STALE_TTL=2
  LANE_A="$(new_lane a feat-a)"; add_skill "$LANE_A" zz-b18-a
  LANE_B="$(new_lane b feat-b)"; add_skill "$LANE_B" zz-b18-b

  # A acquires then is SIGKILLed before pushing (crash seam after gate).
  LAND_TEST_CRASH_AFTER=gate start_land A "$LANE_A"
  wait_land A
  [ "$ST_A" -ne 0 ]

  sleep 3   # exceed stale TTL
  run land "$LANE_B"
  [ "$status" -eq 0 ]

  c="$(fresh_clone)"
  [ -d "$c/skills/zz-b18-b" ]
  [ ! -d "$c/skills/zz-b18-a" ]   # A's partial work never half-landed

  # A is recoverable: --abort clears any marker, then A re-lands cleanly.
  ( cd "$LANE_A" && scripts/land.sh --abort ) || true
  run land "$LANE_A"
  [ "$status" -eq 0 ]
  c="$(fresh_clone)"; [ -d "$c/skills/zz-b18-a" ]
}
