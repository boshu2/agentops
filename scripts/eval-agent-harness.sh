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
  --prompt <text>      Override the default prompt (ignores prompt.md)
  --generic-prompt     Force generic prompt even when prompt.md exists
  --timeout <secs>     Agent invocation timeout (default: 120)
  --runs <n>           Number of runs for pass@k tracking (default: 1)
  --compare            Run both with and without hooks (A/B mode)
  --hooks-disabled     Set AGENTOPS_HOOKS_DISABLED=1 for skill-off leg
  --retry              Give agent a second attempt with test failure output (Aider pattern)
  --dry-run            Skip agent invocation, output synthetic result
  -h, --help           Show this help
USAGE
}

TASK_ID=""
AGENT=""
PROMPT=""
GENERIC_PROMPT=false
TIMEOUT=120
RUNS=1
COMPARE=false
HOOKS_DISABLED=false
RETRY=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --task) TASK_ID="$2"; shift 2 ;;
    --agent) AGENT="$2"; shift 2 ;;
    --prompt) PROMPT="$2"; shift 2 ;;
    --generic-prompt) GENERIC_PROMPT=true; shift ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    --runs) RUNS="$2"; shift 2 ;;
    --compare) COMPARE=true; shift ;;
    --hooks-disabled) HOOKS_DISABLED=true; shift ;;
    --retry) RETRY=true; shift ;;
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

build_prompt() {
  local workspace="$1"

  if [[ -n "$PROMPT" ]]; then
    echo "$PROMPT"
    return
  fi

  if [[ "$GENERIC_PROMPT" == "false" && -f "$TASK_DIR/prompt.md" ]]; then
    local task_prompt
    task_prompt="$(cat "$TASK_DIR/prompt.md")"
    echo "You are in a software project at $workspace. $task_prompt"
  else
    echo "You are in a software project at $workspace. There are failing tests or issues. Fix the code so all tests pass and the project builds cleanly. Do not explain — just fix the files."
  fi
}

run_agent() {
  local workspace="$1"
  local hooks_off="${2:-false}"
  local prompt
  prompt="$(build_prompt "$workspace")"

  if [[ "$DRY_RUN" == "true" ]]; then
    return 0
  fi

  local agent_env=()
  if [[ "$hooks_off" == "true" ]]; then
    agent_env+=(AGENTOPS_HOOKS_DISABLED=1)
  fi

  case "$AGENT" in
    claude)
      if [[ ${#agent_env[@]} -gt 0 ]]; then
        env "${agent_env[@]}" timeout "$TIMEOUT" claude -p "$prompt" --allowedTools "Edit,Write,Read,Bash" >/dev/null 2>&1 || true
      else
        timeout "$TIMEOUT" claude -p "$prompt" --allowedTools "Edit,Write,Read,Bash" >/dev/null 2>&1 || true
      fi
      ;;
    codex)
      if [[ ${#agent_env[@]} -gt 0 ]]; then
        env "${agent_env[@]}" timeout "$TIMEOUT" codex exec "$prompt" >/dev/null 2>&1 || true
      else
        timeout "$TIMEOUT" codex exec "$prompt" >/dev/null 2>&1 || true
      fi
      ;;
    *)
      echo "error: unsupported agent: $AGENT (use claude or codex)" >&2
      exit 1
      ;;
  esac
}

run_single() {
  local hooks_off="${1:-false}"

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "{\"score\": 0, \"total\": 1, \"pass\": false, \"skipped\": true, \"reason\": \"dry-run mode\"}"
    return
  fi

  local workspace
  workspace="$(mktemp -d)"

  bash "$TASK_DIR/setup.sh" "$workspace" >/dev/null 2>&1

  run_agent "$workspace" "$hooks_off"

  local result
  result="$(bash "$TASK_DIR/score.sh" "$workspace" 2>/dev/null | tail -1)"

  # Retry pattern (Aider-style): if first attempt fails, give agent test output
  if [[ "$RETRY" == "true" ]]; then
    local is_pass
    is_pass="$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('pass',False))" 2>/dev/null || echo "False")"
    if [[ "$is_pass" == "False" || "$is_pass" == "false" ]]; then
      local test_output=""
      # Capture test failure output for retry prompt
      if echo "$TASK_ID" | grep -q "^go-"; then
        test_output="$(cd "$workspace" && go test ./... 2>&1 || true)"
      elif echo "$TASK_ID" | grep -q "^py-"; then
        test_output="$(cd "$workspace" && source .venv/bin/activate 2>/dev/null && python -m pytest tests/ -q 2>&1 || true)"
      elif echo "$TASK_ID" | grep -q "^ops-"; then
        test_output="$(cd "$workspace" && bash tests/test-deploy.sh 2>&1 || bash tests/test-healthcheck.sh 2>&1 || true)"
      fi

      if [[ -n "$test_output" ]]; then
        local retry_prompt="You are in a software project at $workspace. Your previous fix attempt did not fully resolve the issue. Here are the test failures:\n\n$test_output\n\nFix the remaining issues so all tests pass. Do not explain — just fix the files."
        PROMPT="$retry_prompt" run_agent "$workspace" "$hooks_off"
        result="$(bash "$TASK_DIR/score.sh" "$workspace" 2>/dev/null | tail -1)"
      fi
    fi
  fi

  rm -rf "$workspace"
  echo "$result"
}

# Multi-run mode: compute pass@k and pass^k
run_multi() {
  local hooks_off="${1:-false}"
  local passes=0
  local results=()

  for ((i=1; i<=RUNS; i++)); do
    local result
    result="$(run_single "$hooks_off")"
    results+=("$result")

    local is_pass
    is_pass="$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('pass',False))" 2>/dev/null || echo "False")"
    if [[ "$is_pass" == "True" || "$is_pass" == "true" ]]; then
      passes=$((passes + 1))
    fi
  done

  local pass_at_k pass_all_k
  pass_at_k="$(python3 -c "print(f'{1 - (1 - $passes/$RUNS) if $RUNS > 0 else 0:.3f}')" 2>/dev/null || echo "0")"
  pass_all_k="$(python3 -c "print(f'{($passes/$RUNS) ** $RUNS if $RUNS > 0 else 0:.3f}')" 2>/dev/null || echo "0")"

  # Aggregate score from all runs
  local total_score total_possible
  total_score="$(printf '%s\n' "${results[@]}" | python3 -c "
import sys, json
total = 0
for line in sys.stdin:
    line = line.strip()
    if line:
        try:
            d = json.loads(line)
            total += d.get('score', 0)
        except: pass
print(total)
" 2>/dev/null)"
  total_possible="$(printf '%s\n' "${results[@]}" | python3 -c "
import sys, json
total = 0
for line in sys.stdin:
    line = line.strip()
    if line:
        try:
            d = json.loads(line)
            total += d.get('total', 0)
        except: pass
print(total)
" 2>/dev/null)"

  cat <<EOF
{"task": "$TASK_ID", "runs": $RUNS, "passes": $passes, "pass_at_k": $pass_at_k, "pass_all_k": $pass_all_k, "avg_score": $(python3 -c "print(f'{$total_score/$RUNS:.1f}')" 2>/dev/null), "avg_total": $(python3 -c "print(f'{$total_possible/$RUNS:.1f}')" 2>/dev/null), "hooks_disabled": $([[ "$hooks_off" == "true" ]] && echo "true" || echo "false")}
EOF
}

# A/B comparison mode
if [[ "$COMPARE" == "true" ]]; then
  if [[ "$RUNS" -gt 1 ]]; then
    result_with="$(run_multi "false")"
    result_without="$(run_multi "true")"
    echo "{\"task\": \"$TASK_ID\", \"compare\": {\"with_hooks\": $result_with, \"without_hooks\": $result_without}}"
  else
    result_with="$(run_single "false")"
    result_without="$(run_single "true")"
    echo "{\"task\": \"$TASK_ID\", \"compare\": {\"with_hooks\": $result_with, \"without_hooks\": $result_without}}"
  fi
  exit 0
fi

# Standard execution
if [[ "$RUNS" -gt 1 ]]; then
  run_multi "$([[ "$HOOKS_DISABLED" == "true" ]] && echo "true" || echo "false")"
else
  run_single "$([[ "$HOOKS_DISABLED" == "true" ]] && echo "true" || echo "false")"
fi
