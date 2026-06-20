#!/usr/bin/env bats
# Regression test for scripts/check-release-due.sh — the 'release due' nudge.
# Asserts the commit-count and days-since-tag thresholds, the no-tag fallback,
# and that the script is always non-blocking (exit 0).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-release-due.sh"
  WORK="$(mktemp -d "${TMPDIR:-/tmp}/release-due-XXXXXX")"
  cd "$WORK"
  git init -q
  git config user.email t@t.t
  git config user.name t
  echo a > a && git add a && git commit -q -m "init"
}

teardown() {
  rm -rf "$WORK"
}

@test "no release tag -> first-release-pending, exit 0" {
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"no vX.Y.Z release tag found"* ]]
}

@test "few commits + recent tag -> not due" {
  git tag v1.0.0
  echo b > b && git add b && git commit -q -m "one commit"
  # 1 commit, 0 days, default thresholds (50 commits / 14 days) -> not due
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"release-due: no"* ]]
  [[ "$output" == *"1 commits"* ]]
  [[ "$output" == *"since v1.0.0"* ]]
}

@test "commit threshold crossed -> due (commits reason)" {
  git tag v1.0.0
  echo b > b && git add b && git commit -q -m "two"
  echo c > c && git add c && git commit -q -m "three"
  # 2 commits, threshold lowered to 2 -> due by commits
  RELEASE_DUE_COMMITS=2 run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"release-due: YES"* ]]
  [[ "$output" == *"2 commits ≥ 2"* ]]
}

@test "days threshold crossed via NOW override -> due (days reason)" {
  git tag v1.0.0
  TAG_EPOCH="$(git log -1 --format=%ct v1.0.0)"
  # No new commits; advance "now" 30 days past the tag, days threshold 14 -> due by days
  RELEASE_DUE_COMMITS=99999 RELEASE_DUE_NOW_EPOCH=$((TAG_EPOCH + 30 * 86400)) run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"release-due: YES"* ]]
  [[ "$output" == *"30d ≥ 14d"* ]]
}

@test "picks the highest semver tag, not the most recently created" {
  git tag v1.0.0
  echo b > b && git add b && git commit -q -m "b"
  git tag v2.0.0
  echo c > c && git add c && git commit -q -m "c"
  # v1.1.0 created LAST but lower than v2.0.0 — must report against v2.0.0
  git tag v1.1.0
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"since v2.0.0"* ]]
}

@test "outside a git repository -> benign skip, exit 0 (never breaks a caller)" {
  local nogit
  nogit="$(mktemp -d "${TMPDIR:-/tmp}/release-due-nogit-XXXXXX")"
  run bash -c "cd '$nogit' && bash '$SCRIPT'"
  rm -rf "$nogit"
  [ "$status" -eq 0 ]
  [[ "$output" == *"not a git repository"* ]]
}

@test "malformed numeric env overrides fall back to defaults, exit 0" {
  git tag v1.0.0
  echo b > b && git add b && git commit -q -m "one commit"
  # Non-numeric thresholds and NOW must not crash under set -u; defaults apply.
  RELEASE_DUE_COMMITS=abc RELEASE_DUE_DAYS=xyz RELEASE_DUE_NOW_EPOCH=nope run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  # 1 commit < default 50 and recent tag < default 14d -> not due
  [[ "$output" == *"release-due: no"* ]]
  [[ "$output" == *"thresholds: 50 commits / 14d"* ]]
}

@test "leading-zero thresholds (08/09) are base-10 safe, no arithmetic error" {
  git tag v1.0.0
  echo b > b && git add b && git commit -q -m "two"
  echo c > c && git add c && git commit -q -m "three"
  # "08" must read as 8 (base 10), not invalid octal: 2 commits >= 8 is false -> not due,
  # and crucially stderr carries no "value too great for base" arithmetic error.
  run bash -c "RELEASE_DUE_COMMITS=08 RELEASE_DUE_DAYS=09 bash '$SCRIPT' 2>&1"
  [ "$status" -eq 0 ]
  [[ "$output" != *"value too great for base"* ]]
  [[ "$output" == *"thresholds: 8 commits / 9d"* ]]
}

@test "absurdly huge thresholds do not overflow to negative (no false-positive due)" {
  git tag v1.0.0
  echo b > b && git add b && git commit -q -m "one commit"
  # 20-digit value would overflow bash int64 and wrap negative; norm_uint must
  # reject it and fall back to the default 50 -> 1 commit is NOT due, no negative
  # threshold leaks into the output.
  RELEASE_DUE_COMMITS=99999999999999999999 RELEASE_DUE_DAYS=99999999999999999999 run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"release-due: no"* ]]
  [[ "$output" == *"thresholds: 50 commits / 14d"* ]]
  [[ "$output" != *"-9"* ]]
}
