#!/usr/bin/env bats
# Codex CLI 0.154.0 documented canonical PreToolUse Bash input, including its
# native metadata. Live runtime scheduling/trust needs a separate session proof.
GUARD="${GUARD:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/codex-read-budget-guard.sh}"
bats_require_minimum_version 1.5.0

setup() {
  export TMPDIR="$(mktemp -d)"
  export HOME="$TMPDIR/home"
  export AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/guardrail-telemetry.jsonl"
  unset AOP_WAIVE AOP_WAIVER_FILE AGENTOPS_HOOKS_DISABLED AOP_READ_BUDGET_LINES
  mkdir -p "$HOME" "$TMPDIR/project/sub" "$TMPDIR/other"
  PROJECT="$TMPDIR/project"
  seq 1 400 > "$PROJECT/big.txt"
  seq 1 100 > "$PROJECT/small.txt"
  seq 1 350 > "$PROJECT/budget.txt"
  printf '\0binary\n' > "$PROJECT/binary.txt"
  seq 1 400 >> "$PROJECT/binary.txt"
  EVENT="$TMPDIR/event.json"
}
teardown() { rm -rf "$TMPDIR"; }

event() {
  jq -nc --arg c "$PROJECT" --arg command "$1" '{
    hook_event_name:"PreToolUse",tool_name:"Bash",tool_input:{command:$command},
    cwd:$c,session_id:"codex-test-session",turn_id:"turn-123",tool_use_id:"call-456",
    model:"gpt-5.6-luna",permission_mode:"never",transcript_path:"/private/session.jsonl"
  }' > "$EVENT"
}
invoke() {
  run --separate-stderr bash "$GUARD" < "$EVENT"
  [ -z "$output" ]
}

@test "codex guard: canonical shell event denies before execution with native advice" {
  event 'cat big.txt'
  invoke
  [ "$status" -eq 2 ]
  [[ "$stderr" == *"core.context:unbounded-read"* ]]
  [[ "$stderr" == *"sed -n"* ]]
  [[ "$stderr" == *"Codex: delegate"*"bulk-reader"* ]]
  [[ "$stderr" != *"Workflow:"* ]]
  [[ "$stderr" != *"Agent tool:"* ]]
  [[ "$stderr" != *"Read(file_path"* ]]
}

@test "codex guard: every attempt denies while repeated advice is one short line" {
  event 'cat big.txt'
  invoke
  [ "$status" -eq 2 ]
  invoke
  [ "$status" -eq 2 ]
  [ "${#stderr_lines[@]}" -eq 1 ]
  [[ "$stderr" == *"full reason shown earlier"* ]]
  [[ "$stderr" != *"offset+limit"* ]]
  [ "$(wc -l < "$AGENTOPS_GUARDRAIL_TELEMETRY" | tr -d ' ')" -eq 2 ]
}

@test "codex guard: bounded, at-budget, binary, missing and unmonitored calls are silent" {
  for command in 'head -n 100 big.txt' 'tail -n 100 big.txt' 'cat small.txt' \
    'cat budget.txt' 'cat binary.txt' 'cat missing.txt' 'git status' \
    'cat big.txt | head -n 10' 'cat big.txt > output.txt' \
    'echo "text; cat big.txt; more text"'; do
    event "$command"
    invoke
    [ "$status" -eq 0 ]
    [ -z "$stderr" ]
  done
  [ ! -e "$AGENTOPS_GUARDRAIL_TELEMETRY" ]
}

@test "codex guard: resolves relative paths through event cwd, not hook cwd" {
  event 'cat big.txt'
  cd "$TMPDIR/other"
  invoke
  [ "$status" -eq 2 ]
  [[ "$stderr" == *"$PROJECT/big.txt"* ]]
}

@test "codex guard: waivers preserve one hashed record and remain silent" {
  event 'AOP_WAIVE=core.context:unbounded-read cat big.txt'
  invoke
  [ "$status" -eq 0 ]; [ -z "$stderr" ]
  event 'cat big.txt'
  export AOP_WAIVE=core.context:unbounded-read
  invoke
  [ "$status" -eq 0 ]; [ -z "$stderr" ]
  unset AOP_WAIVE
  export AOP_WAIVER_FILE="$TMPDIR/waivers"
  printf 'core.context:unbounded-read %s\n' "$(( $(date +%s) + 600 ))" > "$AOP_WAIVER_FILE"
  invoke
  [ "$status" -eq 0 ]; [ -z "$stderr" ]
  run jq -se 'length == 3 and all(.[]; .decision == "waived")' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -eq 0 ]
}

@test "codex guard: hashed telemetry keeps the shared schema without native private metadata" {
  event 'cat big.txt'
  invoke
  [ "$status" -eq 2 ]
  run jq -se 'length == 1 and (.[0] |
    keys == ["budget","decision","lines","mode","path_sha256","session","token_class","tool","ts"] and
    .tool == "Bash" and .lines == 400 and .budget == 350 and .mode == "deny" and
    .decision == "deny" and (.path_sha256 | test("^[0-9a-f]{64}$")))' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -eq 0 ]
  run grep -E 'big.txt|cat big|private/session|turn-123|call-456' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -eq 1 ]
}

@test "codex guard: budget setting and kill switch are honored" {
  event 'cat big.txt'
  export AOP_READ_BUDGET_LINES=500
  invoke
  [ "$status" -eq 0 ]; [ -z "$stderr" ]
  unset AOP_READ_BUDGET_LINES
  export AGENTOPS_HOOKS_DISABLED=1
  invoke
  [ "$status" -eq 0 ]; [ -z "$stderr" ]
  [ ! -e "$AGENTOPS_GUARDRAIL_TELEMETRY" ]
}

@test "codex guard: malformed or unverified event shapes fail open silently" {
  for payload in '{' 'null' '[]' '{}' \
    '{"hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"cat big.txt"}}' \
    '{"hook_event_name":"PreToolUse","tool_name":"read_file","tool_input":{"path":"big.txt"}}' \
    '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":17}}'; do
    printf '%s' "$payload" > "$EVENT"
    invoke
    [ "$status" -eq 0 ]; [ -z "$stderr" ]
  done
  [ ! -e "$AGENTOPS_GUARDRAIL_TELEMETRY" ]
}

@test "codex guard: missing jq or missing sibling fails open silently" {
  event 'cat big.txt'
  mkdir "$TMPDIR/empty"
  run --separate-stderr env PATH="$TMPDIR/empty" /bin/bash "$GUARD" < "$EVENT"
  [ "$status" -eq 0 ]; [ -z "$output" ]; [ -z "$stderr" ]
  cp "$GUARD" "$TMPDIR/empty/codex-read-budget-guard.sh"
  run --separate-stderr bash "$TMPDIR/empty/codex-read-budget-guard.sh" < "$EVENT"
  [ "$status" -eq 0 ]; [ -z "$output" ]; [ -z "$stderr" ]
}
