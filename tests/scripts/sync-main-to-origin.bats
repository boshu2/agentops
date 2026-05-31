#!/usr/bin/env bats
# Regression tests for skills/evolve/scripts/sync-main-to-origin.sh (ag-6jt).
#
# The evolve-cron-rpi discovery loop diffed candidate slices against a STALE
# local `main` (the rpi worktree's local main lags origin/main), so work already
# merged to origin/main read as "open" and was re-seeded. These tests pin the
# fix: discovery's diff base must be origin/main AFTER a fetch, and local main
# must be fast-forwarded to it — never forced past a divergence.
#
# Hermetic: a throwaway bare repo plays "origin"; we advance origin/main beyond
# the local clone's main, then assert the script resolves the base to origin's
# tip and fast-forwards local main to match.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/skills/evolve/scripts/sync-main-to-origin.sh"
  TMP="$(mktemp -d)"

  export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t \
         GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t

  # Bare "origin" remote.
  git init -q --bare "$TMP/origin.git"

  # A seed clone we use to publish commits to origin/main.
  git clone -q "$TMP/origin.git" "$TMP/seed"
  (
    cd "$TMP/seed"
    git checkout -q -b main
    echo a > f.txt && git add f.txt && git commit -q -m "base commit"
    git push -q origin main
  )

  # The "worktree" clone whose local main will go STALE.
  git clone -q "$TMP/origin.git" "$TMP/work"
  ( cd "$TMP/work" && git checkout -q main )
  STALE_LOCAL_MAIN="$(cd "$TMP/work" && git rev-parse main)"

  # Advance origin/main beyond the work clone's local main (simulates work
  # merged to origin/main after the worktree last synced).
  (
    cd "$TMP/seed"
    echo b >> f.txt && git commit -q -am "merged-after-stale commit"
    git push -q origin main
  )
  NEW_ORIGIN_MAIN="$(cd "$TMP/seed" && git rev-parse main)"
}

teardown() {
  rm -rf "$TMP"
}

@test "DIFF_BASE is origin/main's tip after fetch, NOT the stale local main" {
  # main is not the checked-out branch in the work clone.
  ( cd "$TMP/work" && git checkout -q -b feature )
  run bash -c "cd '$TMP/work' && '$SCRIPT'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"DIFF_BASE: origin/main $NEW_ORIGIN_MAIN"* ]]
  # The base must be the fresh origin tip, never the stale local main.
  [[ "$output" != *"$STALE_LOCAL_MAIN"* ]]
}

@test "local main is fast-forwarded to origin/main when main is not checked out" {
  ( cd "$TMP/work" && git checkout -q -b feature )
  run bash -c "cd '$TMP/work' && '$SCRIPT'"
  [ "$status" -eq 0 ]
  local after
  after="$(cd "$TMP/work" && git rev-parse main)"
  [ "$after" = "$NEW_ORIGIN_MAIN" ]
  [ "$after" != "$STALE_LOCAL_MAIN" ]
}

@test "local main is fast-forwarded when main IS the checked-out branch" {
  # work clone is on main (stale) before the sync.
  run bash -c "cd '$TMP/work' && '$SCRIPT'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"DIFF_BASE: origin/main $NEW_ORIGIN_MAIN"* ]]
  local after
  after="$(cd "$TMP/work" && git rev-parse HEAD)"
  [ "$after" = "$NEW_ORIGIN_MAIN" ]
}

@test "a diverged local main is refused, never force-moved" {
  # Create a divergent local main commit that is NOT an ancestor of origin/main.
  (
    cd "$TMP/work"
    git checkout -q main
    echo divergent > d.txt && git add d.txt && git commit -q -m "local-only divergent commit"
  )
  local diverged
  diverged="$(cd "$TMP/work" && git rev-parse main)"
  run bash -c "cd '$TMP/work' && '$SCRIPT'"
  [ "$status" -ne 0 ]
  [[ "$output" == *"fast-forward"* ]] || [[ "$output" == *"diverged"* ]]
  # Local main must be untouched (no force/reset).
  local after
  after="$(cd "$TMP/work" && git rev-parse main)"
  [ "$after" = "$diverged" ]
}

@test "respects a custom remote name via SYNC_MAIN_REMOTE" {
  ( cd "$TMP/work" && git checkout -q -b feature && git remote rename origin upstream )
  run bash -c "cd '$TMP/work' && SYNC_MAIN_REMOTE=upstream '$SCRIPT'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"DIFF_BASE: upstream/main $NEW_ORIGIN_MAIN"* ]]
}
