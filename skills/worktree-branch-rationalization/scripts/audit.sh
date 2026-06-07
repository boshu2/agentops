#!/usr/bin/env bash
# audit.sh — READ-ONLY inventory of worktrees and local branches.
# Emits ground truth for Phase 1. Mutates nothing. Run from the repo root.
# Exit 0 always (it is a report); non-git dir exits 2.
set -euo pipefail

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  echo "audit.sh: not inside a git repository" >&2
  exit 2
fi

default_branch="$(git symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##' || true)"
[[ -z "$default_branch" ]] && default_branch="$(git rev-parse --abbrev-ref HEAD)"

echo "== Worktrees =="
git worktree list --porcelain

echo ""
echo "== Local branches (newest-first: name | upstream | date | sha) =="
git for-each-ref --sort=-committerdate \
  --format='%(refname:short)|%(upstream:short)|%(committerdate:short)|%(objectname:short)' \
  refs/heads/

echo ""
echo "== Already merged into ${default_branch} (likely duplicate/stale) =="
git branch --merged "$default_branch" | sed 's/^[* ] //'

echo ""
echo "== NOT merged into ${default_branch} (needs harvest before any delete) =="
git branch --no-merged "$default_branch" | sed 's/^[* ] //'

echo ""
echo "Summary: worktrees=$(git worktree list | wc -l | tr -d ' ') branches=$(git for-each-ref refs/heads/ | wc -l | tr -d ' ')"
echo "Default branch: ${default_branch}. This was READ-ONLY — nothing changed."
