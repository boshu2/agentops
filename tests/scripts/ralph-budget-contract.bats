#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  RALPH="$REPO_ROOT/bin/ralph"
  TMP="$(mktemp -d)"

  git -C "$TMP" init -q
  git -C "$TMP" config user.email bats@test.local
  git -C "$TMP" config user.name "Bats Test"
  git -C "$TMP" commit --allow-empty -qm init
}

teardown() {
  rm -rf "$TMP"
}

@test "help advertises the enforceable phase timeout, not a dollar budget" {
  run "$RALPH" --help

  [ "$status" -eq 0 ]
  [[ "$output" == *"--phase-timeout"* ]]
  [[ "$output" != *"max-budget"* ]]
  [[ "$output" != *"Budget:"* ]]
}

@test "legacy max-budget fails closed before Codex or filesystem execution" {
  marker="$TMP/codex-ran"
  codex_stub="$TMP/codex"
  cat > "$codex_stub" <<STUB
#!/usr/bin/env bash
touch "$marker"
exit 99
STUB
  chmod +x "$codex_stub"

  cd "$TMP"
  run env CODEX_BIN="$codex_stub" "$RALPH" --max-budget 20 "legacy caller"

  [ "$status" -eq 2 ]
  [[ "$output" == *"cannot enforce dollar budgets"* ]]
  [[ "$output" == *"--phase-timeout"* ]]
  [ ! -e "$marker" ]
  [ ! -e "$TMP/.agents/ralph" ]
}

@test "checkpoints written before budget removal still resume" {
  mkdir -p "$TMP/.agents/ralph"
  cat > "$TMP/.agents/ralph/resume.checkpoint" <<CKPT
# Ralph checkpoint — source this to resume
GOAL=Resume\ compatibility
BRANCH=feat/resume-compatibility
BASE_BRANCH=main
WORKDIR=$TMP
SLUG=resume
SKIP_PRE_MORTEM=false
SPEC_FILE=''
RALPH_LOG=.agents/ralph/resume.log
LAST_PHASE=pr
CKPT

  cd "$TMP"
  run env CODEX_BIN="$TMP/does-not-exist" "$RALPH" \
    --resume .agents/ralph/resume.checkpoint --dry-run

  [ "$status" -eq 0 ]
  [[ "$output" == *"Resuming after phase: pr"* ]]
  [[ "$output" != *"Budget:"* ]]
  [ ! -e "$TMP/does-not-exist" ]
}
