#!/usr/bin/env bash
set -euo pipefail

# ag-zzpy1: under `set -e`, a failing setup.sh/score.sh aborts the script before
# the normal cleanup runs, leaking a runner-created workspace. This EXIT trap
# cleans the workspace currently in flight on ANY exit path. It tracks ONLY a
# workspace the runner created (mktemp fallback); a caller-provided
# CORPUS_DELTA_WORKSPACE is never tracked here, so it is never deleted.
_CLEANUP_WS=""
trap '[[ -n "${_CLEANUP_WS:-}" ]] && rm -rf "$_CLEANUP_WS"' EXIT

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WORKBENCH="${EVAL_WORKBENCH:-$REPO_ROOT/evals/workbench}"

usage() {
  cat <<'USAGE'
Usage: scripts/eval-agent-harness.sh --task <task-id> --agent <codex> [options]

Run an agent against a workbench task and score the result.

Options:
  --task <id>          Task ID (e.g., go-01, py-04, ops-01)
  --agent <name>       Agent CLI to use: codex
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

  # --skip-git-repo-check (ag-o9x): each task workspace is a fresh NON-git dir;
  # modern `codex exec` REFUSES to launch there ("Not inside a trusted directory
  # and --skip-git-repo-check was not specified", exit 1) without this flag — so the
  # agent never ran and every task scored 0 in both arms. Capture the agent exit
  # status (do NOT swallow with `|| true`) and return it, so run_single can mark a
  # launch/timeout failure as `degraded` instead of an invisible score-0.
  local rc=0
  case "$AGENT" in
    codex)
      if [[ ${#agent_env[@]} -gt 0 ]]; then
        env "${agent_env[@]}" timeout "$TIMEOUT" codex exec --skip-git-repo-check -C "$workspace" -s workspace-write "$prompt" >/dev/null 2>&1 || rc=$?
      else
        timeout "$TIMEOUT" codex exec --skip-git-repo-check -C "$workspace" -s workspace-write "$prompt" >/dev/null 2>&1 || rc=$?
      fi
      ;;
    *)
      echo "error: unsupported agent: $AGENT (use codex)" >&2
      exit 1
      ;;
  esac
  return "$rc"
}

run_single() {
  local hooks_off="${1:-false}"

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "{\"score\": 0, \"total\": 1, \"pass\": false, \"skipped\": true, \"reason\": \"dry-run mode\"}"
    return
  fi

  # ag-zzpy1: honor the caller's isolation sandbox. The corpus-delta harness builds
  # a per-arm sandbox (isolated HOME + workspace) and exports CORPUS_DELTA_WORKSPACE
  # so the agent runs INSIDE that sandbox. A raw mktemp here bypasses it, leaving the
  # ag-5apc context-isolation proof valid only for the stub runner, not the real run.
  # Use the caller's workspace when provided; else fall back to a fresh temp dir.
  local workspace
  if [[ -n "${CORPUS_DELTA_WORKSPACE:-}" ]]; then
    workspace="$CORPUS_DELTA_WORKSPACE"
    mkdir -p "$workspace"
  else
    workspace="$(mktemp -d)"
    _CLEANUP_WS="$workspace"   # runner-created → tracked for trap cleanup on abort
  fi

  bash "$TASK_DIR/setup.sh" "$workspace" >/dev/null 2>&1

  # ag-o9x: capture the agent exit status (run_agent no longer swallows it). A
  # non-zero status means the agent itself failed to run cleanly (e.g. the
  # trusted-directory refusal, or a `timeout` kill: 124/137) — that is a DEGRADED
  # run, not an honest grader-fail, and must not be counted as a silent score-0.
  local agent_rc=0
  run_agent "$workspace" "$hooks_off" || agent_rc=$?

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
        agent_rc=0
        PROMPT="$retry_prompt" run_agent "$workspace" "$hooks_off" || agent_rc=$?
        result="$(bash "$TASK_DIR/score.sh" "$workspace" 2>/dev/null | tail -1)"
      fi
    fi
  fi

  # ag-o9x: a non-zero agent exit = DEGRADED run (the agent failed to launch/finish,
  # e.g. the trusted-dir refusal or a timeout kill). Mark it so the caller can tell a
  # degraded run apart from an honest grader-fail instead of silently scoring it 0.
  # Happy path (agent_rc==0) leaves `result` byte-identical to the prior contract.
  if [[ "$agent_rc" -ne 0 ]]; then
    result="$(printf '%s' "$result" | AGENT_RC="$agent_rc" python3 -c '
import sys, json, os
rc = int(os.environ["AGENT_RC"])
try:
    d = json.load(sys.stdin)
except Exception:
    d = {"score": 0, "total": 0, "pass": False}
d["pass"] = False
d["degraded"] = True
d["agent_exit"] = rc
print(json.dumps(d))
' 2>/dev/null || printf '{"score": 0, "total": 0, "pass": false, "degraded": true, "agent_exit": %s}' "$agent_rc")"
  fi

  # ag-zzpy1: clean ONLY a workspace we created. A caller-provided
  # CORPUS_DELTA_WORKSPACE (the harness's per-arm sandbox) is the caller's to
  # manage — removing it here would delete the isolation sandbox out from under
  # the harness.
  if [[ -z "${CORPUS_DELTA_WORKSPACE:-}" ]]; then
    rm -rf "$workspace"
    _CLEANUP_WS=""   # cleaned on the happy path → trap has nothing to do
  fi
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
