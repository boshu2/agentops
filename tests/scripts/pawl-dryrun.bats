#!/usr/bin/env bats
# age-l3xj (D1): script-side dry-run seam. Read-only verbs (health/doctor/smoke/metrics)
# may run under `ao --dry-run` to inspect real state, but must not MUTATE it — including
# the prompt-clearing key sends clear_known_prompts fires into live panes. PAWL_DRY_RUN=1
# (exported by the Go wrapper on the read-only dry-run path) suppresses every send.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"; ORIG_PATH="$PATH"; mkdir -p "$TMP/bin"
  # tmux mock LOGS every invocation so a key send is detectable
  cat > "$TMP/bin/tmux" <<EOF
#!/usr/bin/env bash
echo "\$@" >> "$TMP/tmux-calls.log"
exit 0
EOF
  chmod +x "$TMP/bin/tmux"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$TMP/bin/atm"; chmod +x "$TMP/bin/atm"
  export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"
  log() { :; }
}
teardown() { export PATH="$ORIG_PATH"; rm -rf "$TMP"; unset PAWL_DRY_RUN; }

@test "clear_known_prompts: PAWL_DRY_RUN=1 sends NO keys (no pane mutation under dry-run)" {
  export PAWL_DRY_RUN=1
  _pane_live_text() { echo "press enter to continue"; }
  detect_blocking_prompt() { echo "continue"; }
  prompt_dismiss_key() { echo "Enter"; }
  run clear_known_prompts 1
  [ "$status" -ne 0 ]   # reports "did not clear" — callers already treat that as inert
  ! grep -q "send-keys" "$TMP/tmux-calls.log" 2>/dev/null
}

@test "clear_known_prompts: without the seam it still clears (live behavior unchanged)" {
  _pane_live_text() { echo "press enter to continue"; }
  detect_blocking_prompt() { echo "continue"; }
  prompt_dismiss_key() { echo "Enter"; }
  SESSION="s"
  sleep() { :; }
  run clear_known_prompts 1
  [ "$status" -eq 0 ]
}
