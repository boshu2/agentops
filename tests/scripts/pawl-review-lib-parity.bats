#!/usr/bin/env bats
# pawl-review-lib-parity.bats — the delegation lock for age-gate-the-ungated-egwt.13.
# pawl-review.sh now routes its COLD `codex exec` through codex_exec_guarded
# (scripts/lib/codex-exec.sh), the single source of truth for the STALL/ECHO/MISSING
# defenses. This file proves the cold path still classifies the .8 fixture matrix
# (success-with-marker / hang / echo-only / missing-codex) EXACTLY as the pre-delegation
# flow did: exit code + verdict-written must be identical. The pre-delegation outcomes
# (captured from origin/main during the change, PRE|POST both IDENTICAL) are frozen here:
#   success-with-marker -> exit 0, verdict WRITTEN
#   hang (killed)       -> exit 1, NO verdict (a non-zero reviewer run is never trusted)
#   echo-only           -> exit 1, NO verdict (no parseable/ trustworthy verdict)
#   missing-codex       -> exit 2, NO verdict (a PRECONDITION, not a result)
# The real codex binary is NEVER invoked — every case uses a STUB on PATH (or a codex-free
# PATH for the missing case). Everything runs in a throwaway repo (AGENTOPS_REPO_ROOT).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  # STAY cd'd inside the throwaway repo for the whole test (mirrors pawl-review.bats):
  # the provenance emit + ledger auto-bind resolve their root FROM CWD
  # (_ledger_root_from_cwd in pawl-verdict.sh), so a test that runs from the real
  # checkout would bind junk age-rev-test edges into the HOST repo's ledger.
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > README.md; git add README.md; git commit --quiet -m init
  echo change >> README.md; git add README.md
  git commit --quiet -m "feat(x): a change (age-rev-test)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-rev-test.json"
  export PAWL_NO_SERVICE=1            # force the cold codex-exec path (never route to a warm pane)
  export PAWL_REVIEW_TIMEOUT=2        # a tiny pinned budget so the hang case is killed fast
  export PAWL_AUTOBIND=0              # belt-and-suspenders: a test run must NEVER create a ledger bind commit
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

# A stub that SUCCEEDS with the codex marker + a real CONFIRMED verdict (+ the 'tokens used'
# marker a real run emits, so the lib's echo-detection never mis-flags it).
_stub_success() {
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
cat >/dev/null
echo codex
echo "Reviewed; no defects. tokens used: 1234"
echo "VERDICT: CONFIRMED"
exit 0
FAKE
  chmod +x "$BIN/codex"
}

# A stub that HANGS past the pinned 2s budget (killed -> STALL-TIMEOUT in the lib).
_stub_hang() {
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
cat >/dev/null
sleep 30
FAKE
  chmod +x "$BIN/codex"
}

# A stub that ECHOES the whole prompt back with NO verdict + NO marker (the ECHO/WANDER mode).
_stub_echo() {
  cat > "$BIN/codex" <<'FAKE'
#!/usr/bin/env bash
cat
exit 0
FAKE
  chmod +x "$BIN/codex"
}

_have_timeout() { command -v timeout >/dev/null 2>&1 || command -v gtimeout >/dev/null 2>&1; }

@test "cold-lib parity (success-with-marker): CONFIRMED -> exit 0, verdict WRITTEN" {
  _stub_success
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  grep -q '"disposition": "CONFIRMED"' "$VFILE"
}

@test "cold-lib parity (hang): a killed reviewer -> exit 1, NO verdict" {
  _have_timeout || skip "no timeout/gtimeout on PATH — the hang cannot be killed to test the timeout classification"
  _stub_hang
  start=$SECONDS
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  elapsed=$(( SECONDS - start ))
  [ "$status" -eq 1 ]          # a non-zero reviewer run is never trusted (fail-closed)
  [ ! -f "$VFILE" ]
  [ "$elapsed" -lt 20 ]        # killed at ~2s, not the full 30s sleep
}

@test "cold-lib parity (echo-only): prompt reflected, no verdict -> exit 1, NO verdict" {
  _stub_echo
  run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 1 ]
  [ ! -f "$VFILE" ]
}

@test "cold-lib parity (missing-codex): codex absent -> exit 2 precondition, NO verdict" {
  # A sanitized PATH with every tool pawl-review needs EXCEPT codex, so `command -v codex`
  # fails and the MISSING-codex precondition fires regardless of the host's real codex.
  toolbin="$TMP/toolbin"; mkdir -p "$toolbin"
  for t in bash sh env git jq sed grep awk cat mktemp rm printf wc tr date head tail cut sort dirname basename timeout gtimeout shasum sha256sum; do
    src="$(command -v "$t" 2>/dev/null)" && ln -sf "$src" "$toolbin/$t"
  done
  run env PATH="$toolbin" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 2 ]
  [ ! -f "$VFILE" ]
  [[ "$output" == *"MISSING DEPENDENCY"* ]]
}
