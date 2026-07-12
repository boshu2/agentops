#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_ROOT="$(cd "$SKILL_DIR/../.." && pwd)"
HELPER="$SKILL_DIR/scripts/mvp-helper-state.sh"
SCHEMA="$REPO_ROOT/schemas/execution-packet.schema.json"
AGGREGATE="$REPO_ROOT/cli/internal/domain/packet/aggregate.go"
PACKET_FIXTURE="$REPO_ROOT/cli/cmd/ao/testdata/live-execution-packet.json"
TMP="$(mktemp -d)"
trap 'rm -rf -- "$TMP"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

write_state() {
  printf '%s\n' '{"goal":"fixture","phase":1,"cycle":1,"start_phase":1,"verdicts":{},"attempts":{}}' >"$1"
}

assert_no_dispatch_on_claim_failure() {
  local failure_bin="$1"
  local state="$TMP/state-$failure_bin.json" marker="$TMP/dispatch-$failure_bin"
  write_state "$state"
  if [[ "$failure_bin" == jq ]]; then
    if DISCOVERY_JQ_BIN=false bash "$HELPER" claim "$state" >/dev/null 2>&1; then
      : >"$marker"
    fi
  else
    if DISCOVERY_MV_BIN=false bash "$HELPER" claim "$state" >/dev/null 2>&1; then
      : >"$marker"
    fi
  fi
  [[ ! -e "$marker" ]] || fail "helper dispatched after forced $failure_bin claim failure"
}

assert_no_continue_on_transition_failure() {
  local failure_bin="$1"
  local state="$TMP/state-transition-$failure_bin.json" marker="$TMP/continued-$failure_bin"
  write_state "$state"
  bash "$HELPER" claim "$state" >/dev/null
  if [[ "$failure_bin" == jq ]]; then
    if DISCOVERY_JQ_BIN=false bash "$HELPER" transition "$state" UNSTUCK >/dev/null 2>&1; then
      : >"$marker"
    fi
  else
    if DISCOVERY_MV_BIN=false bash "$HELPER" transition "$state" UNSTUCK >/dev/null 2>&1; then
      : >"$marker"
    fi
  fi
  [[ ! -e "$marker" ]] || fail "continued after forced $failure_bin transition failure"
}

assert_resume_guard_is_single_use() {
  local state="$TMP/state-resume.json" marker="$TMP/second-dispatch"
  write_state "$state"
  bash "$HELPER" claim "$state" >/dev/null
  if bash "$HELPER" claim "$state" >/dev/null 2>&1; then
    : >"$marker"
  fi
  [[ ! -e "$marker" ]] || fail "second helper claim succeeded"
  jq -e '.attempts.discovery_mvp_helper == 1 and .verdicts.discovery_mvp_helper == "ESCALATE"' "$state" >/dev/null \
    || fail "resume guard did not persist single-use ESCALATE"
}

assert_concurrent_claim_has_one_winner() {
  local state="$TMP/state-concurrent.json" run_dir="$TMP/concurrent"
  local i=1 claimed dispatches winners=0 rc
  mkdir -p "$run_dir"
  write_state "$state"

  while (( i <= 80 )); do
    (
      if bash "$HELPER" claim "$state" >"$run_dir/out.$i" 2>"$run_dir/err.$i"; then
        : >"$run_dir/dispatch.$i"
        echo 0 >"$run_dir/rc.$i"
      else
        echo "$?" >"$run_dir/rc.$i"
      fi
    ) &
    i=$((i + 1))
  done
  wait

  claimed="$( { grep -l '^CLAIMED$' "$run_dir"/out.* || true; } | wc -l | tr -d ' ')"
  dispatches="$(find "$run_dir" -type f -name 'dispatch.*' | wc -l | tr -d ' ')"
  [[ "$claimed" -eq 1 ]] || fail "concurrent claim produced $claimed CLAIMED results (want 1)"
  [[ "$dispatches" -eq 1 ]] || fail "concurrent claim produced $dispatches dispatches (want 1)"

  for rc_file in "$run_dir"/rc.*; do
    rc="$(cat "$rc_file")"
    case "$rc" in
      0) winners=$((winners + 1)) ;;
      4) ;;
      *) fail "concurrent loser returned unexpected rc=$rc" ;;
    esac
  done
  [[ "$winners" -eq 1 ]] || fail "concurrent claim had $winners successful return codes (want 1)"
  jq -e '.attempts.discovery_mvp_helper == 1' "$state" >/dev/null \
    || fail "concurrent claim did not persist attempt=1"
}

assert_lock_contention_fails_closed() {
  local state="$TMP/state-locked.json" marker="$TMP/locked-dispatch"
  local lock_dir="${state}.mvp-helper.lock"
  write_state "$state"
  mkdir -p "$lock_dir"
  printf 'pid=fixture started=1970-01-01T00:00:00Z\n' >"$lock_dir/owner"
  if DISCOVERY_LOCK_ATTEMPTS=2 DISCOVERY_LOCK_DELAY=0.001 \
    bash "$HELPER" claim "$state" >/dev/null 2>&1; then
    : >"$marker"
  fi
  [[ ! -e "$marker" ]] || fail "helper dispatched while lock was held"
  [[ -d "$lock_dir" ]] || fail "helper auto-broke a lock it did not own"
  grep -q '^pid=fixture ' "$lock_dir/owner" \
    || fail "helper changed another owner's lock metadata"
}

assert_packet_schema_and_aggregate_support() {
  local positive="$TMP/packet-positive.json" negative="$TMP/packet-negative.json"
  jq '. + {
    discovery_artifacts:[".agents/duel/run/mvp-helper/"],
    evaluator_artifacts:{discovery_mvp_helper:".agents/duel/run/mvp-helper/decision.json"}
  }' "$PACKET_FIXTURE" >"$positive"
  python3 -m jsonschema -i "$positive" "$SCHEMA" >/dev/null 2>&1 \
    || fail "canonical schema rejected supported helper artifact fields"
  grep -q 'DiscoveryArtifacts.*json:"discovery_artifacts,omitempty"' "$AGGREGATE" \
    || fail "Go aggregate lacks discovery_artifacts"
  grep -q 'EvaluatorArtifacts.*json:"evaluator_artifacts,omitempty"' "$AGGREGATE" \
    || fail "Go aggregate lacks evaluator_artifacts"

  jq '.invented_helper_field=true' "$positive" >"$negative"
  if python3 -m jsonschema -i "$negative" "$SCHEMA" >/dev/null 2>&1; then
    fail "canonical schema accepted invented_helper_field"
  fi
}

assert_no_dispatch_on_claim_failure jq
assert_no_dispatch_on_claim_failure mv
assert_no_continue_on_transition_failure jq
assert_no_continue_on_transition_failure mv
assert_resume_guard_is_single_use
assert_concurrent_claim_has_one_winner
assert_lock_contention_fails_closed
assert_packet_schema_and_aggregate_support

echo "Discovery helper state and packet contract fixtures passed."
