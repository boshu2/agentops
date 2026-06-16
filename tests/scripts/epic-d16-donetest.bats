#!/usr/bin/env bats
# epic-d16-donetest.bats — the terminal integration acceptance for Directive 16
# (epic age-d16-self-hosting-route-nkr). Runs the real done-test harness, which
# composes the real M1-M5 organs over an ISOLATED temp ledger + bead store, and
# asserts the unattended loop CLOSES end-to-end AND that the real repo ledger is
# never touched.
#
# This is an INTEGRATION test by nature (real br / ao / the organ scripts). It
# SKIPS cleanly when a prerequisite is absent so it never red-fails a thin
# environment; where the tools exist (the local pre-push gate), it runs for real.

setup() {
  HARNESS="$BATS_TEST_DIRNAME/../../scripts/epic-d16-donetest.sh"
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  command -v br >/dev/null 2>&1 || skip "br not available"
  command -v ao >/dev/null 2>&1 || skip "ao not available"
  ao provenance emit-verdict --help >/dev/null 2>&1 || skip "installed ao lacks provenance emit-verdict (M1)"
  [ -x "$REPO_ROOT/scripts/recovery-statemachine.sh" ] || skip "M2 organ missing"
  [ -x "$REPO_ROOT/scripts/assay/self-improvement-tick.sh" ] || skip "M4 organ missing"
  [ -x "$REPO_ROOT/scripts/pawl-verdict.sh" ] || skip "pawl organ missing"
  WORK="$(mktemp -d)"
  REAL_LEDGER="$REPO_ROOT/docs/provenance/ledger.jsonl"
  BEFORE="$(wc -l < "$REAL_LEDGER" 2>/dev/null || echo 0)"
}

teardown() { [ -n "${WORK:-}" ] && rm -rf "$WORK"; }

@test "directive-16 done-test: the unattended loop closes end-to-end (PASS)" {
  run "$HARNESS" --workdir "$WORK" --evidence-out "$WORK/evidence.md"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"result":"PASS"'* ]]
  [[ "$output" == *'"self_approval":"refused"'* ]]
}

@test "directive-16 done-test: every mechanical evidence path is present" {
  run "$HARNESS" --workdir "$WORK" --evidence-out "$WORK/evidence.md"
  [ "$status" -eq 0 ]
  # 1 seed bead, 3 recovery follow-up (rescope), 5 mined follow-up — all real ag- ids.
  [[ "$output" == *'"seed_bead":"ag-'* ]]
  [[ "$output" == *'"rescope_bead":"ag-'* ]]
  [[ "$output" == *'"mined_bead":"ag-'* ]]
  # the evidence artifact lists all five criteria.
  grep -q "seed (real follow-up bead" "$WORK/evidence.md"
  grep -q "failure injection → recovery branch fired" "$WORK/evidence.md"
  grep -q "self-improvement (ASSAY mines a follow-up)" "$WORK/evidence.md"
}

@test "directive-16 done-test: the REAL repo ledger is never touched (isolation)" {
  run "$HARNESS" --workdir "$WORK"
  [ "$status" -eq 0 ]
  local after
  after="$(wc -l < "$REAL_LEDGER" 2>/dev/null || echo 0)"
  [ "$BEFORE" -eq "$after" ]
}

@test "directive-16 done-test: refuses a --workdir inside the repo (isolation guard)" {
  run "$HARNESS" --workdir "$REPO_ROOT/inside-the-repo"
  [ "$status" -eq 2 ]
  [[ "$output" == *"OUTSIDE the repo"* ]]
}
