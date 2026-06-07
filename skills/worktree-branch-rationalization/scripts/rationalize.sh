#!/usr/bin/env bash
# rationalize.sh — guided harvest -> confirm -> prune. MUTATES the repo.
# Pauses for confirmation before any destructive step. Run from the repo root.
#
#   rationalize.sh --staging <branch> [--apply]
#
# Without --apply it runs in DRY-RUN mode (prints the plan, makes the staging
# branch, but performs NO deletions). With --apply it still prompts before each
# destructive batch. Exit 0 = completed; 2 = bad invocation / not a git repo.
set -euo pipefail

STAGING=""
APPLY=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --staging) STAGING="${2:-}"; shift 2 ;;
    --apply)   APPLY=1; shift ;;
    *) echo "rationalize.sh: unknown arg '$1'" >&2; exit 2 ;;
  esac
done

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  echo "rationalize.sh: not inside a git repository" >&2; exit 2
fi
if [[ -z "$STAGING" ]]; then
  echo "rationalize.sh: --staging <branch> is required" >&2; exit 2
fi

default_branch="$(git symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null | sed 's#^origin/##' || true)"
[[ -z "$default_branch" ]] && default_branch="$(git rev-parse --abbrev-ref HEAD)"

echo "Phase 1: inventory (read-only) — review before continuing."
git worktree list

echo ""
echo "Phase 2: create staging branch '${STAGING}' off ${default_branch}."
if git show-ref --verify --quiet "refs/heads/${STAGING}"; then
  echo "  staging branch already exists — reusing it."
else
  git switch "$default_branch" >/dev/null 2>&1 || true
  git switch -c "$STAGING"
fi
echo "  -> harvest keep-worthy branches/dirty worktrees onto '${STAGING}' (see references/harvest.md)."
echo "     This script does not auto-merge: harvest is a judgment step. Do it now, then re-run with --apply."

echo ""
echo "Phase 3: containment check (what is NOT yet in ${STAGING}):"
while IFS= read -r b; do
  [[ -z "$b" ]] && continue
  [[ "$b" == "$STAGING" || "$b" == "$default_branch" ]] && continue
  missing="$(git cherry "$STAGING" "$b" 2>/dev/null | grep -c '^+' || true)"
  echo "  ${b}: ${missing} commit(s) not in ${STAGING}"
done < <(git for-each-ref --format='%(refname:short)' refs/heads/)

if [[ "$APPLY" -ne 1 ]]; then
  echo ""
  echo "DRY-RUN complete. No worktrees removed, no branches deleted."
  echo "Re-run with --apply once the harvest is done and containment shows 0 for delete candidates."
  exit 0
fi

echo ""
echo "Phase 4: prune (DESTRUCTIVE). Each batch will prompt."
read -r -p "Proceed with worktree/branch cleanup? [y/N] " ans
[[ "$ans" == "y" || "$ans" == "Y" ]] || { echo "Aborted — nothing deleted."; exit 0; }
echo "  Removing only worktrees with a clean status; using 'git branch -d' (refuses unmerged)."
echo "  Perform the per-item removals manually per the report — this guard intentionally"
echo "  does not bulk-delete. Write .agents/worktree-rationalization/report.md when done."
