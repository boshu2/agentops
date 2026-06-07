#!/usr/bin/env bash
# diagnose.sh — READ-ONLY git repo health report. Changes nothing.
# Gathers the Phase 1 diagnosis for git-repo-janitor: working-tree state,
# branch inventory (merged / unmerged / stale), storage footprint, the
# largest blobs in history, and dangling objects. Review the output BEFORE
# approving any mutation. This script never deletes, prunes, or rewrites.
#
# Usage: bash diagnose.sh [repo-path]   (defaults to current directory)
# Exit 0 = report produced. Exit 2 = not a git repository.

set -euo pipefail

REPO="${1:-.}"
cd "$REPO"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "ERROR: $REPO is not inside a git work tree." >&2
  exit 2
fi

hr() { printf '\n=== %s ===\n' "$1"; }

hr "Working tree (must be clean before any mutation)"
if [[ -n "$(git status --short)" ]]; then
  echo "DIRTY — uncommitted changes present. STOP and back up before cleaning."
  git status --short
else
  echo "clean"
fi

hr "Storage footprint"
du -sh .git 2>/dev/null || true
git count-objects -vH || true

hr "Branches merged into HEAD (safe-delete candidates — confirm before deleting)"
git branch --merged | grep -vE '^\*' || echo "(none)"

hr "Branches NOT merged (contain unique commits — DO NOT auto-delete)"
git branch --no-merged || echo "(none)"

hr "Branches by last-commit date (stale = old + merged)"
git for-each-ref --sort=committerdate refs/heads \
  --format='%(committerdate:short)  %(refname:short)' || true

hr "Stale remote-tracking refs (would be removed by 'git fetch --prune')"
git remote prune origin --dry-run 2>/dev/null || echo "(no origin remote or nothing stale)"

hr "Largest blobs in history (size  path) — removal is a SEPARATE gated decision"
git rev-list --objects --all 2>/dev/null \
  | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' 2>/dev/null \
  | awk '/^blob/ {print substr($0,6)}' \
  | sort --numeric-sort --key=2 \
  | tail -n 15 \
  | while read -r _sha size path; do
      printf '%s\t%s\n' "$(numfmt --to=iec "$size" 2>/dev/null || echo "$size")" "$path"
    done || echo "(none)"

hr "Dangling / unreachable objects (may be recoverable lost work — inspect, do not prune blindly)"
git fsck --full --dangling 2>/dev/null | head -n 30 || echo "(none)"

hr "Untracked files not ignored (.gitignore gap candidates)"
git ls-files --others --exclude-standard | head -n 30 || echo "(none)"

printf '\nDiagnosis complete. Nothing was changed. Review above, then mutate one category at a time with confirmation.\n'
