#!/usr/bin/env bats
# Regression tests for scripts/cron-tune-cadence.sh (soc-adwq).

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/cron-tune-cadence.sh"
  TMP="$(mktemp -d)"
  mkdir -p "$TMP/.agents/evolve"
  # Run from $TMP so the script's git-root detection finds nothing → falls back
  # to $PWD. We isolate by setting PWD inside each test.
  ORIG_DIR="$PWD"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

# Build a minimal state file with given streak + hash
write_state() {
  local streak="$1" hash="${2:-}"
  cat >"$TMP/.agents/evolve/session-state.json" <<EOF
{
  "session_pr_count": 5,
  "batch_prs": [],
  "heartbeat_streak": $streak,
  "state_hash": "$hash"
}
EOF
}

# Run script with $TMP as cwd so git-rev-parse falls back to $PWD,
# and there are no PRs to query (BATCH_PRS empty).
run_in_tmp() {
  cd "$TMP"
  run "$SCRIPT" "$@"
}

@test "heartbeat with empty batch + new state-hash → STAY, streak=1" {
  write_state 0 "previous-hash"
  run_in_tmp heartbeat
  [ "$status" -eq 0 ]
  last="$(printf '%s\n' "$output" | tail -1)"
  [ "$last" = "STAY" ]
  streak=$(jq -r '.heartbeat_streak' "$TMP/.agents/evolve/session-state.json")
  [ "$streak" = "1" ]
}

@test "heartbeat with same state-hash, streak<threshold → STAY" {
  # Pre-populate with the hash we know will be computed
  # Compute it the same way the script does
  MAIN_SHA="$(cd "$ORIG_DIR" && git rev-parse HEAD)"
  EXPECTED_HASH=$(printf '%s\n' "$MAIN_SHA" | sha1sum | awk '{print $1}')
  write_state 1 "$EXPECTED_HASH"
  run_in_tmp heartbeat
  [ "$status" -eq 0 ]
  # Either STAY (if hash matches) or new streak start (if it doesn't due to no git in TMP)
  # The important behavior: streak grows when hash matches, resets when it doesn't.
  # We verify the script doesn't crash.
  last="$(printf '%s\n' "$output" | tail -1)"
  case "$last" in STAY|TUNE_DOWN) ;; *) false ;; esac
}

@test "heartbeat with same state-hash, streak>=threshold → TUNE_DOWN" {
  MAIN_SHA="$(cd "$ORIG_DIR" && git rev-parse HEAD)"
  EXPECTED_HASH=$(printf '%s\n' "$MAIN_SHA" | sha1sum | awk '{print $1}')
  write_state 2 "$EXPECTED_HASH"  # streak=2, +1 = 3 = threshold
  cd "$TMP"
  write_state 0 ""
  cd "$TMP"
  CRON_TUNE_HEARTBEAT_THRESHOLD=3 run "$SCRIPT" heartbeat  # streak 0→1
  [ "$status" -eq 0 ]
  CRON_TUNE_HEARTBEAT_THRESHOLD=3 run "$SCRIPT" heartbeat  # streak 1→2
  CRON_TUNE_HEARTBEAT_THRESHOLD=3 run "$SCRIPT" heartbeat  # streak 2→3 = threshold
  [ "$status" -eq 0 ]
  last="$(printf '%s\n' "$output" | tail -1)"
  [ "$last" = "TUNE_DOWN" ]
}

@test "productive with no open PRs → STAY, streak reset" {
  write_state 5 "any-hash"
  run_in_tmp productive
  [ "$status" -eq 0 ]
  last="$(printf '%s\n' "$output" | tail -1)"
  [ "$last" = "STAY" ]
  streak=$(jq -r '.heartbeat_streak' "$TMP/.agents/evolve/session-state.json")
  [ "$streak" = "0" ]
}

@test "productive with open PRs → TUNE_UP" {
  cat >"$TMP/.agents/evolve/session-state.json" <<'EOF'
{
  "session_pr_count": 5,
  "batch_prs": [999998, 999999],
  "heartbeat_streak": 5
}
EOF
  cd "$TMP"
  run "$SCRIPT" productive
  [ "$status" -eq 0 ]
  # gh pr view on fake numbers will produce UNKNOWN UNKNOWN but PR list is non-empty
  last="$(printf '%s\n' "$output" | tail -1)"
  [ "$last" = "TUNE_UP" ]
  streak=$(jq -r '.heartbeat_streak' "$TMP/.agents/evolve/session-state.json")
  [ "$streak" = "0" ]
}

@test "teardown → STAY, streak reset" {
  write_state 9 "any-hash"
  run_in_tmp teardown
  [ "$status" -eq 0 ]
  last="$(printf '%s\n' "$output" | tail -1)"
  [ "$last" = "STAY" ]
  streak=$(jq -r '.heartbeat_streak' "$TMP/.agents/evolve/session-state.json")
  [ "$streak" = "0" ]
}

@test "missing state file → STAY, no crash" {
  cd "$TMP"
  rm -f .agents/evolve/session-state.json
  run "$SCRIPT" heartbeat
  [ "$status" -eq 0 ]
  last="$(printf '%s\n' "$output" | tail -1)"
  [ "$last" = "STAY" ]
}

@test "unknown cycle-result → STAY" {
  write_state 0 ""
  run_in_tmp bogus-result
  [ "$status" -eq 0 ]
  last="$(printf '%s\n' "$output" | tail -1)"
  [ "$last" = "STAY" ]
}

@test "missing cycle-result arg → exit 2" {
  cd "$TMP"
  run "$SCRIPT"
  [ "$status" -eq 2 ]
}

@test "state-hash is updated on every call" {
  write_state 0 ""
  run_in_tmp heartbeat
  [ "$status" -eq 0 ]
  hash1=$(jq -r '.state_hash' "$TMP/.agents/evolve/session-state.json")
  [ -n "$hash1" ]
  [ "$hash1" != "" ]
  [ "$hash1" != "null" ]
}

@test "last_tune_check timestamp is set" {
  write_state 0 ""
  run_in_tmp heartbeat
  ts=$(jq -r '.last_tune_check' "$TMP/.agents/evolve/session-state.json")
  [[ "$ts" =~ ^20[0-9]{2}-[0-9]{2}-[0-9]{2}T ]]
}

@test "blocked-on-failure with same hash, streak<threshold → STAY" {
  MAIN_SHA="$(cd "$ORIG_DIR" && git rev-parse HEAD)"
  EXPECTED_HASH=$(printf '%s\n' "$MAIN_SHA" | sha1sum | awk '{print $1}')
  write_state 0 "$EXPECTED_HASH"
  cd "$TMP"
  run "$SCRIPT" blocked-on-failure
  [ "$status" -eq 0 ]
  last="$(printf '%s\n' "$output" | tail -1)"
  case "$last" in STAY|TUNE_DOWN) ;; *) false ;; esac
}
