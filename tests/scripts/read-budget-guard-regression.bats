#!/usr/bin/env bats
# Reproductions from the independent e32e88c review. Stdout and stderr are
# captured independently so every allow and deny also checks hook silence.
GUARD="${GUARD:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/read-budget-guard.sh}"

setup() {
  export TMPDIR="$(mktemp -d)"
  export HOME="$TMPDIR/home"
  export AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/telemetry.jsonl"
  export AOP_WAIVER_FILE="$TMPDIR/waivers"
  unset AOP_WAIVE AOP_READ_BUDGET_LINES AGENTOPS_HOOKS_DISABLED || true
  WORK="$TMPDIR/work"
  mkdir -p "$HOME" "$WORK/sub"
  seq 1 400 > "$WORK/big.txt"
  seq 1 100 > "$WORK/sub/big.txt"
  seq 1 400 > "$WORK/my notes.md"
  seq 1 400 > "$WORK/-n"
}
teardown() { rm -rf "$TMPDIR"; }

guard_command() {
  local expected="$1" command="$2"
  jq -nc --arg t Bash --arg s "regression-$BATS_TEST_NUMBER" --arg c "$WORK" \
    --arg command "$command" \
    '{tool_name:$t,tool_input:{command:$command},session_id:$s,cwd:$c}' > "$TMPDIR/input.json"
  local result=0
  bash "$GUARD" < "$TMPDIR/input.json" > "$TMPDIR/stdout" 2> "$TMPDIR/stderr" || result=$?
  [ "$result" -eq "$expected" ]
  [ ! -s "$TMPDIR/stdout" ]
  if [ "$expected" -eq 0 ]; then
    [ ! -s "$TMPDIR/stderr" ]
  else
    [[ "$(cat "$TMPDIR/stderr")" == *"bulk-read"* ]]
  fi
}

@test "REGRESSION: ANSI-C quoted prose does not become a cat command" {
  guard_command 0 "echo \$'it\\'s; cat big.txt; end'"
}

@test "REGRESSION: cd-chain filename collision never reads the original cwd" {
  guard_command 0 'cd sub && cat big.txt'
}

@test "REGRESSION: head excluding 390 of 400 lines is a bounded ten-line read" {
  guard_command 0 'head -n -390 big.txt'
}

@test "REGRESSION: negative head reports its effective count" {
  guard_command 2 'head -n -20 big.txt'
  [[ "$(cat "$TMPDIR/stderr")" == *"is 380 lines"* ]]
  [ "$(jq -r .lines "$AGENTOPS_GUARDRAIL_TELEMETRY")" -eq 380 ]
}

@test "REGRESSION: cat help or version never reads its file argument" {
  guard_command 0 'cat --help big.txt'
  guard_command 0 'cat --version big.txt'
}

@test "REGRESSION: unknown read flags fail open even with a large line limit" {
  guard_command 0 'cat --nonsense big.txt'
  guard_command 0 'head --nonsense -n 400 big.txt'
  guard_command 0 'tail --nonsense -n 400 big.txt'
}

@test "REGRESSION: quoted space is one literal file argument" {
  guard_command 2 'cat "my notes.md"'
  [[ "$(cat "$TMPDIR/stderr")" == *"my notes.md is 400 lines"* ]]
  guard_command 2 "cat 'my notes.md'"
}

@test "REGRESSION: escaped space is one literal file argument" {
  guard_command 2 'cat my\ notes.md'
}

@test "REGRESSION: double-quoted backslash-newline joins a literal path" {
  guard_command 2 $'cat "big\\\n.txt"'
}

@test "REGRESSION: -- ends option parsing for head and tail" {
  guard_command 2 'head -n 400 -- -n'
  guard_command 2 'tail -n 400 -- -n'
}

@test "REGRESSION: huge positive budget cannot wrap into a stricter one" {
  export AOP_READ_BUDGET_LINES=18446744073709551617
  guard_command 0 'cat big.txt'
  export AOP_READ_BUDGET_LINES=0000000000000000000000000000000000000000500
  guard_command 0 'cat big.txt'
}

@test "REGRESSION: out-of-range counts fail open without arithmetic wraparound" {
  guard_command 0 'head -n 18446744073709551617 big.txt'
  guard_command 0 'head -n -18446744073709551617 big.txt'
  guard_command 0 'tail -n +18446744073709551617 big.txt'
}

@test "REGRESSION: a quoted tilde is literal, not a HOME expansion" {
  seq 1 400 > "$HOME/big.txt"
  guard_command 0 'cat "~/big.txt"'
}

@test "REGRESSION: a quoted assignment is a command word, not a prefix" {
  guard_command 0 '"LC_ALL=C" cat big.txt'
}

@test "REGRESSION: unmatched later quotes cannot deny a command that fails to parse" {
  guard_command 0 'cat big.txt; echo "unfinished'
}

@test "REGRESSION: shell control flow and directory builtins are not judged at stale cwd" {
  guard_command 0 'builtin cd sub; cat big.txt'
  guard_command 0 'pushd sub; cat big.txt'
  guard_command 0 'if false; then :; cat big.txt; fi'
}

@test "REGRESSION: tail +0 counts the full file without adding a phantom line" {
  export AOP_READ_BUDGET_LINES=400
  guard_command 0 'tail -n +0 big.txt'
}

@test "REGRESSION: a small spaced file is not attributed to its large sibling" {
  seq 1 400 > "$WORK/my"
  seq 1 100 > "$WORK/my notes.md"
  guard_command 0 'cat "my notes.md"'
}

@test "REGRESSION: malformed command separators fail open before any denial" {
  guard_command 0 'cat big.txt ;; echo done'
  guard_command 0 'cat big.txt &&'
  guard_command 0 'cat big.txt && ; echo done'
}
