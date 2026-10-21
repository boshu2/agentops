#!/usr/bin/env bats

setup() {
  command -v node >/dev/null || skip "node is required"
  HARNESS="$BATS_TEST_DIRNAME/../fixtures/implement-wave-workflow.mjs"
}

@test "implement-wave: lanes run one at a time, then one verifier" {
  run node "$HARNESS" sequential
  [ "$status" -eq 0 ]
}

@test "implement-wave: a dead lane is reported and later lanes still run" {
  run node "$HARNESS" dead-lane
  [ "$status" -eq 0 ]
}

@test "implement-wave: prompts and meta claim no concurrency or enforced scope" {
  run node "$HARNESS" prompt-honesty
  [ "$status" -eq 0 ]
}

@test "implement-wave: invalid args fail before any agent is dispatched" {
  run node "$HARNESS" invalid-args-before-dispatch
  [ "$status" -eq 0 ]
}
