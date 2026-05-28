#!/usr/bin/env bash
# verified-rebase.sh <expected-head-subject>
#
# Wraps `git rebase --continue` with post-condition checks that catch the silent-
# failure pathology observed in evolve cron loops (cycles 223/224, PRs #337/#344):
# `git rebase --continue` can return 0 yet leave the rebase stuck or drop a commit,
# so a follow-on `regen + push` runs against the wrong HEAD and the push is a no-op.
#
# Promoted from an inline cron-prompt function so crons, agents, and sessions share
# one verified path instead of re-deriving it (soc-e9n6).
#
# Usage:
#   verified-rebase.sh "<subject the commit at HEAD should have after a good rebase>"
# Exit 0 only if: no rebase is still in progress AND HEAD's subject == expected.
set -uo pipefail

expected="${1:-}"
if [ -z "$expected" ]; then
  echo "usage: verified-rebase.sh <expected-head-subject>" >&2
  exit 2
fi

gitdir="$(git rev-parse --git-dir 2>/dev/null)" || { echo "verified-rebase: not a git repository" >&2; exit 2; }

rebase_in_progress() { [ -d "$gitdir/rebase-merge" ] || [ -d "$gitdir/rebase-apply" ]; }

# Continue only if a rebase is actually in progress. Force a non-interactive
# editor — this script is built for unattended cron/agent use, where the default
# commit-message editor on `rebase --continue` would hang or fail ("Terminal is
# dumb, but EDITOR unset"). GIT_EDITOR=true accepts the existing message as-is.
if rebase_in_progress; then
  if ! GIT_EDITOR=true GIT_SEQUENCE_EDITOR=true git rebase --continue; then
    echo "verified-rebase: FAIL — 'git rebase --continue' returned non-zero" >&2
    exit 1
  fi
fi

# Post-condition 1: the rebase must be fully done (the silent-stuck catch).
if rebase_in_progress; then
  echo "verified-rebase: FAIL — rebase still in progress after --continue (silent failure)" >&2
  exit 1
fi

# Post-condition 2: HEAD must be the expected commit (the dropped-commit catch).
actual="$(git log -1 --format=%s 2>/dev/null || true)"
if [ "$actual" != "$expected" ]; then
  echo "verified-rebase: FAIL — HEAD subject '$actual' != expected '$expected' (commit may have been dropped)" >&2
  exit 1
fi

echo "verified-rebase: OK — HEAD is '$actual', no rebase in progress"
