#!/usr/bin/env bats
# Regression tests for scripts/doctor-evolve.sh (soc-kcy4).
#
# The script resolves repo root via `git rev-parse --show-toplevel`. Each
# test builds an isolated fixture repo + the precondition files (or
# deliberately omits them) and runs the script from inside it.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/doctor-evolve.sh"
  TUNE_SCRIPT_SRC="$REPO_ROOT/scripts/cron-tune-cadence.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"

  git init --quiet --initial-branch=main "$TMP/repo"
  cd "$TMP/repo"
  git config user.email t@t.test
  git config user.name tester
  git commit --quiet --allow-empty -m "initial"
  mkdir -p .agents/evolve .agents/rpi scripts
  # cron-tune-cadence is checked by doctor-evolve; install a working copy
  # so check #3 passes by default.
  if [ -r "$TUNE_SCRIPT_SRC" ]; then
    cp "$TUNE_SCRIPT_SRC" scripts/cron-tune-cadence.sh
    chmod +x scripts/cron-tune-cadence.sh
  fi
  # Minimal GOALS.md so check #1 passes.
  echo '# GOALS' > GOALS.md
  cd "$ORIG_DIR"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

run_dr() {
  cd "$TMP/repo"
  run "$SCRIPT" "$@"
}

# --- Tests ---

@test "all clean fixture passes with exit 0" {
  cat > "$TMP/repo/.agents/evolve/session-state.json" <<EOF
{"session_pr_count":0,"batch_prs":[],"heartbeat_streak":0}
EOF
  cat > "$TMP/repo/.agents/rpi/next-work.jsonl" <<EOF
{"source_epic":"test","items":[],"consumed":false}
EOF
  cat > "$TMP/repo/.agents/evolve/cycle-history.jsonl" <<EOF
{"cycle":1,"result":"productive","ts":"2026-05-20T12:00:00Z"}
EOF
  run_dr
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
  [[ "$output" == *"0 FAIL"* ]]
}

@test "missing GOALS.md is a FAIL" {
  rm -f "$TMP/repo/GOALS.md"
  run_dr
  [ "$status" -eq 1 ]
  [[ "$output" == *"FAIL"* ]]
  [[ "$output" == *"GOALS.md"* ]] || [[ "$output" == *"goals"* ]]
}

@test "session-state.json missing is WARN not FAIL" {
  run_dr
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
  [[ "$output" == *"session-state"* ]]
}

@test "session-state.json malformed is a FAIL" {
  echo "{ not json }" > "$TMP/repo/.agents/evolve/session-state.json"
  run_dr
  [ "$status" -eq 1 ]
  [[ "$output" == *"FAIL"* ]]
  [[ "$output" == *"session-state"* ]]
}

@test "cron-tune-cadence missing is a FAIL" {
  rm -f "$TMP/repo/scripts/cron-tune-cadence.sh"
  run_dr
  [ "$status" -eq 1 ]
  [[ "$output" == *"cron-tune"* ]]
  [[ "$output" == *"FAIL"* ]]
}

@test "STOP marker is reported as WARN" {
  touch "$TMP/repo/.agents/evolve/STOP"
  run_dr
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
  [[ "$output" == *"STOP"* ]] || [[ "$output" == *"markers"* ]]
}

@test "stale in-progress claim is reported as WARN" {
  cat > "$TMP/repo/.agents/rpi/next-work.jsonl" <<EOF
{"items":[],"consumed":false,"claim_status":"in_progress","claimed_at":"2020-01-01T00:00:00Z"}
EOF
  run_dr
  [ "$status" -eq 0 ]
  [[ "$output" == *"stale-claims"* ]] || [[ "$output" == *"WARN"* ]]
  [[ "$output" == *"in_progress"* ]] || [[ "$output" == *"stale"* ]]
}

@test "fresh in-progress claim (today) does NOT trigger stale WARN" {
  now="$(date -u +%FT%TZ)"
  cat > "$TMP/repo/.agents/rpi/next-work.jsonl" <<EOF
{"items":[],"consumed":false,"claim_status":"in_progress","claimed_at":"$now"}
EOF
  run_dr
  [ "$status" -eq 0 ]
  [[ "$output" == *"no claims older than 24h"* ]]
}

@test "cycle-history malformed tail is FAIL" {
  echo "not even close to json" >> "$TMP/repo/.agents/evolve/cycle-history.jsonl"
  run_dr
  [ "$status" -eq 1 ]
  [[ "$output" == *"cycle-history"* ]]
  [[ "$output" == *"FAIL"* ]]
}

@test "next-work.jsonl malformed tail is FAIL" {
  echo "garbage" >> "$TMP/repo/.agents/rpi/next-work.jsonl"
  run_dr
  [ "$status" -eq 1 ]
  [[ "$output" == *"next-work"* ]]
  [[ "$output" == *"FAIL"* ]]
}

@test "--strict promotes any WARN to FAIL exit" {
  # WARN-only fixture (no FAIL): session-state and cycle-history missing.
  run_dr --strict
  [ "$status" -eq 1 ]
  [[ "$output" == *"WARN"* ]]
}

@test "--json output is parseable and matches counts" {
  # Ensure all 7 checks fire by giving the fixture a next-work.jsonl too.
  cat > "$TMP/repo/.agents/rpi/next-work.jsonl" <<EOF
{"items":[],"consumed":false}
EOF
  run_dr --json
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '(.pass + .warn + .fail) == 7' >/dev/null
  echo "$output" | jq -e '.findings | length == 7' >/dev/null
}

@test "--quiet suppresses PASS detail lines, keeps WARN/FAIL" {
  echo "garbage" >> "$TMP/repo/.agents/rpi/next-work.jsonl"
  run_dr --quiet
  [ "$status" -eq 1 ]
  # The summary header always shows "N PASS / N WARN / N FAIL". The quiet
  # contract is "no PER-CHECK PASS rows", not "no occurrence of word PASS".
  pass_rows="$(printf '%s\n' "$output" | grep -c 'PASS [[:alpha:]-]' || true)"
  [ "$pass_rows" -eq 0 ]
  [[ "$output" == *"FAIL"* ]]
}

@test "unknown flag exits 2 with usage error" {
  run_dr --weasel
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown"* ]]
}
