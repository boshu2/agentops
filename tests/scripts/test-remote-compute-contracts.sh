#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS_COUNT=0
FAIL_COUNT=0

pass() {
  echo "PASS: $1"
  PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
  echo "FAIL: $1"
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

assert_file() {
  local path="$1"
  local description="$2"

  if [[ -f "$REPO_ROOT/$path" ]]; then
    pass "$description"
  else
    fail "$description (missing $path)"
  fi
}

assert_json() {
  local path="$1"
  local description="$2"

  if python3 -m json.tool "$REPO_ROOT/$path" >/dev/null; then
    pass "$description"
  else
    fail "$description (invalid JSON: $path)"
  fi
}

assert_contains() {
  local path="$1"
  local needle="$2"
  local description="$3"

  if grep -Fq "$needle" "$REPO_ROOT/$path"; then
    pass "$description"
  else
    fail "$description (missing '$needle' in $path)"
  fi
}

assert_not_contains_regex() {
  local path="$1"
  local pattern="$2"
  local description="$3"

  if rg -q "$pattern" "$REPO_ROOT/$path"; then
    fail "$description (unexpected match '$pattern' in $path)"
  else
    pass "$description"
  fi
}

assert_no_public_bushido_cli() {
  local bushido_file="$REPO_ROOT/cli/cmd/ao/bushido.go"

  if [[ -e "$bushido_file" ]]; then
    fail "public Bushido CLI command file is absent"
  else
    pass "public Bushido CLI command file is absent"
  fi

  assert_not_contains_regex \
    "cli/docs/COMMANDS.md" \
    '### `ao bushido`|#### `ao bushido`' \
    "generated CLI docs do not expose an ao bushido command family"
}

assert_schema_invariants() {
  if python3 - "$REPO_ROOT" <<'PY'; then
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
errors = []

def load(rel):
    with (root / rel).open(encoding="utf-8") as handle:
        return json.load(handle)

def require(condition, message):
    if not condition:
        errors.append(message)

target = load("schemas/remote-compute-target.schema.json")
event = load("schemas/remote-session-event.schema.json")

target_required = set(target.get("required", []))
require(
    {
        "schema_version",
        "target_id",
        "provider",
        "gascity",
        "bootstrap_transport",
        "bootstrap_profile",
        "auth_ref",
        "capabilities",
        "redaction",
    }.issubset(target_required),
    "target schema must require identity, GasCity, bootstrap, auth, capabilities, and redaction fields",
)
require(target.get("additionalProperties") is False, "target schema must reject additional root properties")
require(target["properties"]["provider"].get("enum") == ["gascity"], "target provider must be GasCity only")
require(
    set(target["properties"]["bootstrap_transport"].get("enum", [])) == {"ssh", "local", "manual", "none"},
    "bootstrap_transport enum must be ssh/local/manual/none",
)
require(
    set(target["properties"]["gascity"].get("required", [])) == {"endpoint", "city"},
    "gascity object must require endpoint and city",
)
require(
    set(target["properties"]["capabilities"].get("required", []))
    == {"api_sse", "sessions", "transcripts", "artifacts", "cancel", "provider_readiness", "context_sync"},
    "capabilities must cover API/SSE, sessions, transcripts, artifacts, cancel, readiness, and context sync",
)
require(
    target["properties"]["redaction"]["properties"]["omit_secrets"].get("const") is True,
    "redaction.omit_secrets must be const true",
)

event_required = set(event.get("required", []))
require(
    {
        "schema_version",
        "event_id",
        "occurred_at",
        "event_type",
        "session_id",
        "provider",
        "target",
        "event_cursor",
        "terminal_status",
        "command_id",
        "idempotency_key",
        "artifact_refs",
    }.issubset(event_required),
    "event schema must require durable session, target, cursor, terminal, command, and artifact fields",
)
require(event.get("additionalProperties") is False, "event schema must reject additional root properties")
require(event["properties"]["provider"].get("enum") == ["gascity"], "event provider must be GasCity only")
require(
    {
        "session_recorded",
        "session_started",
        "command_recorded",
        "command_delivery_attempted",
        "command_accepted",
        "command_rejected",
        "command_delivery_unknown",
        "event_cursor_advanced",
        "transcript_ref",
        "artifact_ref",
        "terminal_state",
    }.issubset(set(event["properties"]["event_type"].get("enum", []))),
    "event_type enum must cover session, command, cursor, transcript, artifact, and terminal events",
)

terminal_variants = event["properties"]["terminal_status"]["oneOf"][1]["enum"]
require(
    set(terminal_variants) == {"completed", "failed", "cancelled", "lost", "provider_unreachable", "unknown"},
    "terminal_status enum must preserve failure and unknown classifications",
)
command_variants = event["properties"]["command_status"]["oneOf"][1]["enum"]
require(
    set(command_variants) == {"recorded", "delivery_attempted", "accepted", "rejected", "delivery_unknown", "superseded"},
    "command_status enum must include delivery_unknown instead of requiring blind resend",
)

if errors:
    for error in errors:
        print(f"FAIL: {error}", file=sys.stderr)
    raise SystemExit(1)
PY
    pass "remote compute schemas preserve required invariants"
  else
    fail "remote compute schemas preserve required invariants"
  fi
}

echo "================================"
echo "Testing remote compute contracts"
echo "================================"
echo ""

assert_file "docs/contracts/remote-compute.md" "remote compute contract exists"
assert_file "docs/contracts/agent-worker.md" "agent worker contract exists"
assert_file "docs/contracts/gascity-integration.md" "gascity integration contract exists"
assert_file "schemas/remote-compute-target.schema.json" "remote compute target schema exists"
assert_file "schemas/remote-session-event.schema.json" "remote session event schema exists"

assert_json "schemas/remote-compute-target.schema.json" "remote compute target schema is valid JSON"
assert_json "schemas/remote-session-event.schema.json" "remote session event schema is valid JSON"
assert_schema_invariants

assert_contains "docs/contracts/remote-compute.md" "## GasCity First" "remote compute contract is GasCity-first"
assert_contains "docs/contracts/remote-compute.md" "SSH/tmux is bootstrap and rescue only" "remote compute contract keeps SSH/tmux as bootstrap and rescue"
assert_contains "docs/contracts/remote-compute.md" "delivery_unknown" "remote compute contract records delivery_unknown recovery"
assert_contains "docs/contracts/remote-compute.md" "bootstrap_transport" "remote compute contract names bootstrap_transport"
assert_contains "docs/contracts/remote-compute.md" "RemoteTarget" "remote compute contract names RemoteTarget"
assert_contains "docs/contracts/remote-compute.md" "RemoteSession" "remote compute contract names RemoteSession"
assert_contains "docs/contracts/remote-compute.md" "RemoteCommand" "remote compute contract names RemoteCommand"
assert_contains "docs/contracts/remote-compute.md" "RemoteNode" "remote compute contract names RemoteNode"
assert_contains "docs/contracts/remote-compute.md" "private dogfood target and soak harness" "remote compute contract keeps Bushido private"
assert_contains "docs/contracts/remote-compute.md" "create an \`ao bushido\` command family" "remote compute contract blocks public Bushido CLI namespace"

assert_contains "docs/contracts/gascity-integration.md" "## Remote Compute Usage" "GasCity contract has remote compute usage section"
assert_contains "docs/contracts/gascity-integration.md" "public GasCity API/SSE surface" "GasCity contract keeps product sessions on public API/SSE"
assert_contains "docs/contracts/gascity-integration.md" "delivery_unknown" "GasCity contract preserves delivery_unknown recovery"
assert_contains "docs/contracts/gascity-integration.md" "bootstrap_transport" "GasCity contract names bootstrap_transport"

assert_contains "docs/contracts/agent-worker.md" "Remote compute uses the same" "AgentWorker contract aligns remote compute with AgentWorker"
assert_contains "docs/contracts/agent-worker.md" "idempotency_key" "AgentWorker contract records command idempotency key"
assert_contains "docs/contracts/agent-worker.md" "unknown" "AgentWorker contract allows unknown terminal classification"

assert_contains "docs/documentation-index.md" "[Remote Compute Contract](contracts/remote-compute.md)" "documentation index catalogs remote compute contract"
assert_contains "docs/documentation-index.md" "[Remote Compute Target Schema](https://github.com/boshu2/agentops/blob/main/schemas/remote-compute-target.schema.json)" "documentation index catalogs remote target schema"
assert_contains "docs/documentation-index.md" "[Remote Session Event Schema](https://github.com/boshu2/agentops/blob/main/schemas/remote-session-event.schema.json)" "documentation index catalogs remote session event schema"
assert_contains "docs/SCHEMAS.md" "remote-compute-target.schema.json" "schema index catalogs remote target schema"
assert_contains "docs/SCHEMAS.md" "remote-session-event.schema.json" "schema index catalogs remote session event schema"

assert_no_public_bushido_cli

echo ""
echo "================================"
echo "Results: $PASS_COUNT PASS, $FAIL_COUNT FAIL"
echo "================================"

if [[ $FAIL_COUNT -gt 0 ]]; then
  exit 1
fi

echo "PASS: remote compute contract tests"
exit 0
