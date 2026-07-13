#!/usr/bin/env bats
# pawl-review.sh — BIND-RECURSION GUARD (age-33nx).
#
# pawl-verdict auto-binds a `#trivial` provenance commit onto HEAD even for a
# REFUTED round, so a re-run used to review the BIND commit instead of the change
# under review — the reviewer then (correctly) refutes the incoherent ledger row
# and the loop can never converge (age-77g6 rounds 2-3, 2026-07-03). These tests
# prove pawl-review walks the review target back over genuine #trivial
# provenance-only commits (shared lib/trivial-waiver.sh semantics) and binds the
# verdict to the underlying change commit — and that the walk-back NEVER fires on
# a #trivial commit that fails the waiver (non-provenance paths).
#
# Same harness as pawl-review.bats: codex is a PATH stub, everything runs in a
# temp repo (AGENTOPS_REPO_ROOT), cold path forced via PAWL_NO_SERVICE=1.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
prompt="$(cat)"   # the refuter prompt
printf '%s\n' "$prompt" > "${PROMPT_CAPTURE:-/dev/null}"
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
  REAL_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"
  mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-rev-test.json"
  export PAWL_NO_SERVICE=1
  export PROMPT_CAPTURE="$TMP/prompt.txt"
}

teardown() {
  cd "$ORIG_DIR" || true
  rm -rf "$TMP"
}

# Append a GENUINE #trivial provenance-only commit (waiver rc=0: trailing
# #trivial subject marker + every changed file under docs/provenance/).
_add_trivial_bind_commit() {
  mkdir -p docs/provenance
  echo "{\"edge\":\"$1\"}" >> docs/provenance/ledger.jsonl
  git add docs/provenance/ledger.jsonl
  git commit --quiet -m "chore(provenance): bind pawl REFUTED verdict for age-rev-test #trivial"
}

@test "head scope: walks back over a #trivial provenance-bind tip and binds the verdict to the change commit" {
  _add_trivial_bind_commit r1
  TIP_SHA="$(git rev-parse HEAD)"
  run bash "$SCRIPT" age-rev-test
  [ "$status" -eq 0 ]
  [[ "$output" == *"walked back"* ]]
  [ -f "$VFILE" ]
  [ "$(jq -r .head_sha "$VFILE")" = "$REAL_SHA" ]
  [ "$(jq -r .head_sha "$VFILE")" != "$TIP_SHA" ]
  # The reviewer saw the CHANGE's diff, not the bind commit's ledger append.
  grep -q 'feat(x): a change' "$PROMPT_CAPTURE"
  ! grep -q 'chore(provenance): bind pawl' "$PROMPT_CAPTURE"
}

@test "head scope: walks back over a STACK of #trivial bind commits (the age-77g6 recursion)" {
  _add_trivial_bind_commit r1
  _add_trivial_bind_commit r2
  run bash "$SCRIPT" age-rev-test
  [ "$status" -eq 0 ]
  [ "$(jq -r .head_sha "$VFILE")" = "$REAL_SHA" ]
}

@test "head scope: a #trivial tip touching NON-provenance paths is NOT walked back (waiver refused)" {
  echo not-provenance >> README.md
  git add README.md
  git commit --quiet -m "feat(y): sneaky change #trivial"
  TIP_SHA="$(git rev-parse HEAD)"
  # This case tests the WALK-BACK waiver (a #trivial tip touching non-provenance
  # files is not walked back). pawl-review's newer, orthogonal PAWL-AMEND-GUARD
  # hard-refuses that exact commit shape *before* the walk-back logic runs, so
  # opt it out here to exercise the waiver path this test is about.
  run env PAWL_NO_AMEND_GUARD=1 bash "$SCRIPT" age-rev-test
  [ "$status" -eq 0 ]
  [[ "$output" != *"walked back"* ]]
  [ "$(jq -r .head_sha "$VFILE")" = "$TIP_SHA" ]
}

@test "head scope: a non-trivial tip reviews HEAD exactly as before (no walk-back note)" {
  run bash "$SCRIPT" age-rev-test
  [ "$status" -eq 0 ]
  [[ "$output" != *"walked back"* ]]
  [ "$(jq -r .head_sha "$VFILE")" = "$REAL_SHA" ]
}
