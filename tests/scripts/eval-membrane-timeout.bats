#!/usr/bin/env bats
# age-9h3d: scripts/eval-membrane.sh wraps the PRODUCER in a timeout but the
# MEMBRANE review had NONE — a stalled reviewer (a hung `codex exec` froze a real
# harvest run for 22 min) hung the whole run forever. These cases pin the fix: the
# --membrane-timeout option kills a stalled review and the existing degraded path
# excludes that task, so the run finishes fast instead of hanging. Exercised with
# stub producer/membrane shell commands (no real codex/agy), deterministically.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/eval-membrane.sh"
  FIX="$(mktemp -d)"
  mkdir -p "$FIX/tasks/tdemo"
  printf '#!/usr/bin/env bash\n:\n' > "$FIX/tasks/tdemo/setup.sh"
  printf '#!/usr/bin/env bash\necho "{\\"pass\\":false}"\n' > "$FIX/tasks/tdemo/score.sh"
  printf 'implement\n' > "$FIX/tasks/tdemo/prompt.md"
  chmod +x "$FIX/tasks/tdemo/"*.sh
  # A membrane that is FAST on the smoke probe but STALLS (sleep 9) on a real task
  # review — so the per-task timeout, not the smoke, is what's under test.
  SMOKE_FAST_TASK_SLOW='case "$1" in *"Smoke check"*) printf "VERDICT: ACK\nWHY: ok\n";; *) sleep 9; printf "VERDICT: ACK\n";; esac'
}

teardown() { rm -rf "$FIX"; }

# Skip when no timeout binary exists — the fix degrades to the old (no-timeout)
# behavior there by design, so the assertion would not hold.
require_timeout() {
  command -v gtimeout >/dev/null 2>&1 || command -v timeout >/dev/null 2>&1 || skip "no gtimeout/timeout on PATH"
}

@test "a stalled membrane review is killed and the task is excluded as degraded" {
  require_timeout
  start=$SECONDS
  run bash "$SCRIPT" --tasks-dir "$FIX/tasks" --task tdemo \
    --producer-cmd 'printf "x\n" > "$1/x.txt"' --producer-label stub \
    --membrane-cmd "$SMOKE_FAST_TASK_SLOW" --membrane-label slowtask \
    --membrane-timeout 2 --output "$FIX/sc.json"
  elapsed=$(( SECONDS - start ))
  [ "$status" -eq 0 ]
  # Finished fast — the 9s stall was cut at ~2s, never allowed to hang the run.
  [ "$elapsed" -lt 7 ]
  # The stalled task is degraded, NOT counted as caught/escaped (no fabricated verdict).
  run jq -r '.totals | "\(.degraded) \(.caught) \(.escaped)"' "$FIX/sc.json"
  [ "$output" = "1 0 0" ]
}

@test "a fast membrane within the timeout still adjudicates normally" {
  require_timeout
  run bash "$SCRIPT" --tasks-dir "$FIX/tasks" --task tdemo \
    --producer-cmd 'printf "x\n" > "$1/x.txt"' --producer-label stub \
    --membrane-cmd 'printf "VERDICT: REFUTE\nWHY: incomplete\n"' --membrane-label faststub \
    --membrane-timeout 5 --output "$FIX/sc.json"
  [ "$status" -eq 0 ]
  # oracle FAIL + membrane REFUTE = caught (the review ran, not degraded).
  run jq -r '.totals | "\(.degraded) \(.caught) \(.escaped)"' "$FIX/sc.json"
  [ "$output" = "0 1 0" ]
}

@test "--membrane-timeout is documented in usage" {
  run bash "$SCRIPT" --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"--membrane-timeout"* ]]
}
