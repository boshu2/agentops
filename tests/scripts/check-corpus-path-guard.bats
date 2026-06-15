#!/usr/bin/env bats
# ag-ao0eo (epic ag-k7tq9): check-corpus-path-guard.sh is the fail-closed pre-push
# PATH guard — a private artifact (.agents/learnings/, _beads/, untraceable
# docs/wiki/) must never be committed/pushed to the PUBLIC boshu2/agentops repo.
# These L2 cases exercise the real range logic in a throwaway git repo: a "public"
# base commit, then an offending or clean change on a branch. We point the guard's
# base ref at the base commit (CORPUS_PATH_GUARD_BASE) so origin/main..HEAD is
# simulated without a remote.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/check-corpus-path-guard.sh"
  REPO="$(mktemp -d)"
  cd "$REPO"
  git init -q
  git config user.email t@t.t
  git config user.name t
  # Base commit standing in for public origin/main.
  mkdir -p cli docs
  printf 'package main\n' > cli/main.go
  git add -A
  git commit -qm "base"
  BASE="$(git rev-parse HEAD)"
  export CORPUS_PATH_GUARD_BASE="$BASE"
}

teardown() {
  rm -rf "$REPO"
}

@test "FAILS when a committed file lands under .agents/learnings/" {
  mkdir -p .agents/learnings
  printf 'private learning\n' > .agents/learnings/leak.md
  git add -A && git commit -qm "add private learning"
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *".agents/learnings/leak.md"* ]]
}

@test "FAILS when a committed file lands under _beads/" {
  mkdir -p _beads
  printf '{}\n' > _beads/issues.jsonl
  git add -A && git commit -qm "add private tracker"
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"_beads/issues.jsonl"* ]]
}

@test "FAILS when a committed file lands under docs/wiki/ (untraceable until S5)" {
  mkdir -p docs/wiki
  printf '# wiki\n' > docs/wiki/page.md
  git add -A && git commit -qm "add wiki page"
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"docs/wiki/page.md"* ]]
}

@test "FAILS when a private path is only STAGED (not yet committed)" {
  mkdir -p .agents/learnings
  printf 'private\n' > .agents/learnings/staged.md
  git add -A
  # no commit — staged-only must still be caught (fail closed both ways)
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *".agents/learnings/staged.md"* ]]
}

@test "FAILS closed when base ref is missing and a forbidden path is in an EARLIER commit (Navi regression)" {
  # The fail-OPEN hole: with no authoritative base, a HEAD~1..HEAD window would
  # miss a forbidden path introduced earlier but still present in HEAD. With the
  # fix, a missing base scans the FULL HEAD tree, so this MUST fail.
  mkdir -p .agents/learnings
  printf 'private\n' > .agents/learnings/leak.md
  git add -A && git commit -qm "earlier: add private learning"
  printf 'package main\n\nfunc main() {}\n' > cli/main.go
  git add -A && git commit -qm "clean tip commit"
  run env CORPUS_PATH_GUARD_BASE="missing-ref-does-not-exist" bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *".agents/learnings/leak.md"* ]]
}

@test "PASSES for an ordinary cli/ + docs (non-wiki) change" {
  printf 'package main\n\nfunc main() {}\n' > cli/main.go
  printf '# guide\n' > docs/guide.md
  git add -A && git commit -qm "ordinary code+docs change"
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"ok"* ]]
}
