#!/usr/bin/env bats
# pawl-review.sh — EDGE-UNBOUND exit 6 (F2, age-pawl-intent-zhndq.2), end-to-end.
#
# A cold CONFIRMED review whose `ao provenance emit-verdict` FAILS must exit 6 (EDGE-UNBOUND),
# keep the verdict file (the recovery input), print the exact re-emit command, and NOT print
# "ready to push". PAWL_EDGE_FAIL_OPEN=1 restores exit 0 (warn-and-continue). This proves the
# do_write(7) -> pawl-review exit-6 propagation through the real cold path.
#
# Harness mirrors pawl-review.bats: codex is a PATH stub; AO_BIN is a
# shim that FAILS on `provenance emit-verdict`; everything runs in a temp repo.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export PAWL_NO_PREFLIGHT=1
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"; ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
cat >/dev/null
printf 'codex\n%s\n' "${CODEX_STUB:-VERDICT: CONFIRMED}"
exit 0
STUB
  chmod +x "$BIN/codex"
  # AO_BIN shim: FAIL on `provenance emit-verdict` (the emit F2 fail-closes on); succeed otherwise.
  cat > "$BIN/ao-fail-emit" <<'STUB'
#!/usr/bin/env bash
if [[ "${1:-}" == "provenance" && "${2:-}" == "emit-verdict" ]]; then
  echo "ao: simulated emit-verdict failure" >&2; exit 1
fi
exit 0
STUB
  chmod +x "$BIN/ao-fail-emit"
  PATH="$BIN:$PATH"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > README.md; git add README.md; git commit --quiet -m init
  echo change >> README.md; git add README.md
  git commit --quiet -m "feat(x): a change (age-eu-test)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-eu-test.json"
  export PAWL_NO_SERVICE=1
  # This file asserts the STRICT fail-closed edge-unbound exit. Strip any
  # ambient PAWL_EDGE_FAIL_OPEN the CI harness sets suite-wide; the one test
  # that wants warn-and-continue sets it inline on its own `env` invocation.
  unset PAWL_EDGE_FAIL_OPEN
}
teardown() { cd "$ORIG_DIR"; rm -rf "$TMP"; }

@test "edge-unbound: CONFIRMED review + failing emit -> exit 6, verdict kept, recovery printed" {
  run env PATH="$BIN:$PATH" AO_BIN="$BIN/ao-fail-emit" CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-eu-test --scope head --author-family claude
  [ "$status" -eq 6 ]
  [[ "$output" == *"EDGE-UNBOUND"* ]]
  [[ "$output" == *"ao provenance emit-verdict --file"* ]]     # the recovery command
  [[ "$output" != *"ready to push"* ]]                          # never falsely authorized
  [ -f "$VFILE" ]                                               # verdict survives (recovery input)
  grep -q CONFIRMED "$VFILE"
}

@test "edge-unbound: PAWL_EDGE_FAIL_OPEN=1 restores warn-and-continue (exit 0)" {
  run env PATH="$BIN:$PATH" AO_BIN="$BIN/ao-fail-emit" PAWL_EDGE_FAIL_OPEN=1 \
    CODEX_STUB="VERDICT: CONFIRMED" \
    bash "$SCRIPT" age-eu-test --scope head --author-family claude
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
}
