#!/usr/bin/env bats
# Repeated installs must refresh owned assets without overwriting any settings
# backup. Freeze date so this regression cannot pass by crossing a second.

REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"

setup() {
  TEST_HOME="$BATS_TEST_TMPDIR/home"
  TEST_SETTINGS="$TEST_HOME/.claude/settings.json"
  mkdir -p "$TEST_HOME/.claude" "$BATS_TEST_TMPDIR/bin"
  printf '#!/bin/sh\nprintf "%%s\\n" 20260922120000\n' > "$BATS_TEST_TMPDIR/bin/date"
  chmod +x "$BATS_TEST_TMPDIR/bin/date"
  backup_paths=()
  snapshots=()
}

run_installer() {
  run env -u AGENTOPS_REPO_ROOT HOME="$TEST_HOME" SETTINGS="$TEST_SETTINGS" \
    PATH="$BATS_TEST_TMPDIR/bin:$PATH" bash "$installer"
  [ "$status" -eq 0 ]
}

assert_new_backup() {
  local expected="$1" backup previous found=0 i
  local backups=("$TEST_SETTINGS".bak.20260922120000*)
  [ "${#backups[@]}" -eq "$((${#backup_paths[@]} + 1))" ]
  for ((i = 0; i < ${#backup_paths[@]}; i++)); do
    cmp -s "${backup_paths[i]}" "${snapshots[i]}"
  done
  for backup in "${backups[@]}"; do
    previous=0
    for ((i = 0; i < ${#backup_paths[@]}; i++)); do
      if [ "$backup" = "${backup_paths[i]}" ]; then previous=1; fi
    done
    if [ "$previous" -eq 0 ]; then
      cmp -s "$backup" "$expected"
      backup_paths+=("$backup")
      snapshots+=("$expected")
      found=$((found + 1))
    fi
  done
  [ "$found" -eq 1 ]
}

assert_installed() {
  local i
  for ((i = 0; i < ${#sources[@]}; i++)); do
    cmp -s "${sources[i]}" "${destinations[i]}"
  done
  [ -x "${destinations[0]}" ]
  run jq -e --arg cmd "${destinations[0]}" --argjson matchers "$matchers" '
    .model == "keep me" and .permissions.allow == ["Read"] and
    .hooks.Stop == [{hooks:[{type:"command",command:"echo keep stop"}]}] and
    ([.hooks.PreToolUse[] | select(.hooks[0].command == "echo keep pre")] ==
      [{matcher:"Read",hooks:[{type:"command",command:"echo keep pre"}]}]) and
    ([.hooks.PreToolUse[] | select(any(.hooks[]; .command == $cmd)) | .matcher] | sort) ==
      ($matchers | sort) and
    ([.hooks.PreToolUse[].hooks[] | select(.command == $cmd and .type == "command")] |
      length) == ($matchers | length)
  ' "$TEST_SETTINGS"
  [ "$status" -eq 0 ]
}

exercise_repeated_install() {
  printf '%s\n' '{"model":"keep me","permissions":{"allow":["Read"]},"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo keep stop"}]}],"PreToolUse":[{"matcher":"Read","hooks":[{"type":"command","command":"echo keep pre"}]}]}}' > "$TEST_SETTINGS"
  cp "$TEST_SETTINGS" "$BATS_TEST_TMPDIR/original.json"
  run_installer
  assert_installed
  assert_new_backup "$BATS_TEST_TMPDIR/original.json"

  cp "$TEST_SETTINGS" "$BATS_TEST_TMPDIR/installed.json"
  local destination
  for destination in "${destinations[@]}"; do
    printf 'stale owned asset\n' > "$destination"
  done
  run_installer
  assert_installed
  cmp -s "$TEST_SETTINGS" "$BATS_TEST_TMPDIR/installed.json"
  assert_new_backup "$BATS_TEST_TMPDIR/installed.json"

  jq '.operator_note = "changed between installs"' "$TEST_SETTINGS" > "$BATS_TEST_TMPDIR/changed.json"
  cp "$BATS_TEST_TMPDIR/changed.json" "$TEST_SETTINGS"
  run_installer
  assert_installed
  cmp -s "$TEST_SETTINGS" "$BATS_TEST_TMPDIR/changed.json"
  assert_new_backup "$BATS_TEST_TMPDIR/changed.json"
}

policy_fixture() {
  local skill="$1"
  installer="$skill/scripts/install-hooks.sh"
  sources=("$skill/hooks/policy-dispatch.sh" "$skill/policies/policies.json")
  destinations=("$TEST_HOME/.claude/hooks/aop/policy-dispatch.sh" "$TEST_HOME/.claude/hooks/aop/policies.json")
  matchers='["Bash","Edit|Write"]'
}

edit_guard_fixture() {
  local checkout="$1"
  installer="$checkout/scripts/install-installed-skill-edit-guard.sh"
  sources=("$checkout/hooks/guards/hooks/installed-skill-edit-guard.sh")
  destinations=("$TEST_HOME/.claude/hooks/installed-skill-edit-guard.sh")
  matchers='["Edit|Write"]'
}

@test "policy source installer retains same-second backups while refreshing stale assets" {
  policy_fixture "$REPO/hooks/guards"
  exercise_repeated_install
}

@test "copied policy package retains same-second backups without a repository" {
  cp -R "$REPO/hooks/guards" "$BATS_TEST_TMPDIR/copied guards"
  policy_fixture "$BATS_TEST_TMPDIR/copied guards"
  exercise_repeated_install
}

@test "edit guard source installer retains same-second backups while refreshing stale assets" {
  edit_guard_fixture "$REPO"
  exercise_repeated_install
}

@test "copied edit guard installer retains same-second backups without Git metadata" {
  local checkout="$BATS_TEST_TMPDIR/copied checkout"
  mkdir -p "$checkout/scripts/lib" "$checkout/hooks/guards/hooks"
  cp "$REPO/scripts/install-installed-skill-edit-guard.sh" "$checkout/scripts/"
  cp "$REPO/scripts/lib/repo-root.sh" "$checkout/scripts/lib/"
  cp "$REPO/hooks/guards/hooks/installed-skill-edit-guard.sh" "$checkout/hooks/guards/hooks/"
  edit_guard_fixture "$checkout"
  exercise_repeated_install
}
