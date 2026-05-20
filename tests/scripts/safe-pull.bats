#!/usr/bin/env bats
# Regression tests for scripts/safe-pull.sh (soc-x8pl).
#
# Fixture model: a bare "origin" repo + two clones (`local`, `peer`) that
# we mutate to simulate "remote has new commits", "local has tracked drift",
# and the cross-product cases. The script itself only runs inside `local`.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/safe-pull.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  # Bare origin.
  git -C "$TMP" init --bare --quiet --initial-branch=main origin.git

  # `peer` makes the initial commit; we'll add follow-up commits from it
  # later to simulate remote moves.
  git clone --quiet "$TMP/origin.git" "$TMP/peer"
  cd "$TMP/peer"
  git config user.email t@t.test
  git config user.name tester
  echo "v1" > base.txt
  git add base.txt
  git commit --quiet -m "v1"
  git push --quiet -u origin main

  # `local` is where the script runs. Clone after origin has its first commit.
  git clone --quiet "$TMP/origin.git" "$TMP/local"
  git -C "$TMP/local" config user.email t@t.test
  git -C "$TMP/local" config user.name tester
  cd "$ORIG_DIR"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

# Helpers ------------------------------------------------------------------

advance_remote() {
  # Make a new commit on origin/main via the peer clone.
  local file="$1" content="$2"
  cd "$TMP/peer"
  git pull --quiet --ff-only
  echo "$content" > "$file"
  git add "$file"
  git commit --quiet -m "advance $file"
  git push --quiet
  cd "$ORIG_DIR"
}

run_local() {
  cd "$TMP/local"
  run "$SCRIPT" "$@"
}

# Tests --------------------------------------------------------------------

@test "succeeds on a clean tree with no remote changes" {
  run_local
  [ "$status" -eq 0 ]
  [[ "$output" == *"no drift"* ]] || [[ "$output" == *"pulled"* ]]
}

@test "fast-forwards a clean tree to new origin commits" {
  advance_remote remote-add.txt "added"
  run_local
  [ "$status" -eq 0 ]
  [[ "$output" == *"no drift to restore"* ]]
  [ -f "$TMP/local/remote-add.txt" ]
}

@test "stashes tracked drift, pulls, and pops cleanly when no overlap" {
  # Local edits a NEW file; remote also adds a DIFFERENT new file.
  echo "local-only" > "$TMP/local/local-scratch.txt"
  cd "$TMP/local" && git add local-scratch.txt && cd "$ORIG_DIR"
  advance_remote remote-side.txt "remote-only"
  run_local
  [ "$status" -eq 0 ]
  [[ "$output" == *"drift restored cleanly"* ]]
  # Both files present, no orphaned stash.
  [ -f "$TMP/local/local-scratch.txt" ]
  [ -f "$TMP/local/remote-side.txt" ]
  stash_count=$(git -C "$TMP/local" stash list | wc -l | tr -d ' ')
  [ "$stash_count" -eq 0 ]
}

@test "preserves stash and exits 1 when pop produces a conflict" {
  # Local and remote both modify the SAME file at the SAME line → conflict
  # on stash pop.
  cd "$TMP/local"
  echo "local-edit" >> base.txt
  cd "$ORIG_DIR"
  advance_remote base.txt "remote-edit-clobber"
  run_local
  [ "$status" -eq 1 ]
  [[ "$output" == *"conflict"* ]]
  [[ "$output" == *"base.txt"* ]] || [[ "$output" == *"CONFLICT"* ]]
  # Stash remains in the list so the operator can recover.
  stash_count=$(git -C "$TMP/local" stash list | wc -l | tr -d ' ')
  [ "$stash_count" -ge 1 ]
}

@test "exit 2 when pull --ff-only is rejected (local diverged)" {
  # Make a local commit that's not on origin, then advance origin too →
  # ff-only refuses.
  cd "$TMP/local"
  git config user.email t@t.test
  git config user.name tester
  echo "local-fork" > fork.txt
  git add fork.txt
  git commit --quiet -m "local-fork"
  cd "$ORIG_DIR"
  advance_remote remote-fork.txt "remote-fork"
  run_local
  [ "$status" -eq 2 ]
  [[ "$output" == *"pull --ff-only failed"* ]]
}

@test "dry-run reports plan without mutating the tree" {
  echo "drift" > "$TMP/local/drift.txt"
  cd "$TMP/local" && git add drift.txt && cd "$ORIG_DIR"
  advance_remote remote-x.txt "x"
  run_local --dry-run
  [ "$status" -eq 0 ]
  [[ "$output" == *"DRY-RUN"* ]]
  # Remote file should NOT be present (no pull happened).
  [ ! -f "$TMP/local/remote-x.txt" ]
  # Drift still staged.
  staged=$(git -C "$TMP/local" diff --cached --name-only)
  [ "$staged" = "drift.txt" ]
}

@test "--no-pop leaves stash for manual recovery" {
  echo "drift" > "$TMP/local/drift.txt"
  cd "$TMP/local" && git add drift.txt && cd "$ORIG_DIR"
  advance_remote rxx.txt "x"
  run_local --no-pop
  [ "$status" -eq 0 ]
  [[ "$output" == *"left for manual pop"* ]]
  stash_count=$(git -C "$TMP/local" stash list | wc -l | tr -d ' ')
  [ "$stash_count" -eq 1 ]
}

@test "rejects detached HEAD with usage error" {
  cd "$TMP/local"
  local sha
  sha="$(git rev-parse HEAD)"
  git checkout --quiet --detach "$sha"
  cd "$ORIG_DIR"
  run_local
  [ "$status" -eq 3 ]
  [[ "$output" == *"detached"* ]]
}

@test "rejects unknown flag with usage error" {
  run_local --weasel
  [ "$status" -eq 3 ]
  [[ "$output" == *"unknown"* ]]
}

@test "unstaged tracked edits also trigger the stash path" {
  # Modify a tracked file without staging — diff against HEAD should still
  # see the drift.
  cd "$TMP/local"
  echo "unstaged" >> base.txt
  cd "$ORIG_DIR"
  advance_remote sibling.txt "sibling"
  run_local
  [ "$status" -eq 0 ]
  [[ "$output" == *"drift restored cleanly"* ]]
  [ -f "$TMP/local/sibling.txt" ]
}
