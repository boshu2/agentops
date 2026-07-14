#!/usr/bin/env bats
bats_require_minimum_version 1.5.0
# pawl-review-verify-config.bats — locks for the per-repo verify-config hook
# (age-rk3r.17): scripts/lib/verify-config.sh, sourced once at pawl-review.sh's entry,
# resolves the checked-in .aoverify.yaml (at the REVIEWED repo's root) into exported
# PAWL_* via the .5 Go bridge `ao verify --export-env`.
#
# Two levels:
#   HOOK unit tests   — source verify-config.sh directly and inspect the exported vars,
#                       across the three ao-resolution contexts (AO_BIN / PATH / none)
#                       + zero-config + unknown-key + untrusted-mode + target-repo walk.
#   END-TO-END tests  — run the real scripts/pawl-review.sh with a stub codex + a stub
#                       `timeout` that RECORDS the budget, proving the config's
#                       review_timeout reaches the cold exec's `timeout <budget> codex`
#                       wrapper, that env overrides it, and that zero-config uses 300.
#
# The real codex/agy CLIs are NEVER invoked — stubs on PATH only. The real `ao` IS used
# (built once in setup_file) because the bridge is a Go layer; tests skip if go/build is
# unavailable.

setup_file() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export AO_BIN_BUILT="$BATS_FILE_TMPDIR/ao"
  # Build the real ao once (the bridge lives in Go: cli/internal/verifycfg + verify.go).
  # Fail-soft: if go is absent or the build fails, leave AO_BIN_BUILT non-executable and
  # every ao-dependent test skips (like the jq/codex skips elsewhere in this suite).
  ( cd "$REPO_ROOT/cli" && go build -o "$AO_BIN_BUILT" ./cmd/ao ) >/dev/null 2>&1 || true
}

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  HOOK="$REPO_ROOT/scripts/lib/verify-config.sh"
  AO="$AO_BIN_BUILT"
  # The e2e tests run the REAL pawl-review.sh with the real ao; its preflight
  # `ao gate check --fast` runs against the throwaway reviewed repo and fails
  # there (unrelated gates go red on a 2-commit temp repo). These tests exercise
  # the review_timeout config reaching the cold exec, not the preflight, so opt
  # out of it (the documented PAWL_NO_PREFLIGHT=1 escape). Harmless to the HOOK
  # unit tests, which never invoke pawl-review.sh.
  export PAWL_NO_PREFLIGHT=1
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"
  SEEN_BUDGET="$TMP/seen_budget"
  # A throwaway repo is the REVIEWED repo (its cwd root is what the bridge resolves the
  # config from). init + a change commit so `git show HEAD` has a reviewable diff.
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  printf 'first line\n' > note.txt
  git add note.txt; git commit --quiet -m init
  printf 'first line\nan added line under review\n' > note.txt
  git add note.txt; git commit --quiet -m "feat(x): a change (age-rev-test)"
  # A tiny driver that sources the hook (which applies config from CWD's repo) and echoes
  # the two PAWL_* keys pawl-review actually reads/exports. `set -uo pipefail` mirrors
  # pawl-review.sh's mode, so the hook is proven safe under nounset.
  DRIVER="$TMP/report.sh"
  cat > "$DRIVER" <<EOF
#!/usr/bin/env bash
set -uo pipefail
. "$HOOK"
echo "PAWL_REVIEW_TIMEOUT=\${PAWL_REVIEW_TIMEOUT:-<unset>}"
echo "PAWL_AUTOBIND=\${PAWL_AUTOBIND:-<unset>}"
EOF
  chmod +x "$DRIVER"
  # e2e seams (shared): review the throwaway repo, cold path only, never a bind commit.
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-rev-test.json"
  export PAWL_NO_SERVICE=1
  export PAWL_AUTOBIND=0
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

# Historical codex stub shape: consume stdin, emit the "tokens used" genuine-run marker
# + a clean CONFIRMED verdict.
_stub_codex_confirmed() {
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

# A stub `timeout` that RECORDS the budget (its first arg) the cold exec wrapper was built
# with, then execs the real command. Lets a test assert the effective review_timeout that
# reached codex_exec_guarded's `timeout <budget> codex ...`. The absolute $SEEN_BUDGET path
# is baked in at write time (unquoted heredoc); $1/$@ stay literal.
_stub_timeout_record() {
  cat > "$BIN/timeout" <<FAKE
#!/usr/bin/env bash
printf '%s' "\$1" > "$SEEN_BUDGET"
shift
exec "\$@"
FAKE
  chmod +x "$BIN/timeout"
}

# ---------------------------------------------------------------------------
# HOOK unit tests — the three ao-resolution contexts + policy behavior
# ---------------------------------------------------------------------------

@test "context (c): NO ao available -> silent no-op (no exports, NO stderr) — zero-config byte-identical" {
  # AO_BIN unset + a PATH with no `ao`: the hook must resolve nothing and touch nothing.
  printf 'review_timeout: 123\n' > "$REPO/.aoverify.yaml"   # present, but unreadable w/o ao
  run --separate-stderr env -u AO_BIN PATH="/usr/bin:/bin" "$DRIVER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL_REVIEW_TIMEOUT=<unset>"* ]]
  # Never an error and never a warning that changes output (the task's context-c contract).
  [ -z "$stderr" ]
}

@test "context (b): AO_BIN + a checked-in .aoverify.yaml -> review_timeout reaches the shell" {
  [ -x "$AO" ] || skip "ao build unavailable"
  printf 'review_timeout: 123\nautobind: false\n' > "$REPO/.aoverify.yaml"
  run env -u PAWL_REVIEW_TIMEOUT AO_BIN="$AO" "$DRIVER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL_REVIEW_TIMEOUT=123"* ]]
  [[ "$output" == *"PAWL_AUTOBIND=0"* ]]
}

@test "env OVERRIDES file: PAWL_REVIEW_TIMEOUT env wins over the config value" {
  [ -x "$AO" ] || skip "ao build unavailable"
  printf 'review_timeout: 123\n' > "$REPO/.aoverify.yaml"
  run env AO_BIN="$AO" PAWL_REVIEW_TIMEOUT=456 "$DRIVER"
  [ "$status" -eq 0 ]
  # env > file: the shell sees 456, not the file's 123.
  [[ "$output" == *"PAWL_REVIEW_TIMEOUT=456"* ]]
}

@test "zero-config: no .aoverify.yaml -> hook emits nothing (PAWL_* stay unset)" {
  [ -x "$AO" ] || skip "ao build unavailable"
  [ ! -f "$REPO/.aoverify.yaml" ]
  run env -u PAWL_REVIEW_TIMEOUT AO_BIN="$AO" "$DRIVER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL_REVIEW_TIMEOUT=<unset>"* ]]
}

@test "unknown config key -> ONE warning line on stderr, exit unchanged, known keys still applied" {
  [ -x "$AO" ] || skip "ao build unavailable"
  printf 'review_timeout: 77\nbogus_key: 9\n' > "$REPO/.aoverify.yaml"
  run --separate-stderr env -u PAWL_REVIEW_TIMEOUT AO_BIN="$AO" "$DRIVER"
  [ "$status" -eq 0 ]
  # The warning is the Go parser's, surfaced via stderr (not stdout).
  [[ "$stderr" == *"unknown key"* ]]
  [[ "$stderr" == *"bogus_key"* ]]
  # Exit unchanged and the good key still reaches the shell.
  [[ "$output" == *"PAWL_REVIEW_TIMEOUT=77"* ]]
}

@test "context (a): AO_BIN unset -> ao resolved from PATH, config applied" {
  [ -x "$AO" ] || skip "ao build unavailable"
  cp "$AO" "$BIN/ao"
  printf 'review_timeout: 210\n' > "$REPO/.aoverify.yaml"
  run env -u AO_BIN -u PAWL_REVIEW_TIMEOUT PATH="$BIN:/usr/bin:/bin" "$DRIVER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL_REVIEW_TIMEOUT=210"* ]]
}

@test "untrusted mode: PAWL_UNTRUSTED_REPO=1 with no AO_BIN REFUSES a PATH ao (no-op)" {
  [ -x "$AO" ] || skip "ao build unavailable"
  cp "$AO" "$BIN/ao"
  printf 'review_timeout: 999\n' > "$REPO/.aoverify.yaml"
  # A repo under review is cwd; the hook must not resolve `ao` from PATH in untrusted mode
  # (mirrors resolve_ao's RCE stance). AO_BIN is normally pinned here, so this is the belt.
  run env -u AO_BIN -u PAWL_REVIEW_TIMEOUT PAWL_UNTRUSTED_REPO=1 PATH="$BIN:/usr/bin:/bin" "$DRIVER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL_REVIEW_TIMEOUT=<unset>"* ]]
}

@test "target-repo resolution: config is read from the REVIEWED repo's root even from a nested cwd" {
  [ -x "$AO" ] || skip "ao build unavailable"
  printf 'review_timeout: 222\n' > "$REPO/.aoverify.yaml"
  mkdir -p "$REPO/deep/nested"
  cd "$REPO/deep/nested"
  # The .5 root-walk (findRoot) climbs from cwd to the repo's .git root and reads the
  # config THERE — so a nested review still gets the reviewed repo's root policy, never
  # the AgentOps checkout the bundle came from.
  run env -u PAWL_REVIEW_TIMEOUT AO_BIN="$AO" "$DRIVER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PAWL_REVIEW_TIMEOUT=222"* ]]
}

# ---------------------------------------------------------------------------
# END-TO-END through pawl-review.sh — the config reaches the timeout plumbing
# ---------------------------------------------------------------------------

@test "e2e: config review_timeout reaches pawl-review's cold exec timeout budget" {
  [ -x "$AO" ] || skip "ao build unavailable"
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirmed
  _stub_timeout_record
  printf 'review_timeout: 137\n' > "$REPO/.aoverify.yaml"
  run env -u PAWL_REVIEW_TIMEOUT PATH="$BIN:$PATH" AO_BIN="$AO" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  # The config value (137), not the built-in default (300), is the budget the cold exec
  # wrapper was built with — proving config -> hook -> export -> pawl-review timeout plumbing.
  [ "$(cat "$SEEN_BUDGET")" = "137" ]
}

@test "e2e: env overrides the config value through the full pawl-review run (456 beats 137)" {
  [ -x "$AO" ] || skip "ao build unavailable"
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirmed
  _stub_timeout_record
  printf 'review_timeout: 137\n' > "$REPO/.aoverify.yaml"
  run env PATH="$BIN:$PATH" AO_BIN="$AO" PAWL_REVIEW_TIMEOUT=456 bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [ "$(cat "$SEEN_BUDGET")" = "456" ]
}

@test "e2e: NO config -> pawl-review uses the 300 default + still writes a CONFIRMED verdict (byte-identical)" {
  [ -x "$AO" ] || skip "ao build unavailable"
  command -v jq >/dev/null 2>&1 || skip "jq required"
  _stub_codex_confirmed
  _stub_timeout_record
  [ ! -f "$REPO/.aoverify.yaml" ]
  run env -u PAWL_REVIEW_TIMEOUT PATH="$BIN:$PATH" AO_BIN="$AO" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]
  [ -f "$VFILE" ]
  # Zero-config emits nothing, so the built-in default (300) is used — unchanged from
  # pre-hook behavior.
  [ "$(cat "$SEEN_BUDGET")" = "300" ]
}
