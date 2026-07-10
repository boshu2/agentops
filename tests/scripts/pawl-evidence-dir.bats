#!/usr/bin/env bats
# pawl-evidence-dir.bats — evidence survives disposable worktrees
# (age-pawl-evidence-worktree-loss-np1e).
#
# The loss class (2026-07-09): reviews run from a land worktree wrote evidence to
# <worktree>/.agents/pawl-evidence/, the landed ledger recorded that absolute
# path, and `git worktree remove` destroyed both the transcripts and the path.
# The resolver must return the CANONICAL checkout's evidence dir from any linked
# worktree, so transcripts outlive the worktree and ledger paths never dangle.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/lib/pawl-evidence-dir.sh"
  R="$BATS_TEST_TMPDIR/r"
  git init -q "$R"; git -C "$R" config user.email t@t.local; git -C "$R" config user.name t
  echo x > "$R/f.txt"; git -C "$R" add -A; git -C "$R" commit -qm init
  unset AGENTOPS_PAWL_EVIDENCE_DIR
}

@test "canonical checkout resolves to its own .agents/pawl-evidence" {
  expected="$(cd "$R" && pwd -P)/.agents/pawl-evidence"
  run pawl_evidence_dir "$R"
  [ "$status" -eq 0 ]
  [ "$output" = "$expected" ]
}

@test "linked worktree resolves to the CANONICAL checkout's evidence dir" {
  git -C "$R" worktree add -q "$BATS_TEST_TMPDIR/wt" -b wt-branch
  expected="$(cd "$R" && pwd -P)/.agents/pawl-evidence"
  run pawl_evidence_dir "$BATS_TEST_TMPDIR/wt"
  [ "$status" -eq 0 ]
  [ "$output" = "$expected" ]
}

@test "AGENTOPS_PAWL_EVIDENCE_DIR override wins" {
  AGENTOPS_PAWL_EVIDENCE_DIR="/somewhere/else" run pawl_evidence_dir "$R"
  [ "$status" -eq 0 ]
  [ "$output" = "/somewhere/else" ]
}

@test "non-git directory falls back to its own path" {
  mkdir -p "$BATS_TEST_TMPDIR/plain"
  run pawl_evidence_dir "$BATS_TEST_TMPDIR/plain"
  [ "$status" -eq 0 ]
  [ "$output" = "$BATS_TEST_TMPDIR/plain/.agents/pawl-evidence" ]
}

@test "pawl_canonical_root: worktree maps to main checkout; non-git maps to itself" {
  git -C "$R" worktree add -q "$BATS_TEST_TMPDIR/wt2" -b wt2-branch
  expected="$(cd "$R" && pwd -P)"
  run pawl_canonical_root "$BATS_TEST_TMPDIR/wt2"
  [ "$status" -eq 0 ]
  [ "$output" = "$expected" ]
  mkdir -p "$BATS_TEST_TMPDIR/plain2"
  run pawl_canonical_root "$BATS_TEST_TMPDIR/plain2"
  [ "$status" -eq 0 ]
  [ "$output" = "$BATS_TEST_TMPDIR/plain2" ]
}
