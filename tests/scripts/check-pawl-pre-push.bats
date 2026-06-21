#!/usr/bin/env bats
# age-58o: push-to-main pawl gate via scripts/check-pawl-pre-push.sh

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-pawl-pre-push.sh"
  PAWL="$REPO_ROOT/scripts/pawl-verdict.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"
  mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  SHA="cafef00dbabe1234cafef00dbabe1234cafef00d"
  printf 'fresh-context review evidence\n' > "$TMP/evidence.txt"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

seed_verdict() {
  local bead="$1" head="$2"
  bash "$PAWL" write "$bead" 0 \
    --disposition CONFIRMED --head "$head" \
    --author-context author-ctx \
    --refuter "claude:CONFIRMED:fresh-reviewer-ctx:$TMP/evidence.txt" \
    --dir "$AGENTOPS_PAWL_VERDICT_DIR" >/dev/null
}

make_repo_with_commit() {
  local bead="$1"
  local msg="${2:-fix(test): wire pawl ($bead)}"
  REPO="$TMP/repo"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  echo change >> README.md
  git add README.md
  git commit --quiet -m "$msg"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA
}

@test "check-pawl-pre-push skips when no pre-push stdin" {
  run bash "$SCRIPT" </dev/null
  [ "$status" -eq 0 ]
  [[ "$output" == *"no pre-push stdin"* ]]
}

@test "check-pawl-pre-push skips non-main remote ref" {
  make_repo_with_commit age-58o-test-a
  status=0
  output="$(printf 'refs/heads/feat %s refs/heads/feat 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" != *"PAWL-HOLD"* ]]
}

@test "check-pawl-pre-push blocks main push without verdict" {
  make_repo_with_commit age-58o-test-b
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
}

@test "check-pawl-pre-push authorizes main push with CONFIRMED verdict" {
  make_repo_with_commit age-58o-test-c
  seed_verdict age-58o-test-c "$HEAD_SHA"
  printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" > "$TMP/push.txt"
  status=0
  output="$(bash "$SCRIPT" < "$TMP/push.txt" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"push authorized"* ]]
}

@test "check-pawl-pre-push blocks main push when bead missing from commit" {
  make_repo_with_commit age-58o-test-d "chore: no bead cited"
  seed_verdict age-58o-test-d "$HEAD_SHA"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"cites no bead id"* ]]
}

@test "check-pawl-pre-push waives #trivial commits on main" {
  make_repo_with_commit age-58o-test-e "chore: trivial doc #trivial"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"pawl waived"* ]]
}

# age-w2ny: build a commit whose body (after the subject + blank line) is fully
# operator-controlled, so we can place #trivial in prose vs. on its own line.
make_repo_with_body_commit() {
  local subject="$1" body="$2"
  REPO="$TMP/repo"
  rm -rf "$REPO"
  mkdir -p "$REPO"
  cd "$REPO"
  git init --quiet
  git config user.email test@example.com
  git config user.name Test
  echo ok > README.md
  git add README.md
  git commit --quiet -m "init"
  echo change >> README.md
  git add README.md
  printf '%s\n\n%s\n' "$subject" "$body" | git commit --quiet -F -
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO" HEAD_SHA
}

@test "check-pawl-pre-push does NOT waive #trivial mentioned only in body prose (age-w2ny)" {
  make_repo_with_body_commit \
    "feat(x): real feature (age-w2ny-test-a)" \
    "This explains code that marks something #trivial in an inline sentence."
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  [[ "$output" != *"pawl waived"* ]]
}

@test "check-pawl-pre-push does NOT waive #trivial mentioned mid-subject as prose (age-w2ny)" {
  # The #trivial token is NOT a trailing tag — it is prose inside the subject.
  make_repo_with_commit age-w2ny-test-c "fix(pawl): prevent #trivial from bypassing the gate (age-w2ny-test-c)"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 1 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  [[ "$output" != *"pawl waived"* ]]
}

@test "check-pawl-pre-push waives #trivial as a standalone trailer line in the body (age-w2ny)" {
  make_repo_with_body_commit \
    "chore(x): provenance-only edge (age-w2ny-test-b)" \
    "some body explanation here

#trivial"
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"pawl waived"* ]]
}

@test "check-pawl-pre-push honors AGENTOPS_PREPUSH_SKIP_PAWL=1" {
  make_repo_with_commit age-58o-test-f
  status=0
  output="$(printf 'refs/heads/main %s refs/heads/main 0000000000000000000000000000000000000000\n' "$HEAD_SHA" | env AGENTOPS_PREPUSH_SKIP_PAWL=1 bash "$SCRIPT" 2>&1)" || status=$?
  [ "$status" -eq 0 ]
  [[ "$output" == *"skipped"* ]]
}
