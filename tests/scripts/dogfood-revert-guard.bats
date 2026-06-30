#!/usr/bin/env bats
# age-skills-coexist-harden-04h2.2: dogfood-setup.sh --revert must refuse to
# restore a backup substantially smaller than the live skills dir. A stale
# backup (few skills) would silently wipe a healthy set — the best-fit cause of
# the 26-vs-125 drift incident. The guard is overridable with --force-revert.
#
# Tests run against a fake $HOME so the real ~/.claude is never touched
# (the script derives CLAUDE_DIR="$HOME/.claude").

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/dogfood-setup.sh"
  FIX="$(mktemp -d)"
  export HOME="$FIX"
  mkdir -p "$FIX/.claude/skills" "$FIX/.claude/plugins"
}

teardown() {
  rm -rf "$FIX"
}

# Make the live skills dir hold $1 entries and the latest backup hold $2.
seed() {
  local live="$1" backup="$2" i
  rm -rf "$FIX/.claude/skills" "$FIX/.claude"/skills.backup.*
  mkdir -p "$FIX/.claude/skills"
  for ((i=1; i<=live; i++)); do mkdir -p "$FIX/.claude/skills/live$i"; done
  mkdir -p "$FIX/.claude/skills.backup.20200101"
  for ((i=1; i<=backup; i++)); do mkdir -p "$FIX/.claude/skills.backup.20200101/bak$i"; done
}

@test "refuses when backup is far smaller than live (stale-backup wipe)" {
  seed 20 5
  run bash "$SCRIPT" --revert
  [ "$status" -eq 1 ]
  [[ "$output" == *"Refusing to revert"* ]]
  # live dir is untouched — still 20 entries, none deleted
  [ "$(find "$FIX/.claude/skills" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ')" -eq 20 ]
}

@test "--force-revert bypasses the guard" {
  seed 20 5
  run bash "$SCRIPT" --revert --force-revert
  [[ "$output" != *"Refusing to revert"* ]]
  [[ "$output" == *"Restored skills"* ]]
}

@test "allows restore when backup is same-size-or-larger" {
  seed 20 20
  run bash "$SCRIPT" --revert
  [[ "$output" != *"Refusing to revert"* ]]
  [[ "$output" == *"Restored skills"* ]]
}

@test "refuses at the odd-count boundary (no floor fail-open)" {
  # live=125, backup=62 is <50% but 125/2 floors to 62; must still refuse.
  seed 125 62
  run bash "$SCRIPT" --revert
  [ "$status" -eq 1 ]
  [[ "$output" == *"Refusing to revert"* ]]
}

@test "refuses when backup is empty and live has one skill" {
  seed 1 0
  run bash "$SCRIPT" --revert
  [ "$status" -eq 1 ]
  [[ "$output" == *"Refusing to revert"* ]]
}
