#!/usr/bin/env bats
# Regression tests for scripts/gc-stale-worktrees.sh (soc-eymp).
#
# Strategy 1 (ancestor) is exercised directly; strategies 2 and 3 (gh PR
# lookup) are exercised via a path-stubbed `gh` so tests don't hit the
# network. Each test builds an isolated repo under $TMP with explicit
# worktree states (merged-ff / fresh / unmerged / detached / dirty).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/gc-stale-worktrees.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  ORIG_PATH="$PATH"

  # Build a fixture repo with a fake "origin/main" we can manipulate.
  mkdir -p "$TMP/origin.git"
  git -C "$TMP/origin.git" init --bare --quiet --initial-branch=main

  git init --quiet --initial-branch=main "$TMP/main"
  cd "$TMP/main"
  git config user.email t@t.test
  git config user.name tester
  git commit --quiet --allow-empty -m "initial"
  git remote add origin "$TMP/origin.git"
  git push --quiet -u origin main
  cd "$ORIG_DIR"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

# Stub `gh` to return a controlled merged-PR list. Args:
#   $1 = newline-joined list of merged head ref names
#   $2 = optional space-separated list of "merged PR numbers" (for `gh pr view`)
stub_gh() {
  local merged_branches="$1" merged_pr_nums="${2:-}"
  mkdir -p "$TMP/bin"
  cat >"$TMP/bin/gh" <<EOF
#!/usr/bin/env bash
# Stubbed gh for gc-stale-worktrees bats.
if [ "\$1" = "pr" ] && [ "\$2" = "list" ]; then
  # Emit one ref per line, matching --json headRefName -q '.[].headRefName'.
  printf '%s\n' $(printf '%q ' "$merged_branches")
  exit 0
fi
if [ "\$1" = "pr" ] && [ "\$2" = "view" ]; then
  pr_num="\$3"
  for n in $merged_pr_nums; do
    if [ "\$pr_num" = "\$n" ]; then
      echo MERGED
      exit 0
    fi
  done
  echo OPEN
  exit 0
fi
exit 0
EOF
  chmod +x "$TMP/bin/gh"
  export PATH="$TMP/bin:$ORIG_PATH"
}

run_script() {
  # Always run from inside the fixture repo so its git context is used.
  cd "$TMP/main"
  run "$SCRIPT" "$@"
}

@test "skips the main checkout itself" {
  run_script
  [ "$status" -eq 0 ]
  [[ "$output" == *"would remove 0"* ]]
}

@test "keeps a fresh branch whose tip is identical to origin/main" {
  cd "$TMP/main"
  git worktree add --quiet -b feat/fresh "$TMP/fresh" origin/main
  run_script
  [ "$status" -eq 0 ]
  [[ "$output" == *"would remove 0"* ]]
  # The fresh worktree must still exist after dry-run.
  [ -d "$TMP/fresh" ]
}

@test "keeps an unmerged branch with unique commits" {
  cd "$TMP/main"
  git worktree add --quiet -b feat/unmerged "$TMP/unmerged" origin/main
  git -C "$TMP/unmerged" commit --quiet --allow-empty -m "wip"
  run_script
  [ "$status" -eq 0 ]
  [[ "$output" == *"would remove 0"* ]]
  [ -d "$TMP/unmerged" ]
}

@test "flags a squash-merged branch as removable (dry run keeps it on disk)" {
  cd "$TMP/main"
  # AgentOps uses squash-merge only: branch keeps its commits; main gets a
  # NEW commit that's not in the branch's history. Strategy 1 (is-ancestor)
  # fails; we simulate strategy 2 via the gh stub.
  git worktree add --quiet -b feat/squashed "$TMP/squashed" origin/main
  git -C "$TMP/squashed" commit --quiet --allow-empty -m "squashed work"
  stub_gh "feat/squashed" ""
  run_script
  [ "$status" -eq 0 ]
  [[ "$output" == *"would remove 1"* ]]
  [[ "$output" == *"$TMP/squashed"* ]]
  [ -d "$TMP/squashed" ]
}

@test "--apply actually removes the merged worktree" {
  cd "$TMP/main"
  git worktree add --quiet -b feat/squashed "$TMP/squashed" origin/main
  git -C "$TMP/squashed" commit --quiet --allow-empty -m "squashed work"
  stub_gh "feat/squashed" ""
  run_script --apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"removed 1"* ]]
  [ ! -d "$TMP/squashed" ]
}

@test "skips a worktree with detached HEAD" {
  cd "$TMP/main"
  local sha
  sha="$(git rev-parse HEAD)"
  git worktree add --quiet --detach "$TMP/detached" "$sha"
  run_script
  [ "$status" -eq 0 ]
  [[ "$output" == *"detached HEAD"* ]]
  [ -d "$TMP/detached" ]
}

@test "skips a worktree with uncommitted tracked changes" {
  cd "$TMP/main"
  git worktree add --quiet -b feat/dirty "$TMP/dirty" origin/main
  git -C "$TMP/dirty" commit --quiet --allow-empty -m "squashed work"
  stub_gh "feat/dirty" ""
  # Mess up the tracked tree before apply.
  echo "wip" > "$TMP/dirty/scratch.txt"
  git -C "$TMP/dirty" add scratch.txt
  run_script --apply
  [ "$status" -eq 0 ]
  [[ "$output" == *"uncommitted changes"* ]]
  [ -d "$TMP/dirty" ]
}

@test "--force removes even when there are uncommitted changes" {
  cd "$TMP/main"
  git worktree add --quiet -b feat/dirty "$TMP/dirty" origin/main
  git -C "$TMP/dirty" commit --quiet --allow-empty -m "squashed work"
  stub_gh "feat/dirty" ""
  echo "wip" > "$TMP/dirty/scratch.txt"
  git -C "$TMP/dirty" add scratch.txt
  run_script --apply --force
  [ "$status" -eq 0 ]
  [[ "$output" == *"removed 1"* ]]
  [ ! -d "$TMP/dirty" ]
}

@test "--json produces a single-line summary record" {
  run_script --json
  [ "$status" -eq 0 ]
  # Should be parseable JSON.
  echo "$output" | jq . >/dev/null
  echo "$output" | jq -e '.ref == "origin/main" and .apply == false' >/dev/null
}

@test "errors out when the configured ref does not exist" {
  run_script --ref origin/does-not-exist
  [ "$status" -eq 1 ]
  [[ "$output" == *"ref not found"* ]]
}

@test "strategy 2: gh PR list catches squash-merged branches" {
  cd "$TMP/main"
  git worktree add --quiet -b feat/squashed "$TMP/squashed" origin/main
  git -C "$TMP/squashed" commit --quiet --allow-empty -m "squashed work"
  # Branch is NOT an ancestor of origin/main (no merge).
  stub_gh "feat/squashed" ""
  run_script
  [ "$status" -eq 0 ]
  [[ "$output" == *"would remove 1"* ]]
  [[ "$output" == *"$TMP/squashed"* ]]
}

@test "strategy 3: pr-<N> alias catches renamed merged branches" {
  cd "$TMP/main"
  git worktree add --quiet -b pr-42 "$TMP/pr-42-wt" origin/main
  git -C "$TMP/pr-42-wt" commit --quiet --allow-empty -m "renamed branch work"
  # Empty merged-list so strategy 2 misses; strategy 3 looks up PR 42 directly.
  stub_gh "" "42"
  run_script
  [ "$status" -eq 0 ]
  [[ "$output" == *"would remove 1"* ]]
  [[ "$output" == *"$TMP/pr-42-wt"* ]]
}

@test "protected branch ($PROTECTED_BRANCH from ref) never gets removed" {
  # Even if main becomes "ancestor of main" (trivially true), it must be kept.
  # We can't easily check this from a side worktree, but the main checkout
  # already covers the same protection; this asserts the no-op outcome.
  run_script
  [ "$status" -eq 0 ]
  [[ "$output" != *"REMOVE: $TMP/main"* ]]
}
