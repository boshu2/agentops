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

@test "FAILS when a BD export or local redirect is staged under .beads/" {
  mkdir -p .beads
  printf '{}\n' > .beads/issues.jsonl
  printf '/private/tracker/.beads\n' > .beads/redirect
  git add -A
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *".beads/issues.jsonl"* ]]
  [[ "$output" == *".beads/redirect"* ]]
}

@test "FAILS for committed Dolt data and identity backup under .beads/" {
  mkdir -p .beads/dolt
  printf 'private database\n' > .beads/dolt/data
  printf 'private backup\n' > .beads/identity.toml.bak
  git add -A && git commit -qm "private BD data"
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *".beads/dolt/data"* ]]
  [[ "$output" == *".beads/identity.toml.bak"* ]]
}

@test "PASSES the existing public .beads/identity.toml without admitting sibling data" {
  mkdir -p .beads
  printf 'project_id = "public-identity"\n' > .beads/identity.toml
  git add -A && git commit -qm "public project identity"
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  printf '{}\n' > .beads/issues.jsonl
  git add -A
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *".beads/issues.jsonl"* ]]
}

@test "FAILS for a staged private BD filename containing a newline and quote" {
  mkdir -p .beads
  private_path=$'.beads/private\n"export.jsonl'
  printf '{}\n' > "$private_path"
  git add -A
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"forbidden:"* ]]
}

@test "FAILS when private BD data was added then deleted in outgoing history" {
  mkdir -p .beads
  printf '{}\n' > .beads/issues.jsonl
  git add -A && git commit -qm "private file introduced"
  git rm -q .beads/issues.jsonl && git commit -qm "private file removed"
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *".beads/issues.jsonl"* ]]
}

@test "FAILS with a missing base when removed private data remains in reachable history" {
  mkdir -p .beads
  printf '{}\n' > .beads/issues.jsonl
  git add -A && git commit -qm "private file introduced"
  git rm -q .beads/issues.jsonl && git commit -qm "private file removed"
  run env CORPUS_PATH_GUARD_BASE=missing-base bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *".beads/issues.jsonl"* ]]
}

@test "FAILS for nested BD exports and identities; only the root identity is public" {
  mkdir -p cli/.beads
  printf '{}\n' > cli/.beads/issues.jsonl
  printf 'private identity\n' > cli/.beads/identity.toml
  git add -A
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"cli/.beads/issues.jsonl"* ]]
  [[ "$output" == *"cli/.beads/identity.toml"* ]]
}

@test "FAILS when a BD directory name is staged as a file or symlink" {
  printf 'private route\n' > .beads
  ln -s /private/tracker cli/.beads
  git add -A
  run bash "$SCRIPT"
  [ "$status" -ne 0 ]
  [[ "$output" == *"forbidden: .beads"* ]]
  [[ "$output" == *"forbidden: cli/.beads"* ]]
}
