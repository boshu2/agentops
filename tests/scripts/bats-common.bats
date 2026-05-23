#!/usr/bin/env bats
# Self-test for lib/bats-common.bash (soc-jhq6) — the shared bats fixture helpers.

setup() {
  source "$(git rev-parse --show-toplevel)/lib/bats-common.bash"
  TMP="$(mktemp -d)"
}

teardown() { rm -rf "$TMP"; }

@test "bats_repo_root returns the git toplevel directory" {
  run bats_repo_root
  [ "$status" -eq 0 ]
  [ -d "$output" ]
  # .git is a dir in a normal checkout, a file in a worktree — -e matches both.
  [ -e "$output/.git" ]
}

@test "bats_init_repo makes a committable repo with an identity" {
  bats_init_repo "$TMP"
  [ "$PWD" = "$TMP" ]
  [ -d "$TMP/.git" ]
  [ "$(git config user.email)" = "bats@test.local" ]
  echo x > f && git add f
  run git commit -qm "commit works without prompting"
  [ "$status" -eq 0 ]
}

@test "bats_init_repo fails clearly on a missing dir arg" {
  run bats_init_repo
  [ "$status" -ne 0 ]
}

@test "bats_stub_bin creates an executable stub that runs" {
  bats_stub_bin "$TMP/bin" faketool 'echo stubbed-output; exit 0'
  [ -x "$TMP/bin/faketool" ]
  run "$TMP/bin/faketool"
  [ "$status" -eq 0 ]
  [ "$output" = "stubbed-output" ]
}

@test "bats_stub_bin honors a non-zero exit body" {
  bats_stub_bin "$TMP/bin" failtool 'exit 3'
  run "$TMP/bin/failtool"
  [ "$status" -eq 3 ]
}
