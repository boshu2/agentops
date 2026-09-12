#!/usr/bin/env bats
# Value-proof telemetry contract for skills/cc-hooks/hooks/read-budget-guard.sh.
#
# The guard emits EXACTLY one gate-blind JSONL line per FIRE and per WAIVED
# call (none on pass / disabled / fail-open):
#   {ts, session, token_class, path_sha256, mode, decision, tool, lines, budget}
# PRIVACY: neither the raw path nor the raw command is ever persisted — only a
# SHA-256 of the RESOLVED offending path. lines and budget are JSON numbers so
# sum(lines) over fires is a stated-denominator estimate of lines kept out of
# context (see references/GUARDRAIL-VALUE-PROOF.md). Telemetry is inert until
# the guard fires, and the guard ships inert / opt-in.
#
# Fixture fidelity: every case round-trips the REAL PreToolUse JSON input built
# with jq, with TMPDIR / HOME isolated and the ledger pointed into TMPDIR via
# AGENTOPS_GUARDRAIL_TELEMETRY.

GUARD="${GUARD:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/read-budget-guard.sh}"
POLICY="core.context:unbounded-read"

setup() {
  export TMPDIR="$(mktemp -d)"
  export HOME="$TMPDIR/home"
  mkdir -p "$HOME"
  export AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/telemetry.jsonl"
  export AOP_WAIVER_FILE="$TMPDIR/waivers"
  unset AOP_WAIVE AOP_READ_BUDGET_LINES AGENTOPS_HOOKS_DISABLED || true

  WORK="$TMPDIR/work"
  mkdir -p "$WORK"
  seq 1 400 > "$WORK/big-secret-name.txt"
  seq 1 100 > "$WORK/small.txt"
  seq 1 200 > "$WORK/a.txt"
  seq 1 200 > "$WORK/b.txt"
}
teardown() { rm -rf "$TMPDIR"; }

read_payload() {
  jq -nc --arg p "$1" --arg s "$2" --arg c "$WORK" \
    '{tool_name:"Read", tool_input:{file_path:$p}, session_id:$s, cwd:$c}'
}
bash_payload() {
  jq -nc --arg c "$1" --arg s "$2" --arg d "$WORK" \
    '{tool_name:"Bash", tool_input:{command:$c}, session_id:$s, cwd:$d}'
}
run_read() { read_payload "$@" | bash "$GUARD"; }
run_bash() { bash_payload "$@" | bash "$GUARD"; }

telemetry_lines() {
  [ -f "$AGENTOPS_GUARDRAIL_TELEMETRY" ] || { echo 0; return; }
  wc -l < "$AGENTOPS_GUARDRAIL_TELEMETRY" | tr -d ' '
}

# The SHA-256 the guard would store for a path, mirroring its hasher order.
expected_hash() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s' "$1" | sha256sum | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    printf '%s' "$1" | shasum -a 256 | cut -d' ' -f1
  else
    printf '%s' "$1" | openssl dgst -sha256 | sed 's/^.*= *//'
  fi
}

# --- one well-formed line per fire -------------------------------------------

@test "telemetry: a Read fire appends EXACTLY one JSONL line" {
  run run_read "$WORK/big-secret-name.txt" "t1"
  [ "$status" -eq 2 ]
  [ "$(telemetry_lines)" -eq 1 ]
}

@test "telemetry: the line is valid JSON carrying every contract field" {
  run run_read "$WORK/big-secret-name.txt" "t2"
  run jq -e '.ts and .session and .token_class and .path_sha256 and .mode and .decision and .tool and (.lines != null) and (.budget != null)' \
    "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -eq 0 ]
  run jq -r '.session + " " + .token_class + " " + .mode + " " + .decision + " " + .tool' \
    "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "t2 $POLICY deny deny Read" ]
  run jq -r '.ts' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [[ "$output" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]
}

@test "telemetry: lines and budget are JSON numbers with the observed values" {
  run run_read "$WORK/big-secret-name.txt" "t3"
  run jq -r '(.lines|type) + " " + (.budget|type)' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "number number" ]
  run jq -r '"\(.lines) \(.budget)"' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "400 350" ]
}

@test "telemetry: a raised budget is recorded as the number in force" {
  AOP_READ_BUDGET_LINES=399 run run_read "$WORK/big-secret-name.txt" "t3b"
  [ "$status" -eq 2 ]
  run jq -r '.budget' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "399" ]
}

@test "telemetry: a Bash cat-sum fire records tool Bash and lines = the total" {
  run run_bash "cat a.txt b.txt" "t4"
  [ "$status" -eq 2 ]
  [ "$(telemetry_lines)" -eq 1 ]
  run jq -r '.tool + " " + (.lines|tostring)' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "Bash 400" ]
}

# --- PRIVACY: hash of the RESOLVED path only ----------------------------------

@test "telemetry: path_sha256 equals the hash of the resolved path and is 64 hex chars" {
  local p="$WORK/big-secret-name.txt"
  run run_read "$p" "t5"
  run jq -r '.path_sha256' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "$(expected_hash "$p")" ]
  [[ "$output" =~ ^[0-9a-f]{64}$ ]]
}

@test "telemetry: a relative Bash path is hashed as its cwd-RESOLVED form" {
  run run_bash "cat big-secret-name.txt" "t6"
  [ "$status" -eq 2 ]
  run jq -r '.path_sha256' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "$(expected_hash "$WORK/big-secret-name.txt")" ]
}

@test "telemetry: the raw path and the raw command NEVER appear in the ledger" {
  run run_read "$WORK/big-secret-name.txt" "t7"
  run run_bash "cat -n big-secret-name.txt" "t7"
  [ "$(telemetry_lines)" -eq 2 ]
  run grep -F "big-secret-name" "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -ne 0 ]
  run grep -F "cat -n" "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -ne 0 ]
  run grep -F "$WORK" "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -ne 0 ]
}

# --- happy path / disabled write nothing ------------------------------------

@test "telemetry: the happy path (bounded / small / other command) writes NOTHING" {
  run run_read "$WORK/small.txt" "h1"
  [ "$status" -eq 0 ]
  run run_bash "head -n 20 big-secret-name.txt" "h2"
  [ "$status" -eq 0 ]
  run run_bash "git status" "h3"
  [ "$status" -eq 0 ]
  [ ! -f "$AGENTOPS_GUARDRAIL_TELEMETRY" ]
}

@test "telemetry: AGENTOPS_HOOKS_DISABLED=1 writes NOTHING" {
  AGENTOPS_HOOKS_DISABLED=1 run run_read "$WORK/big-secret-name.txt" "d1"
  [ "$status" -eq 0 ]
  [ ! -f "$AGENTOPS_GUARDRAIL_TELEMETRY" ]
}

@test "telemetry: fail-open (malformed JSON) writes NOTHING" {
  run bash -c 'printf "{" | bash "$1"' _ "$GUARD"
  [ "$status" -eq 0 ]
  [ ! -f "$AGENTOPS_GUARDRAIL_TELEMETRY" ]
}

# --- waived / repeated fires ---------------------------------------------------

@test "telemetry: a waived call writes ONE line with decision waived (env and inline)" {
  AOP_WAIVE="$POLICY" run run_read "$WORK/big-secret-name.txt" "w1"
  [ "$status" -eq 0 ]
  [ "$(telemetry_lines)" -eq 1 ]
  run jq -r '.decision + " " + .mode + " " + .tool' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "waived deny Read" ]
  run run_bash "AOP_WAIVE=$POLICY cat big-secret-name.txt" "w2"
  [ "$status" -eq 0 ]
  [ "$(telemetry_lines)" -eq 2 ]
  run jq -r '.decision' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = $'waived\nwaived' ]
}

@test "telemetry: two fires in one session write TWO lines (every attempt is counted)" {
  run run_read "$WORK/big-secret-name.txt" "same"
  [ "$status" -eq 2 ]
  run run_read "$WORK/big-secret-name.txt" "same"
  [ "$status" -eq 2 ]
  [ "$(telemetry_lines)" -eq 2 ]
  run jq -r '.decision' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = $'deny\ndeny' ]
}

@test "telemetry: telemetry failure never changes the exit decision" {
  # Point the ledger at a path that cannot be created (a file where a dir is needed).
  : > "$TMPDIR/not-a-dir"
  AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/not-a-dir/telemetry.jsonl" run run_read "$WORK/big-secret-name.txt" "tf"
  [ "$status" -eq 2 ]
  [[ "$output" == *"policy $POLICY"* ]]
}

# --- an unwritable ledger never leaks onto an exit-0 path (HOOK-PARSING-2) ---

@test "telemetry: waived call with an UNWRITABLE ledger is exit 0 and fully silent" {
  mkdir -p "$TMPDIR/ledger-is-a-dir"
  export AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/ledger-is-a-dir"
  export AOP_WAIVE="$POLICY"
  run run_read "$WORK/big-secret-name.txt" "u1"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "telemetry: a fire with an UNWRITABLE ledger still blocks and the first stderr line is the policy line" {
  mkdir -p "$TMPDIR/ledger-is-a-dir"
  export AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/ledger-is-a-dir"
  run run_read "$WORK/big-secret-name.txt" "u2"
  [ "$status" -eq 2 ]
  [[ "${lines[0]}" == "⛔ policy $POLICY" ]]
}

@test "telemetry: a QUOTED inline waiver writes its 'waived' line (countermetric kept)" {
  run run_bash 'AOP_WAIVE="core.context:unbounded-read" cat big-secret-name.txt' "w9"
  [ "$status" -eq 0 ]
  run jq -r '.decision' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "waived" ]
}

@test "telemetry: no HOME and no AGENTOPS_* location writes nothing (never anchors at /)" {
  unset AGENTOPS_GUARDRAIL_TELEMETRY AGENTOPS_HOME
  run env -u HOME -u AGENTOPS_GUARDRAIL_TELEMETRY -u AGENTOPS_HOME bash -c \
    'printf "%s" "$1" | bash "$2"' _ "$(read_payload "$WORK/big-secret-name.txt" "w10")" "$GUARD"
  [ "$status" -eq 2 ]
  [ ! -e /.agents/ao/guardrail-telemetry.jsonl ] || ! grep -q '"session":"w10"' /.agents/ao/guardrail-telemetry.jsonl
}
