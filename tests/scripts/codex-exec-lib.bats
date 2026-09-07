#!/usr/bin/env bats
# codex-exec-lib.bats — the one-shot contract for scripts/lib/codex-exec.sh.
# The runner must classify the three known failure modes (STALL /
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
  # Require a timeout binary for the STALL/timeout cases. Missing enforcement
  # now fails closed; hang fixtures need the supported capability installed.
  # NOTE: keep this a single `if` so setup's terminal exit status is always 0.
  # A bare `command -v gtimeout ... && HAVE_TIMEOUT=1` as the last setup line
  # returns non-zero on Linux CI (gtimeout is a macOS/coreutils name only),
  # which bats treats as a setup FAILURE and errors every test in the file.
  HAVE_TIMEOUT=0
  if command -v timeout >/dev/null 2>&1 || command -v gtimeout >/dev/null 2>&1; then
    HAVE_TIMEOUT=1
    REAL_TIMEOUT="$(command -v timeout || command -v gtimeout)"
  fi
}

teardown() { rm -rf "$TMP"; }

# Stop fixtures independently if a regression leaves a process behind. A zombie
# is already dead; only an executing survivor violates the cleanup contract.
assert_stopped() {
  local pid="$1" state
  state="$(ps -o stat= -p "$pid" 2>/dev/null)" || return 0
  [[ "$state" == *Z* ]] && return 0
  kill -KILL "$pid" 2>/dev/null || true
  echo "surviving process: $pid ($state)" >&2
  return 1
}

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

# A CLI-shaped stub that rejects a leading-hyphen prompt unless the caller
# inserted the standard end-of-options marker first.
stub_option_parser() {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
seen_terminator=0
for arg in "$@"; do
  if [ "$arg" = "--" ]; then seen_terminator=1; continue; fi
  if [ "$seen_terminator" -eq 0 ] && [ "${arg#-}" != "$arg" ]; then
    case "$arg" in exec|--skip-git-repo-check|--sandbox|-m|-C|-c) continue;; esac
  fi
done
[ "$seen_terminator" -eq 1 ] || { echo 'missing option terminator' >&2; exit 2; }
[ "${!#}" = '--- canonical skill bytes' ] || { echo 'prompt mismatch' >&2; exit 3; }
printf 'accepted prompt\n'
printf 'tokens used: 1\n'
FAKE
  chmod +x "$TMP/bin/codex"
}

# A stub codex that records how many times it was invoked and prints nothing.
stub_flat_then_success() {
  cat > "$TMP/bin/codex" <<FAKE
#!/usr/bin/env bash
COUNT="$TMP/flat-count"
n=\$(cat "\$COUNT" 2>/dev/null || echo 0)
n=\$((n + 1)); echo "\$n" > "\$COUNT"
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

@test "(a2) caller can capture structured stdout without merging stderr" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
printf '{"type":"turn.completed"}\n'
printf 'runtime warning\n' >&2
FAKE
  chmod +x "$TMP/bin/codex"
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_OUT_FILE="'"$TMP"'/stdout.jsonl" \
    CODEX_EXEC_STDERR_FILE="'"$TMP"'/stderr.log" \
    REVIEWER_MARKER="turn.completed" CODEX_EXEC_PROMPT_ARG="probe" \
    CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [ "$output" = "" ]
  [ "$(cat "$TMP/stdout.jsonl")" = '{"type":"turn.completed"}' ]
  [ "$(cat "$TMP/stderr.log")" = "runtime warning" ]
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

@test "(c2) an arg prompt beginning with hyphens is protected by an option terminator" {
  stub_option_parser
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_PROMPT_ARG="--- canonical skill bytes" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"accepted prompt"* ]]
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

# --- (e) flat 0-byte output is reported after exactly one call ----------------
@test "(e) a flat 0-byte run is not retried" {
  stub_flat_then_success
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_PROMPT_ARG="do the thing" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 124 ]
  [ "$(cat "$TMP/flat-count")" -eq 1 ]
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
  [[ "$output" == *"runtime evidence, not a review verdict"* ]]
  [[ "$output" != *"genuine reviewer failure"* ]]
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
  # shellcheck disable=SC2016 # Assert literal parameters in the emitted template.
  [ "$output" = 'timeout "$3" codex exec --skip-git-repo-check -C "$1" -s workspace-write "$2" >/dev/null 2>&1' ]
  run bash -c '. "'"$LIB"'"; codex_exec_producer_template membrane'
  [ "$status" -eq 0 ]
  # shellcheck disable=SC2016 # Assert literal parameters in the emitted template.
  [ "$output" = 'codex exec --skip-git-repo-check "$1" 2>/dev/null' ]
}

# --- CODEX_EXEC_WRAP: an external (filesystem-sealing) prefix around codex ----
# The probe harness seals reps with an OUTER macOS `sandbox-exec` profile. Codex's
# own seatbelt cannot nest inside it (sandbox_apply: Operation not permitted), so
# a wrapped rep must run codex with --dangerously-bypass-approvals-and-sandbox (the
# flag codex documents for exactly "externally sandboxed" use) instead of
# --sandbox <value>. Wrapping via CODEX_EXEC_BIN is NOT acceptable: it flips the
# metadata tool's coverage_eligible to false. Hence a dedicated prefix array.

# A stub codex that records its argv one-per-line to $TMP/codex-argv, then
# succeeds with the genuine-run marker.
stub_argv_recorder() {
  cat > "$TMP/bin/codex" <<FAKE
#!/usr/bin/env bash
printf '%s\n' "\$@" > "$TMP/codex-argv"
echo "the answer is 42"
echo "tokens used: 1234"
exit 0
FAKE
  chmod +x "$TMP/bin/codex"
}

# A stub wrapper (stands in for `sandbox-exec -p <profile>`): records its own argv
# to $TMP/wrapper-argv, drops its one flag, then execs the rest (the codex argv).
stub_wrapper() {
  cat > "$TMP/bin/stub-wrapper" <<FAKE
#!/usr/bin/env bash
printf '%s\n' "\$@" > "$TMP/wrapper-argv"
[ "\$1" = "--flag" ] || { echo 'wrapper: expected --flag first' >&2; exit 97; }
shift
exec "\$@"
FAKE
  chmod +x "$TMP/bin/stub-wrapper"
}

# A stub `timeout` (shadows the real one on PATH) that records its argv to
# $TMP/timeout-argv, drops the budget, then execs the rest — so the test can
# assert the exact prefix ORDER: timeout, then wrapper, then codex.
stub_timeout() {
  cat > "$TMP/bin/timeout" <<FAKE
#!/usr/bin/env bash
# --foreground is probed functionally by the library, so the stub has to accept
# it the way GNU timeout does: `timeout --foreground 1 true` must exit 0.
if [ "\$1" = "--foreground" ]; then shift; fi
printf '%s\n' "\$@" > "$TMP/timeout-argv"
shift
exec "\$@"
FAKE
  chmod +x "$TMP/bin/timeout"
}

@test "(wrap-a) CODEX_EXEC_WRAP unset: codex argv is byte-identical to the unwrapped shape" {
  stub_argv_recorder
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_EXTRA_ARGS=(--json --ephemeral)
    CODEX_EXEC_SKIP_GIT_CHECK=1 CODEX_EXEC_SANDBOX=workspace-write \
    CODEX_EXEC_MODEL=gpt-test CODEX_EXEC_DIR=/tmp/work \
    CODEX_EXEC_PROMPT_ARG="do the thing" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  expected="$(printf '%s\n' exec --skip-git-repo-check --sandbox workspace-write -m gpt-test -C /tmp/work --json --ephemeral -- "do the thing")"
  [ "$(cat "$TMP/codex-argv")" = "$expected" ]
  [ ! -e "$TMP/wrapper-argv" ]
}

@test "(wrap-b) CODEX_EXEC_WRAP set: wrapper runs first with its flag, then codex with --dangerously-bypass-approvals-and-sandbox and NO --sandbox" {
  stub_argv_recorder
  stub_wrapper
  stub_timeout
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_WRAP=("'"$TMP"'/bin/stub-wrapper" --flag)
    CODEX_EXEC_EXTRA_ARGS=(--json --ephemeral)
    CODEX_EXEC_SKIP_GIT_CHECK=1 CODEX_EXEC_SANDBOX=workspace-write \
    CODEX_EXEC_MODEL=gpt-test CODEX_EXEC_DIR=/tmp/work \
    CODEX_EXEC_PROMPT_ARG="do the thing" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"the answer is 42"* ]]
  # The wrapper is OUTERMOST and was exec'd with its flag first; the timeout
  # wrapper sits inside it, so the sandbox is the outermost process.
  [ -s "$TMP/wrapper-argv" ]
  [ "$(sed -n 1p "$TMP/wrapper-argv")" = "--flag" ]
  [ "$(sed -n 2p "$TMP/wrapper-argv")" = "$TMP/bin/timeout" ]
  [ "$(sed -n 3p "$TMP/wrapper-argv")" = "--foreground" ]
  [ "$(sed -n 4p "$TMP/wrapper-argv")" = "10" ]
  [ "$(sed -n 5p "$TMP/wrapper-argv")" = "codex" ]
  [ "$(sed -n 6p "$TMP/wrapper-argv")" = "exec" ]
  # codex saw the bypass flag in place of --sandbox <value>; everything else intact.
  expected="$(printf '%s\n' exec --skip-git-repo-check --dangerously-bypass-approvals-and-sandbox -m gpt-test -C /tmp/work --json --ephemeral -- "do the thing")"
  [ "$(cat "$TMP/codex-argv")" = "$expected" ]
  ! grep -qx -- '--sandbox' "$TMP/codex-argv"
  ! grep -qx -- 'workspace-write' "$TMP/codex-argv"
  grep -qx -- '--dangerously-bypass-approvals-and-sandbox' "$TMP/codex-argv"
}

@test "(wrap-c) the seal is outermost: order is wrapper, timeout --foreground, codex" {
  # GNU timeout calls setpgid(0,0), so with timeout OUTSIDE the wrapper the
  # reviewer and every child landed in timeout's process group and a caller
  # reaping "the rep's group" signalled a group the rep was never in. The
  # wrapper also has to be outermost for the timeout binary itself to run
  # INSIDE the sandbox rather than being resolved from PATH outside it.
  stub_argv_recorder
  stub_wrapper
  stub_timeout
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_WRAP=("'"$TMP"'/bin/stub-wrapper" --flag)
    CODEX_EXEC_PROMPT_ARG="do the thing" CODEX_EXEC_TIMEOUT=7 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [ -s "$TMP/wrapper-argv" ]
  [ "$(sed -n 1p "$TMP/wrapper-argv")" = "--flag" ]
  [ "$(sed -n 2p "$TMP/wrapper-argv")" = "$TMP/bin/timeout" ]
  [ "$(sed -n 3p "$TMP/wrapper-argv")" = "--foreground" ]
  [ "$(sed -n 4p "$TMP/wrapper-argv")" = "7" ]
  [ "$(sed -n 5p "$TMP/wrapper-argv")" = "codex" ]
  [ -s "$TMP/timeout-argv" ]
  [ "$(sed -n 1p "$TMP/timeout-argv")" = "7" ]
  [ "$(sed -n 2p "$TMP/timeout-argv")" = "codex" ]
  [ "$(sed -n 3p "$TMP/timeout-argv")" = "exec" ]
  [ -s "$TMP/codex-argv" ]
}

@test "(wrap-e) a timeout that refuses --foreground fails closed as MISSING" {
  # Running without the flag would silently put the reviewer back in timeout's
  # process group, which is the defect this whole ordering exists to close.
  stub_argv_recorder
  cat > "$TMP/bin/timeout" <<'FAKE'
#!/usr/bin/env bash
if [ "$1" = "--foreground" ]; then
  printf 'timeout: unrecognized option --foreground\n' >&2
  exit 125
fi
shift
exec "$@"
FAKE
  chmod +x "$TMP/bin/timeout"
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_PROMPT_ARG="do the thing" CODEX_EXEC_TIMEOUT=7 codex_exec_guarded
  '
  [ "$status" -eq 2 ]
  [[ "$output" == *"MISSING-TIMEOUT"* ]]
  [ ! -e "$TMP/codex-argv" ]
}

@test "(wrap-d) a wrapped run preserves the exit-code contract (genuine non-zero verbatim, stall 124)" {
  stub_wrapper
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
echo "codex refused to run" >&2
exit 5
FAKE
  chmod +x "$TMP/bin/codex"
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_WRAP=("'"$TMP"'/bin/stub-wrapper" --flag)
    CODEX_EXEC_EXPECT_OUTPUT=0 CODEX_EXEC_PROMPT_ARG="x" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 5 ]
  [ "$HAVE_TIMEOUT" -eq 1 ] || skip "no timeout/gtimeout on PATH"
  stub_hang
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_WRAP=("'"$TMP"'/bin/stub-wrapper" --flag)
    CODEX_EXEC_PROMPT_ARG="x" CODEX_EXEC_TIMEOUT=1 codex_exec_guarded
  '
  [ "$status" -eq 124 ]
}

@test "(wrap-e) CODEX_EXEC_WRAP is codex-only: the agy adapter ignores it and keeps --sandbox" {
  stub_wrapper
  cat > "$TMP/bin/agy" <<FAKE
#!/usr/bin/env bash
printf '%s\n' "\$@" > "$TMP/agy-argv"
echo "VERDICT: CONFIRMED"
FAKE
  chmod +x "$TMP/bin/agy"
  run bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_WRAP=("'"$TMP"'/bin/stub-wrapper" --flag)
    REVIEWER=agy CODEX_EXEC_PROMPT_ARG="review this" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [ ! -e "$TMP/wrapper-argv" ]
  grep -qx -- '--sandbox' "$TMP/agy-argv"
  ! grep -qx -- '--dangerously-bypass-approvals-and-sandbox' "$TMP/agy-argv"
}

@test "bounds: omitted timeout uses the finite 600 second default" {
  stub_argv_recorder
  stub_timeout
  run bash -c '. "'"$LIB"'"; CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 0 ]
  [ "$(sed -n 1p "$TMP/timeout-argv")" = "600" ]
}

@test "bounds: invalid timeout and capture limits prevent reviewer launch" {
  stub_argv_recorder
  for setting in CODEX_EXEC_TIMEOUT CODEX_EXEC_MAX_OUTPUT_BYTES; do
    for invalid in '' 0 -1 NaN inf 1e999 bad; do
      run env "$setting=$invalid" bash -c '. "'"$LIB"'"; CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
      [ "$status" -eq 2 ]
      [[ "$output" == *"INVALID-LIMIT"* ]]
      [ ! -e "$TMP/codex-argv" ]
    done
  done
}

@test "bounds: invalid and expired inherited deadlines prevent launch" {
  stub_argv_recorder
  for deadline in '' 0 -1 NaN later; do
    run env "CODEX_EXEC_DEADLINE_EPOCH=$deadline" bash -c '. "'"$LIB"'"; CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
    [ "$status" -eq 2 ]
    [ ! -e "$TMP/codex-argv" ]
  done
  run env CODEX_EXEC_DEADLINE_EPOCH=1 bash -c '. "'"$LIB"'"; CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 124 ]
  [ ! -e "$TMP/codex-argv" ]
}

@test "bounds: missing timeout enforcement fails closed" {
  stub_argv_recorder
  run bash -c '. "'"$LIB"'"; CODEX_EXEC_TIMEOUT_BIN=/nonexistent/timeout CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 2 ]
  [[ "$output" == *"MISSING-TIMEOUT"* ]]
  [ ! -e "$TMP/codex-argv" ]
}

@test "bounds: TERM resistant reviewer and child are killed with partial evidence" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
trap '' TERM
echo $$ > "$TMPDIR/reviewer-pid"
bash -c 'trap "" TERM; echo $$ > "$TMPDIR/child-pid"; exec sleep 30' &
printf 'partial result\n'
wait
FAKE
  chmod +x "$TMP/bin/codex"
  start=$SECONDS
  run timeout --kill-after=1 5 bash -c '. "'"$LIB"'"; CODEX_EXEC_TIMEOUT=.3 CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 124 ]
  [ $((SECONDS - start)) -lt 4 ]
  assert_stopped "$(cat "$TMP/reviewer-pid")"
  assert_stopped "$(cat "$TMP/child-pid")"
  [[ "$output" == *"partial result"* ]]
}

@test "bounds: clean parent exit with children is degraded even when cleanup succeeds" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
bash -c 'trap "" TERM; echo $$ > "$TMPDIR/child-pid"; exec sleep 30' &
while [ ! -s "$TMPDIR/child-pid" ]; do sleep .01; done
printf 'tokens used: 1\n'
exit 0
FAKE
  chmod +x "$TMP/bin/codex"
  start=$SECONDS
  run timeout --kill-after=1 5 bash -c '. "'"$LIB"'"; CODEX_EXEC_TIMEOUT=3 CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 122 ]
  [ $((SECONDS - start)) -lt 3 ]
  assert_stopped "$(cat "$TMP/child-pid")"
  [[ "$output" == *"rep-survivor"* ]]
  [[ "$output" == *"tokens used: 1"* ]]
  [[ "$output" != *"genuine reviewer failure"* ]]
}

@test "bounds: silent producer descendants are degraded without inherited capture pipes" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
sleep 30 </dev/null >/dev/null 2>&1 &
echo $! > "$TMPDIR/child-pid"
exit 0
FAKE
  chmod +x "$TMP/bin/codex"
  run timeout --kill-after=1 5 bash -c '. "'"$LIB"'"; CODEX_EXEC_TIMEOUT=3 CODEX_EXEC_EXPECT_OUTPUT=0 CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 122 ]
  [[ "$output" == *"rep-survivor"* ]]
  assert_stopped "$(cat "$TMP/child-pid")"
}

@test "bounds: continuous stdout and stderr share one exact capture cap" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
trap '' TERM
echo $$ > "$TMPDIR/reviewer-pid"
while :; do printf 'stdout data\n'; printf 'stderr data\n' >&2; done
FAKE
  chmod +x "$TMP/bin/codex"
  run timeout --kill-after=1 5 bash -c '
    . "'"$LIB"'"
    CODEX_EXEC_TIMEOUT=3 CODEX_EXEC_MAX_OUTPUT_BYTES=1024 \
    CODEX_EXEC_OUT_FILE="'"$TMP"'/out" CODEX_EXEC_STDERR_FILE="'"$TMP"'/err" \
    CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded
  '
  [ "$status" -eq 123 ]
  [ $(( $(wc -c < "$TMP/out") + $(wc -c < "$TMP/err") )) -eq 1024 ]
  [[ "$output" == *"OUTPUT-LIMIT"* ]]
  assert_stopped "$(cat "$TMP/reviewer-pid")"
}

@test "bounds: repeated invocations cannot renew a shared deadline" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
printf 'invoked\n' >> "$TMPDIR/invocations"
sleep .8
printf 'tokens used: 1\n'
FAKE
  chmod +x "$TMP/bin/codex"
  run timeout --kill-after=1 5 bash -c '
    . "'"$LIB"'"
    export CODEX_EXEC_DEADLINE_EPOCH="$(/usr/bin/perl -MTime::HiRes=time -e "print time + 1.5")"
    export CODEX_EXEC_TIMEOUT=10 CODEX_EXEC_PROMPT_ARG=x
    codex_exec_guarded || exit 91
    codex_exec_guarded; second=$?
    codex_exec_guarded; third=$?
    [ "$second" -eq 124 ] && [ "$third" -eq 124 ]
  '
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$TMP/invocations" | tr -d ' ')" -eq 2 ]
}

@test "bounds: cancellation of the sourced caller cleans workers and restores traps" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
trap '' TERM
echo $$ > "$TMPDIR/reviewer-pid"
bash -c 'trap "" TERM; echo $$ > "$TMPDIR/child-pid"; exec sleep 30' &
printf 'partial before cancellation\n'
wait
FAKE
  chmod +x "$TMP/bin/codex"
  cat > "$TMP/caller.sh" <<'CALLER'
source "$1"
trap 'echo original-term-handler' TERM
original=$(trap -p TERM)
CODEX_EXEC_TIMEOUT=10 CODEX_EXEC_OUT_FILE="$TMPDIR/evidence" CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded
result=$?
[ "$(trap -p TERM)" = "$original" ] || exit 92
exit "$result"
CALLER
  run timeout --kill-after=1 5 bash -c '
    bash "'"$TMP"'/caller.sh" "'"$LIB"'" & runner=$!
    for attempt in {1..100}; do [ -s "'"$TMP"'/child-pid" ] && break; sleep .02; done
    kill -TERM "$runner"
    wait "$runner"
  '
  [ "$status" -eq 143 ]
  grep -q 'partial before cancellation' "$TMP/evidence"
  assert_stopped "$(cat "$TMP/reviewer-pid")"
  assert_stopped "$(cat "$TMP/child-pid")"
}

@test "bounds: parent death also stops the owned worker group" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
trap '' TERM
echo $$ > "$TMPDIR/reviewer-pid"
printf 'partial before parent death\n'
exec sleep 30
FAKE
  chmod +x "$TMP/bin/codex"
  run timeout --kill-after=1 5 bash -c '
    bash -c '\''source "$1"; CODEX_EXEC_TIMEOUT=10 CODEX_EXEC_OUT_FILE="$TMPDIR/evidence" CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'\'' _ "'"$LIB"'" & runner=$!
    for attempt in {1..100}; do [ -s "'"$TMP"'/reviewer-pid" ] && break; sleep .02; done
    kill -KILL "$runner"
    wait "$runner" || true
    sleep .5
  '
  [ "$status" -eq 0 ]
  assert_stopped "$(cat "$TMP/reviewer-pid")"
  grep -q 'partial before parent death' "$TMP/evidence"
}

@test "bounds: missing default timeout and hanging capability probe prevent launch" {
  stub_argv_recorder
  mkdir "$TMP/no-timeout"
  ln -s "$(command -v tr)" "$TMP/no-timeout/tr"
  run env PATH="$TMP/no-timeout" CODEX_EXEC_BIN="$TMP/bin/codex" /bin/bash -c '. "'"$LIB"'"; CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 2 ]
  [[ "$output" == *"MISSING-TIMEOUT"* ]]
  [ ! -e "$TMP/codex-argv" ]
  cat > "$TMP/bin/timeout" <<'FAKE'
#!/usr/bin/env bash
trap '' TERM
echo $$ > "$TMPDIR/probe-pid"
exec sleep 30
FAKE
  chmod +x "$TMP/bin/timeout"
  start=$SECONDS
  run "$REAL_TIMEOUT" --kill-after=1 5 bash -c '. "'"$LIB"'"; CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 2 ]
  [ $((SECONDS - start)) -lt 4 ]
  [ ! -e "$TMP/codex-argv" ]
  assert_stopped "$(cat "$TMP/probe-pid")"
}

@test "bounds: file and stdin prompt bytes survive bounded capture" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
cat > "$TMPDIR/received-prompt"
printf 'tokens used: 1\n'
FAKE
  chmod +x "$TMP/bin/codex"
  printf 'file prompt\n\n' > "$TMP/prompt"
  run bash -c '. "'"$LIB"'"; CODEX_EXEC_PROMPT_FILE="'"$TMP"'/prompt" CODEX_EXEC_PROMPT_ARG=ignored codex_exec_guarded'
  [ "$status" -eq 0 ]
  cmp "$TMP/prompt" "$TMP/received-prompt"
  run bash -c '. "'"$LIB"'"; printf "stdin prompt\n\n" | codex_exec_guarded'
  [ "$status" -eq 0 ]
  [ "$(cat "$TMP/received-prompt")" = 'stdin prompt' ]
}

@test "bounds: cancelling the caller during a hanging capability probe prevents launch" {
  stub_argv_recorder
  cat > "$TMP/bin/timeout" <<'FAKE'
#!/usr/bin/env bash
trap '' TERM
echo $$ > "$TMPDIR/probe-pid"
exec sleep 30
FAKE
  chmod +x "$TMP/bin/timeout"
  run "$REAL_TIMEOUT" --kill-after=1 5 bash -c '
    bash -c '\''source "$1"; CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'\'' _ "'"$LIB"'" & runner=$!
    for attempt in {1..100}; do [ -s "'"$TMP"'/probe-pid" ] && break; sleep .02; done
    kill -TERM "$runner"
    wait "$runner" || true
    sleep .5
  '
  [ "$status" -eq 0 ]
  [ ! -e "$TMP/codex-argv" ]
  assert_stopped "$(cat "$TMP/probe-pid")"
}

@test "bounds: unopened prompt FIFO expires before reviewer launch" {
  stub_argv_recorder
  mkfifo "$TMP/prompt-pipe"
  run timeout --kill-after=1 5 bash -c '. "'"$LIB"'"; CODEX_EXEC_TIMEOUT=.3 CODEX_EXEC_PROMPT_FILE="'"$TMP"'/prompt-pipe" codex_exec_guarded'
  [ "$status" -eq 124 ]
  [[ "$output" == *"partial input preserved at"* ]]
  [ ! -e "$TMP/codex-argv" ]
}

@test "bounds: capture limits do not limit reviewer workspace file writes" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
dd if=/dev/zero of="$TMPDIR/work-product" bs=4096 count=8 2>/dev/null
printf 'tokens used: 1\n'
FAKE
  chmod +x "$TMP/bin/codex"
  run bash -c '. "'"$LIB"'"; CODEX_EXEC_MAX_OUTPUT_BYTES=32 CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 0 ]
  [ "$(wc -c < "$TMP/work-product" | tr -d ' ')" -eq 32768 ]
}

@test "bounds: default capture limit is finite and retains exactly ten MiB" {
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
dd if=/dev/zero bs=1048576 count=11 2>/dev/null
FAKE
  chmod +x "$TMP/bin/codex"
  run "$REAL_TIMEOUT" --kill-after=1 5 bash -c '. "'"$LIB"'"; CODEX_EXEC_OUT_FILE="'"$TMP"'/evidence" CODEX_EXEC_PROMPT_ARG=x codex_exec_guarded'
  [ "$status" -eq 123 ]
  [ "$(wc -c < "$TMP/evidence" | tr -d ' ')" -eq 10485760 ]
}
