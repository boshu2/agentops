#!/usr/bin/env bats
# ag-zzpy1: the default runner must honor CORPUS_DELTA_WORKSPACE (the harness's
# isolation sandbox) instead of making its own mktemp workspace — else the agent
# runs outside the arm sandbox and the ag-5apc context-isolation proof is a stub.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  RUNNER="$REPO_ROOT/scripts/eval-agent-harness.sh"
  TMP="$(mktemp -d)"
  # Fake `codex` on PATH: record the -C workspace it was launched with, no LLM call.
  mkdir -p "$TMP/bin"
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
# args look like: exec -C <workspace> -s workspace-write <prompt>
prev=""
for a in "$@"; do
  if [ "$prev" = "-C" ]; then echo "$a" > "$CODEX_C_RECORD"; fi
  prev="$a"
done
exit 0
FAKE
  chmod +x "$TMP/bin/codex"
  export PATH="$TMP/bin:$PATH"
  export CODEX_C_RECORD="$TMP/codex-c.txt"
}

teardown() { rm -rf "$TMP"; }

@test "runner honors CORPUS_DELTA_WORKSPACE (runs agent in the caller's sandbox, not a fresh mktemp)" {
  SANDBOX="$TMP/arm-sandbox"
  run env CORPUS_DELTA_WORKSPACE="$SANDBOX" "$RUNNER" --task cd-am-1 --agent codex --runs 1
  [ "$status" -eq 0 ]
  # the fake codex recorded the -C dir it was launched in; it MUST be the sandbox
  [ -f "$CODEX_C_RECORD" ]
  [ "$(cat "$CODEX_C_RECORD")" = "$SANDBOX" ]
  # and the runner must NOT delete a caller-provided sandbox (clean only what it creates)
  [ -d "$SANDBOX" ]
}

@test "runner falls back to its own temp workspace when CORPUS_DELTA_WORKSPACE is unset" {
  run env -u CORPUS_DELTA_WORKSPACE "$RUNNER" --task cd-am-1 --agent codex --runs 1
  [ "$status" -eq 0 ]
  [ -f "$CODEX_C_RECORD" ]
  # it made its OWN workspace (not the caller's sandbox path) and cleaned it up after
  recorded="$(cat "$CODEX_C_RECORD")"
  [ "$recorded" != "$TMP/arm-sandbox" ]
  [ ! -d "$recorded" ]
}

# NOTE: the EXIT trap (eval-agent-harness.sh) cleans a runner-created workspace on
# an abort path as defense-in-depth. It is intentionally NOT unit-tested here: the
# `set -e` + `result="$(run_single)"` command-substitution interaction means a
# failing setup.sh does not reliably abort the parent, so a "leak on failure" test
# cannot be made to bite deterministically without coupling to that bash subtlety.
# A green-but-vacuous test would assert coverage it does not have, so it is omitted.
# Tests 1-2 above cover the load-bearing behavior (workspace honoring + ownership)
# with proven teeth (test 1 fails against the pre-fix bare-mktemp).

# ag-o9x: a non-zero agent exit (launch refusal, timeout kill) must surface as a
# DEGRADED run, not a silent score-0 grader-fail. This is the regression that hid
# the harness never launching codex (--skip-git-repo-check missing → exit 1 → `|| true`).
@test "non-zero agent exit marks the run degraded (not a silent grader-fail)" {
  # fake codex that REFUSES to run (mimics the trusted-dir launch failure: exit 1, no work)
  cat > "$TMP/bin/codex" <<'FAKE'
#!/usr/bin/env bash
echo "Not inside a trusted directory and --skip-git-repo-check was not specified." >&2
exit 1
FAKE
  chmod +x "$TMP/bin/codex"
  run env -u CORPUS_DELTA_WORKSPACE "$RUNNER" --task cd-am-1 --agent codex --runs 1
  [ "$status" -eq 0 ]
  # the single-run result must be flagged degraded with the agent's exit code, pass=false
  echo "$output" | grep -q '"degraded": *true'
  echo "$output" | grep -q '"agent_exit": *1'
  echo "$output" | grep -q '"pass": *false'
}

# ag-o9x: the happy path (agent exit 0) must NOT carry a degraded marker — the
# pre-fix output contract is preserved byte-for-shape for clean runs.
@test "clean agent exit (0) carries no degraded marker" {
  # default setup() fake codex exits 0
  run env -u CORPUS_DELTA_WORKSPACE "$RUNNER" --task cd-am-1 --agent codex --runs 1
  [ "$status" -eq 0 ]
  ! echo "$output" | grep -q 'degraded'
}
