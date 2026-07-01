#!/usr/bin/env bash
set -euo pipefail

# Shared fail-closed codex runner (STALL/ECHO/MISSING defenses + distinct exit
# codes). age-gate-the-ungated-egwt.8. `CDPATH=` is an intentional env-prefix
# (clears CDPATH for that one cd), not a botched assignment.
# shellcheck disable=SC1007
. "$(CDPATH= cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/codex-exec.sh"

EXPECTED_CSV="using-agentops,swarm,research"
WORKDIR="/tmp"
PROFILE="${CODEX_VALIDATE_PROFILE:-safe}"

usage() {
  cat <<'EOF'
validate-codex-cli-skills.sh

Open a fresh non-interactive Codex session and verify that expected AgentOps
skills are visible to the runtime.

Options:
  --expected <a,b,c>  Comma-separated skill names to require
  --workdir <dir>     Working directory for the ephemeral Codex session
  --profile <name>    Codex profile to use (default: safe)
  --help              Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --expected)
      EXPECTED_CSV="${2:-}"
      shift 2
      ;;
    --workdir)
      WORKDIR="${2:-}"
      shift 2
      ;;
    --profile)
      PROFILE="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

command -v codex >/dev/null 2>&1 || {
  echo "codex CLI not found in PATH" >&2
  exit 1
}

mkdir -p "$WORKDIR"
OUTPUT_FILE="$(mktemp)"
cleanup() {
  rm -f "$OUTPUT_FILE"
}
trap cleanup EXIT

PROMPT="List the available skill names you can see in this session. Return only a comma-separated list."
# The runner reads CODEX_EXEC_EXTRA_ARGS as an ARRAY from the current shell scope
# (it runs as a sourced function, not a subprocess) — a bash array cannot be set
# via a `VAR=... cmd` command-prefix, so assign it here before the call.
# shellcheck disable=SC2034  # consumed by codex_exec_guarded in lib/codex-exec.sh (sourced).
CODEX_EXEC_EXTRA_ARGS=(--profile "$PROFILE" --json)
if ! CODEX_EXEC_SANDBOX=read-only CODEX_EXEC_SKIP_GIT_CHECK=1 \
  CODEX_EXEC_PROMPT_ARG="$PROMPT" CODEX_EXEC_OUT_FILE="$OUTPUT_FILE" \
  codex_exec_guarded; then
  echo "headless codex run failed while checking skill discovery" >&2
  exit 1
fi

python3 - "$OUTPUT_FILE" "$EXPECTED_CSV" <<'PY'
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
expected = [item.strip() for item in sys.argv[2].split(",") if item.strip()]
messages = []
for line in path.read_text().splitlines():
    line = line.strip()
    if not line:
        continue
    try:
        payload = json.loads(line)
    except json.JSONDecodeError:
        continue
    item = payload.get("item")
    if payload.get("type") == "item.completed" and isinstance(item, dict) and item.get("type") == "agent_message":
        text = item.get("text", "").strip()
        if text:
            messages.append(text)

if not messages:
    print("No agent message found in headless codex output", file=sys.stderr)
    sys.exit(1)

visible = {part.strip() for part in messages[-1].split(",") if part.strip()}
missing = [name for name in expected if name not in visible]
if missing:
    print("Missing expected skills: " + ", ".join(missing), file=sys.stderr)
    print("Visible skills: " + ", ".join(sorted(visible)), file=sys.stderr)
    sys.exit(1)

print("Visible skills: " + ", ".join(sorted(visible)))
print("Required skills present: " + ", ".join(expected))
PY
