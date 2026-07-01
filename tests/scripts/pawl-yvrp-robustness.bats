#!/usr/bin/env bats
# age-yvrp: warm-panel robustness. _ready_subset lets cmd_up DEGRADE to the panes that came ready
# (reuse the tier door) instead of dying; _route_in_progress + the cmd_down busy-guard stop a
# concurrent lane / idle reaper from killing a mid-route pawl. All pure/mockable — no live substrate.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"; ORIG_PATH="$PATH"; mkdir -p "$TMP/bin"
  # minimal atm/tmux mocks so cmd_down's kill path is inert.
  for b in atm tmux; do printf '#!/usr/bin/env bash\nexit 0\n' > "$TMP/bin/$b"; chmod +x "$TMP/bin/$b"; done
  export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"   # source-guard returns before dispatch
  ROUTE_LOCK="$TMP/route.lock"; ROUTE_TIMEOUT=320
  log() { :; }
}
teardown() { export PATH="$ORIG_PATH"; rm -rf "$TMP"; }

# --- _route_in_progress ---

@test "_route_in_progress: fresh lock -> in progress (0)" {
  date +%s > "$ROUTE_LOCK"
  run _route_in_progress
  [ "$status" -eq 0 ]
}

@test "_route_in_progress: no lock -> not in progress (1)" {
  rm -f "$ROUTE_LOCK"
  run _route_in_progress
  [ "$status" -ne 0 ]
}

@test "_route_in_progress: STALE lock -> not in progress (1) AND cleans the lock" {
  echo $(( $(date +%s) - 100000 )) > "$ROUTE_LOCK"
  run _route_in_progress
  [ "$status" -ne 0 ]
  [ ! -f "$ROUTE_LOCK" ]
}

@test "_route_in_progress: malformed lock -> treated as epoch 0 -> stale -> not in progress" {
  echo "garbage" > "$ROUTE_LOCK"
  run _route_in_progress
  [ "$status" -ne 0 ]
}

# --- cmd_down busy-guard ---

@test "cmd_down: refuses (exit 3) while a route is in progress (fresh lock)" {
  date +%s > "$ROUTE_LOCK"
  session_exists() { return 0; }
  run cmd_down
  [ "$status" -eq 3 ]
}

@test "cmd_down --force: overrides the busy-guard even with a fresh lock" {
  date +%s > "$ROUTE_LOCK"
  session_exists() { return 1; }   # no session => no-op kill, but must get PAST the guard (not exit 3)
  run cmd_down --force
  [ "$status" -ne 3 ]
}

@test "cmd_down: proceeds when no route is in progress (no lock)" {
  rm -f "$ROUTE_LOCK"
  session_exists() { return 1; }
  run cmd_down
  [ "$status" -ne 3 ]
}

# --- _ready_subset (degrade selection) ---

@test "_ready_subset: agy down in a tri-spawn -> 'cc cod' (degrade target)" {
  ENABLED="cc cod agy"; CC_PANE=1; COD_PANE=2; AGY_PANE=3
  clear_known_prompts() { :; }
  cc_ready() { return 0; }
  codex_state() { echo codex-live; }
  agy_ready() { return 1; }
  run _ready_subset
  [ "$output" = "cc cod" ]
}

@test "_ready_subset: none ready -> empty (cmd_up then dies, fail-closed)" {
  ENABLED="cc cod agy"; CC_PANE=1; COD_PANE=2; AGY_PANE=3
  clear_known_prompts() { :; }
  cc_ready() { return 1; }
  codex_state() { echo absent; }
  agy_ready() { return 1; }
  run _ready_subset
  [ -z "$output" ]
}
