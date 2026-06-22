#!/usr/bin/env bats
# mine-all-sessions.bats — acceptance for E6.4 (age-membrane-memory-arch-tz2s.6.4),
# the runner that drives the BUILT `ao provenance mine-session` miner over a real
# session corpus and populates the LOCAL, gitignored provenance event store.
#
# These assert the RUNNER's contract (the miner's own parsing is covered by
# cli/cmd/ao/provenance_mine_session_test.go): a real session populates the store,
# a re-run is incremental (idempotent via --state, not re-mining), the summary
# artifact is valid, and the single-writer lock holds. The store path is relative
# to CWD, so every test runs inside an isolated temp project — the real repo is
# never touched.
#
# Integration by nature: SKIPS only on a thin environment (go absent — cannot build
# the shipped ao). It NEVER skips on a build failure (loud RED), matching
# em-loop-donetest.bats.

setup_file() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export REPO_ROOT
  command -v go >/dev/null 2>&1 || return 0 # per-test skip handles the thin env
  # Build the shipped ao ONCE for the whole file (expensive; reused by every test).
  AO_BIN="$BATS_FILE_TMPDIR/ao"
  ( cd "$REPO_ROOT/cli" && go build -o "$AO_BIN" ./cmd/ao ) || { echo "go build ao FAILED" >&3; return 1; }
  export AO_BIN
}

setup() {
  SCRIPT="$REPO_ROOT/scripts/provenance/mine-all-sessions.sh"
  command -v go >/dev/null 2>&1 || skip "go not available (thin env — cannot build the shipped ao binary)"
  [ -f "$SCRIPT" ] || { echo "runner missing: $SCRIPT"; false; }
  [ -x "$AO_BIN" ] || { echo "ao binary missing (setup_file build failed): $AO_BIN"; false; }
  # Isolated work dir: the store is CWD-relative, so this keeps it out of the repo.
  WORK="$BATS_TEST_TMPDIR/work"
  SESS="$BATS_TEST_TMPDIR/sessions"
  mkdir -p "$WORK" "$SESS"
  # A synthetic session with two real tool_use blocks (Read + Bash) and one
  # tool_result (an OUTPUT — must NOT become an event). Mirrors the format the
  # miner's own unit test uses.
  cat >"$SESS/sess-a.jsonl" <<'JSONL'
{"type":"user","message":{"role":"user","content":"go"}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{}},{"type":"tool_result","content":"x"}]}}
{"type":"tool_use","tool_name":"Bash","tool_input":{"command":"ls"}}
JSONL
}

@test "mine-all: a real session populates the local event store with tool_call events" {
  cd "$WORK"
  run env AGENTOPS_AO_BIN="$AO_BIN" bash "$SCRIPT" "$SESS"
  [ "$status" -eq 0 ]
  [[ "$output" == *"mine-all"* ]]
  # The store exists, is gitignored-by-convention (.agents/provenance/), and holds
  # the mined per-inference events.
  store="$WORK/.agents/provenance/mine-events.jsonl"
  [ -f "$store" ]
  # Two tool_use blocks -> two tool_call events; the tool_result is filtered out.
  n="$(grep -c '"kind":"tool_call"' "$store")"
  [ "$n" -eq 2 ]
  # No tool_result leaked in as an event.
  ! grep -q '"name":"tool_result"' "$store"
}

@test "mine-all: re-run is incremental (idempotent) — already-mined events are not re-appended" {
  cd "$WORK"
  env AGENTOPS_AO_BIN="$AO_BIN" bash "$SCRIPT" "$SESS" >/dev/null
  store="$WORK/.agents/provenance/mine-events.jsonl"
  before="$(grep -c '"schema_version"' "$store")"
  # Second run over the SAME unchanged session: --state has advanced, so 0 new.
  run env AGENTOPS_AO_BIN="$AO_BIN" bash "$SCRIPT" "$SESS"
  [ "$status" -eq 0 ]
  [[ "$output" == *"+0 new event"* ]]
  after="$(grep -c '"schema_version"' "$store")"
  [ "$before" -eq "$after" ]
}

@test "mine-all: writes a valid summary artifact with reconciled counts" {
  cd "$WORK"
  env AGENTOPS_AO_BIN="$AO_BIN" bash "$SCRIPT" "$SESS" >/dev/null
  summary="$WORK/.agents/provenance/mine-summary.json"
  [ -f "$summary" ]
  run jq -e '.sessions_mined==1 and .new_events==2 and .store_total==2 and .failed==0' "$summary"
  [ "$status" -eq 0 ]
}

@test "mine-all: a held lock makes a concurrent miner exit 2 (single writer)" {
  cd "$WORK"
  mkdir -p .agents/provenance
  mkdir .agents/provenance/.mine.lock # simulate another miner holding the lock
  run env AGENTOPS_AO_BIN="$AO_BIN" bash "$SCRIPT" "$SESS"
  [ "$status" -eq 2 ]
  [[ "$output" == *"another miner holds the lock"* ]]
}

@test "mine-all: a missing sessions dir is a hard error (exit 1), not a silent no-op" {
  cd "$WORK"
  run env AGENTOPS_AO_BIN="$AO_BIN" bash "$SCRIPT" "$BATS_TEST_TMPDIR/does-not-exist"
  [ "$status" -eq 1 ]
  [[ "$output" == *"no sessions dir"* ]]
}

@test "mine-all: a per-session miner failure exits 1, counts it, and does NOT advance state (re-run re-mines)" {
  # Stub miner that ADVANCES the --state file then FAILS — the exact data-loss shape the
  # pawl flagged: a newly-created advanced state with no backup must be rolled back (removed)
  # so the failed session re-mines, and the run must exit non-zero (not false-green exit 0).
  stub="$BATS_TEST_TMPDIR/ao-fail"
  cat >"$stub" <<'STUB'
#!/usr/bin/env bash
# args: provenance mine-session --file <f> --state <state> --json
state=""
while [ $# -gt 0 ]; do [ "$1" = "--state" ] && { state="$2"; shift; }; shift; done
[ -n "$state" ] && echo '{"cursor":99}' > "$state"   # advance state, as the real miner would
exit 1                                                 # ...then FAIL before emitting events
STUB
  chmod +x "$stub"
  cd "$WORK"
  run env AGENTOPS_AO_BIN="$stub" bash "$SCRIPT" "$SESS"
  [ "$status" -eq 1 ]                                   # exit reflects the failure (defect 1)
  [[ "$output" == *"(1 failed)"* ]]
  # The advanced state was rolled back (removed) — not left to skip never-stored events.
  [ ! -f "$WORK/.agents/provenance/state/sess-a.json" ]
  # Nothing was appended to the store.
  [ ! -s "$WORK/.agents/provenance/mine-events.jsonl" ] || ! grep -q . "$WORK/.agents/provenance/mine-events.jsonl"
}
