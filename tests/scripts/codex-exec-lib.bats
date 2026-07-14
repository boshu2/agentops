#!/usr/bin/env bats
# codex-exec-lib.bats — the fail-closed contract for scripts/lib/codex-exec.sh.
# age-gate-the-ungated-egwt.8: ONE hardened codex runner, extracted from the pawl
# surfaces + eval-membrane, must classify the three known failure modes (STALL /
# ECHO / MISSING-CODEX) into DISTINCT exit codes so a caller can tell NO-VERDICT
# apart from a genuine result. Every case here uses a STUB `codex` on PATH — the
# real codex binary is NEVER invoked.

setup() {
  LIB="$BATS_TEST_DIRNAME/../../scripts/lib/codex-exec.sh"
  [ -f "$LIB" ] || skip "lib not found: $LIB"
  TMP="$(mktemp -d)"
  mkdir -p "$TMP/bin"
  export PATH="$TMP/bin:$PATH"
  export TMPDIR="$TMP"
  # Require a timeout binary for the STALL/timeout cases (the lib degrades to
  # no-timeout when neither exists, so the kill-based assertions can't hold).
  # NOTE: keep this a single `if` so setup's terminal exit status is always 0.
  # A bare `command -v gtimeout ... && HAVE_TIMEOUT=1` as the last setup line
  # returns non-zero on Linux CI (gtimeout is a macOS/coreutils name only),
  # which bats treats as a setup FAILURE and errors every test in the file.
  HAVE_TIMEOUT=0
  if command -v timeout >/dev/null 2>&1 || command -v gtimeout >/dev/null 2>&1; then
    HAVE_TIMEOUT=1
  fi
}

teardown() { rm -rf "$TMP"; }

# --- stub factories -----------------------------------------------------------

# A stub codex that SUCCEEDS and prints a real answer WITH the `tokens used`
# marker a real run emits (so echo-detection does NOT mis-flag it).
stub_success() {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
echo "the answer is 42"
echo "tokens used: 1234"
exit 0
FAKE
  chmod +x "$TMP/bin/codex"
}

# A stub codex that HANGS well past a tiny timeout (STALL via kill).
stub_hang() {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
sleep 30
FAKE
  chmod +x "$TMP/bin/codex"
}

# A stub codex that ECHOES its prompt back (arg-prompt mode) with NO `tokens
# used` marker — the ECHO/WANDER failure the lib must catch fail-closed.
stub_echo() {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
# Reflect the LAST positional (the prompt) back verbatim — an echo, no marker.
prompt=""
for a in "$@"; do prompt="$a"; done
printf '%s' "$prompt"
exit 0
FAKE
  chmod +x "$TMP/bin/codex"
}

# A stub codex that prints NOTHING on the first run (flat 0-byte) and a real
# answer on the second — proves the retry-once-on-empty defense.
stub_flat_then_success() {
  cat > "$TMP/bin/codex" <<FAKE
#!/usr/bin/env bash
COUNT="$TMP/flat-count"
n=\$(cat "\$COUNT" 2>/dev/null || echo 0)
n=\$((n + 1)); echo "\$n" > "\$COUNT"
if [ "\$n" -eq 1 ]; then exit 0; fi   # first run: no output, exit clean (a stall)
echo "real answer after retry"
echo "tokens used: 9"
exit 0
FAKE
  chmod +x "$TMP/bin/codex"
}

# --- (a) SUCCESS with the tokens marker ---------------------------------------
@test "(a) success with 'tokens used' marker -> exit 0, output on stdout" {
  stub_success
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_PROMPT_ARG="do the thing" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"the answer is 42"* ]]
}

# --- (b) HANG -> STALL-TIMEOUT (exit 124) within budget -----------------------
@test "(b) a hung codex is killed and returns STALL-TIMEOUT (124) within budget" {
  [ "$HAVE_TIMEOUT" -eq 1 ] || skip "no timeout/gtimeout on PATH"
  stub_hang
  start=$SECONDS
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_PROMPT_ARG="do the thing" CODEX_EXEC_TIMEOUT=1 codex_exec_guarded
  '
  elapsed=$(( SECONDS - start ))
  [ "$status" -eq 124 ]
  # Killed at ~1s, never allowed to run the full 30s sleep.
  [ "$elapsed" -lt 10 ]
}

# --- (c) ECHO-only -> ECHO (exit 125) -----------------------------------------
@test "(c) an echo-only run (output ~= prompt, no marker) returns ECHO (125)" {
  stub_echo
  # A prompt long enough that the reflected echo clears the 80% size band.
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_PROMPT_ARG="review this large change and reply with a verdict; do not wander the filesystem" \
      CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 125 ]
}

# --- (d) MISSING codex -> MISSING (exit 2) ------------------------------------
@test "(d) a missing codex binary returns MISSING (2), a precondition not a result" {
  # No stub installed AND point CODEX_EXEC_BIN at a name that does not exist.
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_BIN="codex-does-not-exist-xyz" CODEX_EXEC_PROMPT_ARG="x" codex_exec_guarded
  '
  [ "$status" -eq 2 ]
  [[ "$output" == *"MISSING DEPENDENCY"* ]]
}

# --- (e) flat 0-byte then success -> retry-once works (exit 0) -----------------
@test "(e) a flat 0-byte first run is retried once, then succeeds (exit 0)" {
  stub_flat_then_success
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_PROMPT_ARG="do the thing" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"real answer after retry"* ]]
  # The stub was invoked TWICE (first empty, then real) — the retry fired.
  [ "$(cat "$TMP/flat-count")" -eq 2 ]
}

# --- fire-and-score caller: empty output on a clean exit is SUCCESS, not STALL -
@test "(f) EXPECT_OUTPUT=0: a clean exit-0 with empty output is SUCCESS, not a stall" {
  # A stub that does real work but prints nothing (like a producer writing to disk).
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
exit 0
FAKE
  chmod +x "$TMP/bin/codex"
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_EXPECT_OUTPUT=0 CODEX_EXEC_PROMPT_ARG="build it" CODEX_EXEC_TIMEOUT=10 \
      codex_exec_guarded
  '
  [ "$status" -eq 0 ]
}

# --- genuine non-zero exit is preserved verbatim (not remapped) ----------------
@test "(g) a genuine codex non-zero exit is returned verbatim (e.g. 1), distinct from a stall" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
echo "codex refused to run: not a trusted directory" >&2
exit 1
FAKE
  chmod +x "$TMP/bin/codex"
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_EXPECT_OUTPUT=0 CODEX_EXEC_PROMPT_ARG="x" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  # codex's own rc (1) is preserved — NOT collapsed to a fixed constant and NOT
  # mistaken for a stall (124).
  [ "$status" -eq 1 ]
}

# --- the distinct exit-code constants are defined and unique -------------------
@test "(h) the documented exit-code constants are defined and mutually distinct" {
  run bash -c '
    . "'"$LIB"'"
    echo "$CODEX_EXEC_OK $CODEX_EXEC_MISSING $CODEX_EXEC_STALL_TIMEOUT $CODEX_EXEC_ECHO"
  '
  [ "$status" -eq 0 ]
  [ "$output" = "0 2 124 125" ]
}

# --- the producer/membrane templates are byte-stable --------------------------
@test "(i) codex_exec_producer_template emits the historical byte-identical defaults" {
  run bash -c '. "'"$LIB"'"; codex_exec_producer_template producer'
  [ "$status" -eq 0 ]
  [ "$output" = 'timeout "$3" codex exec --skip-git-repo-check -C "$1" -s workspace-write "$2" >/dev/null 2>&1' ]
  run bash -c '. "'"$LIB"'"; codex_exec_producer_template membrane'
  [ "$status" -eq 0 ]
  [ "$output" = 'codex exec --skip-git-repo-check "$1" 2>/dev/null' ]
}
