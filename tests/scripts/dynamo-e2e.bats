#!/usr/bin/env bats
# ag-veotd: the dynamo-e2e harness runs one full loop cycle and proves it closes.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  HARNESS="$REPO_ROOT/scripts/dynamo-e2e.sh"
  # build ao once for the whole file (the harness calls it 4x)
  if [[ -z "${AO_E2E_BIN:-}" ]]; then
    AO_E2E_BIN="$BATS_FILE_TMPDIR/ao"
    ( cd "$REPO_ROOT/cli" && go build -o "$AO_E2E_BIN" ./cmd/ao ) || skip "go build ao failed (toolchain unavailable)"
  fi
}

@test "dynamo-e2e runs one full cycle and the loop CLOSES" {
  run env AO_BIN="$AO_E2E_BIN" "$HARNESS" --run-id=bats-e2e
  [ "$status" -eq 0 ]
  [[ "$output" == *"LOOP CLOSED"* ]]
  [[ "$output" == *"DYNAMO E2E OK"* ]]
  # every organ signal surfaced in the gauge readout
  [[ "$output" == *"A (accepted)"* ]]
  [[ "$output" == *"Q (first-pass yield)"* ]]
  [[ "$output" == *"C (corpus delta)"* ]]
  [[ "$output" == *"Shadow-mode actuation"* ]]
  # C is honestly pending, never fabricated
  [[ "$output" == *"pending"* ]]
}

@test "the loop-closed proof has teeth: a non-codex AO_BIN that emits nothing fails" {
  # point AO_BIN at `true` (emits nothing, gauge sees no events) — A stays 0,
  # so the harness must report the loop did NOT close (flowed-value assertion bites).
  run env AO_BIN="$(command -v true)" "$HARNESS" --run-id=bats-empty
  [ "$status" -ne 0 ]
}
