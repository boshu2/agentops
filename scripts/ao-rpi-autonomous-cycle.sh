#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/ao-rpi-autonomous-cycle.sh [options]

Runs one autonomous AO RPI cycle via the canonical supervisor path:
  ao rpi loop --supervisor

Options:
  --goal <text>                Run one supervised cycle for an explicit goal/bead id.
  --max-cycles <n>             Queue mode cycle cap (default: 1).
  --repo-filter <name>         Queue mode filter for target_repo.
  --gate-policy <policy>       Gate policy: off|best-effort|required (default: required).
  --landing-policy <policy>    Landing policy: off|commit|sync-push (default: off).
  --landing-branch <name>      Landing target branch (optional).
  --bd-sync-policy <policy>    Landing beads sync policy: auto|always|never (default: auto).
  --failure-policy <policy>    Failure policy: stop|continue (default: continue).
  --kill-switch-path <path>    Loop kill-switch path (default: .agents/rpi/KILL).
  --auto-clean-stale-after <d> Stale age threshold for auto-clean/ensure-cleanup (default: 24h).
  --no-gates                   Shortcut for --gate-policy off.
  --no-push                    Deprecated alias for --landing-policy off.
  --push-branch <name>         Deprecated alias for --landing-policy sync-push + --landing-branch.
  -h, --help                   Show help.
USAGE
}

GOAL=""
MAX_CYCLES="1"
REPO_FILTER=""
GATE_POLICY="required"
LANDING_POLICY="off"
LANDING_BRANCH=""
BD_SYNC_POLICY="auto"
FAILURE_POLICY="continue"
KILL_SWITCH_PATH=".agents/rpi/KILL"
AUTO_CLEAN_STALE_AFTER="24h"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --goal)
      GOAL="${2:-}"
      shift 2
      ;;
    --max-cycles)
      MAX_CYCLES="${2:-}"
      shift 2
      ;;
    --repo-filter)
      REPO_FILTER="${2:-}"
      shift 2
      ;;
    --gate-policy)
      GATE_POLICY="${2:-}"
      shift 2
      ;;
    --landing-policy)
      LANDING_POLICY="${2:-}"
      shift 2
      ;;
    --landing-branch)
      LANDING_BRANCH="${2:-}"
      shift 2
      ;;
    --bd-sync-policy)
      BD_SYNC_POLICY="${2:-}"
      shift 2
      ;;
    --failure-policy)
      FAILURE_POLICY="${2:-}"
      shift 2
      ;;
    --kill-switch-path)
      KILL_SWITCH_PATH="${2:-}"
      shift 2
      ;;
    --auto-clean-stale-after)
      AUTO_CLEAN_STALE_AFTER="${2:-}"
      shift 2
      ;;
    --no-gates)
      GATE_POLICY="off"
      shift
      ;;
    --no-push)
      LANDING_POLICY="off"
      shift
      ;;
    --push-branch)
      LANDING_POLICY="sync-push"
      LANDING_BRANCH="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "Error: must run inside a git repository." >&2
  exit 1
fi

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

if ! command -v ao >/dev/null 2>&1; then
  echo "Error: ao CLI not found on PATH." >&2
  exit 1
fi

loop_args=(
  rpi loop --supervisor
  --max-cycles "$MAX_CYCLES"
  --failure-policy "$FAILURE_POLICY"
  --gate-policy "$GATE_POLICY"
  --landing-policy "$LANDING_POLICY"
  --bd-sync-policy "$BD_SYNC_POLICY"
  --kill-switch-path "$KILL_SWITCH_PATH"
  --auto-clean
  --auto-clean-stale-after "$AUTO_CLEAN_STALE_AFTER"
  --ensure-cleanup
)

if [[ -n "$REPO_FILTER" ]]; then
  loop_args+=(--repo-filter "$REPO_FILTER")
fi

if [[ -n "$LANDING_BRANCH" ]]; then
  loop_args+=(--landing-branch "$LANDING_BRANCH")
fi

if [[ -n "$GOAL" ]]; then
  loop_args+=("$GOAL")
fi

echo "Running canonical supervisor path: ao ${loop_args[*]}"
ao "${loop_args[@]}"

echo "Autonomous RPI cycle complete."
