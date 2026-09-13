#!/usr/bin/env bats
# Pair guard expectations with the native utility's exit and output. GNU-only
# forms run through a real GNU inode named cat/head, not through a BSD command.
GUARD="${GUARD:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/read-budget-guard.sh}"

setup() {
  export TMPDIR="$(mktemp -d)"
  export HOME="$TMPDIR/home"
  export AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/telemetry.jsonl"
  export AOP_WAIVER_FILE="$TMPDIR/waivers"
  unset AOP_WAIVE AOP_READ_BUDGET_LINES AGENTOPS_HOOKS_DISABLED || true
  WORK="$TMPDIR/work"
  mkdir -p "$HOME" "$WORK/bin"
  seq 1 400 > "$WORK/big.txt"
}
teardown() { rm -rf "$TMPDIR"; }

actual_read() {
  local expected_status="$1" expected_lines="$2" result=0
  shift 2
  "$@" "$WORK/big.txt" > "$TMPDIR/actual.stdout" 2> "$TMPDIR/actual.stderr" || result=$?
  [ "$result" -eq "$expected_status" ]
  [ "$(wc -l < "$TMPDIR/actual.stdout" | tr -d ' ')" -eq "$expected_lines" ]
}

guard_read() {
  local expected="$1" command="$2" result=0
  jq -nc --arg t Bash --arg s "utility-$BATS_TEST_NUMBER" --arg c "$WORK" --arg command "$command" \
    '{tool_name:$t,tool_input:{command:$command},session_id:$s,cwd:$c}' > "$TMPDIR/input.json"
  bash "$GUARD" < "$TMPDIR/input.json" > "$TMPDIR/stdout" 2> "$TMPDIR/stderr" || result=$?
  [ "$result" -eq "$expected" ]
  [ ! -s "$TMPDIR/stdout" ]
  if [ "$expected" -eq 0 ]; then [ ! -s "$TMPDIR/stderr" ]; else [ -s "$TMPDIR/stderr" ]; fi
}

require_gnu() {
  local candidate
  candidate="$(command -v "g$1" || command -v "$1")"
  [[ "$("$candidate" --version 2>/dev/null)" == *"GNU coreutils"* ]] || skip "GNU $1 is not installed"
  ln -s "$candidate" "$WORK/bin/$1"
}

@test "UTILITY: native head with positive signed count reads and blocks 400 lines" {
  actual_read 0 400 head -n +400
  guard_read 2 'head -n +400 big.txt'
}

@test "UTILITY: Darwin cat rejects GNU formatting flags before reading" {
  [ "$(uname -s)" = Darwin ] || skip "Darwin BSD cat contract"
  local flag
  for flag in -A -E -T -An -nE --number; do
    actual_read 1 0 /bin/cat "$flag"
    guard_read 0 "/bin/cat $flag big.txt"
  done
}

@test "UTILITY: Darwin head rejects a negative count before reading" {
  [ "$(uname -s)" = Darwin ] || skip "Darwin BSD head contract"
  actual_read 1 0 /usr/bin/head -n -20
  guard_read 0 '/usr/bin/head -n -20 big.txt'
}

@test "UTILITY: Darwin head rejects unsupported display flags before reading" {
  [ "$(uname -s)" = Darwin ] || skip "Darwin BSD head contract"
  local flag
  for flag in -q -v --quiet --silent --verbose; do
    actual_read 1 0 /usr/bin/head "$flag" -n 400
    guard_read 0 "/usr/bin/head $flag -n 400 big.txt"
  done
}

@test "UTILITY: GNU negative head still counts and blocks its real output" {
  require_gnu head
  actual_read 0 380 "$WORK/bin/head" -n -20
  guard_read 2 "\"$WORK/bin/head\" -n -20 big.txt"
  [[ "$(cat "$TMPDIR/stderr")" == *"is 380 lines"* ]]
  actual_read 0 10 "$WORK/bin/head" -n -390
  guard_read 0 "\"$WORK/bin/head\" -n -390 big.txt"
}

@test "UTILITY: GNU cat formatting flags still read and block 400 lines" {
  require_gnu cat
  local flag
  for flag in -A -E -T -An -nE --number; do
    actual_read 0 400 "$WORK/bin/cat" "$flag"
    guard_read 2 "\"$WORK/bin/cat\" $flag big.txt"
  done
}

@test "UTILITY: system inode detection follows symlinks and relative command paths" {
  [ "$(uname -s)" = Darwin ] || skip "Darwin BSD utility identities"
  ln -s /bin/cat "$WORK/bin/cat"
  ln -s /usr/bin/head "$WORK/bin/head"
  actual_read 1 0 "$WORK/bin/cat" -A
  guard_read 0 './bin/cat -A big.txt'
  actual_read 1 0 "$WORK/bin/head" -n -20
  guard_read 0 './bin/head -n -20 big.txt'
}

@test "UTILITY: nonexistent literal command paths cannot read a real file" {
  guard_read 0 '/nonexistent/bin/cat big.txt'
}

@test "UTILITY: literal PATH assignments select the actual GNU executable" {
  require_gnu cat
  actual_read 0 400 env "PATH=$WORK/bin" cat -A
  guard_read 2 "PATH=$WORK/bin cat -A big.txt"
}

@test "UTILITY: assignment-only PATH changes do not reuse the hook PATH in later segments" {
  local separator
  for separator in ';' '&&' $'\n'; do
    actual_read 127 0 bash -c "PATH=/nonexistent $separator cat \"\$1\"" _
    guard_read 0 "PATH=/nonexistent $separator cat big.txt"
  done
}

@test "UTILITY: a trailing assignment does not hide an earlier unbounded read" {
  actual_read 0 400 bash -c 'cat "$1"; PATH=/nonexistent' _
  guard_read 2 'cat big.txt; PATH=/nonexistent'
}
