#!/usr/bin/env bats
# cod_dead correctness guard (age-standing-pawl-service-ml8.3).
#
# cod_dead decides whether the codex pane has dropped to a shell (and must be respawned).
# It uses the DETERMINISTIC foreground-process signal (`tmux display-message
# #{pane_current_command}`), NOT scraped scrollback text — because a dropped pane retains the
# codex TUI's scrollback above the new shell prompt, and a shell's cwd/output can contain any
# marker, so text-scraping yields false-alives (the pawl refuter demonstrated several). This
# guard locks: a shell foreground => DEAD; the codex binary (or any non-shell) => alive; and the
# rotate_account return contract that makes its `|| true` guard load-bearing under set -e.
#
# `tmux` is STUBBED via PATH (display-message emits $PANE_CMD); pawl.sh is SOURCED (its
# execute-only guard suppresses command dispatch) so the pure helper is exercised directly.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin"
  cat >"$TMP/bin/tmux" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = "display-message" ]; then printf '%s\n' "${PANE_CMD:-}"; fi
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

@test "cod_dead: foreground is the codex binary -> NOT dead" {
  export PANE_CMD="codex-aarch64-a"
  run cod_dead
  [ "$status" -eq 1 ]
}

@test "cod_dead: foreground is a shell (zsh) -> DEAD" {
  export PANE_CMD="zsh"
  run cod_dead
  [ "$status" -eq 0 ]
}

@test "cod_dead: login shell (-zsh) -> DEAD" {
  export PANE_CMD="-zsh"
  run cod_dead
  [ "$status" -eq 0 ]
}

@test "cod_dead: a shell with stale 'OpenAI Codex'/gpt-5 scrollback is STILL DEAD (process, not text)" {
  # The class of false-alive the refuter kept finding: scrollback chrome above a shell prompt.
  # The process signal is the shell regardless of what text remains on screen -> DEAD. Immune.
  export PANE_CMD="bash"
  run cod_dead
  [ "$status" -eq 0 ]
}

@test "cod_dead: any other foreground process (e.g. node) -> NOT dead (conservative, no needless respawn)" {
  export PANE_CMD="node"
  run cod_dead
  [ "$status" -eq 1 ]
}

@test "cod_dead: empty/unreadable foreground -> NOT dead (fail safe, no respawn on a read glitch)" {
  export PANE_CMD=""
  run cod_dead
  [ "$status" -eq 1 ]
}

@test "rotate_account returns non-zero when not opted in -> call sites MUST guard with || true (set -e)" {
  unset PAWL_AUTO_ROTATE
  run rotate_account cod
  [ "$status" -ne 0 ]   # this is WHY `rotate_account cod || true` is load-bearing under set -e
}
