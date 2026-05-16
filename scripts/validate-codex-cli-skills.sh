#!/usr/bin/env bash
set -euo pipefail

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
if ! codex exec \
  --skip-git-repo-check \
  --sandbox read-only \
  --profile "$PROFILE" \
  --json \
  "$PROMPT" >"$OUTPUT_FILE"; then
  echo "codex exec failed while checking skill discovery" >&2
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
    print("No agent message found in codex exec output", file=sys.stderr)
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
