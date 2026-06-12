#!/usr/bin/env bats
# §9 Guards & protected paths — B62–B66

load helpers

setup()    { sandbox_setup; }
teardown() { sandbox_teardown; }

@test "B62: guard install lifecycle is explicit; an unprotected clone is never silent" {
  LANE="$(new_lane a feat-a)"; add_skill "$LANE" zz-b62
  before="$(remote_main_sha)"

  # fresh clone, discipline never installed: EITHER direct push blocked OR
  # land.sh refuses until --install — one documented behavior, never silent.
  direct_push_blocked=0
  if ! git -C "$LANE" push -q origin HEAD:main 2>/dev/null; then
    direct_push_blocked=1
  else
    # push went through on a naked clone — then land.sh MUST refuse until install
    git --git-dir="$REMOTE" update-ref refs/heads/main "$before"
    run land "$LANE"
    [ "$status" -ne 0 ]
    [[ "$output" =~ install ]]
  fi

  # --install is idempotent and reports version upgrades
  run land "$LANE" --install
  [ "$status" -eq 0 ]
  run land "$LANE" --install
  [ "$status" -eq 0 ]
  [[ "$output" == *"already installed"* ]]

  # stale guard version upgraded in place with a logged version change
  hook="$LANE/.git/hooks/pre-push"
  if [ -f "$hook" ]; then
    sed -i.bak 's/[0-9][0-9a-zA-Z.-]*/0.0.0-stale/' "$hook" 2>/dev/null || true
    printf '\n# land-guard-version: 0.0.0-stale\n' >> "$hook"
    rm -f "$hook.bak"
    run land "$LANE" --install
    [ "$status" -eq 0 ]
    [[ "$output" =~ 0\.0\.0-stale|upgrad ]]
  fi

  # reports the real origin's branch-protection posture when detectable
  run land "$LANE" --install
  [[ "$output" =~ [Pp]rotect|posture|sandbox ]]
}

@test "B63: the land.sh bypass marker cannot be replayed outside a live land" {
  LANE="$(new_lane a feat-a)"
  run land "$LANE" --install
  [ "$status" -eq 0 ]
  add_skill "$LANE" zz-b63
  before="$(remote_main_sha)"

  # land once to capture a plausible marker, then try to replay it
  run land "$LANE"
  [ "$status" -eq 0 ]

  LANE2="$(new_lane b feat-b)"
  run land "$LANE2" --install
  add_skill "$LANE2" zz-b63-replay

  # copy the env marker/token land.sh uses; push WITHOUT holding the lock
  run env LAND_PUSH_TOKEN="replayed-token" LAND_BYPASS=1 LAND_NONCE="deadbeef" \
    git -C "$LANE2" push origin HEAD:main
  [ "$status" -ne 0 ]
  [[ "$output" == *"use scripts/land.sh"* ]]
  c="$(fresh_clone)"; [ ! -d "$c/skills/zz-b63-replay" ]
}

@test "B64: branch-side .gitignore edits cannot blind the checks" {
  LANE="$(new_lane a hide-it)"
  printf 'registry.json\n%s\n_beads/\n' "$LAND_LOCK_DIR" >> "$LANE/.gitignore"
  git -C "$LANE" commit -qam "gitignore the derived surface + lock dir"
  # branch also carries generated drift that must STILL be caught/regenerated
  add_skill "$LANE" zz-b64
  jq '. + {blinded: true}' "$LANE/registry.json" > "$LANE/registry.json.tmp" && mv "$LANE/registry.json.tmp" "$LANE/registry.json"
  git -C "$LANE" add -A -f && git -C "$LANE" commit -qm "drift under ignore cover"

  run land "$LANE"
  [ "$status" -eq 0 ]
  c="$(fresh_clone)"
  ( cd "$c" && bash scripts/regen-all.sh --check )                 # drift still caught/fixed
  ! jq -e '.blinded' "$c/registry.json" >/dev/null 2>&1
  jq -e '.skills[] | select(.name=="zz-b64")' "$c/registry.json" >/dev/null

  # --status still reads the lock despite the ignore edits
  run status_json "$LANE"
  [ "$status" -eq 0 ]
  jq -e '.state' <<<"$output" >/dev/null
}

@test "B65: private _beads paths can never be staged or pushed by land.sh" {
  LANE="$(new_lane a feat-a)"
  mkdir -p "$LANE/_beads"
  echo '{"private":"bead"}' > "$LANE/_beads/issues.jsonl"
  add_skill "$LANE" zz-b65
  echo '{"more":"private"}' > "$LANE/_beads/extra.jsonl"   # untracked dirt under _beads/

  run land "$LANE"
  # ONE documented rule: exempt from the dirty check (lands) or reported
  # separately (refuses naming _beads) — never auto-staged either way.
  if [ "$status" -eq 0 ]; then
    c="$(fresh_clone)"
    [ -d "$c/skills/zz-b65" ]
  else
    [[ "$output" == *"_beads"* ]]
    # fix: drop the dirt, land must then succeed
    rm -rf "$LANE/_beads"
    run land "$LANE"
    [ "$status" -eq 0 ]
  fi

  # no _beads path appears in any commit land.sh created
  c="$(fresh_clone)"
  base="$(git -C "$c" rev-list --max-parents=0 origin/main | head -1)"
  ! git -C "$c" log --stat --format= "origin/main" | grep -q '_beads/'
}

@test "B66: a branch that modifies land.sh itself is handled, not trusted blindly" {
  LANE="$(new_lane a self-mod)"
  echo "# self-modification marker $(date +%s)" >> "$LANE/scripts/land.sh"
  echo "# regen tweak" >> "$LANE/scripts/regen-all.sh"
  git -C "$LANE" commit -qam "self-mod: edit land.sh and regen-all.sh"

  run land "$LANE"
  if [ "$status" -ne 0 ]; then
    # documented policy 1: refuse
    [[ "$output" =~ self-modifying\ land ]]
    [[ "$output" =~ land\ manually|review ]]
    before="$(remote_main_sha)"
    c="$(fresh_clone)"
    ! grep -q 'self-modification marker' "$c/scripts/land.sh"
  else
    # documented policy 2: re-exec the post-rebase version before gating —
    # if landed, the result passes the full gate on a fresh clone (B52)
    [[ "$output" =~ re-exec ]]
    c="$(fresh_clone)"
    grep -q 'self-modification marker' "$c/scripts/land.sh"
    ( cd "$c" && bash scripts/regen-all.sh --check )
  fi
}
