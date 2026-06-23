#!/usr/bin/env bats
# Regression tests for the fail-closed/fail-safe guards in the session's local
# membrane eval tooling — each was added or hardened by a cross-family pawl and a
# silent regression would reintroduce a real failure (running the wrong reviewer, or
# a wrapper aborting an eval run / fabricating a verdict). No model calls.

setup() {
  REPO="$BATS_TEST_DIRNAME/../.."
  RUNNER="$REPO/evals/membrane/harvest-2026-06-22/run-harvest-series.sh"
  MEMBRANE="$REPO/evals/membrane/membranes/local-mlx-membrane.sh"
  FIX="$(mktemp -d)"
}
teardown() { rm -rf "$FIX"; }

# --- run-harvest-series.sh: HARVEST_MEMBRANE is fail-closed (age-546s) -----------

@test "an unknown HARVEST_MEMBRANE value fails closed (exit 2), never silently runs codex" {
  # Valid positional args so it reaches the membrane-choice case, not the arg check.
  run env HARVEST_MEMBRANE=locl bash "$RUNNER" lbl http://127.0.0.1:9/x model "$FIX/series.jsonl"
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown HARVEST_MEMBRANE"* ]]
  # Fail-closed means it must NOT have produced a series row.
  [ ! -f "$FIX/series.jsonl" ]
}

# --- local-mlx-membrane.sh: self-contained fail-safe (empty stdout + exit 0) -----

@test "an unreachable endpoint yields EMPTY stdout and exit 0 (degraded), never aborts" {
  # stderr carries an informational "emitting empty (degraded)" warning by design;
  # the CONTRACT is empty STDOUT (what the harness greps for a VERDICT), so discard
  # stderr and assert stdout is empty.
  run bash -c "MLX_ENDPOINT='http://127.0.0.1:9/none' MLX_TIMEOUT=2 bash '$MEMBRANE' 'review this' 2>/dev/null"
  [ "$status" -eq 0 ]            # fail-SAFE: must not abort the eval run
  [ -z "$output" ]              # empty stdout -> the harness records the task as degraded
}

@test "a value with shell metacharacters is never interpreted (no injection)" {
  # The wrapper builds the request via python from env, so a metacharacter-laden
  # model name must not be shell-evaluated. Unreachable endpoint -> degraded, but
  # crucially the marker file must NOT be created by any injected command.
  marker="$FIX/INJECTED"
  run env MLX_ENDPOINT="http://127.0.0.1:9/none" MLX_TIMEOUT=2 \
      MLX_MODEL="m; touch $marker" bash "$MEMBRANE" "review"
  [ "$status" -eq 0 ]
  [ ! -e "$marker" ]            # injection did NOT execute
}

@test "the membrane wrapper requires a prompt argument" {
  run bash "$MEMBRANE"
  [ "$status" -ne 0 ]
}
