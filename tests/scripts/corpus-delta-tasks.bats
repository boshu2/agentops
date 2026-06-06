#!/usr/bin/env bats
# ag-nfux (ag-8p8o W1b): the held corpus-delta task set (operator-approved: cd-ci-1, cd-ci-4).
# These cases verify the TASK FIXTURES THEMSELVES are well-formed: each grader must
# DISCRIMINATE — the golden reference solution scores pass, an empty workspace scores fail.
# A grader that always passes (or always fails) would make the A/B meaningless.

TASKS="$BATS_TEST_DIRNAME/../../evals/workbench/tasks"

run_task_with_solution() {
  local task="$1" outfile="$2" solution="$3"   # solution: path to drop in, or "" for none
  local wd; wd="$BATS_TEST_TMPDIR/$task-$RANDOM"
  bash "$TASKS/$task/setup.sh" "$wd"
  if [[ -n "$solution" ]]; then cp "$TASKS/$task/$solution" "$wd/$outfile"; fi
  bash "$TASKS/$task/score.sh" "$wd"
}

@test "cd-ci-1: golden solution scores pass (grader rewards a correct gate)" {
  run run_task_with_solution cd-ci-1 check-no-advisory-tier.sh golden-solution.sh
  [ "$status" -eq 0 ]
  echo "$output" | tail -1 | jq -e '.pass == true and .score == 3' >/dev/null
}

@test "cd-ci-1: empty workspace scores fail (grader does not pass nothing)" {
  run run_task_with_solution cd-ci-1 check-no-advisory-tier.sh ""
  [ "$status" -eq 0 ]
  echo "$output" | tail -1 | jq -e '.pass == false' >/dev/null
}

@test "cd-ci-4: golden solution scores pass" {
  run run_task_with_solution cd-ci-4 check-removed-job-assertions.sh golden-solution.sh
  [ "$status" -eq 0 ]
  echo "$output" | tail -1 | jq -e '.pass == true and .score == 3' >/dev/null
}

@test "cd-ci-4: empty workspace scores fail" {
  run run_task_with_solution cd-ci-4 check-removed-job-assertions.sh ""
  [ "$status" -eq 0 ]
  echo "$output" | tail -1 | jq -e '.pass == false' >/dev/null
}

@test "both tasks ship prompt.md + setup.sh + score.sh + golden-solution.sh" {
  for t in cd-ci-1 cd-ci-4; do
    [ -f "$TASKS/$t/prompt.md" ]
    [ -f "$TASKS/$t/setup.sh" ]
    [ -f "$TASKS/$t/score.sh" ]
    [ -f "$TASKS/$t/golden-solution.sh" ]
  done
}
