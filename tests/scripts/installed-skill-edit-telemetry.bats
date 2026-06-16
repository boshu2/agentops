#!/usr/bin/env bats
# age-workflow-guardrail-hooks-j39.2 — value-proof telemetry contract for
# installed-skill-edit-guard.sh.
#
# The guard emits EXACTLY one gate-blind JSONL line per FIRE:
#   {ts, session, token_class, path_sha256}
# PRIVACY: the raw path is NEVER persisted — only its SHA-256 hash. The happy
# path emits nothing. Telemetry is inert until the guard fires (and the guard
# ships inert / opt-in).
#
# We round-trip the REAL PreToolUse JSON input on stdin (built with jq), and
# point the guard at an isolated telemetry file via AGENTOPS_GUARDRAIL_TELEMETRY,
# per the guard-test fixture-fidelity rule.

GUARD="${GUARD:-$BATS_TEST_DIRNAME/../../skills/cc-hooks/hooks/installed-skill-edit-guard.sh}"

setup() {
  export TMPDIR="$(mktemp -d)"
  export AGENTOPS_GUARDRAIL_TELEMETRY="$TMPDIR/telemetry.jsonl"
}
teardown() { rm -rf "$TMPDIR"; }

# $1 = file_path, $2 = session_id
run_guard() {
  jq -nc --arg p "$1" --arg s "${2:-sess-$RANDOM}" \
    '{tool_name:"Edit", tool_input:{file_path:$p}, session_id:$s}' \
    | bash "$GUARD"
}

# Compute the SHA-256 the guard would store for a path, using whatever hasher
# this host has — mirrors the guard's own preference order.
expected_hash() {
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s' "$1" | sha256sum | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    printf '%s' "$1" | shasum -a 256 | cut -d' ' -f1
  else
    printf '%s' "$1" | openssl dgst -sha256 | sed 's/^.*= *//'
  fi
}

# --- FIRE emits exactly one well-formed JSONL line ------------------------

@test "telemetry: a fire appends EXACTLY one JSONL line" {
  local p="$HOME/.claude/skills/evolve/SKILL.md"
  run run_guard "$p" "t1"
  [ "$status" -eq 2 ]
  [ -f "$AGENTOPS_GUARDRAIL_TELEMETRY" ]
  run wc -l < "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$(echo "$output" | tr -d '[:space:]')" -eq 1 ]
}

@test "telemetry: the line is valid JSON with the contract fields" {
  run run_guard "$HOME/.claude/skills/research/SKILL.md" "t2"
  run jq -e '.ts and .session and .token_class and .path_sha256' \
    "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -eq 0 ]
}

@test "telemetry: session and token_class carry the expected values" {
  run run_guard "$HOME/.claude/skills/plan/SKILL.md" "sess-XYZ"
  run jq -r '.session' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "sess-XYZ" ]
  run jq -r '.token_class' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "installed-skill-edit" ]
}

# --- PRIVACY: hash only, never the raw path ------------------------------

@test "telemetry: path_sha256 is the HASH of the path, not the raw path" {
  local p="$HOME/.claude/skills/secret-skill-name/SKILL.md"
  run run_guard "$p" "t3"
  # The hash field matches the computed hash...
  run jq -r '.path_sha256' "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$output" = "$(expected_hash "$p")" ]
  # ...and is 64 hex chars.
  [[ "$output" =~ ^[0-9a-f]{64}$ ]]
}

@test "telemetry: the raw path NEVER appears anywhere in the ledger (privacy)" {
  local p="$HOME/.claude/skills/super-private-path/SKILL.md"
  run run_guard "$p" "t4"
  run grep -F "super-private-path" "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -ne 0 ]    # grep finds nothing -> raw path not persisted
  run grep -F ".claude/skills" "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$status" -ne 0 ]
}

# --- HAPPY PATH emits nothing --------------------------------------------

@test "telemetry: repo skills/ edit (happy path) writes NO telemetry" {
  run run_guard "/Users/bo/dev/agentops/skills/evolve/SKILL.md" "h1"
  [ "$status" -eq 0 ]
  [ ! -f "$AGENTOPS_GUARDRAIL_TELEMETRY" ]
}

@test "telemetry: unrelated source file (happy path) writes NO telemetry" {
  run run_guard "/Users/bo/dev/agentops/cli/cmd/ao/main.go" "h2"
  [ "$status" -eq 0 ]
  [ ! -f "$AGENTOPS_GUARDRAIL_TELEMETRY" ]
}

# --- once-per-session: only the first fire emits -------------------------

@test "telemetry: once-per-session — only the first fire emits a line" {
  run run_guard "$HOME/.claude/skills/evolve/SKILL.md" "same"     # fires + emits
  run run_guard "$HOME/.claude/skills/research/SKILL.md" "same"   # self-relaxes, no emit
  run wc -l < "$AGENTOPS_GUARDRAIL_TELEMETRY"
  [ "$(echo "$output" | tr -d '[:space:]')" -eq 1 ]
}
