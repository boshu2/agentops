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

# --- ag-5apc: always-loaded-root contamination fix --------------------------------
# Isolating AO_AGENTS_DIR alone is INSUFFICIENT: the agent also auto-loads knowledge
# from always-loaded roots — the repo-root CLAUDE.md, .claude/rules/*, the user-global
# ~/.claude/CLAUDE.md, and the auto-memory MEMORY.md. Read in BOTH arms, those leave no
# delta to measure. So each arm runs in an isolated SANDBOX (HOME + workspace) and these
# surfaces are treated as CORPUS — present in context_on, absent from context_off.
# Sources are env-overridable so the self-test can point them at marker fixtures.
SRC_REPO_ROOT="${CORPUS_DELTA_REPO_ROOT:-$REPO_ROOT}"
SRC_USER_CLAUDE="${CORPUS_DELTA_USER_CLAUDE:-$HOME/.claude/CLAUDE.md}"
SRC_MEM_DIR="${CORPUS_DELTA_MEM_DIR:-$HOME/.claude/projects/-home-boful-dev-agentops/memory}"
# Optional auth base: a dir copied into every sandbox HOME's .claude BEFORE context is
# applied, so real agents keep their credentials/settings while context is still stripped
# from the off arm (off = base − context, on = base + context). Empty in tests.
HOME_BASE="${CORPUS_DELTA_HOME_BASE:-}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# build_arm_sandbox <variant> <corpus_root> -> echoes "<home> <workspace> <agents_dir>"
build_arm_sandbox() {
  local variant="$1" corpus_root="$2" sb home ws agents mem_target
  sb="$(mktemp -d "$WORK/corpus-delta.XXXXXX")"
  home="$sb/home"; ws="$sb/ws"; agents="$ws/.agents"
  mkdir -p "$home/.claude" "$ws" "$agents"
  # auth/settings base (preserved in BOTH arms; never carries context)
  if [[ -n "$HOME_BASE" && -d "$HOME_BASE" ]]; then
    cp -r "$HOME_BASE/." "$home/.claude/" 2>/dev/null || true
  fi
  if [[ "$variant" == "context_on" ]]; then
    # project always-loaded surface
    if [[ -f "$SRC_REPO_ROOT/CLAUDE.md" ]]; then cp "$SRC_REPO_ROOT/CLAUDE.md" "$ws/CLAUDE.md"; fi
    if [[ -d "$SRC_REPO_ROOT/.claude/rules" ]]; then
      mkdir -p "$ws/.claude"; cp -r "$SRC_REPO_ROOT/.claude/rules" "$ws/.claude/rules"
    fi
    # user-global surface + auto-memory
    if [[ -f "$SRC_USER_CLAUDE" ]]; then cp "$SRC_USER_CLAUDE" "$home/.claude/CLAUDE.md"; fi
    if [[ -d "$SRC_MEM_DIR" ]]; then
      mem_target="$home/.claude/projects/${ws//\//-}/memory"
      mkdir -p "$mem_target"
      cp -r "$SRC_MEM_DIR/." "$mem_target/" 2>/dev/null || true
    fi
    # organic .agents corpus
    if [[ -d "$corpus_root" ]]; then cp -r "$corpus_root/." "$agents/" 2>/dev/null || true; fi
  else
    # context_off: strip context the base may have carried, so off is provably clean.
    rm -f "$home/.claude/CLAUDE.md"
    rm -rf "$home/.claude/projects" "$home/.claude/rules"
  fi
  echo "$home $ws $agents"
}

# run_arm <variant> <corpus_root> -> echoes "passes total"
# The runner is invoked with HOME=<sandbox home>, AO_AGENTS_DIR=<sandbox agents>, and
# CORPUS_DELTA_WORKSPACE=<sandbox ws>; a real agent MUST run with that HOME and cwd=ws so
# neither arm can reach always-loaded knowledge outside its sandbox.
run_arm() {
  local variant="$1" corpus_root="$2"
  local passes=0 seed line is_pass home ws agents
  echo "[corpus-delta] arm=$variant corpus=$corpus_root seeds=$SEEDS" >&2
  for ((seed = 1; seed <= SEEDS; seed++)); do
    read -r home ws agents < <(build_arm_sandbox "$variant" "$corpus_root")
    if [[ "$RUNNER" == "$DEFAULT_RUNNER" ]]; then
      line="$(cd "$ws" && HOME="$home" AO_AGENTS_DIR="$agents" CORPUS_DELTA_WORKSPACE="$ws" "$RUNNER" --task "$TASK_ID" --agent "$AGENT" --runs 1 2>/dev/null | tail -1)"
    else
      # Injected (test) runner contract: <task> <agent> <seed>; reads HOME, AO_AGENTS_DIR, CORPUS_DELTA_WORKSPACE.
      line="$(cd "$ws" && HOME="$home" AO_AGENTS_DIR="$agents" CORPUS_DELTA_WORKSPACE="$ws" "$RUNNER" "$TASK_ID" "$AGENT" "$seed" 2>/dev/null | tail -1)"
    fi
    is_pass="$(printf '%s' "$line" | jq -r 'if .pass == true then 1 else 0 end' 2>/dev/null || echo 0)"
    [[ "$is_pass" == "1" ]] && passes=$((passes + 1))
  done
  echo "$passes $SEEDS"
}

read -r off_pass off_total < <(run_arm context_off "")
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
