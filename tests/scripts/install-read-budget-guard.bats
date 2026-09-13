#!/usr/bin/env bats
# Contract for scripts/install-read-budget-guard.sh — the opt-in installer for
# the read-budget PreToolUse guard (policy core.context:unbounded-read).
#
# The guard ships INERT: nothing wires it until this script is run explicitly.
# The installer copies the guard to $HOME/.claude/hooks/read-budget-guard.sh
# (mode 0755) and adds ONE idempotent PreToolUse "Read|Bash" matcher to a
# Claude settings.json ($HOME/.claude/settings.json, .claude/settings.json with
# --project, or $SETTINGS), taking a timestamped .bak before mutating.
#
# HOME and SETTINGS live inside an isolated TMPDIR so the real user scope is
# never touched.

INSTALLER="${INSTALLER:-$BATS_TEST_DIRNAME/../../scripts/install-read-budget-guard.sh}"
SRC="${SRC:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/read-budget-guard.sh}"

setup() {
  export TMPDIR="$(mktemp -d)"
  export HOME="$TMPDIR/home"
  mkdir -p "$HOME"
  unset SETTINGS || true
  DST="$HOME/.claude/hooks/read-budget-guard.sh"
  USER_SETTINGS="$HOME/.claude/settings.json"
}
teardown() { rm -rf "$TMPDIR"; }

# file_mode PATH → octal permission bits, GNU stat first (BSD `stat -c` fails
# cleanly; GNU `stat -f` does NOT, it means --file-system — order is load-bearing).
file_mode() { stat -c %a "$1" 2>/dev/null || stat -f %Lp "$1"; }

# matcher_count SETTINGS → number of PreToolUse entries with matcher "Read|Bash".
matcher_count() {
  jq '[.hooks.PreToolUse[]? | select(.matcher == "Read|Bash")] | length' "$1"
}

@test "installer: script exists, is executable, and sources scripts/lib/preamble.sh" {
  [ -f "$INSTALLER" ]
  [ -x "$INSTALLER" ]
  run grep -cF '. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/preamble.sh"' "$INSTALLER"
  [ "$output" = "1" ]
  run grep -cF '"$REPO_ROOT/skills/cc-hooks/hooks/read-budget-guard.sh"' "$INSTALLER"
  [ "$output" = "1" ]
  # It must not also source lib/repo-root.sh (the preamble owns REPO_ROOT).
  run grep -c 'lib/repo-root.sh' "$INSTALLER"
  [ "$output" = "0" ]
}

@test "installer: installs the guard file at \$HOME/.claude/hooks with mode 755" {
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [ -f "$DST" ]
  [ -x "$DST" ]
  [ "$(file_mode "$DST")" = "755" ]
  [[ "$output" == *"✓ installed $DST"* ]]
}

@test "installer: the installed guard byte-equals the repo source" {
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  cmp -s "$SRC" "$DST"
}

@test "installer: adds exactly one Read|Bash PreToolUse matcher whose command is the installed path" {
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [ -f "$USER_SETTINGS" ]
  [ "$(matcher_count "$USER_SETTINGS")" -eq 1 ]
  run jq -r '.hooks.PreToolUse[] | select(.matcher == "Read|Bash") | .hooks[0].type + " " + .hooks[0].command' "$USER_SETTINGS"
  [ "$output" = "command $DST" ]
  run jq '.hooks.PreToolUse[] | select(.matcher == "Read|Bash") | .hooks | length' "$USER_SETTINGS"
  [ "$output" = "1" ]
}

@test "installer: re-running is idempotent (still exactly one matcher)" {
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [ "$(matcher_count "$USER_SETTINGS")" -eq 1 ]
  run jq '[.hooks.PreToolUse[]?.hooks[]? | select(.command == "'"$DST"'")] | length' "$USER_SETTINGS"
  [ "$output" = "1" ]
}

@test "installer: --project (run from a TMPDIR cwd) writes .claude/settings.json there" {
  proj="$TMPDIR/project"
  mkdir -p "$proj"
  cd "$proj"
  run bash "$INSTALLER" --project
  [ "$status" -eq 0 ]
  [ -f "$proj/.claude/settings.json" ]
  [ "$(matcher_count "$proj/.claude/settings.json")" -eq 1 ]
  [ ! -f "$USER_SETTINGS" ]
  # The guard itself still lands under $HOME/.claude/hooks (absolute command).
  [ -f "$DST" ]
  run jq -r '.hooks.PreToolUse[] | select(.matcher == "Read|Bash") | .hooks[0].command' "$proj/.claude/settings.json"
  [ "$output" = "$DST" ]
}

@test "installer: SETTINGS env overrides the target settings file" {
  custom="$TMPDIR/custom/settings.json"
  SETTINGS="$custom" run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [ -f "$custom" ]
  [ "$(matcher_count "$custom")" -eq 1 ]
  [ ! -f "$USER_SETTINGS" ]
}

@test "installer: a timestamped .bak is created when settings pre-existed, and existing keys survive" {
  mkdir -p "$(dirname "$USER_SETTINGS")"
  printf '{"model":"opus","hooks":{"PreToolUse":[{"matcher":"Edit","hooks":[{"type":"command","command":"/x/other.sh"}]}]}}\n' > "$USER_SETTINGS"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"✓ backed up settings"* ]]
  bak="$(ls "$USER_SETTINGS".bak.* | head -n 1)"
  [ -n "$bak" ]
  [ -f "$bak" ]
  run jq -r '.model' "$bak"
  [ "$output" = "opus" ]
  # The backup holds the pre-mutation content: no Read|Bash matcher yet.
  [ "$(matcher_count "$bak")" -eq 0 ]
  # The live file keeps its prior keys and prior matcher alongside the new one.
  run jq -r '.model' "$USER_SETTINGS"
  [ "$output" = "opus" ]
  run jq '.hooks.PreToolUse | length' "$USER_SETTINGS"
  [ "$output" = "2" ]
  [ "$(matcher_count "$USER_SETTINGS")" -eq 1 ]
}

@test "installer: prints ✓ lines and an Uninstall line naming the matcher and file" {
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [[ "$output" == *"✓ installed $DST"* ]]
  [[ "$output" == *"✓ wired Read|Bash PreToolUse guard into $USER_SETTINGS"* ]]
  [[ "$output" == *"Uninstall:"* ]]
  [[ "$output" == *"rm -f $DST"* ]]
}

@test "installer: does not touch the repo's hooks/hooks.json (ships inert)" {
  repo="$BATS_TEST_DIRNAME/../.."
  before="$(cat "$repo/hooks/hooks.json")"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [ "$(cat "$repo/hooks/hooks.json")" = "$before" ]
  run grep -c "read-budget-guard" "$repo/hooks/hooks.json"
  [ "$output" = "0" ]
}

@test "installer: same-second settings changes retain both original backups" {
  mkdir -p "$TMPDIR/bin" "$(dirname "$USER_SETTINGS")"
  printf '#!/bin/sh\nprintf "%%s\\n" 20260912120000\n' > "$TMPDIR/bin/date"
  chmod +x "$TMPDIR/bin/date"
  export PATH="$TMPDIR/bin:$PATH"
  printf '{"model":"first"}\n' > "$USER_SETTINGS"
  cp "$USER_SETTINGS" "$TMPDIR/first.json"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]

  printf '{"model":"second"}\n' > "$USER_SETTINGS"
  cp "$USER_SETTINGS" "$TMPDIR/second.json"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]

  backups=("$USER_SETTINGS".bak.*)
  [ "${#backups[@]}" -eq 2 ]
  first_found=0
  second_found=0
  for backup in "${backups[@]}"; do
    if cmp -s "$TMPDIR/first.json" "$backup"; then first_found=1; fi
    if cmp -s "$TMPDIR/second.json" "$backup"; then second_found=1; fi
  done
  [ "$first_found" -eq 1 ]
  [ "$second_found" -eq 1 ]
}

@test "installer: an unchanged rerun preserves the original backup and creates none" {
  mkdir -p "$TMPDIR/bin" "$(dirname "$USER_SETTINGS")"
  printf '#!/bin/sh\nprintf "%%s\\n" 20260912120000\n' > "$TMPDIR/bin/date"
  chmod +x "$TMPDIR/bin/date"
  export PATH="$TMPDIR/bin:$PATH"
  printf '{"model":"original"}\n' > "$USER_SETTINGS"
  cp "$USER_SETTINGS" "$TMPDIR/original.json"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  cp "$USER_SETTINGS" "$TMPDIR/installed.json"

  printf '#!/bin/sh\nprintf "%%s\\n" 20260912120001\n' > "$TMPDIR/bin/date"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [[ "$output" != *"backed up settings"* ]]
  cmp -s "$USER_SETTINGS" "$TMPDIR/installed.json"
  backups=("$USER_SETTINGS".bak.*)
  [ "${#backups[@]}" -eq 1 ]
  cmp -s "${backups[0]}" "$TMPDIR/original.json"
}

@test "installer: the same command under Edit does not prevent Read|Bash installation" {
  mkdir -p "$(dirname "$USER_SETTINGS")"
  jq -nc --arg cmd "$DST" '{hooks:{PreToolUse:[{matcher:"Edit",hooks:[{type:"command",command:$cmd}]}]}}' > "$USER_SETTINGS"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  [ "$(matcher_count "$USER_SETTINGS")" -eq 1 ]
  run jq -e --arg cmd "$DST" 'any(.hooks.PreToolUse[]; .matcher == "Read|Bash" and any(.hooks[]; .type == "command" and .command == $cmd))' "$USER_SETTINGS"
  [ "$status" -eq 0 ]
  run jq -r '.hooks.PreToolUse[0].matcher' "$USER_SETTINGS"
  [ "$output" = "Edit" ]
}

@test "installer: a non-command hook does not prevent command hook installation" {
  mkdir -p "$(dirname "$USER_SETTINGS")"
  jq -nc --arg cmd "$DST" '{hooks:{PreToolUse:[{matcher:"Read|Bash",hooks:[{type:"prompt",command:$cmd,prompt:"Existing prompt"}]}]}}' > "$USER_SETTINGS"
  run bash "$INSTALLER"
  [ "$status" -eq 0 ]
  run jq -e --arg cmd "$DST" 'any(.hooks.PreToolUse[]; .matcher == "Read|Bash" and any(.hooks[]; .type == "command" and .command == $cmd))' "$USER_SETTINGS"
  [ "$status" -eq 0 ]
  run jq -r '.hooks.PreToolUse[0].hooks[0].prompt' "$USER_SETTINGS"
  [ "$output" = "Existing prompt" ]
}
