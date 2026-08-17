#!/usr/bin/env bats

setup() {
  SKILL_DIR="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  CHECK="$SKILL_DIR/scripts/check-output.sh"
  FIX="$(mktemp -d)"
}

teardown() { rm -rf "$FIX"; }

write_lint() {
  for key in outcome evidence admission bead_graph rpi_boundary ratchet discovery wave_budget hard_budget breaker operator_andon scope self_hosting terminal_reports; do
    printf -- '- %s: PASS - concrete check passed\n' "$key"
  done
}

write_safe() {
  {
    printf '%s\n' 'SAFE_TO_CREATE' '## Goal prompt' 'Goal outcome:' 'Ship bounded behavior.' 'Terminal acceptance and evidence:' '1. Test passes - exact command receipt.' 'Non-goals and authority:' '- No release; writes limited to package.' 'Bead graph:' '- Root: ago-root; experiment: ago-one.' 'Experiment policy:' '- One bead is one RPI.' 'Wave envelope:' '- RPIs: 2' '- concurrency: 1' '- wall minutes: 30' '- tokens: 20000' '- live attempts: 2' 'Hard goal envelope:' '- total RPIs: 6' '- total wall minutes: 120' '- total tokens: 80000' '- total live attempts: 6' '- compactions: 2' '- changed paths: 20' '- No artifact, repair, helper, subject, or wave resets a total.' 'Breaker and andon:' '- no-ratchet threshold RPIs: 2' '- HOLD consults exactly 1 bounded fresh helper.' 'Wave checkpoint:' '- Report matrix, frontier, verdicts, ratchets, churn, and remaining budget.' 'Terminal reports:' '- ACHIEVED: every criterion is proven.' '- NOT_ACHIEVED: hard envelope is exhausted with gaps.' '- NEEDS_OPERATOR: judgment or rescope is required; stop.' '## Goal-tool token budget' '- tokens: 90000' '## Assumptions' '- Tracker exists.' '## Lint'
    write_lint
  } >"$1"
}

@test "complete safe goal satisfies executable budgets and done conditions" {
  write_safe "$FIX/safe"
  run "$CHECK" "$FIX/safe"
  [ "$status" -eq 0 ]
  [[ "$output" == *'PASS (SAFE_TO_CREATE)'* ]]
}

@test "former first-line baseline accepts an incomplete goal that the contract rejects" {
  printf 'SAFE_TO_CREATE\nLooks plausible.\n' >"$FIX/incomplete"
  run bash -c 'head -n1 "$1" | grep -Eq "^(SAFE_TO_CREATE|USE_RPI|UNSAFE_GOAL)\\b"' _ "$FIX/incomplete"
  [ "$status" -eq 0 ]
  run "$CHECK" "$FIX/incomplete"
  [ "$status" -ne 0 ]
}

@test "missing hard budget and an unfilled field invalidate safe output" {
  write_safe "$FIX/unsafe-shape"
  sed -i.bak '/^- total live attempts:/d' "$FIX/unsafe-shape"
  printf '<fill this later>\n' >>"$FIX/unsafe-shape"
  run "$CHECK" "$FIX/unsafe-shape"
  [ "$status" -ne 0 ]
}

@test "single-RPI and unsafe decisions have bounded non-goal shapes" {
  {
    printf '%s\n' 'USE_RPI' '## Rationale' 'One shaped experiment reaches a verdict.' '## Assumptions' '- Scope is supplied.' '## Lint'
    write_lint
  } >"$FIX/rpi"
  run "$CHECK" "$FIX/rpi"
  [ "$status" -eq 0 ]
  {
    printf '%s\n' 'UNSAFE_GOAL' '## Missing decisions' '- Terminal evidence is absent.' '## Assumptions' '- None.' '## Lint'
    write_lint
  } >"$FIX/unsafe"
  run "$CHECK" "$FIX/unsafe"
  [ "$status" -eq 0 ]
  printf 'MAYBE\n' >"$FIX/maybe"
  run "$CHECK" "$FIX/maybe"
  [ "$status" -ne 0 ]
}
