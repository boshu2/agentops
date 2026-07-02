#!/usr/bin/env bats
bats_require_minimum_version 1.5.0
# pawl-review-ergonomics.bats — locks for the age-rk3r.10 VERIFY ERGONOMICS bundle:
#   (a) OPTIONAL change-id (covered in pawl-review.bats — the derive-from-branch lock),
#   (b) reviewer HEARTBEAT (a plain stderr alive-line every ~interval so a healthy long run is
#       distinguishable from a stall), and
#   (c) NO-VERDICT TRIAGE block (STALL / ECHO / NO-VERDICT / MISSING) naming what the exit code
#       means, the ONE next command, and the raw evidence path — plus the DUEL AMENDMENT: a
#       MISSING reviewer names `ao doctor` + the `--smoke` reviewer-optional lane.
#
# THE LOCK: these are stderr/informational additions ONLY. Verdict semantics + exit codes are
# UNCHANGED — the final @test proves a normal CONFIRMED run is byte-behaviorally identical (exit
# 0, verdict written, no heartbeat/triage noise).
#
# Two levels:
#   UNIT — source pawl-review.sh (the source-guard returns before the running flow) and exercise
#          the pure helpers codex_rc_class / triage_block / start_heartbeat / stop_heartbeat.
#   E2E  — run the real scripts/pawl-review.sh with a codex STUB on PATH (cold path only:
#          PAWL_NO_SERVICE=1, PAWL_AUTOBIND=0), proving heartbeats appear on a slow reviewer and
#          each NO-VERDICT class prints its triage. The real codex CLI is NEVER invoked.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  # Default codex stub: emits a genuine CONFIRMED with the "tokens used" genuine-run marker.
  # Individual tests overwrite $BIN/codex for slow/empty/garbled behavior.
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
cat >/dev/null   # drain the prompt on stdin
printf 'codex\n%s\ntokens used: 1\n' "${CODEX_STUB:-VERDICT: CONFIRMED}"
exit "${CODEX_EXIT:-0}"
STUB
  chmod +x "$BIN/codex"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > README.md; git add README.md; git commit --quiet -m init
  echo change >> README.md; git add README.md
  git commit --quiet -m "feat(x): a change (age-rev-test)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-rev-test.json"
  EVIDENCE="$REPO/.agents/pawl-evidence/age-rev-test-pawl-review.txt"
  export PAWL_NO_SERVICE=1     # force the cold codex path (never route to / stand up a warm pane)
  export PAWL_AUTOBIND=0       # a test run must NEVER create a ledger bind commit
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

# ---------------------------------------------------------------------------
# UNIT — pure helpers (sourced; the source-guard returns before the flow)
# ---------------------------------------------------------------------------

@test "codex_rc_class: maps the lib taxonomy (124=STALL, 125=ECHO, 2=MISSING, else NO-VERDICT)" {
  # shellcheck disable=SC1090
  source "$SCRIPT"
  [ "$(codex_rc_class 124)" = "STALL" ]
  [ "$(codex_rc_class 125)" = "ECHO" ]
  [ "$(codex_rc_class 2)" = "MISSING" ]
  [ "$(codex_rc_class 0)" = "NO-VERDICT" ]
  [ "$(codex_rc_class 99)" = "NO-VERDICT" ]
}

@test "triage_block STALL: names the re-run command + the evidence path, flags NOT-a-REFUTED" {
  # shellcheck disable=SC1090
  source "$SCRIPT"
  bead="age-x"; scope="head"
  run triage_block STALL "/ev/age-x.txt"
  [ "$status" -eq 0 ]
  [[ "$output" == *"NO VERDICT (STALL)"* ]]
  [[ "$output" == *"NOT a REFUTED"* ]]
  [[ "$output" == *"ao pawl review age-x --scope head"* ]]
  [[ "$output" == *"ao doctor"* ]]
  [[ "$output" == *"Raw evidence: /ev/age-x.txt"* ]]
}

@test "triage_block NO-VERDICT: names re-run + ao doctor + the evidence path" {
  # shellcheck disable=SC1090
  source "$SCRIPT"
  bead="age-y"; scope="head"
  run triage_block NO-VERDICT "/ev/age-y.txt"
  [ "$status" -eq 0 ]
  [[ "$output" == *"NO VERDICT (NO-VERDICT)"* ]]
  [[ "$output" == *"ao pawl review age-y --scope head"* ]]
  [[ "$output" == *"Raw evidence: /ev/age-y.txt"* ]]
}

@test "triage_block MISSING (DUEL AMENDMENT): names ao doctor + the --smoke reviewer-optional lane" {
  # shellcheck disable=SC1090
  source "$SCRIPT"
  bead="age-z"; scope="head"
  run triage_block MISSING ""     # no evidence yet at a precondition failure
  [ "$status" -eq 0 ]
  [[ "$output" == *"NO VERDICT (MISSING)"* ]]
  [[ "$output" == *"ao doctor"* ]]
  [[ "$output" == *"--smoke"* ]]                 # the reviewer-optional lane is documented
  [[ "$output" == *"reviewer-OPTIONAL lane"* ]]
  [[ "$output" == *"Raw evidence: n/a"* ]]       # empty arg -> a legible n/a
}

@test "start_heartbeat/stop_heartbeat: spawns a LIVE subshell then kills it (no leak)" {
  # shellcheck disable=SC1090
  source "$SCRIPT"
  PAWL_HEARTBEAT_INTERVAL=1 start_heartbeat 300 codex
  [ -n "$_HB_PID" ]
  kill -0 "$_HB_PID" 2>/dev/null            # ALIVE right after start
  stop_heartbeat "$_HB_PID"
  ! kill -0 "$_HB_PID" 2>/dev/null           # DEAD after stop (killed + reaped: no leak)
  [ -z "$_HB_PID" ]                          # and the handle is cleared
}

@test "start_heartbeat: interval <= 0 DISABLES it (no subshell, no pid)" {
  # shellcheck disable=SC1090
  source "$SCRIPT"
  PAWL_HEARTBEAT_INTERVAL=0 start_heartbeat 300 codex
  [ -z "$_HB_PID" ]
}

# ---------------------------------------------------------------------------
# E2E — the full pawl-review.sh flow, cold path, codex stub
# ---------------------------------------------------------------------------

@test "e2e HEARTBEAT+STALL: a slow, empty reviewer -> heartbeats on stderr + STALL triage names the evidence path" {
  # A stub that SLEEPS then emits NOTHING: with retry disabled the lib classifies STALL (124)
  # after the single ~2s run; the 1s heartbeat fires at least once during it. No `timeout`
  # binary is required (the empty-output path drives STALL), so this is host-independent.
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
cat >/dev/null
sleep 2
exit 0
STUB
  chmod +x "$BIN/codex"
  run env PATH="$BIN:$PATH" PAWL_HEARTBEAT_INTERVAL=1 CODEX_EXEC_RETRY_ON_EMPTY=0 \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 1 ]            # fail-closed no-verdict, NOT a REFUTED (3)
  [ ! -f "$VFILE" ]
  # HEARTBEAT: at least one plain alive-line naming elapsed + budget + the reviewer family.
  [[ "$output" == *"still working"* ]]
  [[ "$output" == *"budget remaining"* ]]
  [[ "$output" == *"reviewer (codex)"* ]]
  # NO cursor/TTY escape codes leaked into the (non-tty) pipe.
  [[ "$output" != *$'\e['* ]]
  # TRIAGE: a STALL block naming the ONE next command + the raw evidence path.
  [[ "$output" == *"NO VERDICT (STALL)"* ]]
  [[ "$output" == *"Raw evidence:"* ]]
  [[ "$output" == *"age-rev-test-pawl-review.txt"* ]]
}

@test "e2e TRIAGE (NO-VERDICT): reviewer returns no parseable verdict -> triage names next cmd + ao doctor + evidence" {
  CODEX_STUB="hmm, I am really not sure about this change" \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 1 ]
  [ ! -f "$VFILE" ]
  [[ "$output" == *"NO VERDICT (NO-VERDICT)"* ]]
  [[ "$output" == *"ao pawl review age-rev-test --scope head"* ]]
  [[ "$output" == *"ao doctor"* ]]
  [[ "$output" == *"age-rev-test-pawl-review.txt"* ]]
}

@test "e2e TRIAGE (STALL): a CONFIRMED from a NON-ZERO reviewer -> STALL triage, exit 1, NO verdict" {
  CODEX_STUB="VERDICT: CONFIRMED" CODEX_EXIT=124 \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 1 ]
  [ ! -f "$VFILE" ]
  [[ "$output" == *"non-zero"* ]]                 # the existing defect-#3 message is preserved
  [[ "$output" == *"NO VERDICT (STALL)"* ]]       # + the triage block
  [[ "$output" == *"age-rev-test-pawl-review.txt"* ]]
}

@test "e2e TRIAGE (MISSING, DUEL AMENDMENT): no reviewer CLI -> triage names ao doctor + --smoke lane, exit 2" {
  # Force the reviewer bin missing deterministically (independent of the host's real codex).
  run env PATH="$BIN:$PATH" CODEX_EXEC_BIN=__no_such_codex_bin_xyz__ \
    bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 2 ]                             # precondition (unchanged), NOT a REFUTED
  [ ! -f "$VFILE" ]
  [[ "$output" == *"MISSING DEPENDENCY"* ]]       # the existing precondition message is preserved
  [[ "$output" == *"NO VERDICT (MISSING)"* ]]     # + the triage block
  [[ "$output" == *"ao doctor"* ]]
  [[ "$output" == *"--smoke"* ]]                  # reviewer-optional lane documented (no bare bounce)
}

@test "e2e LOCK: a normal CONFIRMED run is UNCHANGED — exit 0, verdict written, NO heartbeat/triage noise" {
  command -v jq >/dev/null 2>&1 || skip "jq required to write + verify the verdict"
  CODEX_STUB="VERDICT: CONFIRMED" run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  [ "$(jq -r .disposition "$VFILE")" = "CONFIRMED" ]
  # The ergonomics additions are SILENT on a fast, healthy run (default 30s heartbeat interval
  # never fires; no NO-VERDICT triage on success) — verdict semantics + exit codes are unchanged.
  [[ "$output" != *"still working"* ]]
  [[ "$output" != *"NO VERDICT"* ]]
}
