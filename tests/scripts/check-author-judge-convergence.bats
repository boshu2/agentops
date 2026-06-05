#!/usr/bin/env bats
# Tests for scripts/check-author-judge-convergence.sh — the DRIFT #149 guard that
# keeps the no-self-grade (author!=judge) invariant converged on liveness.Disjoint.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  GUARD="$REPO_ROOT/scripts/check-author-judge-convergence.sh"
  FIX="$BATS_TEST_TMPDIR/fix/internal/foo"
  mkdir -p "$FIX"
}

@test "passes on the converged repo (cli routes through liveness.Disjoint)" {
  run bash "$GUARD" "$REPO_ROOT/cli"
  [ "$status" -eq 0 ]
  [[ "$output" == *"author!=judge convergence"* ]]
}

@test "flags a divergent authorID == judgeID equality outside Disjoint" {
  printf 'package foo\nfunc bad(authorID, judgeID string) bool { return authorID == judgeID }\n' > "$FIX/bad.go"
  run bash "$GUARD" "$BATS_TEST_TMPDIR/fix"
  [ "$status" -eq 1 ]
  [[ "$output" == *"divergent author!=judge"* ]]
}

@test "flags a divergent EqualFold(author, judge)" {
  printf 'package foo\nimport "strings"\nfunc bad(author, judge string) bool { return strings.EqualFold(author, judge) }\n' > "$FIX/ef.go"
  run bash "$GUARD" "$BATS_TEST_TMPDIR/fix"
  [ "$status" -eq 1 ]
}

@test "flags the REVERSED grader-left equality (citedByAgent == artifactAuthor)" {
  # The same self-grade predicate with operands swapped (the #746 AMEND catch).
  printf 'package foo\nfunc bad(citedByAgent, artifactAuthor string) bool { return citedByAgent == artifactAuthor }\n' > "$FIX/rev.go"
  run bash "$GUARD" "$BATS_TEST_TMPDIR/fix"
  [ "$status" -eq 1 ]
}

@test "does not flag author == author (not a self-grade pair)" {
  printf 'package foo\nfunc ok(authorName, authorEmail string) bool { return authorName == authorEmail }\n' > "$FIX/aa.go"
  run bash "$GUARD" "$BATS_TEST_TMPDIR/fix"
  [ "$status" -eq 0 ]
}

@test "does not flag a comment describing author == judge" {
  printf 'package foo\n\t// a self-grade is author == judge\nfunc ok() bool { return false }\n' > "$FIX/c.go"
  run bash "$GUARD" "$BATS_TEST_TMPDIR/fix"
  [ "$status" -eq 0 ]
}

@test "does not flag *_test.go files" {
  printf 'package foo\nfunc helper(authorID, judgeID string) bool { return authorID == judgeID }\n' > "$FIX/x_test.go"
  run bash "$GUARD" "$BATS_TEST_TMPDIR/fix"
  [ "$status" -eq 0 ]
}
