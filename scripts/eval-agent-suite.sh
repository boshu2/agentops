#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
HARNESS="$SCRIPT_DIR/eval-agent-harness.sh"

usage() {
  cat <<'USAGE'
Usage: scripts/eval-agent-suite.sh [options]

Run the full 12-task agent eval suite and produce a baseline report.

Modes (pick one):
  --single              One run per task, task-specific prompts (default)
  --pass-at-k <n>       N runs per task for pass@k / pass^k stats
  --compare             A/B: with-hooks vs without-hooks
  --retry               Two-attempt with test feedback on failure
  --generic             Use generic prompt (prompt sensitivity test)

Options:
  --agent <name>        Agent CLI: claude or codex (default: claude)
  --timeout <secs>      Per-task timeout (default: 180)
  --tasks <list>        Comma-separated task IDs (default: all 12)
  --output <dir>        Output directory (default: auto-generated in .agents/evals/runs/)
  --promote             Copy result to baselines/ as the new baseline
  --quiet               Suppress per-task output, only show summary
  -h, --help            Show this help

Examples:
  scripts/eval-agent-suite.sh                          # Single run, all tasks
  scripts/eval-agent-suite.sh --pass-at-k 5            # Statistical baseline (5 runs each)
  scripts/eval-agent-suite.sh --compare                # A/B skill test
  scripts/eval-agent-suite.sh --retry                  # With error-feedback retry
  scripts/eval-agent-suite.sh --generic                # Prompt sensitivity baseline
  scripts/eval-agent-suite.sh --tasks go-01,py-04      # Subset of tasks
USAGE
}

MODE="single"
AGENT="claude"
TIMEOUT=180
TASKS=""
OUTPUT_DIR=""
PROMOTE=false
QUIET=false
PASS_K=5

while [[ $# -gt 0 ]]; do
  case "$1" in
    --single) MODE="single"; shift ;;
    --pass-at-k) MODE="pass-at-k"; PASS_K="$2"; shift 2 ;;
    --compare) MODE="compare"; shift ;;
    --retry) MODE="retry"; shift ;;
    --generic) MODE="generic"; shift ;;
    --agent) AGENT="$2"; shift 2 ;;
    --timeout) TIMEOUT="$2"; shift 2 ;;
    --tasks) TASKS="$2"; shift 2 ;;
    --output) OUTPUT_DIR="$2"; shift 2 ;;
    --promote) PROMOTE=true; shift ;;
    --quiet) QUIET=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "error: unknown option: $1" >&2; exit 1 ;;
  esac
done

ALL_TASKS="go-01 go-02 go-03 go-04 go-05 py-01 py-02 py-03 py-04 ops-01 ops-02 ops-03"
if [[ -n "$TASKS" ]]; then
  ALL_TASKS="${TASKS//,/ }"
fi

if [[ -z "$OUTPUT_DIR" ]]; then
  OUTPUT_DIR="$REPO_ROOT/.agents/evals/runs/$(date +%Y-%m-%d-%H%M%S)-agent-$MODE"
fi
mkdir -p "$OUTPUT_DIR"

task_count=$(echo "$ALL_TASKS" | wc -w)
echo "=== Agent Eval Suite: $MODE mode ==="
echo "Agent: $AGENT | Timeout: ${TIMEOUT}s | Tasks: $task_count"
[[ "$MODE" == "pass-at-k" ]] && echo "Runs per task: $PASS_K"
echo "Output: $OUTPUT_DIR"
echo ""

pass=0
fail=0
total_score=0
total_possible=0
declare -a json_results=()

for task in $ALL_TASKS; do
  harness_flags="--task $task --agent $AGENT --timeout $TIMEOUT"

  case "$MODE" in
    single)
      ;;
    pass-at-k)
      harness_flags="$harness_flags --runs $PASS_K --json"
      ;;
    compare)
      harness_flags="$harness_flags --compare"
      ;;
    retry)
      harness_flags="$harness_flags --retry"
      ;;
    generic)
      harness_flags="$harness_flags --generic-prompt"
      ;;
  esac

  start_ts=$(date +%s)
  # shellcheck disable=SC2086
  result=$(bash "$HARNESS" $harness_flags 2>/dev/null | tail -1)
  end_ts=$(date +%s)
  elapsed=$((end_ts - start_ts))

  # Save individual result
  echo "$result" > "$OUTPUT_DIR/$task.json"

  if [[ "$MODE" == "pass-at-k" ]]; then
    passes_k=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('passes',0))" 2>/dev/null || echo "0")
    pass_at=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('pass_at_k',0))" 2>/dev/null || echo "0")
    [[ "$QUIET" == "false" ]] && echo "  $task: $passes_k/$PASS_K pass (pass@$PASS_K=$pass_at) [${elapsed}s]"
    pass=$((pass + passes_k))
    fail=$((fail + PASS_K - passes_k))
  elif [[ "$MODE" == "compare" ]]; then
    [[ "$QUIET" == "false" ]] && echo "  $task: $result [${elapsed}s]"
  else
    is_pass=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('pass',False))" 2>/dev/null || echo "False")
    score=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('score',0))" 2>/dev/null || echo "0")
    total=$(echo "$result" | python3 -c "import sys,json; print(json.load(sys.stdin).get('total',0))" 2>/dev/null || echo "0")

    total_score=$((total_score + score))
    total_possible=$((total_possible + total))

    if [[ "$is_pass" == "True" || "$is_pass" == "true" ]]; then
      [[ "$QUIET" == "false" ]] && echo "  PASS: $task ($result) [${elapsed}s]"
      pass=$((pass + 1))
    else
      [[ "$QUIET" == "false" ]] && echo "  FAIL: $task ($result) [${elapsed}s]"
      fail=$((fail + 1))
    fi
  fi

  json_results+=("{\"task\": \"$task\", \"result\": $result, \"elapsed_seconds\": $elapsed}")
done

echo ""
echo "=== SUMMARY ==="

if [[ "$MODE" == "pass-at-k" ]]; then
  total_runs=$((task_count * PASS_K))
  pass_rate=$(python3 -c "print(f'{$pass / $total_runs * 100:.1f}' if $total_runs > 0 else '0.0')")
  echo "Total runs: $total_runs"
  echo "Passes: $pass ($pass_rate%)"
  echo "Fails: $fail"
elif [[ "$MODE" == "compare" ]]; then
  echo "A/B results written to: $OUTPUT_DIR/"
  echo "Compare with: jq '.compare' $OUTPUT_DIR/*.json"
else
  total=$((pass + fail))
  pass_rate=$(python3 -c "print(f'{$pass / $total * 100:.1f}' if $total > 0 else '0.0')")
  score_rate=$(python3 -c "print(f'{$total_score / $total_possible * 100:.1f}' if $total_possible > 0 else '0.0')")
  echo "Pass: $pass / $total ($pass_rate%)"
  echo "Score: $total_score / $total_possible ($score_rate%)"
fi

# Write aggregate report
agent_version="$(${AGENT} --version 2>/dev/null | head -1 || echo 'unknown')"
cat > "$OUTPUT_DIR/report.json" <<EOF
{
  "suite_id": "agentops-core.workbench-agent-v1",
  "run_date": "$(date -Iseconds)",
  "mode": "$MODE",
  "agent": "$AGENT",
  "agent_version": "$agent_version",
  "timeout_seconds": $TIMEOUT,
  "task_count": $task_count,
  "summary": {
    "pass": $pass,
    "fail": $fail,
    "pass_rate": $pass_rate$(if [[ "$MODE" != "compare" && "$MODE" != "pass-at-k" ]]; then echo ", \"total_score\": $total_score, \"total_possible\": $total_possible, \"score_rate\": $score_rate"; fi)
  },
  "results": [$(IFS=,; echo "${json_results[*]}")]
}
EOF

echo ""
echo "Report: $OUTPUT_DIR/report.json"

# Promote to baseline if requested
if [[ "$PROMOTE" == "true" ]]; then
  BASELINES_DIR="$REPO_ROOT/.agents/evals/baselines"
  mkdir -p "$BASELINES_DIR"
  cp "$OUTPUT_DIR/report.json" "$BASELINES_DIR/agentops-core.workbench-agent-v1.baseline.json"
  echo "Promoted to baseline: $BASELINES_DIR/agentops-core.workbench-agent-v1.baseline.json"
fi
