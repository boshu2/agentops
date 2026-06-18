#!/usr/bin/env bats
# ag-veotd + ag-7wrje: the dynamo-e2e harness runs the loop end-to-end (clean + rework/ratchet) and proves it closes.

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

@test "rework scenario: the ratchet is honest — reject->rework->accept penalizes Q and L (ag-7wrje)" {
  run env AO_BIN="$AO_E2E_BIN" "$HARNESS" --scenario=rework --run-id=bats-rework
  [ "$status" -eq 0 ]
  [[ "$output" == *"RATCHET HONEST"* ]]
  [[ "$output" == *"REFUTED attempt-1"* ]]
  # the reworked bead is accepted but NOT counted a clean first pass (Q penalized)
  [[ "$output" == *"(0/1 beads clean)"* ]]
  # rework spend is counted as loss (L > 0)
  [[ "$output" != *"L=0.000"* ]]
}

@test "rework-order scenario: attempt-1 spend is rework via the ORDERING join, no phase label (age-vx0)" {
  run env AO_BIN="$AO_E2E_BIN" "$HARNESS" --scenario=rework-order --run-id=bats-rework-order
  [ "$status" -eq 0 ]
  [[ "$output" == *"ordering-join loss, no phase label"* ]]
  # the reworked bead is not a clean first pass (Q penalized)
  [[ "$output" == *"(0/1 beads clean)"* ]]
  # THE TEETH: the L breakdown must show rework spend from the attempt-1 700-token
  # spend — classified rework SOLELY by the attempt-ordering attribution, since the
  # scenario emits no phase=rework row. The old rework scenario could not prove
  # this: its attempt-1 spend read Productive and its loss came from a phase label.
  [[ "$output" == *"rework=700"* ]]
  [[ "$output" == *"productive=500"* ]]
}

@test "the loop-closed proof has teeth: a non-codex AO_BIN that emits nothing fails" {
  # point AO_BIN at `true` (emits nothing, gauge sees no events) — A stays 0,
  # so the harness must report the loop did NOT close (flowed-value assertion bites).
  run env AO_BIN="$(command -v true)" "$HARNESS" --run-id=bats-empty
  [ "$status" -ne 0 ]
}

@test "self-excitation C readout: pending by default, populated from a published --c-delta (ag-1wv7s)" {
  # default: C must be pending, never a fabricated value
  run env AO_BIN="$AO_E2E_BIN" "$HARNESS" --run-id=bats-cpending
  [ "$status" -eq 0 ]
  [[ "$output" == *"C (corpus delta)"*"pending"* ]]
  # with a published delta: C shows the value (self-excitation organ readout)
  run env AO_BIN="$AO_E2E_BIN" "$HARNESS" --run-id=bats-cval --c-delta=0.12
  [ "$status" -eq 0 ]
  [[ "$output" == *"0.120"* ]]
  [[ "$output" != *"C (corpus delta)           pending"* ]]
  # negative delta (field NOT self-exciting) must populate, not error
  run env AO_BIN="$AO_E2E_BIN" "$HARNESS" --run-id=bats-cneg --c-delta=-0.5
  [ "$status" -eq 0 ]
  [[ "$output" == *"-0.500"* ]]
}
