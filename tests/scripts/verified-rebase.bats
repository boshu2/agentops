#!/usr/bin/env bats
# L2 tests for scripts/verified-rebase.sh (soc-e9n6) — proves the post-condition
# checks that catch the `git rebase --continue` silent-failure pathology.

setup() {
  source "$(git rev-parse --show-toplevel)/lib/bats-common.bash"
  REPO_ROOT="$(bats_repo_root)"
  SCRIPT="$REPO_ROOT/scripts/verified-rebase.sh"
  TMP="$(mktemp -d)"
  bats_init_repo "$TMP"   # cd + git init + deterministic identity
}

teardown() { rm -rf "$TMP"; }

@test "verified-rebase: missing arg exits 2 with usage" {
  run bash "$SCRIPT"
  [ "$status" -eq 2 ]
  [[ "$output" == *"usage"* ]]
}

@test "verified-rebase: passes when no rebase and HEAD matches expected" {
  echo a > f && git add f && git commit -qm "the head commit"
  run bash "$SCRIPT" "the head commit"
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
}

@test "verified-rebase: FAILS when HEAD subject does not match (dropped-commit catch)" {
  echo a > f && git add f && git commit -qm "actual subject"
  run bash "$SCRIPT" "expected subject"
  [ "$status" -eq 1 ]
  [[ "$output" == *"!= expected"* ]]
}

@test "verified-rebase: completes + verifies a real rebase --continue with a conflict" {
  echo a > f && git add f && git commit -qm base
  base="$(git rev-parse --abbrev-ref HEAD)"
  git checkout -q -b topic
  echo topic-change > f && git add f && git commit -qm "topic work"
  git checkout -q "$base"
  echo base-change > f && git add f && git commit -qm "base work"
  git checkout -q topic
  # Rebase topic onto base → conflict on f.
  run git rebase "$base"
  [ "$status" -ne 0 ]   # conflict expected
  # Resolve the conflict, then verified-rebase continues + verifies HEAD.
  echo resolved > f && git add f
  run bash "$SCRIPT" "topic work"
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
  # And the rebase is genuinely finished.
  [ ! -d "$(git rev-parse --git-dir)/rebase-merge" ]
}
