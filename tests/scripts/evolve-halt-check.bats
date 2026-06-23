#!/usr/bin/env bats
# L2 tests for scripts/evolve/halt-check.sh + the skill wiring (soc-sfjx).
#
# halt-check.sh is the mechanical pre-cycle gate ported from the mt-olympus
# unbounded-evolve substrate. These tests prove (a) the gate's behavior across
# markers / goal-regression / prior-fail, and — load-bearing — (b) that
# skills/evolve/SKILL.md actually CALLS the gate. (b) is the regression guard
# against the soc-g2qd failure mode: shipping a primitive no consumer invokes.

# Portable file-backdating. GNU `touch -d "N days ago"` is unsupported on BSD/macOS,
# and the prior `touch -A -240000` fallback only adjusts 24h (NOT the intended 10
# days) — leaving the marker fresher than the TTL, so the gate halted (exit 1)
# instead of continuing (exit 0) on macOS. Set the mtime directly via python.
backdate_days() { # $1=file $2=days
  python3 -c "import os,sys,time; t=time.time()-int(sys.argv[2])*86400; os.utime(sys.argv[1],(t,t))" "$1" "$2"
}

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  GATE="$REPO_ROOT/scripts/evolve/halt-check.sh"
  SKILL="$REPO_ROOT/skills/evolve/SKILL.md"
  TMP="$(mktemp -d)"
  mkdir -p "$TMP/.agents/evolve"
  # Isolate the SPC-governor budget probe (age-wy3.3 coherence wire) from the real
  # `ao governor budget` + repo yield ledger: default to ship (exit 0 = continue),
  # so the marker/regression tests below are deterministic. The governor-wire tests
  # override EVOLVE_GOVERNOR_CMD explicitly.
  export EVOLVE_GOVERNOR_CMD="exit 0"
}

teardown() {
  rm -rf "$TMP"
}

# --- (b) consumer-wiring proof: the skill MUST call the gate -----------------

@test "SKILL.md invokes scripts/evolve/halt-check.sh (no orphaned primitive)" {
  run grep -F 'scripts/evolve/halt-check.sh' "$SKILL"
  [ "$status" -eq 0 ]
  # And it is run in Step 1 (the kill-switch step), not merely mentioned in prose.
  run bash -c "sed -n '/### Step 1: Kill Switch Check/,/### Step 1.5/p' '$SKILL' | grep -F 'bash scripts/evolve/halt-check.sh'"
  [ "$status" -eq 0 ]
}

# --- (a) gate behavior -------------------------------------------------------

@test "clean state continues (exit 0, halt false)" {
  run env EVOLVE_DIR="$TMP/.agents/evolve" bash "$GATE" --json
  [ "$status" -eq 0 ]
  [ "$output" = '{"halt":false,"halt_reason":null}' ]
}

@test "fresh repo STOP marker halts as user_halt" {
  touch "$TMP/.agents/evolve/STOP"
  run env EVOLVE_DIR="$TMP/.agents/evolve" bash "$GATE" --json
  [ "$status" -eq 1 ]
  [ "$output" = '{"halt":true,"halt_reason":"user_halt"}' ]
}

@test "stale STOP marker (older than TTL) is bypassed, loop continues" {
  touch "$TMP/.agents/evolve/STOP"
  # Backdate the marker 10 days; TTL set to 7.
  backdate_days "$TMP/.agents/evolve/STOP" 10
  # WARN about staleness goes to stderr (bats merges it into $output); assert the
  # gate continues (exit 0) and the no-halt JSON is present.
  run env EVOLVE_DIR="$TMP/.agents/evolve" EVOLVE_KILL_TTL_DAYS=7 bash "$GATE" --json
  [ "$status" -eq 0 ]
  [[ "$output" == *'{"halt":false,"halt_reason":null}'* ]]
  [[ "$output" == *"STALE"* ]]
}

# --- operator-GLOBAL kill switch (~/.config/evolve/KILL), checked before STOP --
# HOME is redirected to $TMP so the test never touches a real global KILL.

@test "fresh global KILL marker halts as kill (operator emergency stop)" {
  mkdir -p "$TMP/.config/evolve"
  touch "$TMP/.config/evolve/KILL"
  run env HOME="$TMP" EVOLVE_DIR="$TMP/.agents/evolve" bash "$GATE" --json
  [ "$status" -eq 1 ]
  [ "$output" = '{"halt":true,"halt_reason":"kill"}' ]
}

@test "stale global KILL marker is bypassed, loop continues" {
  mkdir -p "$TMP/.config/evolve"
  touch "$TMP/.config/evolve/KILL"
  # Backdate 10 days; TTL set to 7.
  backdate_days "$TMP/.config/evolve/KILL" 10
  run env HOME="$TMP" EVOLVE_DIR="$TMP/.agents/evolve" EVOLVE_KILL_TTL_DAYS=7 bash "$GATE" --json
  [ "$status" -eq 0 ]
  [[ "$output" == *'{"halt":false,"halt_reason":null}'* ]]
  [[ "$output" == *"STALE"* ]]
}

@test "DORMANT with no ready work halts as dormant" {
  # Hermetic: cd to a dir with no .beads (bd ready -> 0) + point harvested ledger
  # at an empty file so neither real repo state leaks in.
  touch "$TMP/.agents/evolve/DORMANT"
  : > "$TMP/empty-next-work.jsonl"
  cd "$TMP"
  run env EVOLVE_DIR="$TMP/.agents/evolve" EVOLVE_NEXTWORK="$TMP/empty-next-work.jsonl" bash "$GATE" --json
  [ "$status" -eq 1 ]
  [ "$output" = '{"halt":true,"halt_reason":"dormant"}' ]
}

@test "DORMANT auto-clears when harvested work exists (soc-5qit non-sticky)" {
  touch "$TMP/.agents/evolve/DORMANT"
  printf '%s\n' '{"consumed":false,"severity":"high"}' > "$TMP/next-work.jsonl"
  cd "$TMP"
  run env EVOLVE_DIR="$TMP/.agents/evolve" EVOLVE_NEXTWORK="$TMP/next-work.jsonl" bash "$GATE" --json
  [ "$status" -eq 0 ]
  [ "$output" = '{"halt":false,"halt_reason":null}' ]
  [ ! -f "$TMP/.agents/evolve/DORMANT" ]   # auto-cleared
}

@test "HANDOFF is cleared and never halts" {
  touch "$TMP/.agents/evolve/HANDOFF"
  run env EVOLVE_DIR="$TMP/.agents/evolve" bash "$GATE" --json
  [ "$status" -eq 0 ]
  [ ! -f "$TMP/.agents/evolve/HANDOFF" ]
}

@test "goal regression (goals_passing drops) halts" {
  printf '%s\n' '{"result":"productive","goals_passing":10,"goals_total":12}' >> "$TMP/.agents/evolve/cycle-history.jsonl"
  printf '%s\n' '{"result":"productive","goals_passing":8,"goals_total":12}'  >> "$TMP/.agents/evolve/cycle-history.jsonl"
  run env EVOLVE_DIR="$TMP/.agents/evolve" bash "$GATE" --json
  [ "$status" -eq 1 ]
  [ "$output" = '{"halt":true,"halt_reason":"goal_regression"}' ]
}

@test "goals_passing rising or flat does NOT halt" {
  printf '%s\n' '{"result":"productive","goals_passing":8,"goals_total":12}'  >> "$TMP/.agents/evolve/cycle-history.jsonl"
  printf '%s\n' '{"result":"productive","goals_passing":9,"goals_total":12}'  >> "$TMP/.agents/evolve/cycle-history.jsonl"
  run env EVOLVE_DIR="$TMP/.agents/evolve" bash "$GATE" --json
  [ "$status" -eq 0 ]
  [ "$output" = '{"halt":false,"halt_reason":null}' ]
}

@test "prior-cycle FAIL result emits restorative signal" {
  printf '%s\n' '{"result":"FAIL: gate red","goals_passing":9,"goals_total":12}' >> "$TMP/.agents/evolve/cycle-history.jsonl"
  run env EVOLVE_DIR="$TMP/.agents/evolve" bash "$GATE" --json
  [ "$status" -eq 1 ]
  [ "$output" = '{"halt":true,"halt_reason":"prior_cycle_fail"}' ]
}

@test "human-readable mode prints OK on clean state" {
  run env EVOLVE_DIR="$TMP/.agents/evolve" bash "$GATE"
  [ "$status" -eq 0 ]
  [ "$output" = "OK: continue" ]
}

# --- (a continued) SPC governor coherence wire (age-wy3.3) -------------------

@test "governor budget burned (exit 3) halts as governor_budget_burned" {
  run env EVOLVE_DIR="$TMP/.agents/evolve" EVOLVE_GOVERNOR_CMD="exit 3" bash "$GATE" --json
  [ "$status" -eq 1 ]
  [ "$output" = '{"halt":true,"halt_reason":"governor_budget_burned"}' ]
}

@test "governor budget ship (exit 0) continues" {
  run env EVOLVE_DIR="$TMP/.agents/evolve" EVOLVE_GOVERNOR_CMD="exit 0" bash "$GATE" --json
  [ "$status" -eq 0 ]
  [ "$output" = '{"halt":false,"halt_reason":null}' ]
}

@test "governor tool error (non-3 non-zero) fails open, does not halt" {
  # A broken/absent governor must NEVER wedge the evolve loop — only exit 3 halts.
  # A WARN goes to stderr (bats merges it into $output), so match the JSON as a substring.
  run env EVOLVE_DIR="$TMP/.agents/evolve" EVOLVE_GOVERNOR_CMD="exit 1" bash "$GATE" --json
  [ "$status" -eq 0 ]
  [[ "$output" == *'{"halt":false,"halt_reason":null}'* ]]
  [[ "$output" == *"failing open"* ]]
}

@test "operator KILL/STOP outranks a burned governor budget (priority order)" {
  # An explicit operator halt must win over the governor verdict; reason stays user_halt.
  touch "$TMP/.agents/evolve/STOP"
  run env EVOLVE_DIR="$TMP/.agents/evolve" EVOLVE_GOVERNOR_CMD="exit 3" bash "$GATE" --json
  [ "$status" -eq 1 ]
  [ "$output" = '{"halt":true,"halt_reason":"user_halt"}' ]
}
