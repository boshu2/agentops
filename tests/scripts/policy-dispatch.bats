#!/usr/bin/env bats
# age-bhsz + age-wnyt — contract for the PreToolUse policy dispatcher and the
# day-1 enforce cohort (git-add-_beads, ledger hand-append, cp-into-skills).
#
# GWT from the bead: GIVEN a registered deny policy and a matching tool call,
# WHEN the dispatcher runs, THEN the call blocks (exit 2) with the routing
# message and a telemetry line records it; GIVEN audit mode, THEN the call
# proceeds and only the events record.
#
# Fixture fidelity: every case round-trips the REAL PreToolUse JSON input shape
# (tool_name / tool_input / session_id) built with jq — never a hand-built
# string — matching the harness contract in skills/cc-hooks/references/HOOK-EVENTS.md.

DISPATCH="${DISPATCH:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/policy-dispatch.sh}"
LINT="${LINT:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/scripts/lint-policies.sh}"
REGISTRY="${REGISTRY:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/policies/policies.json}"

setup() {
  export TMPDIR="$(mktemp -d)"
  export AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/telemetry.jsonl"
  export AOP_POLICIES="$REGISTRY"
  export AOP_WAIVER_FILE="$TMPDIR/waivers"
  unset AOP_WAIVE || true
}
teardown() { rm -rf "$TMPDIR"; }

# $1 = tool_name, $2 = field (command|file_path), $3 = value, $4 = session id
run_dispatch() {
  jq -nc --arg tool "$1" --arg k "$2" --arg v "$3" \
    --arg s "${4:-sess-$RANDOM-$BATS_TEST_NUMBER}" \
    '{tool_name:$tool, tool_input:{($k):$v}, session_id:$s}' \
    | bash "$DISPATCH"
}

telemetry_lines() {
  [ -f "$AGENTOPS_GUARDRAIL_TELEMETRY" ] || { echo 0; return; }
  wc -l < "$AGENTOPS_GUARDRAIL_TELEMETRY" | tr -d ' '
}

# ---------- registry hygiene ------------------------------------------------

@test "lint: shipped registry passes the v2 contract" {
  run bash "$LINT" "$REGISTRY"
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
}

@test "lint: a non-pure predicate in deny mode is rejected (predicate discipline)" {
  bad="$TMPDIR/bad.json"
  jq '.policies[0].predicate_class = "stateful"' "$REGISTRY" > "$bad"
  run bash "$LINT" "$bad"
  [ "$status" -eq 1 ]
  [[ "$output" == *"predicate discipline"* ]]
}

# ---------- policy (a): core.git:add-beads-ledger ---------------------------

@test "FIRE deny: git add _beads/issues.jsonl blocks with route message + telemetry" {
  run run_dispatch Bash command "git add _beads/issues.jsonl"
  [ "$status" -eq 2 ]
  [[ "$output" == *"core.git:add-beads-ledger"* ]]
  [[ "$output" == *"pushing the ledger repo itself"* ]]
  [ "$(telemetry_lines)" -eq 1 ]
  run jq -r '.token_class + " " + .decision' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "core.git:add-beads-ledger deny" ]
}

@test "FIRE deny: chained 'cd x && git add _beads' still blocks" {
  run run_dispatch Bash command "cd /tmp/x && git add _beads"
  [ "$status" -eq 2 ]
}

@test "SILENT: git add of a normal path — exit 0, zero output, zero telemetry" {
  run run_dispatch Bash command "git add docs/research/notes.md"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  [ "$(telemetry_lines)" -eq 0 ]
}

@test "SILENT: _beads mentioned outside a git add segment does not fire" {
  run run_dispatch Bash command "grep -r _beads docs/ && git add docs/a.md"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: git add of a file merely containing the substring (my_beadsfile) does not fire" {
  run run_dispatch Bash command "git add src/my_beadsfile.go"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ---------- policy (b): core.provenance:ledger-hand-append ------------------

@test "FIRE deny: >> redirect onto the provenance ledger blocks, routes to provenance add" {
  run run_dispatch Bash command "echo '{}' >> docs/provenance/ledger.jsonl"
  [ "$status" -eq 2 ]
  [[ "$output" == *"ao provenance add"* ]]
  [ "$(telemetry_lines)" -eq 1 ]
}

@test "FIRE deny: tee -a onto the provenance ledger blocks" {
  run run_dispatch Bash command "some-cmd | tee -a docs/provenance/ledger.jsonl"
  [ "$status" -eq 2 ]
}

@test "FIRE deny: Write tool targeting the ledger file_path blocks" {
  run run_dispatch Write file_path "docs/provenance/ledger.jsonl"
  [ "$status" -eq 2 ]
  [[ "$output" == *"core.provenance:ledger-hand-append"* ]]
}

@test "SILENT: reading the ledger (jq, no redirect) does not fire" {
  run run_dispatch Bash command "jq -r '.hash' docs/provenance/ledger.jsonl | tail -1"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "SILENT: the owning append command itself does not fire" {
  run run_dispatch Bash command "./cli/bin/ao provenance add decision-1 artifact-2 --relation wasGeneratedBy"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ---------- policy (c): core.skills:copy-into-installed ---------------------

@test "FIRE deny: cp -r into ~/.claude/skills blocks, routes to ao skills link" {
  run run_dispatch Bash command "cp -r skills/foo $HOME/.claude/skills/"
  [ "$status" -eq 2 ]
  [[ "$output" == *"ao skills link"* ]]
}

@test "FIRE deny: rsync into ~/.codex/skills/foo blocks" {
  run run_dispatch Bash command "rsync -a build/ $HOME/.codex/skills/foo"
  [ "$status" -eq 2 ]
}

# ---------- policy (d): core.skills:edit-installed-copy ---------------------

@test "FIRE deny: Edit of an installed skill copy blocks, routes to repo skills/" {
  run run_dispatch Edit file_path "$HOME/.claude/skills/evolve/SKILL.md"
  [ "$status" -eq 2 ]
  [[ "$output" == *"core.skills:edit-installed-copy"* ]]
  [[ "$output" == *"INSTALLED skill copy"* ]]
}

@test "SILENT: Edit of a repo skills/ source path does not fire" {
  run run_dispatch Edit file_path "/Users/dev/agentops/skills/cc-hooks/SKILL.md"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ---------- plugin layout -----------------------------------------------------

@test "plugin layout: dispatcher resolves ../policies/policies.json without AOP_POLICIES" {
  plugin="$TMPDIR/plugin/skills/cc-hooks"
  mkdir -p "$plugin"
  cp -R "$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks" "$plugin/hooks"
  cp -R "$BATS_TEST_DIRNAME/../../skills/cc-hooks/policies" "$plugin/policies"
  run bash -c 'unset AOP_POLICIES; jq -nc "{tool_name:\"Bash\", tool_input:{command:\"git add _beads/x\"}, session_id:\"plugin-sess\"}" | bash "$1/hooks/policy-dispatch.sh"' _ "$plugin"
  [ "$status" -eq 2 ]
  [[ "$output" == *"core.git:add-beads-ledger"* ]]
}

@test "SILENT: copying FROM an installed skills dir OUT to the repo does not fire" {
  run run_dispatch Bash command "cp $HOME/.claude/skills/foo/SKILL.md /tmp/inspect.md"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

# ---------- dispatcher mechanics -------------------------------------------

@test "deny on second attempt in the same session STILL blocks (short message)" {
  sid="same-session-$BATS_TEST_NUMBER"
  run run_dispatch Bash command "git add _beads/x" "$sid"
  [ "$status" -eq 2 ]
  run run_dispatch Bash command "git add _beads/x" "$sid"
  [ "$status" -eq 2 ]
  [[ "$output" == *"blocked"* ]]
}

@test "waiver via AOP_WAIVE allows the call and records decision=waived" {
  AOP_WAIVE="core.git:add-beads-ledger" run run_dispatch Bash command "git add _beads/x"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  run jq -r '.decision' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "waived" ]
}

@test "waiver file with unexpired entry allows the call" {
  echo "core.git:add-beads-ledger $(( $(date +%s) + 3600 ))" > "$AOP_WAIVER_FILE"
  run run_dispatch Bash command "git add _beads/x"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "waiver file with EXPIRED entry still blocks" {
  echo "core.git:add-beads-ledger $(( $(date +%s) - 10 ))" > "$AOP_WAIVER_FILE"
  run run_dispatch Bash command "git add _beads/x"
  [ "$status" -eq 2 ]
}

@test "audit mode: call proceeds silently and only the telemetry records" {
  audited="$TMPDIR/audited.json"
  jq '.policies |= map(if .id == "core.git:add-beads-ledger" then .mode = "audit" else . end)' \
    "$REGISTRY" > "$audited"
  AOP_POLICIES="$audited" run run_dispatch Bash command "git add _beads/x"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
  run jq -r '.decision' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "audit" ]
}

@test "route mode: exit 0 with permissionDecision ask JSON on stdout" {
  routed="$TMPDIR/routed.json"
  jq '.policies |= map(if .id == "core.git:add-beads-ledger" then .mode = "route" else . end)' \
    "$REGISTRY" > "$routed"
  AOP_POLICIES="$routed" run run_dispatch Bash command "git add _beads/x"
  [ "$status" -eq 0 ]
  echo "$output" | jq -e '.hookSpecificOutput.permissionDecision == "ask"'
}

@test "stray-stdout hazard: every exit-0 path emits NOTHING on stdout (non-route)" {
  for cmd in "ls -la" "git status" "echo hello"; do
    run run_dispatch Bash command "$cmd"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
  done
}

@test "unmatched tool (Read) is silent even with a matching-looking input" {
  run run_dispatch Read command "git add _beads/x"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "missing registry fails OPEN (exit 0, silent)" {
  AOP_POLICIES="$TMPDIR/does-not-exist.json" run run_dispatch Bash command "git add _beads/x"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "telemetry hashes the matched value — raw command never lands in the file" {
  run run_dispatch Bash command "git add _beads/issues.jsonl"
  [ "$status" -eq 2 ]
  ! grep -q "issues.jsonl" "$AGENTOPS_GUARDRAIL_TELEMETRY"
  run jq -r '.path_sha256 | length' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "64" ]
}
