#!/usr/bin/env bats
# pawl.sh bounded pane-death re-probe (F13-followup, age-pawl-intent-zhndq.17).
#
# cod_dead/agy_dead treated an EMPTY/failed `pane_current_command` read as ALIVE (the conservative
# "*) return 1" bias), so an actually-GONE pane was only caught minutes later by the engage
# deadline. _pane_gone re-probes ONCE and declares death only when BOTH hold: the second read is
# ALSO empty AND the pane produced no scrollback change across the probe. A pane that shows a real
# foreground command on the re-probe, or that is still producing output, stays ALIVE — the
# conservative bias is preserved for the genuinely uncertain case.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl.sh"
  TMP="$(mktemp -d)"
}
teardown() { rm -rf "$TMP"; }

# Run _pane_gone with a stubbed tmux. $1 = the re-probe display-message value ("" = still empty),
# $2/$3 = the capture-pane values for the first/second activity probe (differ => still producing).
run_pane_gone() {
  # NOTE: _pane_activity runs tmux inside a command substitution (a SUBSHELL), so a shell-variable
  # counter would not persist across probes — the stub counts via a FILE.
  : > "$TMP/probe-count"
  cat > "$TMP/t.sh" <<T
source "$SCRIPT" 2>/dev/null
SESSION=s; PAWL_PANE_REPROBE_SLEEP=0
tmux() {
  if [ "\$1" = "display-message" ]; then printf '%s' "\${RP}"; return 0; fi
  printf 'x' >> "$TMP/probe-count"
  if [ "\$(wc -c < "$TMP/probe-count" | tr -d ' ')" -le 1 ]; then printf '%s' "\${CAP1}"; else printf '%s' "\${CAP2}"; fi
}
_pane_gone 2 && echo DEAD || echo ALIVE
T
  RP="$1" CAP1="$2" CAP2="$3" bash "$TMP/t.sh"
}

@test "pane-reprobe: empty re-probe + NO activity change -> DEAD" {
  run run_pane_gone "" "same" "same"
  [[ "$output" == *"DEAD"* ]]
}

@test "pane-reprobe: a REAL foreground command on the re-probe -> ALIVE (conservative)" {
  run run_pane_gone "codex-x86" "same" "same"
  [[ "$output" == *"ALIVE"* ]]
}

@test "pane-reprobe: still producing output (activity changed) -> ALIVE despite an empty read" {
  run run_pane_gone "" "aaa" "bbb"
  [[ "$output" == *"ALIVE"* ]]
}

# THE REGRESSION codex refuted (2026-07-11): a VANISHED pane makes the FIRST display-message read
# FAIL (non-zero). The old `cmd="$(...)" || return 1` short-circuited to ALIVE, so _pane_gone was
# never reached and the pane was only caught at the engage deadline. cod_dead/agy_dead must route a
# FAILED read into the bounded re-probe and report DEAD when the pane is genuinely gone.
@test "pane-reprobe: cod_dead with a FAILING first read reaches the re-probe -> DEAD (codex refute)" {
  cat > "$TMP/fail.sh" <<T
source "$SCRIPT" 2>/dev/null
SESSION=s; COD_PANE=2; PAWL_PANE_REPROBE_SLEEP=0
# tmux display-message ALWAYS FAILS (the vanished-pane shape); capture-pane returns a constant
# (no activity), so a correct implementation must re-probe and conclude DEAD.
tmux() {
  if [ "\$1" = "display-message" ]; then return 1; fi
  printf 'same'
}
cod_dead && echo DEAD || echo ALIVE
T
  run bash "$TMP/fail.sh"
  [[ "$output" == *"DEAD"* ]]
}
