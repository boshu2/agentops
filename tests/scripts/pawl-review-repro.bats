#!/usr/bin/env bats
# age-6idm: AUTO-RUN-THE-REPRO. A REVIEWER REFUTED that NAMES an executable repro is a
# candidate FALSE refute — the 2026-07-02 build-tag class had 3/3 REFUTEDs each naming a
# repro that actually PASSED (hallucinated cobra legacy-tag expectations + an install.sh
# --tier claim). pawl-review extracts the named repro, gates it at the ARGV level (never
# sh -c/eval), runs it ONCE timeboxed, and: repro PASSES -> re-roll the review ONCE (bounded);
# repro FAILS -> REFUTED stands + evidence attached; disallowed/none -> no execution.
#
# The reviewer (codex) + the repro (`go`) are STUBBED on PATH; everything runs in a temp repo
# (AGENTOPS_REPO_ROOT) so the real repo + ledger are never touched. PAWL_NO_SERVICE=1 forces
# the cold path (where the reviewer REFUTED is first materialized).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl-review.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
  BIN="$TMP/bin"; mkdir -p "$BIN"

  # Reviewer stub: drains the prompt on stdin, counts invocations (survives the re-exec via a
  # file), logs the reroll-marker env value per call, and prints the canned output for call N
  # (CODEX_CALL<N>_FILE) or a default CONFIRMED.
  cat > "$BIN/codex" <<'STUB'
#!/usr/bin/env bash
cat >/dev/null
n=1
if [[ -n "${CODEX_COUNT:-}" ]]; then
  n=$(( $(cat "$CODEX_COUNT" 2>/dev/null || echo 0) + 1 ))
  echo "$n" > "$CODEX_COUNT"
fi
[[ -n "${CODEX_ENVLOG:-}" ]] && echo "call=$n reroll=${PAWL_REROLLED_AFTER_FALSE_REPRO:-unset}" >> "$CODEX_ENVLOG"
printf 'codex\n'
fvar="CODEX_CALL${n}_FILE"
if [[ -n "${!fvar:-}" && -f "${!fvar}" ]]; then cat "${!fvar}"
else printf 'VERDICT: CONFIRMED\n'; fi
exit "${CODEX_EXIT:-0}"
STUB
  chmod +x "$BIN/codex"

  # Repro stub: `go` logs its argv + exits GO_EXIT (0 = repro passes, non-zero = repro fails).
  cat > "$BIN/go" <<'STUB'
#!/usr/bin/env bash
echo "go $*" >> "${GO_LOG:-/dev/null}"
exit "${GO_EXIT:-0}"
STUB
  chmod +x "$BIN/go"

  # ao stub: keep the best-effort provenance emit hermetic (appends nothing => no auto-bind).
  printf '#!/usr/bin/env bash\nexit 0\n' > "$BIN/ao"; chmod +x "$BIN/ao"

  PATH="$BIN:$PATH"
  REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"
  git init --quiet; git config user.email t@e.com; git config user.name T
  echo init > README.md; git add README.md; git commit --quiet -m init
  echo change >> README.md; git add README.md
  git commit --quiet -m "feat(x): a change (age-rev-test)"
  HEAD_SHA="$(git rev-parse HEAD)"
  export AGENTOPS_REPO_ROOT="$REPO"
  export AGENTOPS_PAWL_VERDICT_DIR="$TMP/verdicts"; mkdir -p "$AGENTOPS_PAWL_VERDICT_DIR"
  VFILE="$AGENTOPS_PAWL_VERDICT_DIR/age-rev-test.json"
  EVID="$REPO/.agents/pawl-evidence/age-rev-test-pawl-review.txt"
  export PAWL_NO_SERVICE=1
  # cross-run state (files persist across the auto-reroll's exec)
  export CODEX_COUNT="$TMP/codex.count"; printf '0' > "$CODEX_COUNT"
  export CODEX_ENVLOG="$TMP/codex.envlog"; : > "$CODEX_ENVLOG"
  export GO_LOG="$TMP/go.log"; : > "$GO_LOG"
}

teardown() { cd "$ORIG_DIR" 2>/dev/null || true; rm -rf "$TMP"; }

# _refuted <file> <repro>: write a REFUTED reviewer output that names <repro> in backticks.
_refuted() {
  local f="$1" repro="$2"
  { printf 'DEFECTS:\n'
    printf -- '- expectation looks wrong on this build-tag diff. Repro: `%s`\n' "$repro"
    printf 'VERDICT: REFUTED\n'
  } > "$f"
}
_confirmed() { printf 'VERDICT: CONFIRMED\n' > "$1"; }

# ---------------------------------------------------------------------------
# unit: extract_repro_command + repro_argv_allowed (sourced in a subshell so
# the script's `set -u` never leaks into the bats test shell)
# ---------------------------------------------------------------------------
@test "extract_repro_command: a backtick-quoted go command is found" {
  run bash -c 'source "'"$SCRIPT"'"; printf "DEFECTS:\n- x. Repro: \`go test ./cli/cmd/ao -run TestFoo\`\nVERDICT: REFUTED\n" | extract_repro_command'
  [ "$output" = "go test ./cli/cmd/ao -run TestFoo" ]
}

@test "extract_repro_command: a fenced code block go command is found" {
  run bash -c 'source "'"$SCRIPT"'"; printf "DEFECTS:\n- y\n\`\`\`\ngo build ./cli/...\n\`\`\`\nVERDICT: REFUTED\n" | extract_repro_command'
  [ "$output" = "go build ./cli/..." ]
}

@test "extract_repro_command: no repro named => empty (no execution)" {
  run bash -c 'source "'"$SCRIPT"'"; printf "DEFECTS:\n- no repro here\nVERDICT: REFUTED\n" | extract_repro_command'
  [ -z "$output" ]
}

@test "repro_argv_allowed: allows go test/build/vet + bats; rejects other binaries, dangerous flags, absolute + traversal paths" {
  run bash -c 'source "'"$SCRIPT"'"
    chk() { local a; read -r -a a <<<"$1"; if repro_argv_allowed "${a[@]}"; then echo ALLOW; else echo REJECT; fi; }
    chk "go test ./cli/cmd/ao -run TestFoo"
    chk "go build ./cli/..."
    chk "go vet ./..."
    chk "bats tests/scripts/x.bats"
    chk "go generate ./..."
    chk "python -c pass"
    chk "go test -exec /bin/echo ./..."
    chk "go test -ldflags=-X ./..."
    chk "go test /etc/passwd"
    chk "go test ../../x"'
  [ "$status" -eq 0 ]
  expected="ALLOW
ALLOW
ALLOW
ALLOW
REJECT
REJECT
REJECT
REJECT
REJECT
REJECT"
  [ "$output" = "$expected" ]
}

# ---------------------------------------------------------------------------
# orchestration
# ---------------------------------------------------------------------------
@test "repro-passes: re-rolls the review EXACTLY once (marker set on the re-roll), then CONFIRMS" {
  _refuted "$TMP/call1.txt" "go test ./cli/cmd/ao -run TestLegacy"   # call 1: REFUTED naming a repro
  _confirmed "$TMP/call2.txt"                                        # call 2 (the re-roll): CONFIRMED
  CODEX_CALL1_FILE="$TMP/call1.txt" CODEX_CALL2_FILE="$TMP/call2.txt" GO_EXIT=0 \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 0 ]                    # the re-rolled review CONFIRMED
  [ -f "$VFILE" ]
  [ "$(cat "$CODEX_COUNT")" -eq 2 ]      # the review was re-invoked exactly once
  # exactly-one-reroll marker: call 1 had no marker, call 2 (the re-roll) had it set
  grep -q "call=1 reroll=unset" "$CODEX_ENVLOG"
  grep -q "call=2 reroll=1" "$CODEX_ENVLOG"
  ! grep -q "call=3" "$CODEX_ENVLOG"     # never a second auto-reroll
  grep -q "go test ./cli/cmd/ao -run TestLegacy" "$GO_LOG"   # the repro actually ran
}

@test "repro-fails: the REFUTED STANDS and the run's exit code + output are attached as evidence" {
  _refuted "$TMP/call1.txt" "go test ./cli/cmd/ao -run TestLegacy"
  CODEX_CALL1_FILE="$TMP/call1.txt" GO_EXIT=7 \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]                    # REFUTED stands (repro failed)
  [ ! -f "$VFILE" ]                      # no verdict written
  [ "$(cat "$CODEX_COUNT")" -eq 1 ]      # NOT re-rolled (repro did not pass)
  [ -f "$EVID" ]
  grep -q "AUTO-REPRO" "$EVID"
  grep -q "exit code: 7" "$EVID"
  grep -q "go test ./cli/cmd/ao -run TestLegacy" "$GO_LOG"   # the repro ran once
}

@test "disallowed-argv: a repro that fails the allowlist is NOT executed; the REFUTED stands as today" {
  _refuted "$TMP/call1.txt" "go generate ./..."   # argv[1]=generate is disallowed
  CODEX_CALL1_FILE="$TMP/call1.txt" GO_EXIT=0 \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]                    # REFUTED stands
  [ ! -f "$VFILE" ]
  [ "$(cat "$CODEX_COUNT")" -eq 1 ]      # no re-roll
  [ ! -s "$GO_LOG" ]                     # the repro was NEVER executed (allowlist rejected it)
  [[ "$output" == *"NOT in the allowed argv set"* ]]
}

@test "reroll-bounded: on an already-rerolled run, a second REFUTED with a PASSING repro does NOT reroll" {
  _refuted "$TMP/call1.txt" "go test ./cli/cmd/ao -run TestLegacy"
  # Simulate the re-rolled run: the marker is already set, and the repro passes (GO_EXIT=0).
  CODEX_CALL1_FILE="$TMP/call1.txt" GO_EXIT=0 PAWL_REROLLED_AFTER_FALSE_REPRO=1 \
    run env PATH="$BIN:$PATH" bash "$SCRIPT" age-rev-test --scope head
  [ "$status" -eq 3 ]                    # the REFUTED STANDS even though its repro passed
  [ ! -f "$VFILE" ]
  [ "$(cat "$CODEX_COUNT")" -eq 1 ]      # NO second auto-reroll (bounded)
  grep -q "already RE-ROLLED once" "$EVID"
  grep -q "go test ./cli/cmd/ao -run TestLegacy" "$GO_LOG"   # the repro DID run (just did not trigger a reroll)
}
