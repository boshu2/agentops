#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/ao-rpi-autonomous-cycle.sh [options]

Runs one autonomous AO RPI cycle, including landing-plane actions.

Options:
  --goal <text>           Run ao rpi phased for an explicit goal/bead id.
  --max-cycles <n>        For queue mode, pass --max-cycles to ao rpi loop (default: 1).
  --repo-filter <name>    For queue mode, pass --repo-filter to ao rpi loop.
  --no-gates              Skip scripts/validate-go-fast.sh and scripts/security-gate.sh.
  --no-push               Skip git pull/rebase + bd sync + git push.
  --push-branch <name>    Push target branch on origin (default: origin HEAD branch, fallback: main).
  -h, --help              Show help.

Environment:
  AO_RPI_AUTO_CLEAN_AFTER   Duration for stale cleanup (default: 24h)
USAGE
}

GOAL=""
MAX_CYCLES="1"
REPO_FILTER=""
RUN_GATES="1"
RUN_PUSH="1"
PUSH_BRANCH=""

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
    --no-gates)
      RUN_GATES="0"
      shift
      ;;
    --no-push)
      RUN_PUSH="0"
      shift
      ;;
    --push-branch)
      PUSH_BRANCH="${2:-}"
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

current_branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$current_branch" == "HEAD" ]]; then
  echo "Detached HEAD detected. Running detached-safe, no branch created." >&2
fi

stale_after="${AO_RPI_AUTO_CLEAN_AFTER:-24h}"
ao rpi cleanup --all --stale-after "$stale_after" >/dev/null 2>&1 || true

if [[ -n "$GOAL" ]]; then
  echo "Running phased RPI for goal: $GOAL"
  ao rpi phased --auto-clean-stale --auto-clean-stale-after "$stale_after" "$GOAL"
else
  echo "Running queue-driven RPI loop (max cycles: $MAX_CYCLES)"
  loop_args=(rpi loop --max-cycles "$MAX_CYCLES")
  if [[ -n "$REPO_FILTER" ]]; then
    loop_args+=(--repo-filter "$REPO_FILTER")
  fi
  ao "${loop_args[@]}"
fi

if [[ "$RUN_GATES" == "1" ]]; then
  if [[ -x scripts/validate-go-fast.sh ]]; then
    scripts/validate-go-fast.sh
  else
    echo "Skipping fast validation gate (scripts/validate-go-fast.sh not executable)."
  fi

  if [[ -x scripts/security-gate.sh ]]; then
    scripts/security-gate.sh
  else
    echo "Skipping security gate (scripts/security-gate.sh not executable)."
  fi
fi

if [[ "$RUN_PUSH" == "1" ]]; then
  if [[ -z "$PUSH_BRANCH" ]]; then
    PUSH_BRANCH="$(git symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##')"
    if [[ -z "$PUSH_BRANCH" ]]; then
      PUSH_BRANCH="main"
    fi
  fi

  echo "Landing plane to origin/$PUSH_BRANCH"
  git fetch origin "$PUSH_BRANCH"
  git rebase "origin/$PUSH_BRANCH"

  if command -v bd >/dev/null 2>&1; then
    bd sync
  fi

  git push origin "HEAD:$PUSH_BRANCH"
  git status -sb
fi

echo "Autonomous RPI cycle complete."
