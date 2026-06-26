#!/usr/bin/env bats
# Tri-model standing-pawl guards: the AGY pane liveness check (agy_dead) and the pure 3-way
# agreement decision (pawl_decide_agreement). Both are exercised by SOURCING pawl.sh (its
# execute-only guard suppresses command dispatch). agy_dead uses the same deterministic
# foreground-process signal as cod_dead (a shell foreground => dead), so a stale TUI scrollback
# can never yield a false-alive. pawl_decide_agreement encodes the ALL-CONFIRM + degrade-to->=2
# + any-REFUTE-blocks rule and is pure (no tmux), so every branch is locked here.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin"
  cat >"$TMP/bin/tmux" <<'EOF'
#!/usr/bin/env bash
# PANE_CMD drives display-message (the foreground process); PANE_TXT drives capture-pane.
# The sentinel __FAIL__ makes the respective call exit non-zero, simulating a missing /
# unreadable pane so the fail-closed readiness paths can be exercised. send-keys is a no-op.
case "$1" in
  display-message)
    [ "${PANE_CMD:-}" = "__FAIL__" ] && exit 1
    printf '%s\n' "${PANE_CMD:-}" ;;
  capture-pane)
    [ "${PANE_TXT:-}" = "__FAIL__" ] && exit 1
    printf '%s\n' "${PANE_TXT:-}" ;;
  send-keys) : ;;
esac
exit 0
EOF
  chmod +x "$TMP/bin/tmux"
  export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"
}

teardown() {
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

# --- agy_dead: liveness via foreground process (mirrors cod_dead) ---

@test "agy_dead: foreground is the agy binary -> NOT dead" {
  export PANE_CMD="agy"
  run agy_dead
  [ "$status" -eq 1 ]
}

@test "agy_dead: foreground is a shell (zsh) -> DEAD" {
  export PANE_CMD="zsh"
  run agy_dead
  [ "$status" -eq 0 ]
}

@test "agy_dead: login shell (-zsh) -> DEAD" {
  export PANE_CMD="-zsh"
  run agy_dead
  [ "$status" -eq 0 ]
}

@test "agy_dead: a shell with stale 'Gemini 3.5 Flash' scrollback is STILL DEAD (process, not text)" {
  export PANE_CMD="bash"
  run agy_dead
  [ "$status" -eq 0 ]
}

@test "agy_dead: empty/unreadable foreground -> NOT dead (fail safe, no respawn on a read glitch)" {
  export PANE_CMD=""
  run agy_dead
  [ "$status" -eq 1 ]
}

# --- agy_ready: POSITIVE agy-foreground check, fail-closed (membrane-caught fail-opens) ---

@test "agy_ready: foreground=agy + no trust gate -> READY" {
  export PANE_CMD="agy" PANE_TXT="> ready input box"
  run agy_ready
  [ "$status" -eq 0 ]
}

@test "agy_ready: foreground=shell -> NOT ready (fail-closed)" {
  export PANE_CMD="zsh" PANE_TXT="~/dev >"
  run agy_ready
  [ "$status" -ne 0 ]
}

@test "agy_ready: missing/unreadable pane (display-message fails) -> NOT ready (no false tri-model green)" {
  # Membrane defect #1: a display-message failure must NOT be treated as ready, else up/health
  # green-light a two-pane session as tri-model and lazy auto-up never adds AGY.
  export PANE_CMD="__FAIL__" PANE_TXT=""
  run agy_ready
  [ "$status" -ne 0 ]
}

@test "agy_ready: a different non-shell program -> NOT ready (no wrong-program routing)" {
  # Membrane defect #2: any non-shell foreground passed before; now the agy binary is required
  # positively, so routing can never send review packets to the wrong program.
  export PANE_CMD="node" PANE_TXT="> some other tui"
  run agy_ready
  [ "$status" -ne 0 ]
}

@test "agy_ready: foreground=agy but trust gate still showing -> NOT ready" {
  # send-keys is a no-op in the mock, so the gate text persists and readiness must stay false.
  export PANE_CMD="agy" PANE_TXT="Antigravity CLI requires permission to read, edit ...  Yes, I trust this folder"
  run agy_ready
  [ "$status" -ne 0 ]
}

@test "agy_ready: capture-pane failure -> NOT ready (fail-closed on a read glitch)" {
  export PANE_CMD="agy" PANE_TXT="__FAIL__"
  run agy_ready
  [ "$status" -ne 0 ]
}

# --- pawl_decide_agreement: ALL-CONFIRM + degrade + any-REFUTE-blocks ---

@test "agreement: all 3 CONFIRMED -> CONFIRMED:full:3" {
  run pawl_decide_agreement CONFIRMED CONFIRMED CONFIRMED
  [ "$status" -eq 0 ]
  [ "$output" = "CONFIRMED:full:3" ]
}

@test "agreement: 2 CONFIRMED + 1 unavailable (timeout) -> CONFIRMED:degraded:2 (>=2 cross-family)" {
  run pawl_decide_agreement CONFIRMED CONFIRMED ""
  [ "$output" = "CONFIRMED:degraded:2" ]
}

@test "agreement: any single REFUTE blocks even with 2 CONFIRMED -> REFUTED:refuted (recall-biased)" {
  run pawl_decide_agreement CONFIRMED CONFIRMED REFUTED
  [ "${output%%:*}" = "REFUTED" ]
  case "$output" in REFUTED:refuted:*) : ;; *) false ;; esac
}

@test "agreement: a lone REFUTE with 2 timeouts -> REFUTED:refuted (refute dominates)" {
  run pawl_decide_agreement REFUTED "" ""
  case "$output" in REFUTED:refuted:*) : ;; *) false ;; esac
}

@test "agreement: only 1 CONFIRMED, 2 unavailable -> REFUTED:insufficient:1 (need >=2 cross-family, fail-closed)" {
  run pawl_decide_agreement CONFIRMED "" ""
  [ "$output" = "REFUTED:insufficient:1" ]
}

@test "agreement: all 3 unavailable (all timeout) -> REFUTED:insufficient:0 (fail-closed, never fail-open)" {
  run pawl_decide_agreement "" "" ""
  [ "$output" = "REFUTED:insufficient:0" ]
}
