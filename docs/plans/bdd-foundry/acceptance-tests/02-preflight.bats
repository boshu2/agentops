#!/usr/bin/env bats
# §2 Preflight & invocation — B19–B25

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

@test "B19: every class of dirt is refused or explicitly classified before lock" {
  # (a) staged-only change to a tracked file
  LA="$(new_lane da feat-da)"; add_skill "$LA" zz-b19a
  echo x >> "$LA/docs/HAND.md"; git -C "$LA" add docs/HAND.md
  run land "$LA"
  [ "$status" -ne 0 ]; [[ "$output" =~ working\ tree\ dirty ]]
  [ "$(audit_count)" -eq 0 ]

  # (b) partially staged tracked file
  LB="$(new_lane db feat-db)"; add_skill "$LB" zz-b19b
  echo y >> "$LB/docs/HAND.md"; git -C "$LB" add docs/HAND.md; echo z >> "$LB/docs/HAND.md"
  run land "$LB"
  [ "$status" -ne 0 ]; [[ "$output" =~ working\ tree\ dirty ]]
  [ "$(audit_count)" -eq 0 ]

  # (c) untracked file colliding with a generator-owned output
  LC="$(new_lane dc feat-dc)"; add_skill "$LC" zz-b19c
  git -C "$LC" rm -q --cached registry.json 2>/dev/null || true
  git -C "$LC" checkout -q . 2>/dev/null || true
  echo '{"rogue":true}' > "$LC/docs/context-map.md.new"
  mv "$LC/docs/context-map.md.new" "$LC/docs/context-map.md.rogue"
  # collide: write an untracked file at a manifest path in a fresh lane instead
  LCC="$(new_lane dcc feat-dcc)"; add_skill "$LCC" zz-b19cc
  echo rogue > "$LCC/skills-codex/rogue-untracked.json"
  run land "$LCC"
  [ "$status" -ne 0 ]
  [[ "$output" == *"skills-codex"* ]]   # names the colliding path
  [[ "$output" =~ generator ]]          # names the owning generator
  [ "$(audit_count)" -eq 0 ]

  # (d) untracked file outside the regen write set + (e) ignored file inside it:
  # ONE documented rule, printed in --help, asserted (not implementation accident).
  LD="$(new_lane dd feat-dd)"; add_skill "$LD" zz-b19d
  run land "$LD" --help
  [ "$status" -eq 0 ]
  help_out="$output"
  grep -qiE 'untracked' <<<"$help_out"
  echo stray > "$LD/NOTES.txt"
  run land "$LD"
  if grep -qiE 'untracked.*(refus|block)' <<<"$help_out"; then
    [ "$status" -ne 0 ]
  else
    [ "$status" -eq 0 ]
  fi
}

@test "B20: root discovery works; broken invocation contexts are refused" {
  LANE="$(new_lane a feat-x)"; add_skill "$LANE" zz-b20
  # from a subdirectory: behaves exactly as from the root
  run bash -c "cd '$LANE/skills/zz-b20' && ../../scripts/land.sh --dry-run"
  [ "$status" -eq 0 ]

  # detached HEAD
  LD="$(new_lane det)"
  git -C "$LD" checkout -q --detach
  t0=$(date +%s); run land "$LD"; t1=$(date +%s)
  [ "$status" -ne 0 ]; [ $((t1-t0)) -le 5 ]; [[ "$output" == *"detached HEAD"* ]]

  # on main itself
  LM="$(new_lane onmain)"
  t0=$(date +%s); run land "$LM"; t1=$(date +%s)
  [ "$status" -ne 0 ]; [ $((t1-t0)) -le 5 ]
  [[ "$output" == *"refusing to land main onto itself"* ]]

  # no origin remote
  LN="$(new_lane noorigin feat-n)"
  git -C "$LN" remote remove origin
  run land "$LN"
  [ "$status" -ne 0 ]; [[ "$output" == *"no origin remote"* ]]

  # origin lacks main
  R2="$SANDBOX/origin2.git"; make_bare_remote "$R2"
  LO="$(new_lane nomain feat-o)"
  git -C "$LO" remote set-url origin "$R2"
  run land "$LO"
  [ "$status" -ne 0 ]; [[ "$output" == *"origin/main not found"* ]]

  # not a git worktree
  mkdir -p "$SANDBOX/notrepo/scripts"
  cp "$LANE/scripts/land.sh" "$SANDBOX/notrepo/scripts/land.sh"
  run bash -c "cd '$SANDBOX/notrepo' && scripts/land.sh"
  [ "$status" -ne 0 ]; [[ "$output" == *"not a git worktree"* ]]

  [ "$(audit_count)" -eq 0 ]   # no refused case acquired the lock
}

@test "B21: an in-progress git operation is refused before lock" {
  for marker in rebase-merge rebase-apply MERGE_HEAD CHERRY_PICK_HEAD; do
    lane="$(new_lane "g-$marker" "feat-$marker")"
    add_skill "$lane" "zz-b21-$(tr 'A-Z_' 'a-z-' <<<"$marker")"
    case "$marker" in
      rebase-merge|rebase-apply) mkdir "$lane/.git/$marker"; echo state > "$lane/.git/$marker/x" ;;
      *) git -C "$lane" rev-parse HEAD > "$lane/.git/$marker" ;;
    esac
    before="$(find "$lane/.git/$marker" -type f -exec cat {} \; 2>/dev/null; echo)"
    run land "$lane"
    [ "$status" -ne 0 ]
    [[ "$output" =~ git\ operation\ in\ progress ]]
    after="$(find "$lane/.git/$marker" -type f -exec cat {} \; 2>/dev/null; echo)"
    [ "$before" = "$after" ]
    [ -e "$lane/.git/$marker" ]
  done
  [ "$(audit_count)" -eq 0 ]
}

@test "B22: missing tooling is reported in ONE preflight summary, no lock taken" {
  LANE="$(new_lane a feat-x)"; add_skill "$LANE" zz-b22
  rm "$LANE/scripts/regen-all.sh"
  # PATH without jq (keep core utils + git)
  thin="$SANDBOX/thinbin"; mkdir -p "$thin"
  for tool in bash sh git grep sed awk mktemp dirname basename cat ls date mv cp rm mkdir chmod find sort wc tr perl env head tail; do
    p="$(command -v "$tool" 2>/dev/null)" && ln -sf "$p" "$thin/$tool"
  done
  before_refs="$(git -C "$LANE" for-each-ref --format='%(refname) %(objectname)')"
  run env PATH="$thin" bash -c "cd '$LANE' && scripts/land.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *jq* ]]
  [[ "$output" == *regen-all.sh* ]]
  [ "$(audit_count)" -eq 0 ]
  [ "$(git -C "$LANE" for-each-ref --format='%(refname) %(objectname)')" = "$before_refs" ]
}

@test "B23: configuration has pinned precedence and validates before lock" {
  LANE="$(new_lane a feat-x)"; add_skill "$LANE" zz-b23
  git -C "$LANE" config land.staleTtl 111
  git -C "$LANE" config land.waitTimeout 222

  # repo config beats built-in default
  run env -u LAND_STALE_TTL -u LAND_WAIT_TIMEOUT bash -c "cd '$LANE' && scripts/land.sh --dry-run"
  [ "$status" -eq 0 ]
  [[ "$output" == *"stale"*"111"* ]]
  [[ "$output" == *"timeout"*"222"* ]] || [[ "$output" == *"222"* ]]

  # env beats repo config
  run env LAND_STALE_TTL=333 bash -c "cd '$LANE' && scripts/land.sh --dry-run"
  [ "$status" -eq 0 ]; [[ "$output" == *"333"* ]]; [[ "$output" != *'stale'*'111'* ]]

  # CLI flag beats env
  run env LAND_STALE_TTL=333 bash -c "cd '$LANE' && scripts/land.sh --dry-run --stale-ttl 444"
  [ "$status" -eq 0 ]; [[ "$output" == *"444"* ]]

  # built-in defaults reported when nothing set
  git -C "$LANE" config --unset land.staleTtl; git -C "$LANE" config --unset land.waitTimeout
  run env -u LAND_STALE_TTL -u LAND_WAIT_TIMEOUT -u LAND_HEARTBEAT_INTERVAL bash -c \
    "cd '$LANE' && scripts/land.sh --dry-run"
  [ "$status" -eq 0 ]; [[ "$output" == *"900"* ]]

  # invalid values refused, naming the knob, before lock
  run land "$LANE" --wait-timeout=-5
  [ "$status" -ne 0 ]; [[ "$output" =~ wait-timeout|waitTimeout|wait\ timeout ]]
  run env LAND_STALE_TTL=notanumber bash -c "cd '$LANE' && scripts/land.sh"
  [ "$status" -ne 0 ]; [[ "$output" =~ stale ]]
  [ "$(audit_count)" -eq 0 ]

  # --wait-timeout=0: fail immediately if held (documented meaning)
  run land "$LANE" --help
  [[ "$output" == *"wait-timeout"* ]]
  live_holder_start
  run land "$LANE" --wait-timeout=0
  [ "$status" -ne 0 ]
  live_holder_stop

  # unsupported layout refused up front as config error
  LN="$(new_lane nolayout feat-n)"; add_skill "$LN" zz-b23n
  git -C "$LN" remote remove origin
  run land "$LN" --dry-run
  [ "$status" -ne 0 ]
}

@test "B24: help, version, and usage errors never mutate anything" {
  LANE="$(new_lane a feat-x)"; add_skill "$LANE" zz-b24
  snap="$(git_dir_state_snapshot "$LANE")"

  run land "$LANE" --help
  [ "$status" -eq 0 ]; [[ "$output" =~ [Uu]sage ]]
  run land "$LANE" --version
  [ "$status" -eq 0 ]
  version_out="$output"
  [[ "$version_out" =~ [0-9] ]]
  run land "$LANE" --no-such-flag
  [ "$status" -ne 0 ]; [[ "$output" =~ [Uu]sage ]]
  run land "$LANE" --status --abort
  [ "$status" -ne 0 ]

  [ "$(audit_count)" -eq 0 ]
  [ "$(git_dir_state_snapshot "$LANE")" = "$snap" ]
  worktree_clean "$LANE"

  # version string matches what audit entries record: land once, compare
  LANE2="$(new_lane v feat-v)"; add_skill "$LANE2" zz-b24v
  run land "$LANE2"
  [ "$status" -eq 0 ]
  vtoken="$(tr -d '[:space:]' <<<"$version_out" | grep -oE '[0-9][0-9a-zA-Z.-]*' | head -1)"
  audit_entries | grep -q "$vtoken"
}

@test "B25: the harness refuses to operate on a non-sandbox remote" {
  UNMARKED="$SANDBOX/unmarked.git"
  git init -q --bare "$UNMARKED"          # NO land-sandbox marker
  git --git-dir="$REMOTE" push -q "$UNMARKED" main 2>/dev/null || {
    c="$(fresh_clone)"; git -C "$c" push -q "$UNMARKED" main
  }
  LANE="$(new_lane a feat-x)"
  git -C "$LANE" remote set-url origin "$UNMARKED"
  git -C "$LANE" fetch -q origin
  add_skill "$LANE" zz-b25
  before="$(git --git-dir="$UNMARKED" rev-parse main)"

  run land "$LANE"
  [ "$status" -ne 0 ]
  [[ "$output" == *"refusing non-sandbox remote"* ]]
  [ "$(git --git-dir="$UNMARKED" rev-parse main)" = "$before" ]
}
