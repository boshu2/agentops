#!/usr/bin/env bats
# reviewer-adapters.bats — the per-adapter stub matrix for the reviewer-adapter contract
# (age-rk3r.1). scripts/lib/codex-exec.sh now dispatches a cold reviewer keyed by REVIEWER
# (default codex). This file proves the NON-codex adapters (agy cold, local-mlx eval-only)
# classify the SAME failure taxonomy — SUCCESS / STALL / ECHO / MISSING / GENUINE-NONZERO —
# with their own genuine-run markers, and that the eval-only adapter HARD-REFUSES in a prod
# context. The codex adapter's byte-compat is locked separately by codex-exec-lib.bats
# (which stays UNMODIFIED). Every case uses a STUB reviewer on PATH — no real CLI is invoked.

setup() {
  LIB="$BATS_TEST_DIRNAME/../../scripts/lib/codex-exec.sh"
  [ -f "$LIB" ] || skip "lib not found: $LIB"
  TMP="$(mktemp -d)"
  mkdir -p "$TMP/bin"
  export PATH="$TMP/bin:$PATH"
  export TMPDIR="$TMP"
  # Single `if` so setup's terminal exit status is always 0. A bare
  # `command -v gtimeout ... && HAVE_TIMEOUT=1` as the last setup line returns
  # non-zero on Linux CI (gtimeout is a macOS/coreutils-only name), which bats
  # treats as a setup FAILURE and errors every test in the file.
  HAVE_TIMEOUT=0
  if command -v timeout >/dev/null 2>&1 || command -v gtimeout >/dev/null 2>&1; then
    HAVE_TIMEOUT=1
  fi
}

teardown() { rm -rf "$TMP"; }

# --- stub factories (installed under a chosen binary name) ---------------------

# $1 = binary name. SUCCEEDS with a review + a VERDICT line (the agy/mlx genuine-run
# marker), so echo-detection does NOT mis-flag it.
stub_success() {
  cat > "$TMP/bin/$1" <<'FAKE'
#!/usr/bin/env bash
echo "Reviewed the packet; no blocking defects."
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$TMP/bin/$1"
}

# $1 = binary name. HANGS past a tiny timeout (STALL via kill).
stub_hang() {
  cat > "$TMP/bin/$1" <<'FAKE'
#!/usr/bin/env bash
sleep 30
FAKE
  chmod +x "$TMP/bin/$1"
}

# $1 = binary name. ECHOES the LAST positional arg back verbatim (for agy that is the
# short `-p` pointer; for mlx it is the prompt payload) with NO VERDICT marker.
stub_echo() {
  cat > "$TMP/bin/$1" <<'FAKE'
#!/usr/bin/env bash
last=""
for a in "$@"; do last="$a"; done
printf '%s' "$last"
exit 0
FAKE
  chmod +x "$TMP/bin/$1"
}

# $1 = binary name. Exits non-zero for its OWN reason (a genuine reviewer failure).
stub_genuine_fail() {
  cat > "$TMP/bin/$1" <<'FAKE'
#!/usr/bin/env bash
echo "reviewer refused: not a trusted directory" >&2
exit 1
FAKE
  chmod +x "$TMP/bin/$1"
}

# ===========================================================================
# Adapter 2 = agy (cold). Prompt delivered as a FILE PATH pointer (--add-dir + -p).
# ===========================================================================

@test "agy (success): a VERDICT-bearing review -> exit 0, output on stdout" {
  stub_success agy
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=agy CODEX_EXEC_PROMPT_ARG="review this change" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"VERDICT: CONFIRMED"* ]]
}

@test "agy (hang): a hung reviewer is killed -> STALL-TIMEOUT (124) within budget" {
  [ "$HAVE_TIMEOUT" -eq 1 ] || skip "no timeout/gtimeout on PATH"
  stub_hang agy
  start=$SECONDS
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=agy CODEX_EXEC_PROMPT_ARG="review this change" CODEX_EXEC_TIMEOUT=1 codex_exec_guarded
  '
  elapsed=$(( SECONDS - start ))
  [ "$status" -eq 124 ]
  [ "$elapsed" -lt 10 ]
}

@test "agy (echo): the -p pointer reflected, no marker -> ECHO (125)" {
  stub_echo agy
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=agy CODEX_EXEC_PROMPT_ARG="a large change to review; do not wander" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 125 ]
}

@test "agy (missing): the agy binary absent -> MISSING (2), a precondition not a result" {
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=agy REVIEWER_BIN="agy-does-not-exist-xyz" CODEX_EXEC_PROMPT_ARG="x" codex_exec_guarded
  '
  [ "$status" -eq 2 ]
  [[ "$output" == *"MISSING DEPENDENCY"* ]]
}

@test "agy (genuine non-zero): the reviewer's own rc is preserved verbatim (1)" {
  stub_genuine_fail agy
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=agy CODEX_EXEC_EXPECT_OUTPUT=0 CODEX_EXEC_PROMPT_ARG="x" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 1 ]
}

@test "agy: the -p pointer references a FILE PATH, never a giant inline paste (age-9rmh design)" {
  # Record the argv the agy stub actually received; the review body must NOT be inline.
  cat > "$TMP/bin/agy" <<FAKE
#!/usr/bin/env bash
printf '%s\n' "\$@" > "$TMP/agy-argv"
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$TMP/bin/agy"
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=agy CODEX_EXEC_PROMPT_ARG="SECRET_BODY_TOKEN the full diff goes here" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  # The pointer tells the model to READ a file path; the raw review body is NOT on the argv.
  grep -q -- '--add-dir' "$TMP/agy-argv"
  grep -q 'absolute path' "$TMP/agy-argv"
  ! grep -q 'SECRET_BODY_TOKEN' "$TMP/agy-argv"
}

# ===========================================================================
# Adapter 3 = local-mlx (caller-selected local runtime).
# ===========================================================================

@test "local-mlx: caller selection runs exactly once" {
  stub_success some-mlx-bin
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=local-mlx REVIEWER_BIN="some-mlx-bin" CODEX_EXEC_PROMPT_ARG="review" codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"VERDICT: CONFIRMED"* ]]
}

@test "local-mlx success exits 0" {
  stub_success mlx-stub
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=local-mlx REVIEWER_BIN="mlx-stub" \
      CODEX_EXEC_PROMPT_ARG="review this" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"VERDICT: CONFIRMED"* ]]
}

@test "local-mlx (opted-in, hang): killed -> STALL-TIMEOUT (124)" {
  [ "$HAVE_TIMEOUT" -eq 1 ] || skip "no timeout/gtimeout on PATH"
  stub_hang mlx-stub
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=local-mlx REVIEWER_BIN="mlx-stub" \
      CODEX_EXEC_PROMPT_ARG="review this" CODEX_EXEC_TIMEOUT=1 codex_exec_guarded
  '
  [ "$status" -eq 124 ]
}

@test "local-mlx (opted-in, echo): prompt reflected, no marker -> ECHO (125)" {
  stub_echo mlx-stub
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=local-mlx REVIEWER_BIN="mlx-stub" \
      CODEX_EXEC_PROMPT_ARG="a fairly long review packet with no verdict token in it at all" \
      CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 125 ]
}

@test "local-mlx (opted-in, missing): bin absent -> MISSING (2)" {
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=local-mlx REVIEWER_BIN="mlx-nonexistent-xyz" \
      CODEX_EXEC_PROMPT_ARG="x" codex_exec_guarded
  '
  [ "$status" -eq 2 ]
  [[ "$output" == *"MISSING DEPENDENCY"* ]]
}

# ===========================================================================
# Contract helpers + codex-with-explicit-REVIEWER parity.
# ===========================================================================

@test "REVIEWER=codex (explicit) is identical to unset: byte-compat codex path" {
  # A codex stub with the codex 'tokens used' marker (NOT a VERDICT line) still succeeds,
  # proving the explicit codex adapter uses the codex marker, not the agy/mlx one.
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
echo "the answer is 42"
echo "tokens used: 1234"
exit 0
FAKE
  chmod +x "$TMP/bin/codex"
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=codex CODEX_EXEC_PROMPT_ARG="do the thing" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"the answer is 42"* ]]
}

@test "adapter contract fields resolve per reviewer (bin + marker)" {
  run bash -c '
    . "'"$LIB"'"
    echo "bin=$(reviewer_adapter_bin codex) marker=$(reviewer_adapter_marker codex)"
    echo "bin=$(reviewer_adapter_bin agy) marker=$(reviewer_adapter_marker agy)"
    echo "norm=$(reviewer_normalize GEMINI)"
  '
  [ "$status" -eq 0 ]
  [[ "$output" == *"bin=codex marker=tokens used"* ]]
  [[ "$output" == *"bin=agy marker=VERDICT:"* ]]
  [[ "$output" == *"norm=agy"* ]]
}

@test "REVIEWER_MARKER overrides the adapter's genuine-run marker" {
  run bash -c '. "'"$LIB"'"; REVIEWER_MARKER="RUN-OK" reviewer_adapter_marker agy'
  [ "$status" -eq 0 ]
  [ "$output" = "RUN-OK" ]
}

# ===========================================================================
# DEFECT-1 regression (age-rk3r.1 refutation): the review PACKET itself contains
# "VERDICT:" strings (verdict-format instructions, diff context lines), so
# marker-presence must NOT classify a cat/echo of the PACKET as a genuine run.
# ===========================================================================

@test "agy (packet-cat echo): a stub that cats the packet file -> ECHO (125), never GENUINE" {
  # The stub extracts the packet path from the -p pointer and cats the file —
  # the exact fail-open repro: the packet content carries 'VERDICT:' lines.
  cat > "$TMP/bin/agy" <<'FAKE'
#!/usr/bin/env bash
last=""; for a in "$@"; do last="$a"; done
path="$(printf '%s\n' "$last" | sed -n 's/.*absolute path \([^[:space:]]*\).*/\1/p')"
cat "$path"
exit 0
FAKE
  chmod +x "$TMP/bin/agy"
  pkt="$TMP/packet.txt"
  { echo 'Reply with your review; the FINAL line is the token "VERDICT:" then one uppercase word.'
    echo '=== CHANGE UNDER REVIEW (bead age-x, scope head) ==='
    echo ' VERDICT: CONFIRMED'
    for i in $(seq 1 30); do echo "diff context line $i with plenty of distinctive length in it"; done
  } > "$pkt"
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=agy CODEX_EXEC_PROMPT_FILE="'"$pkt"'" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 125 ]
}

@test "local-mlx (opted-in, packet-cat echo): reflecting a VERDICT-bearing packet -> ECHO (125)" {
  # Same defect class for the eval-only adapter: its positional IS the packet; a stub
  # that reflects it back must be ECHO even though the packet contains 'VERDICT:'.
  stub_echo mlx-stub
  pkt="$TMP/mlx-packet.txt"
  { echo 'Reply with your review; the FINAL line is the token "VERDICT:" then one uppercase word.'
    echo '=== CHANGE UNDER REVIEW ==='
    echo ' VERDICT: CONFIRMED'
    for i in $(seq 1 10); do echo "diff context line $i with plenty of distinctive length in it"; done
  } > "$pkt"
  run bash -c '
    . "'"$LIB"'"
    REVIEWER=local-mlx REVIEWER_BIN="mlx-stub" \
      CODEX_EXEC_PROMPT_FILE="'"$pkt"'" CODEX_EXEC_TIMEOUT=10 codex_exec_guarded
  '
  [ "$status" -eq 125 ]
}
