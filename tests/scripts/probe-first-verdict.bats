#!/usr/bin/env bats
# probe-first-verdict.bats — mock-mode e2e for scripts/probe-first-verdict.sh
# (age-wedge-all-in-dyr0.5). The probe scripts the README quickstart golden
# path in a clean temp git repo (quick-start → small change → ao verify) with a
# MOCK reviewer on PATH and asserts a verdict landed in the temp repo's ledger
# under the wall-clock budget. Mock mode proves PATH mechanics + the timing
# floor only, and the probe's own output must say so — asserted here.
#
# The full ao binary is built once per file (the same pattern as
# check-docs-cli-snippets.bats) and injected via PROBE_AO_BIN so the probe
# never rebuilds per test.

setup_file() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export REPO_ROOT
  if ! command -v go >/dev/null 2>&1; then
    export PROBE_BUILD_SKIP="go toolchain not available"
    return 0
  fi
  if ! command -v jq >/dev/null 2>&1; then
    export PROBE_BUILD_SKIP="jq not available"
    return 0
  fi
  export PROBE_AO_BIN="$BATS_FILE_TMPDIR/ao"
  ( cd "$REPO_ROOT/cli" && go build -o "$PROBE_AO_BIN" ./cmd/ao )
}

setup() {
  SCRIPT="$REPO_ROOT/scripts/probe-first-verdict.sh"
}

@test "mock mode: golden path lands a verdict in the temp repo ledger under budget" {
  [ -z "${PROBE_BUILD_SKIP:-}" ] || skip "$PROBE_BUILD_SKIP"
  run bash "$SCRIPT"
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PROBE PASS (mode=mock)"* ]]
  [[ "$output" == *"landed a verdict in the temp repo's ledger"* ]]
}

@test "mock mode output states it measured the mock path, never the honest number" {
  [ -z "${PROBE_BUILD_SKIP:-}" ] || skip "$PROBE_BUILD_SKIP"
  run bash "$SCRIPT"
  echo "$output"
  [ "$status" -eq 0 ]
  [[ "$output" == *"MOCK reviewer"* ]]
  [[ "$output" == *"TIMING FLOOR"* ]]
  [[ "$output" == *"NOT the honest first-verdict number"* ]]
  [[ "$output" == *"--live"* ]]
}

@test "unknown flag is a usage error (exit 2)" {
  run bash "$SCRIPT" --bogus
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown arg"* ]]
}

@test "README drift gate: probe fails when README loses the golden-path command" {
  [ -z "${PROBE_BUILD_SKIP:-}" ] || skip "$PROBE_BUILD_SKIP"
  # Run against a copied repo root whose README dropped `ao verify my-first-change`.
  FAKE_ROOT="$BATS_TEST_TMPDIR/fake-root"
  mkdir -p "$FAKE_ROOT/scripts/lib"
  cp "$REPO_ROOT/scripts/probe-first-verdict.sh" "$FAKE_ROOT/scripts/"
  cp "$REPO_ROOT/scripts/lib/preamble.sh" "$FAKE_ROOT/scripts/lib/"
  grep -vF 'ao verify my-first-change' "$REPO_ROOT/README.md" > "$FAKE_ROOT/README.md"
  run bash "$FAKE_ROOT/scripts/probe-first-verdict.sh"
  echo "$output"
  [ "$status" -eq 1 ]
  [[ "$output" == *"README drift"* ]]
}
