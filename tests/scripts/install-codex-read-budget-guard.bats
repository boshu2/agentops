#!/usr/bin/env bats
INSTALLER="${INSTALLER:-$BATS_TEST_DIRNAME/../../scripts/install-codex-read-budget-guard.sh}"
REPO="$BATS_TEST_DIRNAME/../.."
bats_require_minimum_version 1.5.0

setup() {
  export TMPDIR="$(mktemp -d)"
  export HOME="$TMPDIR/home"
  export CODEX_HOME="$TMPDIR/codex"
  unset CODEX_HOOKS_FILE AOP_WAIVE AGENTOPS_HOOKS_DISABLED
  mkdir -p "$HOME" "$CODEX_HOME" "$TMPDIR/project"
  HOOKS="$CODEX_HOME/hooks.json"
}
teardown() { rm -rf "$TMPDIR"; }

@test "codex installer: adds synchronous ^Bash$ hook and copies both sibling scripts" {
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  run jq -e '.hooks.PreToolUse | length == 1 and (.[0] |
    .matcher == "^Bash$" and (.hooks | length == 1) and
    .hooks[0].type == "command" and .hooks[0].timeout == 10 and
    (.hooks[0].async // false) == false)' "$HOOKS"
  [ "$status" -eq 0 ]
  for name in read-budget-guard.sh codex-read-budget-guard.sh; do
    dst="$CODEX_HOME/hooks/agentops-read-budget/$name"
    [ -x "$dst" ]
    cmp -s "$REPO/skills/cc-hooks/hooks/$name" "$dst"
  done
}

@test "codex installer: preserves unrelated configuration, backs it up, and reruns unchanged" {
  printf '{"description":"Keep me","hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo keep"}]}]}}\n' > "$HOOKS"
  cp "$HOOKS" "$TMPDIR/original.json"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  cp "$HOOKS" "$TMPDIR/installed.json"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [[ "$output" != *"Backed up hooks:"* ]]
  cmp -s "$HOOKS" "$TMPDIR/installed.json"
  backups=("$HOOKS".bak.*)
  [ "${#backups[@]}" -eq 1 ]
  cmp -s "${backups[0]}" "$TMPDIR/original.json"
  run jq -e '.description == "Keep me" and .hooks.Stop[0].hooks[0].command == "echo keep"' "$HOOKS"
  [ "$status" -eq 0 ]
}

@test "codex installer: same-second backups retain each original" {
  mkdir "$TMPDIR/bin"
  printf '#!/bin/sh\nprintf "%%s\\n" 20260912120000\n' > "$TMPDIR/bin/date"
  chmod +x "$TMPDIR/bin/date"
  export PATH="$TMPDIR/bin:$PATH"
  for value in first second; do
    printf '{"description":"%s"}\n' "$value" > "$HOOKS"
    cp "$HOOKS" "$TMPDIR/$value.json"
    run bash "$INSTALLER"
    [ "$status" -eq 0 ]
  done
  backups=("$HOOKS".bak.*)
  [ "${#backups[@]}" -eq 2 ]
  first_found=0; second_found=0
  for backup in "${backups[@]}"; do
    if cmp -s "$backup" "$TMPDIR/first.json"; then first_found=1; fi
    if cmp -s "$backup" "$TMPDIR/second.json"; then second_found=1; fi
  done
  [ "$first_found" -eq 1 ]; [ "$second_found" -eq 1 ]
}

@test "codex installer: a wrong matcher or async existing command cannot hide enforcement" {
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  command="$(jq -r '.hooks.PreToolUse[0].hooks[0].command' "$HOOKS")"
  jq -nc --arg cmd "$command" '{hooks:{PreToolUse:[
    {matcher:"^Edit$",hooks:[{type:"command",command:$cmd,timeout:10}]},
    {matcher:"^Bash$",hooks:[{type:"command",command:$cmd,timeout:10,async:true}]}
  ]}}' > "$HOOKS"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  run jq -e '.hooks.PreToolUse | length == 3 and (.[2] |
    .matcher == "^Bash$" and (.hooks[0].async // false) == false)' "$HOOKS"
  [ "$status" -eq 0 ]
}

@test "codex installer: --project keeps hooks and assets project-local" {
  cd "$TMPDIR/project"
  git init -q .
  run bash "$INSTALLER" --project
  [ "$status" -eq 0 ]
  [ -f .codex/hooks.json ]
  [ -x .codex/hooks/agentops-read-budget/codex-read-budget-guard.sh ]
  [ ! -e "$HOOKS" ]
  [ ! -e "$CODEX_HOME/hooks" ]
}

@test "codex installer: explicit hooks file supports spaces and apostrophes in its path" {
  export CODEX_HOOKS_FILE="$TMPDIR/Bo's test/hooks.json"
  run bash "$INSTALLER" --project
  [ "$status" -eq 0 ]
  [ ! -e "$HOOKS" ]
  [ -e "$CODEX_HOOKS_FILE" ]
  seq 1 400 > "$TMPDIR/project/big.txt"
  jq -nc --arg cwd "$TMPDIR/project" '{hook_event_name:"PreToolUse",tool_name:"Bash",
    tool_input:{command:"cat big.txt"},session_id:"installed",cwd:$cwd}' > "$TMPDIR/event.json"
  command="$(jq -r '.hooks.PreToolUse[0].hooks[0].command' "$CODEX_HOOKS_FILE")"
  cd "$TMPDIR"
  run --separate-stderr sh -c "$command" < "$TMPDIR/event.json"
  [ "$status" -eq 2 ]; [ -z "$output" ]
  [[ "$stderr" == *"core.context:unbounded-read"* ]]
}

@test "codex installer: invalid JSON is preserved without installing assets" {
  printf '{broken' > "$HOOKS"
  run bash "$INSTALLER"
  [ "$status" -ne 0 ]
  [ "$(cat "$HOOKS")" = '{broken' ]
  [ ! -e "$CODEX_HOME/hooks" ]
  run find "$CODEX_HOME" -name '*.tmp.*'
  [ -z "$output" ]
}

@test "codex installer: no trust is granted and default repository hook declarations stay unchanged" {
  printf 'operator-owned config\n' > "$CODEX_HOME/config.toml"
  cp "$REPO/hooks/hooks.json" "$TMPDIR/plugin-hooks.json"
  cp "$REPO/.codex-plugin/plugin.json" "$TMPDIR/plugin.json"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"review"* || "$output" == *"Review"* ]]
  [[ "$output" == *"/hooks"* ]]
  [[ "$output" == *"does not grant trust"* ]]
  [ "$(cat "$CODEX_HOME/config.toml")" = 'operator-owned config' ]
  cmp -s "$REPO/hooks/hooks.json" "$TMPDIR/plugin-hooks.json"
  cmp -s "$REPO/.codex-plugin/plugin.json" "$TMPDIR/plugin.json"
  run find "$CODEX_HOME" -maxdepth 1 -type f ! -name hooks.json ! -name config.toml
  [ -z "$output" ]
}

@test "codex installer: unknown arguments fail before writing configuration" {
  run bash "$INSTALLER" --typo
  [ "$status" -eq 2 ]
  [ ! -e "$HOOKS" ]
  [ ! -e "$CODEX_HOME/hooks" ]
}

make_linked_worktree() {
  PRIMARY="$TMPDIR/primary"
  LINKED="$TMPDIR/linked"
  git init -q "$PRIMARY"
  git -C "$PRIMARY" -c user.name=Fixture -c user.email=fixture@example.invalid \
    commit --allow-empty -qm fixture
  git -C "$PRIMARY" worktree add --detach -q "$LINKED"
}

@test "codex installer: linked --project refuses before writing either checkout" {
  make_linked_worktree
  cd "$LINKED"
  run bash "$INSTALLER" --project
  [ "$status" -eq 2 ]
  [[ "$output" == *"Codex 0.154"*"primary checkout"* ]]
  [[ "$output" == *"without --project"* ]]
  [ ! -e "$LINKED/.codex" ]
  [ ! -e "$PRIMARY/.codex" ]
  [ ! -e "$HOOKS" ]
  [ ! -e "$CODEX_HOME/hooks" ]
}

@test "codex installer: linked worktree permits an explicit hooks file destination" {
  make_linked_worktree
  export CODEX_HOOKS_FILE="$TMPDIR/selected/hooks.json"
  cd "$LINKED"
  run bash "$INSTALLER" --project
  [ "$status" -eq 0 ]
  [ -f "$CODEX_HOOKS_FILE" ]
  [ -x "$TMPDIR/selected/hooks/agentops-read-budget/codex-read-budget-guard.sh" ]
  [ ! -e "$LINKED/.codex" ]
  [ ! -e "$PRIMARY/.codex" ]
  [ ! -e "$HOOKS" ]
  [ ! -e "$CODEX_HOME/hooks" ]
}
