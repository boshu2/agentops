#!/usr/bin/env bats

setup() {
  HOOK="$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/skill-first-coord-guard.sh"
  TEST_TMPDIR="$(mktemp -d)"
}

teardown() {
  rm -rf "$TEST_TMPDIR"
}

run_hook() {
  local payload="$1"
  run env TMPDIR="$TEST_TMPDIR" bash -c \
    'printf "%s" "$1" | bash "$2"' _ "$payload" "$HOOK"
}

@test "allows an ordinary Claude Bash command silently" {
  run_hook '{"session_id":"claude-allow","tool_input":{"command":"git status --short"}}'

  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "blocks a Claude NTM command once per session" {
  local payload='{"session_id":"claude-block","tool_input":{"command":"ntm --robot-status"}}'

  run_hook "$payload"
  [ "$status" -eq 2 ]
  [[ "$output" == *'load the `agent-mail` or `ntm` skill contract'* ]]

  run_hook "$payload"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "accepts Codex command-line and thread-id fields" {
  run_hook '{"thread_id":"codex-block","tool_input":{"command_line":"/Users/bo/.local/bin/ntm --robot-status"}}'

  [ "$status" -eq 2 ]
}

@test "accepts AGY top-level command and conversation-id fields" {
  run_hook '{"conversation_id":"agy-block","command":"tmux send-keys -t agentops:1.0 audit Enter"}'

  [ "$status" -eq 2 ]
}

@test "does not match coordination words inside quoted prose" {
  run_hook '{"session_id":"quoted","tool_input":{"command":"br create --body \"run ntm --robot-status later\""}}'

  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "fails open on malformed JSON" {
  run_hook '{'

  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "kill switch disables the hook silently" {
  local payload='{"session_id":"disabled","tool_input":{"command":"ntm --robot-status"}}'

  run env AGENTOPS_HOOKS_DISABLED=1 TMPDIR="$TEST_TMPDIR" bash -c \
    'printf "%s" "$1" | bash "$2"' _ "$payload" "$HOOK"

  [ "$status" -eq 0 ]
  [ -z "$output" ]
}
