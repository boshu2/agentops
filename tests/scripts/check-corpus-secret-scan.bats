#!/usr/bin/env bats
# ag-y2xy: check-corpus-secret-scan.sh is the git-safety CI backstop. It scans
# only git-TRACKED .agents corpus markdown/JSONL + committed canon projections
# (repo-root .agents is local-only by policy), failing with file:line when a
# credential pattern reaches a committed corpus file. These cases are the
# passing+failing fixture evidence required by the bead's acceptance, exercised
# in a throwaway git repo so they don't touch the real tree.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/check-corpus-secret-scan.sh"
  REPO="$(mktemp -d)"
  cd "$REPO"
  git init -q
  git config user.email t@t.t
  git config user.name t
  mkdir -p .agents/nightly .agents/learnings
}

teardown() {
  rm -rf "$REPO"
}

@test "passes when tracked corpus files are clean" {
  printf '# clean\nThis learning is about caching.\n' > .agents/nightly/clean.md
  git add -A
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ok"* ]]
}

@test "fails with file:line when a tracked corpus file leaks a secret" {
  printf 'leak: AKIAIOSFODNN7EXAMPLE\n' > .agents/nightly/bad.md
  git add -A
  run bash "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *".agents/nightly/bad.md:1:"* ]]
  [[ "$output" == *"FAIL"* ]]
}

@test "does NOT scan untracked local-only corpus files" {
  # .agents/learnings/ is local-only (never git-added) — a secret there must
  # not trip the committed-corpus gate (scope respects check-no-tracked-agents).
  printf 'local secret: AKIAIOSFODNN7EXAMPLE\n' > .agents/learnings/local.md
  # nothing staged/tracked
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
}
