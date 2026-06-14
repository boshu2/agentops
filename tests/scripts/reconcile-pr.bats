#!/usr/bin/env bats
# Hermetic regression tests for scripts/reconcile-pr.sh (ag-9gac).
#
# The script shells out to `gh` and `bd`. We stub BOTH via PATH so the test is
# deterministic and never touches a real PR, repo, or bead database. `jq` is a
# hard dep on every dev box and is used real.
#
# Stub contract (driven by env the test sets):
#   gh pr checks <pr> --json name,state   -> prints $GH_CHECKS_JSON
#   gh pr view <pr> --json state -q .state -> prints $GH_PR_STATE
#   gh pr view <pr> --json headRefName ... -> prints "feat/x"
#   gh pr merge ...                        -> logs "merge <pr>"; exit 0
#   gh run rerun ... / gh run list ...     -> logs "rerun"; exit 0
#   bd update <bead> --status closed       -> logs "close <bead>"; exit 0
#
# Poll cadence is forced to --poll-sleep 0 --poll-max 2 so the loop is instant.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/reconcile-pr.sh"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  ORIG_DIR="$PWD"
  mkdir -p "$TMP/bin"
  ACTION_LOG="$TMP/actions.log"
  : > "$ACTION_LOG"
  export ACTION_LOG
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

# stub_gh writes a fake `gh`. State sources come from files so the flake case
# can flip them between the first and second poll (rerun changes the outcome).
#   $TMP/checks       — JSON for `gh pr checks`
#   $TMP/checks.after — JSON used AFTER a rerun has been logged (optional)
#   $TMP/pr_state     — value for `gh pr view --json state`
stub_gh() {
  cat >"$TMP/bin/gh" <<EOF
#!/usr/bin/env bash
LOG="$ACTION_LOG"
CHECKS="$TMP/checks"
CHECKS_AFTER="$TMP/checks.after"
PR_STATE_FILE="$TMP/pr_state"
if [ "\$1" = "pr" ] && [ "\$2" = "checks" ]; then
  # If a rerun has happened and an "after" fixture exists, serve that.
  if grep -q '^rerun' "\$LOG" 2>/dev/null && [ -f "\$CHECKS_AFTER" ]; then
    cat "\$CHECKS_AFTER"
  else
    cat "\$CHECKS"
  fi
  exit 0
fi
if [ "\$1" = "pr" ] && [ "\$2" = "view" ]; then
  case "\$*" in
    *headRefName*) echo "feat/x"; exit 0 ;;
    *headRefOid*)  cat "$TMP/pr_head" 2>/dev/null || echo "deadbeefcafe1234"; exit 0 ;;
    *state*)       cat "\$PR_STATE_FILE" 2>/dev/null || echo "OPEN"; exit 0 ;;
  esac
  exit 0
fi
if [ "\$1" = "pr" ] && [ "\$2" = "merge" ]; then
  echo "merge \$3" >> "\$LOG"; exit 0
fi
if [ "\$1" = "run" ] && [ "\$2" = "rerun" ]; then
  echo "rerun" >> "\$LOG"; exit 0
fi
if [ "\$1" = "run" ] && [ "\$2" = "list" ]; then
  echo "12345"; exit 0
fi
exit 0
EOF
  chmod +x "$TMP/bin/gh"
}

stub_bd() {
  cat >"$TMP/bin/bd" <<EOF
#!/usr/bin/env bash
LOG="$ACTION_LOG"
if [ "\$1" = "update" ] && [ "\$3" = "--status" ] && [ "\$4" = "closed" ]; then
  echo "close \$2" >> "\$LOG"; exit 0
fi
exit 0
EOF
  chmod +x "$TMP/bin/bd"
}

activate() {
  stub_gh
  stub_bd
  export PATH="$TMP/bin:$ORIG_PATH"
}

# The PR head the verdict is sealed against. The gh stub serves this for
# `gh pr view --json headRefOid`; seed_verdict pins the verdict to it. A test
# can override $TMP/pr_head to simulate a NEW push after review (STALE verdict).
DEFAULT_HEAD="cafef00dbabe1234"

# Seed a cross-family pawl verdict into the per-test verdict dir, so the
# fail-closed pawl gate authorizes (or refuses) the merge. Uses the real
# scripts/pawl-verdict.sh writer. The verdict is COMMIT-BOUND (--head) and
# EVIDENCE-BOUND (each refuter carries a real, non-empty evidence file) — the
# hardened gate requires both, so the seed supplies both by default.
#   seed_verdict <bead> <pr> <disposition> <ref1> [ref2 ...]
# where each refN is "family:CONFIRMED|REFUTED".
seed_verdict() {
  local bead="$1" pr="$2" disp="$3"; shift 3
  # Pin the gh stub's headRefOid to the head we seal against (unless a test set it).
  [ -f "$TMP/pr_head" ] || printf '%s' "$DEFAULT_HEAD" > "$TMP/pr_head"
  local head; head="$(cat "$TMP/pr_head")"
  # A real, non-empty evidence transcript every refuter points at.
  local ev="$TMP/evidence.txt"
  [ -s "$ev" ] || printf 'refuter transcript: review actually ran\n' > "$ev"
  local args=()
  local r
  for r in "$@"; do args+=(--refuter "$r:$ev"); done
  "$REPO_ROOT/scripts/pawl-verdict.sh" write "$bead" "$pr" \
    --disposition "$disp" --head "$head" "${args[@]}" --dir "$TMP/verdicts" >/dev/null
}

# Write a RAW verdict JSON straight to the per-test verdict dir (bypassing the
# writer) so a test can exercise schema-invalid / malformed inputs the writer
# would never emit.
seed_raw_verdict() {
  local bead="$1" json="$2"
  mkdir -p "$TMP/verdicts"
  printf '%s' "$json" > "$TMP/verdicts/$bead.json"
  [ -f "$TMP/pr_head" ] || printf '%s' "$DEFAULT_HEAD" > "$TMP/pr_head"
}

# All reconcile runs point at the per-test verdict dir (empty by default =>
# fail-closed HOLD unless a test seeds a verdict).
run_reconcile() {
  run "$SCRIPT" --poll-sleep 0 --poll-max 2 --verdict-dir "$TMP/verdicts" "$@"
}

@test "all-green WITH CONFIRMED cross-family verdict: merges + closes bead, exit 0" {
  printf '%s' '[{"name":"correctness (ubuntu-latest)","state":"SUCCESS"},{"name":"validate","state":"SUCCESS"},{"name":"claude-review","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_verdict ag-700 700 CONFIRMED claude:CONFIRMED codex:CONFIRMED
  run_reconcile 700 ag-700
  [ "$status" -eq 0 ]
  [[ "$output" == *"MERGED: PR 700"* ]]
  [[ "$output" == *"CLOSED bead=ag-700"* ]]
  grep -qx 'merge 700' "$ACTION_LOG"
  grep -qx 'close ag-700' "$ACTION_LOG"
}

@test "all-green but NO pawl verdict: HOLD exit 5, no merge, no close (green CI is NOT sufficient)" {
  printf '%s' '[{"name":"correctness (ubuntu-latest)","state":"SUCCESS"},{"name":"validate","state":"SUCCESS"},{"name":"claude-review","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  # No seed_verdict: the verdict dir is empty => fail-closed.
  run_reconcile 720 ag-720
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
  ! grep -q '^close' "$ACTION_LOG"
}

@test "all-green but REFUTED verdict: HOLD exit 5, no merge, no close" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_verdict ag-721 721 REFUTED claude:CONFIRMED codex:REFUTED
  run_reconcile 721 ag-721
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
  ! grep -q '^close' "$ACTION_LOG"
}

@test "all-green but ESCALATE verdict (non-convergence): HOLD exit 5, no merge" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_verdict ag-722 722 ESCALATE claude:CONFIRMED codex:CONFIRMED
  run_reconcile 722 ag-722
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
  ! grep -q '^close' "$ACTION_LOG"
}

@test "all-green but single-family verdict: HOLD exit 5 (single-family is NOT cross-family)" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_verdict ag-723 723 CONFIRMED claude:CONFIRMED claude:CONFIRMED
  run_reconcile 723 ag-723
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
}

@test "all-green but verdict for a DIFFERENT pr: HOLD exit 5 (verdict does not transfer)" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  # CONFIRMED verdict exists for this bead but pinned to PR 999, not 724.
  seed_verdict ag-724 999 CONFIRMED claude:CONFIRMED codex:CONFIRMED
  run_reconcile 724 ag-724
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
}

@test "substantive fail: BLOCKED exit 2, no merge, no close" {
  printf '%s' '[{"name":"validate","state":"FAILURE"},{"name":"claude-review","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'OPEN' > "$TMP/pr_state"
  activate
  run_reconcile 701 ag-701
  [ "$status" -eq 2 ]
  [[ "$output" == *"BLOCKED fails=[validate]"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
  ! grep -q '^close' "$ACTION_LOG"
}

@test "claude-review failure alone is NOT blocking: merges (with verdict), exit 0" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"},{"name":"claude-review","state":"FAILURE"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_verdict ag-702 702 CONFIRMED claude:CONFIRMED codex:CONFIRMED
  run_reconcile 702 ag-702
  [ "$status" -eq 0 ]
  grep -qx 'merge 702' "$ACTION_LOG"
  grep -qx 'close ag-702' "$ACTION_LOG"
}

@test "lone correctness-ubuntu flake: reruns ONCE then merges, exit 0" {
  # First read: the flake is failing. After a rerun is logged: all green.
  printf '%s' '[{"name":"correctness (ubuntu-latest)","state":"FAILURE"},{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf '%s' '[{"name":"correctness (ubuntu-latest)","state":"SUCCESS"},{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks.after"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_verdict ag-703 703 CONFIRMED claude:CONFIRMED codex:CONFIRMED
  run_reconcile 703 ag-703
  [ "$status" -eq 0 ]
  grep -qx 'rerun' "$ACTION_LOG"
  grep -qx 'merge 703' "$ACTION_LOG"
  grep -qx 'close ag-703' "$ACTION_LOG"
}

@test "flake persists after rerun: still BLOCKED exit 2" {
  printf '%s' '[{"name":"correctness (ubuntu-latest)","state":"FAILURE"},{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  # No checks.after → stub keeps serving the failing fixture even after rerun.
  printf 'OPEN' > "$TMP/pr_state"
  activate
  run_reconcile 704 ag-704
  [ "$status" -eq 2 ]
  grep -qx 'rerun' "$ACTION_LOG"
  ! grep -q '^merge' "$ACTION_LOG"
}

@test "merge ran but state != MERGED: exit 3, bead NOT closed" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'OPEN' > "$TMP/pr_state"
  activate
  seed_verdict ag-705 705 CONFIRMED claude:CONFIRMED codex:CONFIRMED
  run_reconcile 705 ag-705
  [ "$status" -eq 3 ]
  [[ "$output" == *"merge-FAILED"* ]]
  ! grep -q '^close' "$ACTION_LOG"
}

@test "dry-run on green WITH verdict: no merge, no close, exit 0" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_verdict ag-706 706 CONFIRMED claude:CONFIRMED codex:CONFIRMED
  run_reconcile --dry-run 706 ag-706
  [ "$status" -eq 0 ]
  [[ "$output" == *"DRY-RUN"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
  ! grep -q '^close' "$ACTION_LOG"
}

# --- hardened pawl gate: evidence-bound + commit-bound + schema-validated -----

@test "STALE verdict (head_sha != PR's CURRENT head): HOLD exit 5, no merge" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  # Seal the verdict against the default head...
  seed_verdict ag-730 730 CONFIRMED claude:CONFIRMED codex:CONFIRMED
  # ...then simulate a NEW commit pushed after review: the PR's current head moves.
  printf 'feedfacef00d9999' > "$TMP/pr_head"
  run_reconcile 730 ag-730
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
  ! grep -q '^close' "$ACTION_LOG"
}

@test "pending CI never concludes (valid verdict present): BLOCKED exit 2, no merge" {
  # A check that stays PENDING forever must NOT slip through as 'not failing'.
  printf '%s' '[{"name":"validate","state":"PENDING"},{"name":"correctness (ubuntu-latest)","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_verdict ag-731 731 CONFIRMED claude:CONFIRMED codex:CONFIRMED
  run_reconcile 731 ag-731
  [ "$status" -eq 2 ]
  [[ "$output" == *"still PENDING"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
  ! grep -q '^close' "$ACTION_LOG"
}

@test "schema-invalid verdict (pr is a string, not a number): HOLD exit 5, no merge" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  printf 'ev\n' > "$TMP/evidence.txt"
  seed_raw_verdict ag-732 '{"schema_version":"pawl-verdict.v1","bead_id":"ag-732","pr":"732","head_sha":"cafef00dbabe1234","disposition":"CONFIRMED","generated_at":"2026-01-01T00:00:00Z","refuters":[{"family":"claude","verdict":"CONFIRMED","evidence":"'"$TMP/evidence.txt"'"},{"family":"codex","verdict":"CONFIRMED","evidence":"'"$TMP/evidence.txt"'"}]}'
  run_reconcile 732 ag-732
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
}

@test "malformed JSON verdict: HOLD exit 5, no merge" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_raw_verdict ag-733 '{this is not json'
  run_reconcile 733 ag-733
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
}

@test "fake/unknown family label cannot game >=2 families: HOLD exit 5, no merge" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  printf 'ev\n' > "$TMP/evidence.txt"
  # 'totally-real-family' is off-roster: rejected (schema enum + check normalization).
  seed_raw_verdict ag-734 '{"schema_version":"pawl-verdict.v1","bead_id":"ag-734","pr":734,"head_sha":"cafef00dbabe1234","disposition":"CONFIRMED","generated_at":"2026-01-01T00:00:00Z","refuters":[{"family":"claude","verdict":"CONFIRMED","evidence":"'"$TMP/evidence.txt"'"},{"family":"totally-real-family","verdict":"CONFIRMED","evidence":"'"$TMP/evidence.txt"'"}]}'
  run_reconcile 734 ag-734
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
}

@test "two aliases of ONE canonical family (claude + fable) is single-family: HOLD exit 5" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  # claude and fable both canonicalize to 'claude' — not two distinct families.
  seed_verdict ag-735 735 CONFIRMED claude:CONFIRMED fable:CONFIRMED
  run_reconcile 735 ag-735
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
}

@test "verdict with NO reviewer evidence (self-asserted stamp): HOLD exit 5, no merge" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  # No evidence path on any refuter, no council_artifact => a stamp, not a review.
  seed_raw_verdict ag-736 '{"schema_version":"pawl-verdict.v1","bead_id":"ag-736","pr":736,"head_sha":"cafef00dbabe1234","disposition":"CONFIRMED","generated_at":"2026-01-01T00:00:00Z","refuters":[{"family":"claude","verdict":"CONFIRMED"},{"family":"codex","verdict":"CONFIRMED"}]}'
  run_reconcile 736 ag-736
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
}

@test "refuter evidence path points at a MISSING file: HOLD exit 5, no merge" {
  printf '%s' '[{"name":"validate","state":"SUCCESS"}]' > "$TMP/checks"
  printf 'MERGED' > "$TMP/pr_state"
  activate
  seed_raw_verdict ag-737 '{"schema_version":"pawl-verdict.v1","bead_id":"ag-737","pr":737,"head_sha":"cafef00dbabe1234","disposition":"CONFIRMED","generated_at":"2026-01-01T00:00:00Z","refuters":[{"family":"claude","verdict":"CONFIRMED","evidence":"'"$TMP/does-not-exist.txt"'"},{"family":"codex","verdict":"CONFIRMED","evidence":"'"$TMP/does-not-exist.txt"'"}]}'
  run_reconcile 737 ag-737
  [ "$status" -eq 5 ]
  [[ "$output" == *"PAWL-HOLD"* ]]
  ! grep -q '^merge' "$ACTION_LOG"
}

@test "non-numeric pr exits 4" {
  activate
  run_reconcile notapr ag-707
  [ "$status" -eq 4 ]
  [[ "$output" == *"pr-number must be numeric"* ]]
}

@test "unknown flag exits 4" {
  activate
  run_reconcile --weasel 708 ag-708
  [ "$status" -eq 4 ]
  [[ "$output" == *"unknown flag"* ]]
}
