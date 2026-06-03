#!/usr/bin/env bash
# corpus-delta-harness.sh — ag-u6jh (ag-8p8o Wave-1a)
#
# Runs ONE task in two CONTEXT ARMS and emits a ContextDeltaScorecard:
#   context_off : agent runs against an isolated EMPTY corpus (fresh AO_AGENTS_DIR)
#   context_on  : agent runs against the ORGANIC corpus (--corpus DIR, e.g. the repo's .agents)
# K seeds per arm; arm score = pass-rate over seeds; aggregate_delta = on - off.
#
# WHY a side harness (not the ao eval runner): the deterministic eval runner rejects
# non-shell runtimes ("out of deterministic scope", cli/internal/eval/io.go) and is
# CI-locked. This lane is deliberately OUTSIDE it and is never part of the deterministic
# canary suite. It is also NOT an `ao` subcommand (no new CLI surface).
#
# INJECTABLE RUNNER (so the harness is testable WITHOUT burning LLM calls):
#   Set CORPUS_DELTA_RUNNER to a command. It is invoked as:
#       "$CORPUS_DELTA_RUNNER" <task-id> <agent> <seed>
#   with AO_AGENTS_DIR exported to the arm's corpus root. It MUST print one JSON line:
#       {"pass": true|false, "score": <num>, "total": <num>}
#   Default runner = scripts/eval-agent-harness.sh (real agent: claude|codex).
#
# ⚠️ HONESTY: a run of this harness with a STUB runner (e.g. the bats test) proves the
#    harness PLUMBING only — it is NOT evidence of the corpus delta. The real claim needs
#    a real agent + held, non-engineered tasks + K seeds (ag-nfux W1b, ag-epgk W1c).
#    Do not cite a stub/plumbing run as proof of the moat (cf. the wave0 confusion).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

TASK_ID=""
SEEDS=3
CORPUS_DIR="$REPO_ROOT/.agents"
AGENT="claude"
OUT=""

usage() {
  cat <<'USAGE'
Usage: scripts/corpus-delta-harness.sh --task <id> [options]

  --task <id>        Workbench task id (required)
  --seeds <K>        Seeds per arm (default: 3)
  --corpus <dir>     Organic corpus root for context_on (default: repo .agents)
  --agent <name>     Agent for the default runner: claude|codex (default: claude)
  --out <file>       Write the ContextDeltaScorecard JSON here (default: stdout only)

Override CORPUS_DELTA_RUNNER to inject a custom runner (used by tests).
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --task) TASK_ID="$2"; shift 2 ;;
    --seeds) SEEDS="$2"; shift 2 ;;
    --corpus) CORPUS_DIR="$2"; shift 2 ;;
    --agent) AGENT="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

[[ -n "$TASK_ID" ]] || { echo "error: --task required" >&2; exit 2; }
[[ "$SEEDS" =~ ^[1-9][0-9]*$ ]] || { echo "error: --seeds must be a positive integer" >&2; exit 2; }

DEFAULT_RUNNER="$REPO_ROOT/scripts/eval-agent-harness.sh"
RUNNER="${CORPUS_DELTA_RUNNER:-$DEFAULT_RUNNER}"

canonical_runner_path() {
  local path="$1" resolved

  if resolved="$(command -v "$path" 2>/dev/null)"; then
    path="$resolved"
  fi
  if resolved="$(realpath "$path" 2>/dev/null)"; then
    printf '%s\n' "$resolved"
  else
    printf '%s\n' "$path"
  fi
}

IS_DEFAULT_RUNNER=false
if [[ "$(canonical_runner_path "$RUNNER")" == "$(canonical_runner_path "$DEFAULT_RUNNER")" ]]; then
  IS_DEFAULT_RUNNER=true
fi

# run_arm <variant> <corpus_root> -> echoes "passes total" (passes = #seeds that passed)
run_arm() {
  local variant="$1" corpus_root="$2"
  local passes=0 seed line is_pass
  echo "[corpus-delta] arm=$variant corpus=$corpus_root seeds=$SEEDS" >&2
  for ((seed = 1; seed <= SEEDS; seed++)); do
    if [[ "$IS_DEFAULT_RUNNER" == "true" ]]; then
      line="$(AO_AGENTS_DIR="$corpus_root" "$RUNNER" --task "$TASK_ID" --agent "$AGENT" --runs 1 2>/dev/null | tail -1)"
    else
      # Injected (test) runner contract: <task> <agent> <seed>, reads AO_AGENTS_DIR.
      line="$(AO_AGENTS_DIR="$corpus_root" "$RUNNER" "$TASK_ID" "$AGENT" "$seed" 2>/dev/null | tail -1)"
    fi
    is_pass="$(printf '%s' "$line" | jq -r 'if .pass == true then 1 else 0 end' 2>/dev/null || echo 0)"
    [[ "$is_pass" == "1" ]] && passes=$((passes + 1))
  done
  echo "$passes $SEEDS"
}

# context_off: fresh empty isolated corpus root
OFF_ROOT="$(mktemp -d)"
trap 'rm -rf "$OFF_ROOT"' EXIT

read -r off_pass off_total < <(run_arm context_off "$OFF_ROOT")
read -r on_pass on_total < <(run_arm context_on "$CORPUS_DIR")

# pass-rate per arm; delta = on - off
scorecard="$(jq -n \
  --argjson off_pass "$off_pass" --argjson off_total "$off_total" \
  --argjson on_pass "$on_pass" --argjson on_total "$on_total" \
  --arg task "$TASK_ID" --argjson seeds "$SEEDS" \
  '
  ($off_pass / $off_total) as $off_score |
  ($on_pass / $on_total) as $on_score |
  {
    schema_version: 1,
    suite_id: ("corpus-delta-" + $task),
    suite_path: "scripts/corpus-delta-harness.sh",
    evidence_kind: "harness_plumbing",
    seeds_per_arm: $seeds,
    context_off: { variant: "context_off", passes: $off_pass, total: $off_total, aggregate_score: ($off_score | (.*10000|round)/10000), status: (if $off_score >= 0.75 then "pass" else "fail" end) },
    context_on:  { variant: "context_on",  passes: $on_pass,  total: $on_total,  aggregate_score: ($on_score  | (.*10000|round)/10000), status: (if $on_score  >= 0.75 then "pass" else "fail" end) },
    aggregate_delta: (($on_score - $off_score) | (.*10000|round)/10000)
  }')"

printf '%s\n' "$scorecard"
if [[ -n "$OUT" ]]; then
  printf '%s\n' "$scorecard" > "$OUT"
fi
