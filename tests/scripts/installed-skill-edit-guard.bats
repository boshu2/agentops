#!/usr/bin/env bats
# age-workflow-guardrail-hooks-j39.1 — contract for installed-skill-edit-guard.sh.
#
# The guard is a PreToolUse / Edit|Write hook. It BLOCKS (exit 2 + stderr) an
# Edit/Write whose tool_input.file_path is under an installed skills copy
# (*/.claude/skills/**, .codex, .gemini), routing to the repo skills/ SOT. It is
# SILENT (exit 0, zero output) on a repo skills/** edit or any other path.
#
# We round-trip the REAL PreToolUse JSON input shape on stdin (built with jq),
# never a hand-built fake, per the guard-test fixture-fidelity rule.

GUARD="${GUARD:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/installed-skill-edit-guard.sh}"

setup() { export TMPDIR="$(mktemp -d)"; }
teardown() { rm -rf "$TMPDIR"; }

# $1 = file_path, $2 = session_id (defaults unique per call)
run_guard() {
  jq -nc --arg p "$1" --arg s "${2:-sess-$RANDOM}" \
    '{tool_name:"Edit", tool_input:{file_path:$p}, session_id:$s}' \
    | bash "$GUARD"
}

# --- FIRE (exit 2): installed skill copies ---------------------------------

@test "FIRE: ~/.claude/skills SKILL.md edit blocks (exit 2)" {
  run run_guard "$HOME/.claude/skills/evolve/SKILL.md" "f1"
  [ "$status" -eq 2 ]
  [[ "$output" == *"skills/evolve/"* ]]
  [[ "$output" == *"source of truth"* ]]
}

@test "FIRE: absolute /Users/*/.claude/skills reference edit blocks" {
  run run_guard "/Users/someone/.claude/skills/research/references/x.md" "f2"
  [ "$status" -eq 2 ]
  [[ "$output" == *"skills/research/"* ]]
}

@test "FIRE: .codex/skills installed copy blocks" {
  run run_guard "$HOME/.codex/skills/rpi/SKILL.md" "f3"
  [ "$status" -eq 2 ]
}

@test "FIRE: .gemini/skills installed copy blocks" {
  run run_guard "$HOME/.gemini/skills/plan/SKILL.md" "f4"
  [ "$status" -eq 2 ]
}

# --- SILENT (exit 0, zero output): repo skills/ and other paths ------------

@test "SILENT: repo skills/<name>/SKILL.md edit (exit 0, no output)" {
  run run_guard "/Users/bo/dev/agentops/skills/evolve/SKILL.md" "s1"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: repo relative skills/ path" {
  run run_guard "skills/cc-hooks/SKILL.md" "s2"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: an unrelated source file" {
  run run_guard "/Users/bo/dev/agentops/cli/cmd/ao/main.go" "s3"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: a doc whose path mentions 'claude' but not the installed-skills segment" {
  run run_guard "/Users/bo/dev/agentops/docs/claude-skills-notes.md" "s4"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: no file_path field at all" {
  echo '{"tool_name":"Edit","tool_input":{},"session_id":"s5"}' | bash "$GUARD"
  status=$?
  [ "$status" -eq 0 ]
}

# --- once-per-session: fires then self-relaxes ----------------------------

@test "once-per-session: first fires (2), second self-relaxes (0)" {
  run run_guard "$HOME/.claude/skills/evolve/SKILL.md" "same"
  [ "$status" -eq 2 ]
  run run_guard "$HOME/.claude/skills/research/SKILL.md" "same"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}
