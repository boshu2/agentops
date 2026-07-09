#!/usr/bin/env bats
# age-djfo: a warm pane can be ALIVE yet stuck on a CLI interruption prompt (codex update dialog,
# agy feedback survey) or an unknown hang — leaving health falsely green and burning the full
# ROUTE_TIMEOUT. detect_blocking_prompt classifies the known blockers, prompt_dismiss_key gives
# the dismiss key, and _stall_over_budget decides the early-give-up. _engage_over_deadline
# (age-55qz.10) adds the wall-clock give-up that catches a compacting pane the stall misses.
# All pure — locked here.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"; ORIG_PATH="$PATH"; mkdir -p "$TMP/bin"
  SENT="$TMP/sent"; : > "$SENT"
  # tmux mock: capture-pane emits PANE_TXT; send-keys APPENDS its args to $SENT so a test can
  # assert whether (and what) keys were injected.
  cat >"$TMP/bin/tmux" <<EOF
#!/usr/bin/env bash
case "\$1" in
  capture-pane) printf '%s\n' "\${PANE_TXT:-}" ;;
  send-keys)    echo "\$*" >> "$SENT" ;;
esac
exit 0
EOF
  chmod +x "$TMP/bin/tmux"; export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"   # source-guard returns before dispatch
}

teardown() { export PATH="$ORIG_PATH"; rm -rf "$TMP"; }

# --- detect_blocking_prompt ---

@test "detect_blocking_prompt: trust gate -> trust-gate" {
  run detect_blocking_prompt "Antigravity CLI requires permission to read … Yes, I trust this folder"
  [ "$output" = "trust-gate" ]
}

@test "detect_blocking_prompt: codex trust-directory prompt -> trust-gate" {
  run detect_blocking_prompt "Do you trust the contents of this directory? Press enter to continue"
  [ "$output" = "trust-gate" ]
}

@test "detect_blocking_prompt: codex update dialog -> codex-update" {
  run detect_blocking_prompt "Update available 0.141 -> 0.142. Press enter to continue"
  [ "$output" = "codex-update" ]
}

@test "detect_blocking_prompt: codex update MENU -> codex-update-menu (not codex-update)" {
  # The menu form defaults to "Update now" (runs brew upgrade), so it MUST classify distinctly
  # from the plain continue form — a plain Enter on this menu selects the destructive default.
  run detect_blocking_prompt "✨ Update available! 0.141 -> 0.142
› 1. Update now (runs brew upgrade --cask codex)
  2. Skip
  3. Skip until next version"
  [ "$output" = "codex-update-menu" ]
}

@test "detect_blocking_prompt: agy feedback survey -> agy-survey" {
  run detect_blocking_prompt "How's the CLI experience? [1]Good [2]Fine [3]Bad [0]Skip"
  [ "$output" = "agy-survey" ]
}

@test "detect_blocking_prompt: a normal ready input box -> empty (no false block)" {
  run detect_blocking_prompt "> type your message here   (esc to interrupt)"
  [ -z "$output" ]
}

@test "detect_blocking_prompt: empty capture -> empty" {
  run detect_blocking_prompt ""
  [ -z "$output" ]
}

# --- prompt_dismiss_key ---

@test "prompt_dismiss_key: trust-gate + codex-update -> Enter; agy-survey -> 0 Enter; unknown -> empty" {
  [ "$(prompt_dismiss_key trust-gate)" = "Enter" ]
  [ "$(prompt_dismiss_key codex-update)" = "Enter" ]
  [ "$(prompt_dismiss_key agy-survey)" = "0 Enter" ]
  [ -z "$(prompt_dismiss_key something-else)" ]
}

@test "prompt_dismiss_key: codex-update-menu navigates to Skip, never a bare Enter on 'Update now'" {
  local keys; keys="$(prompt_dismiss_key codex-update-menu)"
  [ "$keys" = "Down Enter" ]      # Down to "Skip", then Enter — not the default "Update now"
  [ "$keys" != "Enter" ]          # the anti-regression: must NOT select the destructive default
}

# --- _stall_over_budget (early-give-up decision) ---

@test "_stall_over_budget: stall >= budget -> give up (0)" {
  run _stall_over_budget 150 150
  [ "$status" -eq 0 ]
  run _stall_over_budget 200 150
  [ "$status" -eq 0 ]
}

@test "_stall_over_budget: stall < budget -> keep waiting (non-zero)" {
  run _stall_over_budget 145 150
  [ "$status" -ne 0 ]
  run _stall_over_budget 0 150
  [ "$status" -ne 0 ]
}

@test "_stall_over_budget: budget 0 disables give-up (never fires, even at huge stall)" {
  run _stall_over_budget 99999 0
  [ "$status" -ne 0 ]
}

# --- _engage_over_deadline (age-55qz.10 absolute per-pane engagement deadline) ---
# Unlike _stall_over_budget, this fires on wall-clock waited-seconds ALONE — no "output went quiet"
# precondition — so it catches a compacting opus pane that re-renders every tick (never stalls) yet
# never emits a verdict. Reads PAWL_ENGAGE_DEADLINE from the environment.

@test "_engage_over_deadline: waited >= deadline -> give up (0), on wall-clock alone" {
  PAWL_ENGAGE_DEADLINE=240
  run _engage_over_deadline 240
  [ "$status" -eq 0 ]
  run _engage_over_deadline 300
  [ "$status" -eq 0 ]
}

@test "_engage_over_deadline: waited < deadline -> keep waiting (non-zero)" {
  PAWL_ENGAGE_DEADLINE=240
  run _engage_over_deadline 239
  [ "$status" -ne 0 ]
  run _engage_over_deadline 0
  [ "$status" -ne 0 ]
}

@test "_engage_over_deadline: deadline 0 disables (never fires, even past any wait)" {
  PAWL_ENGAGE_DEADLINE=0
  run _engage_over_deadline 99999
  [ "$status" -ne 0 ]
}

# --- clear_known_prompts BOTTOM-ANCHOR (the cross-family-review fix): a trigger phrase in the
#     reviewed scrollback BODY must NOT inject keys into a working pane; only the live bottom acts ---

@test "clear_known_prompts: trigger phrase in the scrollback BODY (not the bottom) is IGNORED — no keys sent" {
  # The exact lost-verdict fail-open: a reviewed diff that merely CONTAINS the phrase, far above
  # the live prompt region. >10 benign lines after it push it out of the bottom window.
  PANE_TXT="$(printf 'the diff says: press enter to continue\n'; printf 'benign output %s\n' $(seq 1 14))"
  export PANE_TXT
  run clear_known_prompts 2
  [ "$status" -ne 0 ]          # nothing classified as a prompt
  [ ! -s "$SENT" ]             # and CRUCIALLY: no keys injected into the (working) pane
}

@test "clear_known_prompts: a real prompt at the BOTTOM is dismissed (keys sent)" {
  PANE_TXT="$(printf 'some prior review output\n'; printf 'Update available 0.141 -> 0.142. Press enter to continue\n')"
  export PANE_TXT
  run clear_known_prompts 2
  [ "$status" -eq 0 ]
  grep -q "Enter" "$SENT"      # the codex-update dialog at the bottom IS dismissed
}

@test "clear_known_prompts: agy survey at the bottom -> '0 Enter' sent" {
  PANE_TXT="$(printf 'reviewing…\n'; printf 'How'\''s the CLI experience? [1]Good [2]Fine [3]Bad [0]Skip\n')"
  export PANE_TXT
  run clear_known_prompts 3
  [ "$status" -eq 0 ]
  grep -q "0 Enter" "$SENT"
}
