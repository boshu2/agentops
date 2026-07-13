#!/usr/bin/env bash
# sync-main-to-origin.sh — before the evolve-cron-rpi discovery phase selects a
# slice, fetch origin and fast-forward the local `main` to `origin/main` so that
# discovery diffs candidate slices against the TRUE merge base, not a stale local
# main.
#
# Why this exists (ag-6jt): the rpi worktree's local `main` lags `origin/main`
# (e.g. local main=04b5d7cc vs origin/main=9e9caccb). Phase-1 discovery diffed
# the candidate slice against the stale local `main`, so work already merged to
# origin/main read as "open" — repeatedly re-seeding duplicate work/beads
# (ag-b8m≈ag-jov, ag-6kw≈ag-c2i, 4th recurrence). The work-selection-ladder
# reference claimed "the cron syncs local main to origin/main before discovery",
# but no code actually did it. This script makes that claim real and testable,
# and pairs with duplicate-work-guard.sh (the grep-existing-beads half).
#
# Behavior:
#   1. `git fetch <remote>` (so refs/remotes/<remote>/main is current).
#   2. Resolve the diff base to <remote>/main — ALWAYS, never local main.
#   3. Fast-forward the LOCAL `main` ref to <remote>/main without forcing:
#      - if `main` is checked out: `git merge --ff-only`,
#      - otherwise: `git fetch <remote> main:main` (refuses a non-fast-forward).
#   4. Print the resolved diff base SHA so callers/tests can assert discovery
#      diffs against <remote>/main, not the previously-stale local main.
#
# Usage:  sync-main-to-origin.sh [<remote>]   (remote defaults to "origin")
# Exit:   0 synced (or already up to date) · non-zero on fetch/ff failure
# Output: final line is "DIFF_BASE: <remote>/main <sha>"
# Env:    SYNC_MAIN_REMOTE (default "origin") overrides the remote name.
set -euo pipefail

REMOTE="${1:-${SYNC_MAIN_REMOTE:-origin}}"

# 1. Refresh remote-tracking refs.
if ! git fetch "$REMOTE" >/dev/null 2>&1; then
  echo "FAIL: git fetch $REMOTE failed (offline or remote unreachable)" >&2
  exit 1
fi

# The authoritative diff base after sync is ALWAYS <remote>/main, never local main.
if ! ORIGIN_MAIN_SHA="$(git rev-parse --verify --quiet "refs/remotes/$REMOTE/main")"; then
  echo "FAIL: $REMOTE/main does not exist after fetch" >&2
  exit 1
fi

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"

if [ "$CURRENT_BRANCH" = "main" ]; then
  # main is checked out — fast-forward only (never a merge commit, never force).
  if ! git merge --ff-only "$REMOTE/main" >/dev/null 2>&1; then
    echo "FAIL: local main is not a fast-forward of $REMOTE/main (diverged); resolve manually" >&2
    exit 1
  fi
else
  # main is not checked out — move the ref directly, fast-forward only.
  # `git fetch <remote> main:main` refuses a non-fast-forward update (no force).
  if ! git fetch "$REMOTE" main:main >/dev/null 2>&1; then
    echo "FAIL: cannot fast-forward local main to $REMOTE/main (diverged); resolve manually" >&2
    exit 1
  fi
fi

# Emit the resolved diff base so discovery (and tests) can confirm it is
# <remote>/main, not the previously-stale local main.
echo "DIFF_BASE: $REMOTE/main $ORIGIN_MAIN_SHA"
exit 0
