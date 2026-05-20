#!/usr/bin/env bash
# safe-pull.sh — stash-pull-pop with explicit conflict reporting.
#
# Long-running sessions accumulate uncommitted local drift (registry.json,
# .gitignore, .agents/*, scratch files). `git pull --ff-only` aborts with
# "Your local changes would be overwritten by merge" the moment such drift
# overlaps an incoming change. The recovery dance is always the same:
# stash, pull, pop, deal with conflicts. This script makes it mechanical.
#
# Behavior:
#   1. Save tracked + staged drift to a named stash (auto-safe-pull-<ts>).
#      Untracked-only repos skip this step.
#   2. `git pull --ff-only <remote> <branch>` (defaults: origin, current).
#   3. Restore the stash with `git stash pop`.
#   4. On clean pop: drop the stash (no orphan stash entries).
#   5. On pop conflict: leave the stash in place AND list conflicting files
#      so the operator knows exactly what to fix.
#
# Flags:
#   --remote <name>   default: origin
#   --branch <name>   default: current branch
#   --dry-run         show the plan, take no action
#   --no-pop          stop after pull; leave stash for manual pop
#   --verbose         tee shell-trace to stderr
#
# Exit codes:
#   0 — success (pulled and any drift restored cleanly, OR no drift)
#   1 — pop produced conflicts (stash preserved)
#   2 — pre-pull failure (pull was rejected; stash preserved if created)
#   3 — usage / environment error
#
# Stash is NEVER silently lost; the EXIT trap reports its name if anything
# went wrong mid-flight.

set -euo pipefail

REMOTE="origin"
BRANCH=""
DRY_RUN=0
NO_POP=0
VERBOSE=0

usage() {
  sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --remote) shift; REMOTE="${1:-origin}" ;;
    --branch) shift; BRANCH="${1:-}" ;;
    --dry-run) DRY_RUN=1 ;;
    --no-pop) NO_POP=1 ;;
    --verbose|-v) VERBOSE=1 ;;
    -h|--help) usage 0 ;;
    *) echo "safe-pull: unknown arg: $1" >&2; usage 3 ;;
  esac
  shift || true
done

if [ "$VERBOSE" -eq 1 ]; then
  set -x
fi

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "safe-pull: not inside a git work tree" >&2
  exit 3
fi

if [ -z "$BRANCH" ]; then
  BRANCH="$(git symbolic-ref --short HEAD 2>/dev/null || true)"
fi
if [ -z "$BRANCH" ]; then
  echo "safe-pull: detached HEAD; pass --branch explicitly" >&2
  exit 3
fi

STASH_NAME="auto-safe-pull-$(date +%Y%m%dT%H%M%SZ)"
STASH_CREATED=0

cleanup_report() {
  # Trap: if we exit non-zero and a stash was created but never popped,
  # tell the operator where to find it.
  local rc=$?
  if [ "$rc" -ne 0 ] && [ "$STASH_CREATED" -eq 1 ]; then
    if git stash list 2>/dev/null | grep -qF "$STASH_NAME"; then
      echo "safe-pull: stash preserved as '$STASH_NAME'" >&2
      echo "safe-pull:   restore: git stash pop \"\$(git stash list | grep -F '$STASH_NAME' | sed 's/:.*//;s/.*\\(stash@{[0-9]*}\\).*/\\1/')\"" >&2
    fi
  fi
  return "$rc"
}
trap cleanup_report EXIT

# Step 1: detect tracked drift that would block a pull.
have_drift=0
# `git diff --quiet HEAD` is 0 when no diff vs HEAD (tracked clean), 1 otherwise.
if ! git diff --quiet HEAD 2>/dev/null; then
  have_drift=1
fi
# Staged but uncommitted also counts.
if ! git diff --cached --quiet 2>/dev/null; then
  have_drift=1
fi

if [ "$DRY_RUN" -eq 1 ]; then
  echo "safe-pull: DRY-RUN — would:"
  if [ "$have_drift" -eq 1 ]; then
    echo "  1. git stash push -m '$STASH_NAME' (drift detected)"
  else
    echo "  1. (skip stash; no tracked drift)"
  fi
  echo "  2. git pull --ff-only $REMOTE $BRANCH"
  if [ "$have_drift" -eq 1 ] && [ "$NO_POP" -eq 0 ]; then
    echo "  3. git stash pop (drop on clean, preserve on conflict)"
  elif [ "$NO_POP" -eq 1 ]; then
    echo "  3. (skip pop per --no-pop)"
  fi
  exit 0
fi

# Step 1 (real): stash if needed.
if [ "$have_drift" -eq 1 ]; then
  if ! git stash push -m "$STASH_NAME" --quiet >/dev/null 2>&1; then
    echo "safe-pull: stash push failed (refused empty / merge in progress?)" >&2
    exit 3
  fi
  STASH_CREATED=1
fi

# Step 2: pull.
if ! git pull --ff-only --quiet "$REMOTE" "$BRANCH" 2>/tmp/safe-pull-err.$$; then
  echo "safe-pull: pull --ff-only failed:" >&2
  cat /tmp/safe-pull-err.$$ >&2 || true
  rm -f /tmp/safe-pull-err.$$
  exit 2
fi
rm -f /tmp/safe-pull-err.$$

# Step 3 (optional): pop.
if [ "$STASH_CREATED" -eq 1 ] && [ "$NO_POP" -eq 0 ]; then
  if git stash pop --quiet >/dev/null 2>&1; then
    # Clean pop — git already dropped the stash automatically.
    echo "safe-pull: pulled $REMOTE/$BRANCH; drift restored cleanly"
    exit 0
  fi
  # Conflict. The stash is still in the list (pop without --index keeps the
  # stash on conflict). Surface the conflicting files for the operator.
  echo "safe-pull: pulled $REMOTE/$BRANCH but stash pop produced conflicts:" >&2
  git diff --name-only --diff-filter=U 2>/dev/null | sed 's/^/  CONFLICT: /' >&2
  echo >&2
  echo "safe-pull: stash preserved as '$STASH_NAME'" >&2
  exit 1
fi

if [ "$STASH_CREATED" -eq 1 ] && [ "$NO_POP" -eq 1 ]; then
  echo "safe-pull: pulled $REMOTE/$BRANCH; stash '$STASH_NAME' left for manual pop"
else
  echo "safe-pull: pulled $REMOTE/$BRANCH; no drift to restore"
fi
exit 0
