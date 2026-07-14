#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  GOVERNOR="$REPO_ROOT/skills/rpi/scripts/run-governor.py"
  STATE_DIR="$BATS_TEST_TMPDIR/state"
  mkdir -p "$STATE_DIR"
}

init_run() {
  local run_id="$1"
  shift
  python3 "$GOVERNOR" init \
    --state-dir "$STATE_DIR" \
    --run-id "$run_id" \
    --max-reviewer-tokens 100 \
    --max-elapsed-seconds 100 \
    --max-review-contexts 3 \
    --max-deterministic-executions 4 \
    "$@"
}

admit_zero_cost_wave() {
  local run_id="$1"
  python3 "$GOVERNOR" admit \
    --state-dir "$STATE_DIR" \
    --run-id "$run_id" \
    --action crank-wave \
    --reviewer-tokens 0 \
    --elapsed-seconds 0 \
    --review-contexts 0 \
    --deterministic-executions 0
}

@test "fresh processes resume one run and admit exactly three default waves" {
  run init_run run-resume
  [ "$status" -eq 0 ]

  run admit_zero_cost_wave run-resume
  [ "$status" -eq 0 ]
  run admit_zero_cost_wave run-resume
  [ "$status" -eq 0 ]
  run admit_zero_cost_wave run-resume
  [ "$status" -eq 0 ]
  [ "$(jq -r '.usage.waves' "$STATE_DIR/run-resume.json")" -eq 3 ]

  run admit_zero_cost_wave run-resume
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "ANDON" ]
  [ "$(jq -r '.helper.allowed' <<<"$output")" = "false" ]
  [ "$(jq -r '.usage.waves' "$STATE_DIR/run-resume.json")" -eq 3 ]
}

@test "admission is durably recorded before dispatch and concurrent callers cannot oversubscribe" {
  init_run run-atomic >/dev/null
  rc_dir="$BATS_TEST_TMPDIR/rc"
  mkdir -p "$rc_dir"

  for n in 1 2 3 4 5; do
    (
      if admit_zero_cost_wave run-atomic >"$rc_dir/$n.json"; then
        printf '0\n' >"$rc_dir/$n.rc"
      else
        printf '%s\n' "$?" >"$rc_dir/$n.rc"
      fi
    ) &
  done
  wait

  [ "$(jq -r '.usage.waves' "$STATE_DIR/run-atomic.json")" -eq 3 ]
  [ "$(jq '[.admissions[] | select(.status == "recorded")] | length' "$STATE_DIR/run-atomic.json")" -eq 3 ]
  [ "$(awk '$1 == 0 {n++} END {print n+0}' "$rc_dir"/*.rc)" -eq 3 ]
}

@test "run IDs have independent persistent counters" {
  init_run run-a >/dev/null
  init_run run-b >/dev/null
  admit_zero_cost_wave run-a >/dev/null
  admit_zero_cost_wave run-a >/dev/null
  admit_zero_cost_wave run-b >/dev/null

  [ "$(jq -r '.usage.waves' "$STATE_DIR/run-a.json")" -eq 2 ]
  [ "$(jq -r '.usage.waves' "$STATE_DIR/run-b.json")" -eq 1 ]
}

@test "every declared wave and hard-cost meter ceiling fails closed" {
  local meter option usage_key ceiling
  for meter in reviewer-tokens elapsed-seconds review-contexts deterministic-executions; do
    run_id="meter-${meter}"
    init_run "$run_id" >/dev/null
    option="--${meter}"
    case "$meter" in
      reviewer-tokens) usage_key="reviewer_tokens"; ceiling=100 ;;
      elapsed-seconds) usage_key="elapsed_seconds"; ceiling=100 ;;
      review-contexts) usage_key="review_contexts"; ceiling=3 ;;
      deterministic-executions) usage_key="deterministic_executions"; ceiling=4 ;;
    esac

    run python3 "$GOVERNOR" admit \
      --state-dir "$STATE_DIR" \
      --run-id "$run_id" \
      --action semantic-review \
      --reviewer-tokens 0 \
      --elapsed-seconds 0 \
      --review-contexts 0 \
      --deterministic-executions 0 \
      "$option" 101
    [ "$status" -ne 0 ]
    [ "$(jq -r '.disposition' <<<"$output")" = "ANDON" ]
    [ "$(jq -r '.reason' <<<"$output")" = "hard-ceiling:${usage_key}" ]
    [ "$(jq -r '.helper.allowed' <<<"$output")" = "false" ]

    spent_id="spent-${meter}"
    init_run "$spent_id" >/dev/null
    python3 "$GOVERNOR" admit \
      --state-dir "$STATE_DIR" --run-id "$spent_id" \
      --action semantic-review \
      --reviewer-tokens 0 --elapsed-seconds 0 \
      --review-contexts 0 --deterministic-executions 0 \
      "$option" "$ceiling" >/dev/null
    run python3 "$GOVERNOR" admit \
      --state-dir "$STATE_DIR" --run-id "$spent_id" \
      --action semantic-review \
      --reviewer-tokens 0 --elapsed-seconds 0 \
      --review-contexts 0 --deterministic-executions 0
    [ "$status" -ne 0 ]
    [ "$(jq -r '.reason' <<<"$output")" = "hard-ceiling:${usage_key}" ]
    [ "$(jq -r '.helper.allowed' <<<"$output")" = "false" ]
  done
}

@test "missing state corrupt state and missing meters are non-authorizing" {
  run admit_zero_cost_wave missing-run
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "ANDON" ]

  printf '{broken\n' >"$STATE_DIR/corrupt-run.json"
  run admit_zero_cost_wave corrupt-run
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "ANDON" ]

  init_run missing-meter >/dev/null
  run python3 "$GOVERNOR" admit \
    --state-dir "$STATE_DIR" \
    --run-id missing-meter \
    --action semantic-review \
    --reviewer-tokens 1 \
    --elapsed-seconds 1 \
    --review-contexts 1
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "ANDON" ]
  [ "$(jq -r '.reason' <<<"$output")" = "missing-meter" ]
}

@test "only canonical dispositions are accepted" {
  init_run run-dispositions >/dev/null
  for disposition in NOTE REPAIR REPLAN HOLD ANDON; do
    run python3 "$GOVERNOR" transition \
      --state-dir "$STATE_DIR" --run-id run-dispositions \
      --disposition "$disposition" --reason test
    [ "$status" -eq 0 ]
  done

  run python3 "$GOVERNOR" transition \
    --state-dir "$STATE_DIR" --run-id run-dispositions \
    --disposition RETRY --reason invalid
  [ "$status" -ne 0 ]
}

@test "stuck breakers HOLD for one helper and UNSTUCK resumes with a new approach" {
  for breaker in max-attempts oscillation no-progress; do
    run_id="breaker-${breaker}"
    init_run "$run_id" >/dev/null
    run python3 "$GOVERNOR" break \
      --state-dir "$STATE_DIR" --run-id "$run_id" \
      --kind "$breaker" --blocker-class repeated-failure
    [ "$status" -eq 0 ]
    [ "$(jq -r '.disposition' <<<"$output")" = "HOLD" ]
    [ "$(jq -r '.helper.allowed' <<<"$output")" = "true" ]

    run python3 "$GOVERNOR" helper \
      --state-dir "$STATE_DIR" --run-id "$run_id" \
      --blocker-class repeated-failure --result UNSTUCK \
      --new-approach "change the failing approach"
    [ "$status" -eq 0 ]
    [ "$(jq -r '.disposition' <<<"$output")" = "REPAIR" ]
    [ "$(jq -r '.helper.result' <<<"$output")" = "UNSTUCK" ]
    [ "$(jq -r '.helper.new_approach' <<<"$output")" = "change the failing approach" ]

    run python3 "$GOVERNOR" helper \
      --state-dir "$STATE_DIR" --run-id "$run_id" \
      --blocker-class repeated-failure --result UNSTUCK \
      --new-approach "try again"
    [ "$status" -ne 0 ]
    [ "$(jq -r '.disposition' <<<"$output")" = "ANDON" ]
  done
}

@test "helper ESCALATE and human-only judgment reach ANDON" {
  init_run helper-escalate >/dev/null
  python3 "$GOVERNOR" break \
    --state-dir "$STATE_DIR" --run-id helper-escalate \
    --kind max-attempts --blocker-class stuck >/dev/null
  run python3 "$GOVERNOR" helper \
    --state-dir "$STATE_DIR" --run-id helper-escalate \
    --blocker-class stuck --result ESCALATE
  [ "$status" -eq 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "ANDON" ]

  run admit_zero_cost_wave helper-escalate
  [ "$status" -ne 0 ]
  [ "$(jq -r '.authorized' <<<"$output")" = "false" ]

  init_run human-only >/dev/null
  run python3 "$GOVERNOR" break \
    --state-dir "$STATE_DIR" --run-id human-only \
    --kind human-judgment --blocker-class authority
  [ "$status" -eq 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "ANDON" ]
  [ "$(jq -r '.helper.allowed' <<<"$output")" = "false" ]

  run admit_zero_cost_wave human-only
  [ "$status" -ne 0 ]
  [ "$(jq -r '.authorized' <<<"$output")" = "false" ]
}

@test "Crank and RPI contracts contain no phase-local wave retry or helper multiplier" {
  run rg -n \
    'MAX_EPIC_WAVES|wave=0|wave=\$\(\(wave|Budget: 2 per task|3 total attempts before|RPI_MAX_WAVES' \
    "$REPO_ROOT/skills/rpi/SKILL.md" \
    "$REPO_ROOT/skills/crank/SKILL.md" \
    "$REPO_ROOT/skills/crank/references/execution-preflight.md" \
    "$REPO_ROOT/skills/crank/references/wave-dispatch.md"
  [ "$status" -eq 1 ]
}
