#!/usr/bin/env bats
# pawl-review.sh — RUN the cross-family membrane review + write the verdict on CONFIRMED.
# The real codex refuter is replaced by a STUB on PATH (canned verdict via $CODEX_STUB,
# exit via $CODEX_EXIT), so these prove the ORCHESTRATION (diff -> review -> parse ->
# verdict/exit) without a live model call. Everything runs inside a temp repo
# (AGENTOPS_REPO_ROOT) so the real repo + its ledger are never touched.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
cat >/dev/null   # consume the refuter prompt
printf 'codex\n%s\n' "${CODEX_STUB:-VERDICT: CONFIRMED}"
exit "${CODEX_EXIT:-0}"
STUB
  chmod +x "$BIN/codex"
  PATH="$BIN:$PATH"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > README.md; git add README.md; git commit --quiet -m init
  echo change >> README.md; git add README.md
  git commit --quiet -m "feat(x): a change (age-rev-test)"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"
  mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-rev-test.json"
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

@test "pawl-review: head CONFIRMED writes a commit-bound verdict that passes check (exit 0)" {
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [[ "$output" == *"CONFIRMED + verdict written"* ]]
  [ -f "$VFILE" ]
  grep -q '"disposition": "CONFIRMED"' "$VFILE"
  grep -q "$HEAD_SHA" "$VFILE"
}

@test "pawl-review: REFUTED prints defects, writes NO verdict, exits 3" {
  CODEX_STUB="$(printf 'VERDICT: REFUTED\nDEFECTS:\n - the foo path is fail-open')" \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]
  [[ "$output" == *"REFUTED"* ]]
  [[ "$output" == *"fail-open"* ]]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: no clear verdict is fail-closed (exit 1), no verdict written" {
  CODEX_STUB="maybe it is fine?" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 1 ]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: CONFIRMED from a NON-ZERO-exit reviewer run is fail-closed (defect #3)" {
  CODEX_STUB="VERDICT: CONFIRMED" CODEX_EXIT=124 run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 1 ]
  [[ "$output" == *"non-zero"* ]]
  [ ! -f "$VFILE" ]   # a CONFIRMED from a crashed/timed-out reviewer must NOT write a verdict
}

@test "pawl-review: scope=staged CONFIRMED is REVIEW-ONLY — no verdict written (defect #1)" {
  echo more >> README.md; git add README.md   # stage an uncommitted change
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope staged
  [ "$status" -eq 0 ]
  [[ "$output" == *"review-only"* ]]
  [ ! -f "$VFILE" ]   # staged has no commit to bind — must not certify HEAD
}

@test "pawl-review: same-family author (codex == the codex refuter) is REFUSED (defect #2)" {
  # A codex/openai/gpt author + the codex refuter is a SAME-family review, not the
  # cross-family check this command provides — refuse, write nothing.
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --author-family codex
  [ "$status" -eq 2 ]
  [[ "$output" == *"same-family"* ]]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: a DIFFERENT-family author (default claude) is allowed cross-family" {
  # claude author + codex refuter = genuinely cross-family -> writes the verdict.
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --author-family claude
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
}

@test "pawl-review: same-family guard is CASE-INSENSITIVE (Codex/GPT cannot bypass)" {
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head --author-family Codex
  [ "$status" -eq 2 ]
  [[ "$output" == *"same-family"* ]]
  [ ! -f "$VFILE" ]
}

@test "pawl-review: an echoed template verdict does NOT override the reviewer's FINAL verdict" {
  # The output quotes the prompt template (VERDICT: CONFIRMED / VERDICT: REFUTED) BEFORE
  # the real answer (REFUTED). The earlier template CONFIRMED must not win — last wins.
  CODEX_STUB="$(printf 'Reply with this shape:\nVERDICT: CONFIRMED\n-- or --\nVERDICT: REFUTED\nMy actual answer:\nVERDICT: REFUTED\nDEFECTS:\n - a genuine bug')" \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]            # REFUTED wins, not the echoed CONFIRMED
  [ ! -f "$VFILE" ]
}

@test "pawl-review: empty staged diff is a precondition error (exit 2)" {
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope staged
  [ "$status" -eq 2 ]
  [[ "$output" == *"empty diff"* ]]
}

@test "pawl-review: a flag with no value is a usage error (exit 2)" {
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope
  [ "$status" -eq 2 ]
  [[ "$output" == *"needs a value"* ]]
}

@test "pawl-review: missing bead id is a usage error (exit 2)" {
  run env PATH="$BIN:$PATH" bash "$SCRIPT" --scope head
  [ "$status" -eq 2 ]
  [[ "$output" == *"need <bead-id>"* ]]
}
