#!/usr/bin/env bats
# pawl-review-user-preflight.bats — locks for the EARLY user-readiness reviewer preflight
# (age-7k3i, shift-left epic age-tc0l). A user whose ONLY installed agent CLI is claude has
# NO cold reviewer — the cold path is codex-first BY DESIGN (LAW 0: no cold claude adapter;
# REVIEWER=agy is the sanctioned cold failover). Before the preflight that user hit
# CODEX_EXEC_MISSING=2 DEEP in lib/codex-exec.sh — after the amend-guard / deterministic
# battery / smoke / packet build — with no guidance. These tests lock:
#   1. codex absent (default reviewer) -> exit 2 EARLY, message NAMES codex + the three
#      actionable options (install it / REVIEWER=agy), NO review artifacts.
#   2. REVIEWER=agy + agy absent -> the SAME shape naming agy.
#   3. the preflight never blocks the failover chain: with ANY usable reviewer reachable
#      the run proceeds into the chain loop (the loop still owns failover/degradation).
# The real codex/agy CLIs are NEVER invoked — a sanitized PATH + stubs only (mirrors
# pawl-review-lib-parity.bats' missing-codex lock and the failover-suite stub pattern).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  # Sanitized PATH: every tool pawl-review needs EXCEPT any reviewer CLI (no codex, no
  # agy), so the preflight decision is deterministic regardless of the host's installs.
  TOOLBIN="$TMP/toolbin"; mkdir -p "$TOOLBIN"
  for t in bash sh env git jq sed grep awk cat mktemp rm printf wc tr date head tail cut sort dirname basename timeout gtimeout shasum sha256sum; do
    src="$(command -v "$t" 2>/dev/null)" && ln -sf "$src" "$TOOLBIN/$t"
  done
  BIN="$TMP/bin"; mkdir -p "$BIN"   # stub dir, prepended only where a test wants a reviewer present
  # Throwaway reviewed repo (mirrors the sibling suites): provenance/ledger surfaces
  # resolve their root FROM CWD, so never run from the real checkout.
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  printf 'a line\n' > note.txt
  git add note.txt; git commit --quiet -m init
  printf 'a line\nan added line under review\n' > note.txt
  git add note.txt; git commit --quiet -m "feat(x): a change (age-upf-test)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-upf-test.json"
  export PAWL_NO_SERVICE=1     # cold path only — never route to a warm pane
  export PAWL_NO_PREFLIGHT=1   # the ebec.9 deterministic battery is out of scope here (host-ao nondeterminism)
  export PAWL_REVIEW_TIMEOUT=10
  export PAWL_AUTOBIND=0       # a test run must never create a ledger bind commit
  export STUB_SENTINEL="$TMP/called"   # stubs touch "<sentinel>.<name>" when invoked
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

# The EARLY contract: exit 2 before ANY review work, so no packet/evidence/verdict
# artifact may exist afterward (the deep CODEX_EXEC_MISSING crash happened after these
# surfaces were already touched).
_assert_no_review_artifacts() {
  [ ! -f "$VFILE" ]
  [ ! -d "$REPO/.agents/pawl-evidence" ]
  [ ! -d "$REPO/.agents/pawl-review" ]
}

@test "age-7k3i: claude-only user (default codex reviewer absent) -> EARLY exit 2, named + actionable, NO artifacts" {
  run env PATH="$TOOLBIN" bash "$SCRIPT" age-upf-test --scope head
  [ "$status" -eq 2 ]
  # Named: the message says WHICH reviewer this run resolved and that it is missing.
  [[ "$output" == *"MISSING DEPENDENCY"* ]]
  [[ "$output" == *"cold reviewer for this run is 'codex'"* ]]
  [[ "$output" == *"early user-readiness preflight"* ]]
  # Actionable: escape hatches are offered by name.
  [[ "$output" == *"install the 'codex' CLI"* ]]
  [[ "$output" == *"REVIEWER=agy"* ]]
  # Precondition semantics preserved (exit 2 is never a review result).
  [[ "$output" == *"not a REFUTE"* ]]
  _assert_no_review_artifacts
}

@test "age-7k3i: REVIEWER=agy with agy absent -> the SAME early shape naming agy" {
  run env PATH="$TOOLBIN" REVIEWER=agy bash "$SCRIPT" age-upf-test --scope head
  [ "$status" -eq 2 ]
  [[ "$output" == *"MISSING DEPENDENCY"* ]]
  [[ "$output" == *"cold reviewer for this run is 'agy'"* ]]
  [[ "$output" == *"early user-readiness preflight"* ]]
  [[ "$output" == *"install the 'agy' CLI"* ]]
  # The agy-flavored option set points BACK at the codex default instead of at itself.
  [[ "$output" == *"unset REVIEWER"* ]]
  _assert_no_review_artifacts
}

@test "age-7k3i: preflight never blocks failover — chain codex,agy + codex absent + agy present -> run proceeds into the chain" {
  # A genuine agy stub with a call sentinel (failover-suite pattern): if the run reaches
  # the chain loop, the codex MISSING fail-over invokes agy — proving the early preflight
  # let a partially-reachable chain through instead of exiting 2.
  cat > "$BIN/agy" <<'FAKE'
#!/usr/bin/env bash
[ -n "${STUB_SENTINEL:-}" ] && touch "${STUB_SENTINEL}.agy"
echo "Reviewed note.txt:2 — the added line is safe and matches the commit claim."
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/agy"
  # Full PATH + CODEX_EXEC_BIN pointing at nothing (the failover-suite pattern): codex is
  # deterministically MISSING while the rest of the run keeps its real tools.
  run env PATH="$BIN:$PATH" CODEX_EXEC_BIN=absent-codex-xyz PAWL_REVIEWER_CHAIN="codex,agy" bash "$SCRIPT" age-upf-test --scope head
  # The early preflight must NOT fire (agy is reachable) …
  [[ "$output" != *"early user-readiness preflight"* ]]
  [ "$status" -ne 2 ]
  # … and the chain loop actually ran: codex went MISSING-failover and agy was invoked.
  [[ "$output" == *"failing over"* ]]
  [ -f "$TMP/called.agy" ]
}
