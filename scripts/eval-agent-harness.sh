#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WORKBENCH="$REPO_ROOT/evals/workbench"

usage() {
  cat <<'USAGE'
Usage: scripts/eval-agent-harness.sh --task <task-id> --agent <claude|codex> [options]

Run an agent against a workbench task and score the result.

Options:
  --task <id>          Task ID (e.g., go-01, py-04, ops-01)
  --agent <name>       Agent CLI to use: claude or codex
  --prompt <text>      Override the default prompt
  --timeout <secs>     Agent invocation timeout (default: 120)
  --hooks-disabled     Set AGENTOPS_HOOKS_DISABLED=1 for skill-off A/B leg
  --dry-run            Skip agent invocation, output synthetic result
  -h, --help           Show this help
USAGE
}

TASK_ID=""
AGENT=""
PROMPT=""
TIMEOUT=120
HOOKS_DISABLED=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --task) TASK_ID="$2"; shift 2 ;;
    --agent) AGENT="$2"; shift 2 ;;
    --prompt) PROMPT="$2"; shift 2 ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    --hooks-disabled) HOOKS_DISABLED=true; shift ;;
    --dry-run) DRY_RUN=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 1 ;;
  esac
done

[[ -n "$TASK_ID" ]] || { echo "error: --task required" >&2; exit 1; }
[[ -n "$AGENT" ]] || { echo "error: --agent required" >&2; exit 1; }

TASK_DIR="$WORKBENCH/tasks/$TASK_ID"
[[ -d "$TASK_DIR" ]] || { echo "error: task not found: $TASK_ID" >&2; exit 1; }
[[ -x "$TASK_DIR/setup.sh" ]] || { echo "error: setup.sh not found for $TASK_ID" >&2; exit 1; }
[[ -x "$TASK_DIR/score.sh" ]] || { echo "error: score.sh not found for $TASK_ID" >&2; exit 1; }

if [[ "$DRY_RUN" == "false" ]]; then
  if ! command -v "$AGENT" &>/dev/null; then
    echo "{\"score\": 0, \"total\": 0, \"pass\": false, \"skipped\": true, \"reason\": \"agent not found: $AGENT\"}"
    exit 0
  fi
fi

WORKSPACE="$(mktemp -d)"
trap 'rm -rf "$WORKSPACE"' EXIT

bash "$TASK_DIR/setup.sh" "$WORKSPACE" >/dev/null 2>&1

if [[ -z "$PROMPT" ]]; then
  PROMPT="You are in a software project at $WORKSPACE. There are failing tests or issues. Fix the code so all tests pass and the project builds cleanly. Do not explain — just fix the files."
fi

if [[ "$DRY_RUN" == "true" ]]; then
  echo "{\"score\": 0, \"total\": 1, \"pass\": false, \"skipped\": true, \"reason\": \"dry-run mode\"}"
  exit 0
fi

agent_env=()
if [[ "$HOOKS_DISABLED" == "true" ]]; then
  agent_env+=(AGENTOPS_HOOKS_DISABLED=1)
fi

case "$AGENT" in
  claude)
    env "${agent_env[@]}" timeout "$TIMEOUT" claude -p "$PROMPT" --allowedTools "Edit,Write,Read,Bash" 2>/dev/null || true
    ;;
  codex)
    env "${agent_env[@]}" timeout "$TIMEOUT" codex exec "$PROMPT" 2>/dev/null || true
    ;;
  *)
    echo "error: unsupported agent: $AGENT (use claude or codex)" >&2
    exit 1
    ;;
esac

bash "$TASK_DIR/score.sh" "$WORKSPACE"
