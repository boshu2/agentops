#!/usr/bin/env bats
# verdict_of parser robustness (age-nomq): the route reads a reviewer's verdict from its pane
# capture. The per-route nonce stops STALE-scrollback matches, but it does NOT stop a reviewer
# that NARRATES the verdict format with the concrete words — codex once wrote, mid-review, "the
# final line must be PAWL <nonce> CONFIRMED or PAWL <nonce> REFUTED. I'm keeping notes…", and the
# un-anchored regex matched "PAWL <nonce> REFUTED" inside that prose → a FALSE verdict (and the
# symmetric narrated-CONFIRMED case is a FAIL-OPEN). The fix anchors the verdict to END-OF-LINE.
# These tests pin both directions: narration NEVER yields a verdict; a real verdict line does.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"
  ORIG_PATH="$PATH"
  mkdir -p "$TMP/bin"
  cat >"$TMP/bin/tmux" <<'EOF'
#!/usr/bin/env bash
# capture-pane emits PANE_TXT (the simulated scrollback). Other verbs are no-ops.
case "$1" in
  capture-pane) printf '%s\n' "${PANE_TXT:-}" ;;
  *) : ;;
esac
exit 0
EOF
  chmod +x "$TMP/bin/tmux"
  export PATH="$TMP/bin:$PATH"
  # shellcheck disable=SC1090
  source "$REPO_ROOT/scripts/pawl.sh"
  NONCE="rba6503092"
}

teardown() {
  export PATH="$ORIG_PATH"
  rm -rf "$TMP"
}

@test "verdict_of: a real verdict line (alone) -> the verdict word" {
  export PANE_TXT="PAWL ${NONCE} CONFIRMED"
  run verdict_of 1 "$NONCE"
  [ "$output" = "CONFIRMED" ]
}

@test "verdict_of: a real verdict line with a leading TUI prefix still matches (end-anchored, not start)" {
  export PANE_TXT="• PAWL ${NONCE} REFUTED"
  run verdict_of 1 "$NONCE"
  [ "$output" = "REFUTED" ]
}

@test "verdict_of: a NARRATED REFUTED in prose -> NO verdict (the age-nomq false-REFUTE)" {
  # The exact shape codex emitted mid-review.
  export PANE_TXT="rba6503092 CONFIRMED or PAWL ${NONCE} REFUTED. I'm keeping notes internally,"
  run verdict_of 1 "$NONCE"
  [ -z "$output" ]
}

@test "verdict_of: a NARRATED CONFIRMED in prose -> NO verdict (the fail-OPEN this fix closes)" {
  export PANE_TXT="I will answer PAWL ${NONCE} CONFIRMED once I finish reviewing the diff."
  run verdict_of 1 "$NONCE"
  [ -z "$output" ]
}

@test "verdict_of: narration ENDING exactly at the token -> NO verdict (codex finding: end-anchor alone is insufficient)" {
  # An end-of-line anchor alone matched this; the WHOLE-LINE anchor rejects the prose prefix.
  export PANE_TXT="so after weighing it my answer is PAWL ${NONCE} CONFIRMED"
  run verdict_of 1 "$NONCE"
  [ -z "$output" ]
}

@test "verdict_of: an indented/box-prefixed verdict line still matches (only non-alnum chrome before PAWL)" {
  printf -v PANE_TXT '\t│ PAWL %s REFUTED' "$NONCE"
  export PANE_TXT
  run verdict_of 1 "$NONCE"
  [ "$output" = "REFUTED" ]
}

@test "verdict_of: narration THEN a real final verdict line -> the real verdict" {
  export PANE_TXT="planning to output PAWL ${NONCE} CONFIRMED or PAWL ${NONCE} REFUTED later
some intermediate reasoning here
PAWL ${NONCE} CONFIRMED"
  run verdict_of 1 "$NONCE"
  [ "$output" = "CONFIRMED" ]
}

@test "verdict_of: a different route's nonce -> NO verdict (nonce scoping preserved)" {
  export PANE_TXT="PAWL rOTHER9999 CONFIRMED"
  run verdict_of 1 "$NONCE"
  [ -z "$output" ]
}

@test "verdict_of: no verdict anywhere -> empty (route keeps polling, fails closed)" {
  export PANE_TXT="still working on the review, no conclusion yet"
  run verdict_of 1 "$NONCE"
  [ -z "$output" ]
}
