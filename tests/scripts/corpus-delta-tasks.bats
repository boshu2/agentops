#!/usr/bin/env bats
# ag-nfux (ag-8p8o W1b): the held corpus-delta task set.
# These cases verify the TASK FIXTURES THEMSELVES are well-formed: each grader must
# DISCRIMINATE — the golden reference solution scores pass, an empty workspace scores fail.
# A grader that always passes (or always fails) would make the A/B meaningless.

TASKS="$BATS_TEST_DIRNAME/../../evals/workbench/tasks"

CORPUS_DELTA_TASKS=(
  "cd-ci-1:check-no-advisory-tier.sh"
  "cd-ci-4:check-removed-job-assertions.sh"
  "cd-am-1:check-am-reservation-conflicts.sh"
  "cd-beads-1:check-br-beads-dir.sh"
  "cd-bv-1:check-bv-robot-mode.sh"
  "cd-door9-1:check-no-claude-print.sh"
  "cd-git-1:check-no-main-push.sh"
  "cd-worktree-1:check-worktree-per-bead.sh"
  "cd-generated-1:check-generated-edits.sh"
  "cd-agents-1:check-no-runtime-agents.sh"
)

run_task_with_solution() {
  local task="$1" outfile="$2" solution="$3"   # solution: path to drop in, or "" for none
  local wd; wd="$BATS_TEST_TMPDIR/$task-$RANDOM"
  bash "$TASKS/$task/setup.sh" "$wd"
  if [[ -n "$solution" ]]; then cp "$TASKS/$task/$solution" "$wd/$outfile"; fi
  bash "$TASKS/$task/score.sh" "$wd"
}

@test "held task set has at least 10 corpus-delta tasks" {
  [ "${#CORPUS_DELTA_TASKS[@]}" -ge 10 ]
}

@test "all corpus-delta tasks ship prompt, setup, score, and golden solution" {
  for entry in "${CORPUS_DELTA_TASKS[@]}"; do
    t="${entry%%:*}"
    [ -f "$TASKS/$t/prompt.md" ]
    [ -f "$TASKS/$t/setup.sh" ]
    [ -f "$TASKS/$t/score.sh" ]
    [ -f "$TASKS/$t/golden-solution.sh" ]
  done
}

@test "every golden solution passes its deterministic grader" {
  for entry in "${CORPUS_DELTA_TASKS[@]}"; do
    t="${entry%%:*}"
    outfile="${entry#*:}"
    run run_task_with_solution "$t" "$outfile" golden-solution.sh
    [ "$status" -eq 0 ]
    echo "$output" | tail -1 | jq -e '.pass == true and .score == .total' >/dev/null
  done
}

@test "every empty workspace fails its deterministic grader" {
  for entry in "${CORPUS_DELTA_TASKS[@]}"; do
    t="${entry%%:*}"
    outfile="${entry#*:}"
    run run_task_with_solution "$t" "$outfile" ""
    [ "$status" -eq 0 ]
    echo "$output" | tail -1 | jq -e '.pass == false' >/dev/null
  done
}
