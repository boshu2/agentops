#!/usr/bin/env bats
# Contract for skills/cc-hooks/hooks/read-budget-guard.sh — the opt-in
# PreToolUse / Read|Bash read-budget guard (policy core.context:unbounded-read).
#
# The guard BLOCKS (exit 2 + stderr) an UNBOUNDED read of a text file over the
# line budget (default 350): a Read without a numeric limit, or a Bash cat /
# head / tail whose effective line count exceeds the budget. It is SILENT
# (exit 0, zero stdout, zero stderr) on a bounded slice, a file at/below budget,
# a binary / missing / non-regular path, a piped or redirected command, any
# other command, and every fail-open path. Every attempt blocks (never
# self-relaxes); the first fire in a session prints the full message, later
# fires print one short line.
#
# Fixture fidelity: every case round-trips the REAL PreToolUse JSON input shape
# (tool_name / tool_input / session_id / cwd) built with jq — never a hand-built
# string — per the guard-test fixture-fidelity rule. TMPDIR and HOME are
# isolated per test; telemetry is pointed into TMPDIR.

GUARD="${GUARD:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/read-budget-guard.sh}"
POLICY="core.context:unbounded-read"

setup() {
  export TMPDIR="$(mktemp -d)"
  export HOME="$TMPDIR/home"
  mkdir -p "$HOME"
  export AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/telemetry.jsonl"
  export AOP_WAIVER_FILE="$TMPDIR/waivers"
  unset AOP_WAIVE AOP_READ_BUDGET_LINES AGENTOPS_HOOKS_DISABLED || true

  # The JSON cwd every fixture points at; relative paths resolve against it.
  WORK="$TMPDIR/work"
  mkdir -p "$WORK/sub"
  seq 1 400 > "$WORK/big.txt"        # over budget
  seq 1 100 > "$WORK/small.txt"      # under budget
  seq 1 200 > "$WORK/a.txt"          # a + b = 400 > 350
  seq 1 200 > "$WORK/b.txt"
  seq 1 400 > "$WORK/sub/nested.txt" # only exists under sub/ (cd-chain gap)
  # A binary file: 400 newlines, but NUL bytes in the first 8 KiB.
  : > "$WORK/blob.bin"
  local i
  for i in $(seq 1 400); do printf 'row %d\000\n' "$i"; done >> "$WORK/blob.bin"
}
teardown() { rm -rf "$TMPDIR"; }

# read_payload FILE_PATH SESSION [OFFSET] [LIMIT] — the real Read PreToolUse
# JSON; offset/limit are emitted as JSON numbers only when given.
read_payload() {
  jq -nc --arg p "$1" --arg s "$2" --arg c "$WORK" \
    --arg off "${3:-}" --arg lim "${4:-}" '
    {tool_name:"Read",
     tool_input:({file_path:$p}
       + (if $off != "" then {offset:($off|tonumber)} else {} end)
       + (if $lim != "" then {limit:($lim|tonumber)} else {} end)),
     session_id:$s, cwd:$c}'
}

# bash_payload COMMAND SESSION — the real Bash PreToolUse JSON.
bash_payload() {
  jq -nc --arg c "$1" --arg s "$2" --arg d "$WORK" \
    '{tool_name:"Bash", tool_input:{command:$c}, session_id:$s, cwd:$d}'
}

run_read() { read_payload "$@" | bash "$GUARD"; }
run_bash() { bash_payload "$@" | bash "$GUARD"; }

# stdout_only FN ARGS... — capture ONLY stdout of a guard invocation (stderr
# dropped) into $out and its exit status into $rc, tolerating the exit-2 fire
# so the test's errexit does not trip.
stdout_only() {
  rc=0
  out="$("$@" 2>/dev/null)" || rc=$?
}

# --- FIRE (exit 2, stderr names the policy id) --------------------------------

@test "FIRE: Read of a 400-line file without limit blocks (exit 2, names the policy)" {
  run run_read "$WORK/big.txt" "f-read"
  [ "$status" -eq 2 ]
  [[ "$output" == *"policy $POLICY"* ]]
  [[ "$output" == *"big.txt is 400 lines (budget 350)"* ]]
}

@test "FIRE: Read with offset only (no limit) still blocks — offset alone does not bound" {
  run run_read "$WORK/big.txt" "f-offset" 50
  [ "$status" -eq 2 ]
  [[ "$output" == *"policy $POLICY"* ]]
}

@test "FIRE: Bash 'cat big.txt' blocks" {
  run run_bash "cat big.txt" "f-cat"
  [ "$status" -eq 2 ]
  [[ "$output" == *"policy $POLICY"* ]]
  [[ "$output" == *"$WORK/big.txt is 400 lines"* ]]
}

@test "FIRE: Bash 'cat -n big.txt' blocks (cat flags are not line counts)" {
  run run_bash "cat -n big.txt" "f-cat-n"
  [ "$status" -eq 2 ]
  [[ "$output" == *"policy $POLICY"* ]]
}

@test "FIRE: Bash 'head -n 500 big.txt' blocks (min(500,400)=400 > 350)" {
  run run_bash "head -n 500 big.txt" "f-head-n"
  [ "$status" -eq 2 ]
  [[ "$output" == *"is 400 lines"* ]]
}

@test "FIRE: Bash 'head -500 big.txt' blocks (-N form)" {
  run run_bash "head -500 big.txt" "f-head-N"
  [ "$status" -eq 2 ]
}

@test "FIRE: Bash 'head --lines=500 big.txt' blocks (--lines= form)" {
  run run_bash "head --lines=500 big.txt" "f-head-lines"
  [ "$status" -eq 2 ]
}

@test "FIRE: Bash 'head -n -5 big.txt' blocks (negative count = whole file minus a tail)" {
  run run_bash "head -n -5 big.txt" "f-head-neg"
  [ "$status" -eq 2 ]
  [[ "$output" == *"is 400 lines"* ]]
}

@test "FIRE: Bash 'tail -n 400 big.txt' blocks" {
  run run_bash "tail -n 400 big.txt" "f-tail-n"
  [ "$status" -eq 2 ]
  [[ "$output" == *"is 400 lines"* ]]
}

@test "FIRE: Bash 'tail -n +5 big.txt' blocks (400-5+1 = 396 > 350)" {
  run run_bash "tail -n +5 big.txt" "f-tail-plus"
  [ "$status" -eq 2 ]
  [[ "$output" == *"is 396 lines"* ]]
}

@test "FIRE: Bash 'cat a.txt b.txt' blocks on the SUM (200+200 > 350)" {
  run run_bash "cat a.txt b.txt" "f-cat-sum"
  [ "$status" -eq 2 ]
  [[ "$output" == *"is 400 lines (budget 350)"* ]]
}

@test "FIRE: a relative Read path is resolved through the JSON cwd" {
  run run_read "big.txt" "f-relative"
  [ "$status" -eq 2 ]
  [[ "$output" == *"$WORK/big.txt is 400 lines"* ]]
}

@test "FIRE: quoted path 'cat \"big.txt\"' blocks (one quote layer stripped)" {
  run run_bash 'cat "big.txt"' "f-quoted"
  [ "$status" -eq 2 ]
}

@test "FIRE: an absolute command word '/bin/cat big.txt' blocks (basename match)" {
  run run_bash "/bin/cat big.txt" "f-basename"
  [ "$status" -eq 2 ]
}

@test "FIRE: 'echo x && cat big.txt' and 'cat big.txt; echo y' block (segment split on && and ;)" {
  run run_bash "echo x && cat big.txt" "f-and"
  [ "$status" -eq 2 ]
  run run_bash "cat big.txt; echo y" "f-semi"
  [ "$status" -eq 2 ]
}

@test "FIRE: an inline AOP_WAIVE for a DIFFERENT id does not waive" {
  run run_bash "AOP_WAIVE=core.other:thing cat big.txt" "f-other-waive"
  [ "$status" -eq 2 ]
}

@test "FIRE: second fire in the same session STILL exits 2 and prints the short line" {
  run run_read "$WORK/big.txt" "same-session"
  [ "$status" -eq 2 ]
  [[ "$output" == *"→ Read a slice"* ]]
  run run_bash "cat big.txt" "same-session"
  [ "$status" -eq 2 ]
  [[ "$output" == *"policy $POLICY: $WORK/big.txt is 400 lines (budget 350)"* ]]
  [[ "$output" == *"full reason shown earlier this session"* ]]
  [[ "$output" != *"→ Read a slice"* ]]
}

@test "FIRE: the first fire's message names bulk-reader and both delegation doors" {
  run run_read "$WORK/big.txt" "f-full-msg"
  [ "$status" -eq 2 ]
  [[ "$output" == *"bulk-reader"* ]]
  [[ "$output" == *"→ Read a slice: Read(file_path, offset, limit) with limit ≤ 350"* ]]
  [[ "$output" == *"→ Or delegate the whole file to a cheap reader"* ]]
  [[ "$output" == *"Agent tool: subagent_type \"bulk-reader\""* ]]
  [[ "$output" == *"Workflow: bulk-read { question: \"<question>\", files: [\"$WORK/big.txt\"] }"* ]]
  [[ "$output" == *"Waive once: AOP_WAIVE=$POLICY"* ]]
  [[ "$output" == *"AOP_READ_BUDGET_LINES="* ]]
}

@test "FIRE: stdout is EMPTY on every fire path (block via exit 2 + stderr only)" {
  stdout_only run_read "$WORK/big.txt" "f-stdout-read"
  [ "$rc" -eq 2 ]
  [ -z "$out" ]
  stdout_only run_bash "cat big.txt" "f-stdout-bash"
  [ "$rc" -eq 2 ]
  [ -z "$out" ]
  # ...and the short-line (second fire, same session) path too.
  stdout_only run_bash "cat big.txt" "f-stdout-bash"
  [ "$rc" -eq 2 ]
  [ -z "$out" ]
}

# --- SILENT (exit 0, zero output) ---------------------------------------------

@test "SILENT: Read with limit 100 passes (a bounded slice always passes)" {
  run run_read "$WORK/big.txt" "s-limit" "" 100
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: Read with offset AND limit passes" {
  run run_read "$WORK/big.txt" "s-offset-limit" 200 100
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: Read of a 100-line file passes" {
  run run_read "$WORK/small.txt" "s-small"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: Read of a binary file (NUL bytes, 400 newlines) passes" {
  run run_read "$WORK/blob.bin" "s-binary"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: Read of a missing path passes" {
  run run_read "$WORK/does-not-exist.txt" "s-missing"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: Read of a directory passes" {
  run run_read "$WORK/sub" "s-dir"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'cat big.txt | head -20' passes (pipe = bounded consumer, out of scope)" {
  run run_bash "cat big.txt | head -20" "s-pipe"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'cat big.txt > out.txt' passes (redirect = file sink, out of scope)" {
  run run_bash "cat big.txt > out.txt" "s-redirect"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'head big.txt' passes (default 10 lines)" {
  run run_bash "head big.txt" "s-head-default"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'head -n 50 big.txt' passes" {
  run run_bash "head -n 50 big.txt" "s-head-50"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'tail -n 20 big.txt' passes" {
  run run_bash "tail -n 20 big.txt" "s-tail-20"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'head -c 100 big.txt' and 'tail -f big.txt' pass (byte / follow forms skipped)" {
  run run_bash "head -c 100 big.txt" "s-head-bytes"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  run run_bash "tail -f big.txt" "s-tail-follow"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'grep -n foo big.txt' passes (not a monitored command)" {
  run run_bash "grep -n foo big.txt" "s-grep"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: \"sed -n '1,400p' big.txt\" passes (sed is silent by design)" {
  run run_bash "sed -n '1,400p' big.txt" "s-sed"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'cat small.txt' passes (at/below budget)" {
  run run_bash "cat small.txt" "s-cat-small"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'git status' passes" {
  run run_bash "git status" "s-git"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'cat' with no file passes" {
  run run_bash "cat" "s-cat-nofile"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: 'cd sub && cat nested.txt' passes (documented gap: resolved against the original cwd, not found)" {
  run run_bash "cd sub && cat nested.txt" "s-cd-chain"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: unresolvable tokens (\$VAR, glob) are skipped" {
  run run_bash 'cat $FILE' "s-var"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  run run_bash "cat *.txt" "s-glob"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: tool_name Edit with a big file_path passes (only Read|Bash are judged)" {
  jq -nc --arg p "$WORK/big.txt" --arg c "$WORK" \
    '{tool_name:"Edit", tool_input:{file_path:$p, old_string:"1", new_string:"one"}, session_id:"s-edit", cwd:$c}' \
    > "$TMPDIR/edit.json"
  run bash "$GUARD" < "$TMPDIR/edit.json"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: no session_id in the payload is tolerated (nosession)" {
  jq -nc --arg p "$WORK/small.txt" '{tool_name:"Read", tool_input:{file_path:$p}}' > "$TMPDIR/nosess.json"
  run bash "$GUARD" < "$TMPDIR/nosess.json"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# --- WAIVERS / KILL SWITCH / BUDGET ------------------------------------------

@test "WAIVE: AOP_WAIVE env containing the id allows the call silently (Read and Bash)" {
  AOP_WAIVE="$POLICY" run run_read "$WORK/big.txt" "w-env-read"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  AOP_WAIVE="other,$POLICY" run run_bash "cat big.txt" "w-env-bash"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "WAIVE: inline 'AOP_WAIVE=core.context:unbounded-read cat big.txt' allows the call silently" {
  run run_bash "AOP_WAIVE=$POLICY cat big.txt" "w-inline"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "WAIVE: waiver file with a future expiry allows the call" {
  echo "$POLICY $(( $(date +%s) + 3600 ))" > "$AOP_WAIVER_FILE"
  run run_read "$WORK/big.txt" "w-file"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "WAIVE: an EXPIRED waiver-file entry still fires" {
  echo "$POLICY $(( $(date +%s) - 10 ))" > "$AOP_WAIVER_FILE"
  run run_read "$WORK/big.txt" "w-expired"
  [ "$status" -eq 2 ]
  [[ "$output" == *"policy $POLICY"* ]]
}

@test "KILL SWITCH: AGENTOPS_HOOKS_DISABLED=1 is silent (exit 0)" {
  AGENTOPS_HOOKS_DISABLED=1 run run_read "$WORK/big.txt" "k-read"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  AGENTOPS_HOOKS_DISABLED=1 run run_bash "cat big.txt" "k-bash"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "BUDGET: AOP_READ_BUDGET_LINES=1000 makes the 400-line file pass" {
  AOP_READ_BUDGET_LINES=1000 run run_read "$WORK/big.txt" "b-raised"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  AOP_READ_BUDGET_LINES=1000 run run_bash "cat big.txt" "b-raised-bash"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "BUDGET: a malformed AOP_READ_BUDGET_LINES falls back to 350 (still fires, names 350)" {
  AOP_READ_BUDGET_LINES=lots run run_read "$WORK/big.txt" "b-malformed"
  [ "$status" -eq 2 ]
  [[ "$output" == *"(budget 350)"* ]]
  AOP_READ_BUDGET_LINES=0 run run_read "$WORK/big.txt" "b-zero"
  [ "$status" -eq 2 ]
  [[ "$output" == *"(budget 350)"* ]]
}

# --- FAIL OPEN ----------------------------------------------------------------

@test "FAIL-OPEN: malformed JSON '{' -> exit 0, silent" {
  run bash -c 'printf "{" | bash "$1"' _ "$GUARD"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "FAIL-OPEN: no jq on PATH (empty PATH dir, bash builtins only) -> exit 0, silent" {
  mkdir -p "$TMPDIR/emptybin"
  bashbin="$(command -v bash)"
  read_payload "$WORK/big.txt" "nojq" > "$TMPDIR/payload.json"
  run env PATH="$TMPDIR/emptybin" "$bashbin" "$GUARD" < "$TMPDIR/payload.json"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "FAIL-OPEN: empty stdin -> exit 0, silent" {
  run bash "$GUARD" < /dev/null
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# --- quote discipline: quoted text is never an invocation (HOOK-PARSING-1) ---

@test "SILENT: a commit message that mentions '; cat big.txt' is text, not a read" {
  run run_bash 'git commit -m "guard: block unbounded reads; cat big.txt now routes to bulk-reader"' "q1"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: single-quoted echo containing '; cat big.txt' is text" {
  run run_bash "echo 'x; cat big.txt y'" "q2"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: a segment that ends with a dangling quote ('echo \"x && cat big.txt \"') is text" {
  run run_bash 'echo "x && cat big.txt "' "q3"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: a multi-line commit body naming 'cat big.txt' on its own line is text" {
  run run_bash "$(printf 'git commit -m "wip\n\ncat big.txt is blocked by the guard"')" "q4"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: a quoted path with a space never mis-attributes to a coincidental sibling file" {
  seq 1 400 > "$WORK/big"            # the coincidental sibling the broken token would hit
  seq 1 400 > "$WORK/my big"
  run run_bash 'cat "my big"' "q5"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "FIRE: a fully quoted over-budget path still fires (balanced quotes are an invocation)" {
  run run_bash 'cat "big.txt"' "q6"
  [ "$status" -eq 2 ]
  [[ "$output" == *"$POLICY"* ]]
  run run_bash "cat 'big.txt'" "q7"
  [ "$status" -eq 2 ]
}

# --- tilde paths resolve against HOME (HOOK-PARSING-3) -----------------------

@test "FIRE: 'cat ~/big.txt' resolves the tilde against HOME" {
  seq 1 400 > "$HOME/big.txt"
  run run_bash 'cat ~/big.txt' "t1"
  [ "$status" -eq 2 ]
  [[ "$output" == *"$HOME/big.txt is 400 lines"* ]]
}

@test "SILENT: 'cat ~/missing.txt' (tilde, no such file) passes" {
  run run_bash 'cat ~/missing.txt' "t2"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# --- the budget hint names the hook env, and a budget prefix is NOT honored ---

@test "FIRE: 'AOP_READ_BUDGET_LINES=1000 cat big.txt' as a command prefix still fires (no uncounted self-relax)" {
  run run_bash 'AOP_READ_BUDGET_LINES=1000 cat big.txt' "b1"
  [ "$status" -eq 2 ]
  [[ "$output" == *"in the hook env (an operator setting, not a command prefix)"* ]]
}

# --- quote-AWARE split: the separator must be outside quotes -------------------

@test "SILENT: a 'cat big.txt' sandwiched between separators INSIDE a quoted string is text" {
  run run_bash 'git commit -m "fix; cat big.txt; routes"' "q8"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  run run_bash 'git commit -m "fix && cat big.txt && routes"' "q9"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: a multi-line quoted commit body with 'cat big.txt' as a middle line is text" {
  run run_bash "$(printf 'git commit -m "subject\n\ncat big.txt on its own line\n"')" "q10"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  run run_bash "$(printf 'git commit -m "subject\ncat big.txt\n" -m "trailer"')" "q11"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: escaped quotes are not quotes, but a word with a stray quote is unparseable" {
  run run_bash 'echo \"x; cat big.txt\"' "q12"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "FIRE: a quoted assignment prefix does not hide the read (LC_ALL=\"C\" cat big.txt)" {
  run run_bash 'LC_ALL="C" cat big.txt' "q13"
  [ "$status" -eq 2 ]
  run run_bash "GIT_PAGER='' cat big.txt" "q14"
  [ "$status" -eq 2 ]
}

@test "FIRE: a trailing comment with an apostrophe does not hide the read (cat big.txt # don't)" {
  run run_bash "cat big.txt # don't" "q15"
  [ "$status" -eq 2 ]
}

@test "FIRE: a quoted apostrophe in an earlier segment does not hide a later read" {
  run run_bash "echo \"it's\"; cat big.txt" "q16"
  [ "$status" -eq 2 ]
}

@test "FIRE: 'head -n \"500\" big.txt' (quoted count) still fires" {
  run run_bash 'head -n "500" big.txt' "q17"
  [ "$status" -eq 2 ]
}

@test "WAIVE: a QUOTED inline waiver value still waives silently" {
  run run_bash 'AOP_WAIVE="core.context:unbounded-read" cat big.txt' "q18"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# --- comments and continuations in the quote-aware split ----------------------

@test "SILENT: a separator inside a trailing comment never splits ('ls # step 1; cat big.txt for the log')" {
  run run_bash 'ls # step 1; cat big.txt for the log' "c1"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  run run_bash 'echo hi # then && cat big.txt' "c2"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "FIRE: a quote inside a comment line does not swallow the read on the next line" {
  run run_bash "$(printf "# Don't rerun the build\ncat big.txt")" "c3"
  [ "$status" -eq 2 ]
  run run_bash "$(printf 'echo hi # "note\ncat big.txt')" "c4"
  [ "$status" -eq 2 ]
}

@test "FIRE: a backslash-newline continuation is one command ('cat \\<newline>big.txt')" {
  run run_bash "$(printf 'cat \\\n  big.txt')" "c5"
  [ "$status" -eq 2 ]
  run run_bash "$(printf 'head -n 500 \\\n  big.txt')" "c6"
  [ "$status" -eq 2 ]
}

@test "FIRE: 'head --lines=\"500\" big.txt' (quoted long-option count) fires" {
  run run_bash 'head --lines="500" big.txt' "c7"
  [ "$status" -eq 2 ]
}

@test "FAIL-OPEN: jq present but no awk on PATH -> Bash judging is skipped silently (exit 0)" {
  mkdir -p "$TMPDIR/bin"
  for t in bash jq cat head tr wc sed cut date sha256sum mkdir dirname; do
    p="$(command -v "$t")" && ln -s "$p" "$TMPDIR/bin/$t"
  done
  run env PATH="$TMPDIR/bin" bash -c 'printf "%s" "$1" | bash "$2"' _ "$(bash_payload 'cat big.txt' "c8")" "$GUARD"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "FIRE: a MID-WORD backslash-newline joins without a space ('cat big\\<newline>.txt')" {
  run run_bash "$(printf 'cat big\\\n.txt')" "c9"
  [ "$status" -eq 2 ]
}

@test "SILENT: a '#' right after a close-paren starts a comment ('(true)# ; cat big.txt')" {
  run run_bash '(true)# ; cat big.txt' "c10"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}
