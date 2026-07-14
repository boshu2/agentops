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

admit_zero_cost_review() {
  local run_id="$1"
  python3 "$GOVERNOR" admit \
    --state-dir "$STATE_DIR" \
    --run-id "$run_id" \
    --action semantic-review \
    --reviewer-tokens 0 \
    --elapsed-seconds 0 \
    --review-contexts 0 \
    --deterministic-executions 0
}

assert_corrupt_state_refused() {
  local run_id="$1"
  local filter="$2"
  local state_file="$STATE_DIR/$run_id.json"
  local altered="$BATS_TEST_TMPDIR/$run_id.altered.json"

  init_run "$run_id" >/dev/null
  jq "$filter" "$state_file" >"$altered"
  mv "$altered" "$state_file"
  before="$(shasum -a 256 "$state_file" | awk '{print $1}')"

  run admit_zero_cost_wave "$run_id"
  [ "$status" -ne 0 ]
  [ "$(jq -r '.authorized' <<<"$output")" = "false" ]
  [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
  [ "$(jq -r '.reason' <<<"$output")" = "corrupt-state" ]
  after="$(shasum -a 256 "$state_file" | awk '{print $1}')"
  [ "$before" = "$after" ]
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
    [ "$status" -eq 0 ]

    run python3 "$GOVERNOR" admit \
      --state-dir "$STATE_DIR" --run-id "$spent_id" \
      --action semantic-review \
      --reviewer-tokens 0 --elapsed-seconds 0 \
      --review-contexts 0 --deterministic-executions 0 \
      "$option" 1
    [ "$status" -ne 0 ]
    [ "$(jq -r '.reason' <<<"$output")" = "hard-ceiling:${usage_key}" ]
    [ "$(jq -r '.helper.allowed' <<<"$output")" = "false" ]
  done
}

@test "missing state corrupt state and missing meters are non-authorizing" {
  run admit_zero_cost_wave missing-run
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
  [ "$(jq -r '.authorized' <<<"$output")" = "false" ]

  printf '{broken\n' >"$STATE_DIR/corrupt-run.json"
  run admit_zero_cost_wave corrupt-run
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
  [ "$(jq -r '.authorized' <<<"$output")" = "false" ]

  init_run missing-meter >/dev/null
  run python3 "$GOVERNOR" admit \
    --state-dir "$STATE_DIR" \
    --run-id missing-meter \
    --action semantic-review \
    --reviewer-tokens 1 \
    --elapsed-seconds 1 \
    --review-contexts 1
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
  [ "$(jq -r '.authorized' <<<"$output")" = "false" ]
  [ "$(jq -r '.reason' <<<"$output")" = "missing-meter" ]
}

@test "persisted state is fully schema-conformant and semantically consistent before authorization" {
  assert_corrupt_state_refused corrupt-top-level '.unexpected = true'
  assert_corrupt_state_refused corrupt-authorized '.authorized = "true"'
  assert_corrupt_state_refused corrupt-admission '.admissions = [{"bogus":true}]'
  assert_corrupt_state_refused corrupt-history '.helper_history = {"stuck":{"result":"BOGUS"}}'
  assert_corrupt_state_refused corrupt-schema '.reason = ""'
  assert_corrupt_state_refused corrupt-schema-version '.schema_version = true'
  assert_corrupt_state_refused corrupt-nested '.limits.unexpected = 1'
  assert_corrupt_state_refused corrupt-usage '.usage.reviewer_tokens = 1'
  assert_corrupt_state_refused corrupt-authorization-reason '.reason = "admitted-before-dispatch"'
  assert_corrupt_state_refused corrupt-sequence \
    '.admissions = [{"id":"corrupt-sequence:2","sequence":2,"action":"crank-wave","charge":{"waves":1,"reviewer_tokens":0,"elapsed_seconds":0,"review_contexts":0,"deterministic_executions":0},"status":"recorded"}] | .usage.waves = 1'
  assert_corrupt_state_refused corrupt-bool-sequence \
    '.admissions = [{"id":"corrupt-bool-sequence:1","sequence":true,"action":"crank-wave","charge":{"waves":1,"reviewer_tokens":0,"elapsed_seconds":0,"review_contexts":0,"deterministic_executions":0},"status":"recorded"}] | .usage.waves = 1'
}

@test "only canonical dispositions are accepted" {
  init_run run-dispositions >/dev/null
  for disposition in NOTE REPAIR REPLAN; do
    run python3 "$GOVERNOR" transition \
      --state-dir "$STATE_DIR" --run-id run-dispositions \
      --disposition "$disposition" --reason test
    [ "$status" -eq 0 ]
  done

  for disposition in HOLD ANDON; do
    run python3 "$GOVERNOR" transition \
      --state-dir "$STATE_DIR" --run-id run-dispositions \
      --disposition "$disposition" --reason illegal
    [ "$status" -ne 0 ]
    [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
    [ "$(jq -r '.disposition' "$STATE_DIR/run-dispositions.json")" = "REPLAN" ]
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
    [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
    [ "$(jq -r '.disposition' "$STATE_DIR/$run_id.json")" = "REPAIR" ]
  done
}

@test "generic transitions cannot bypass HOLD or ANDON and explicit authorities own exits" {
  init_run protected-hold >/dev/null
  python3 "$GOVERNOR" break \
    --state-dir "$STATE_DIR" --run-id protected-hold \
    --kind max-attempts --blocker-class same-failure >/dev/null
  [ "$(jq -r '.helper.blocker_class' "$STATE_DIR/protected-hold.json")" = "same-failure" ]

  run python3 "$GOVERNOR" transition \
    --state-dir "$STATE_DIR" --run-id protected-hold \
    --disposition NOTE --reason bypass
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
  [ "$(jq -r '.disposition' "$STATE_DIR/protected-hold.json")" = "HOLD" ]
  [ "$(jq -r '.helper.allowed' "$STATE_DIR/protected-hold.json")" = "true" ]
  [ "$(jq -r '.helper.blocker_class' "$STATE_DIR/protected-hold.json")" = "same-failure" ]

  run python3 "$GOVERNOR" helper \
    --state-dir "$STATE_DIR" --run-id protected-hold \
    --blocker-class same-failure --result UNSTUCK \
    --new-approach "use a different implementation boundary"
  [ "$status" -eq 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "REPAIR" ]

  init_run protected-andon >/dev/null
  python3 "$GOVERNOR" break \
    --state-dir "$STATE_DIR" --run-id protected-andon \
    --kind human-judgment --blocker-class operator-choice >/dev/null
  run python3 "$GOVERNOR" transition \
    --state-dir "$STATE_DIR" --run-id protected-andon \
    --disposition REPAIR --reason bypass
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' "$STATE_DIR/protected-andon.json")" = "ANDON" ]

  run python3 "$GOVERNOR" human \
    --state-dir "$STATE_DIR" --run-id protected-andon \
    --disposition REPAIR --reason "operator supplied authority"
  [ "$status" -eq 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "REPAIR" ]
  run admit_zero_cost_review protected-andon
  [ "$status" -eq 0 ]
}

@test "malformed control input and malformed HOLD metadata refuse without manufacturing ANDON" {
  init_run malformed-control >/dev/null
  run python3 "$GOVERNOR" break \
    --state-dir "$STATE_DIR" --run-id malformed-control \
    --kind max-attempts
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
  [ "$(jq -r '.disposition' "$STATE_DIR/malformed-control.json")" = "NOTE" ]

  python3 "$GOVERNOR" break \
    --state-dir "$STATE_DIR" --run-id malformed-control \
    --kind max-attempts --blocker-class repair-loop >/dev/null
  run python3 "$GOVERNOR" helper \
    --state-dir "$STATE_DIR" --run-id malformed-control \
    --blocker-class repair-loop --result UNSTUCK
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
  [ "$(jq -r '.disposition' "$STATE_DIR/malformed-control.json")" = "HOLD" ]

  init_run malformed-hold >/dev/null
  jq '.disposition = "HOLD" | .reason = "max-attempts" | .helper = {"allowed":true}' \
    "$STATE_DIR/malformed-hold.json" >"$BATS_TEST_TMPDIR/malformed-hold.json"
  mv "$BATS_TEST_TMPDIR/malformed-hold.json" "$STATE_DIR/malformed-hold.json"
  run admit_zero_cost_wave malformed-hold
  [ "$status" -ne 0 ]
  [ "$(jq -r '.disposition' <<<"$output")" = "NOTE" ]
  [ "$(jq -r '.authorized' <<<"$output")" = "false" ]
}

@test "three admitted Crank waves do not block a zero-wave semantic review" {
  init_run validate-after-three >/dev/null
  admit_zero_cost_wave validate-after-three >/dev/null
  admit_zero_cost_wave validate-after-three >/dev/null
  admit_zero_cost_wave validate-after-three >/dev/null

  run python3 "$GOVERNOR" admit \
    --state-dir "$STATE_DIR" --run-id validate-after-three \
    --action semantic-review \
    --reviewer-tokens 1 --elapsed-seconds 1 \
    --review-contexts 1 --deterministic-executions 1
  [ "$status" -eq 0 ]
  [ "$(jq -r '.usage.waves' <<<"$output")" -eq 3 ]
  [ "$(jq -r '.admissions[-1].charge.waves' <<<"$output")" -eq 0 ]
  [ "$(jq -r '.admissions[-1].action' <<<"$output")" = "semantic-review" ]
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

@test "all authoritative Crank and RPI references contain no private phase controller" {
  run rg -n -i \
    'MAX_EPIC_WAVES|wave=0|wave=\$\(\(wave|\$wave -ge 50|global wave limit \(50\)|max budget per task: 2|retry once|max 2|max 3 total attempts|--max-cycles|3 validation failures|3\+ failures|after 3 failures|max 2 attempts|after 2 attempts|max 2 retries|after 2 retries|Retry \$RETRY_COUNT/2|Premortem failed 3x|retry limit|MAX_RETRIES|Attempts: 3/3|attempt: 1/3|Attempt counter: 2/3|--budget=' \
    "$REPO_ROOT/skills/rpi/SKILL.md" \
    "$REPO_ROOT/skills/crank/SKILL.md" \
    "$REPO_ROOT/skills/rpi/references" \
    "$REPO_ROOT/skills/crank/references"
  [ "$status" -eq 1 ]
}
