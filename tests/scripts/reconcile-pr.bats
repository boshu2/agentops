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

# Seed a cross-family pawl verdict into the per-test verdict dir, so the
# fail-closed pawl gate authorizes (or refuses) the merge. Uses the real
# scripts/pawl-verdict.sh writer.
#   seed_verdict <bead> <pr> <disposition> <ref1> [ref2 ...]
# where each refN is "family:CONFIRMED|REFUTED".
seed_verdict() {
  local bead="$1" pr="$2" disp="$3"; shift 3
  local args=()
  local r
  for r in "$@"; do args+=(--refuter "$r"); done
  "$REPO_ROOT/scripts/pawl-verdict.sh" write "$bead" "$pr" \
    --disposition "$disp" "${args[@]}" --dir "$TMP/verdicts" >/dev/null
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
