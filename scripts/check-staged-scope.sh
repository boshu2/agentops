#!/usr/bin/env bash
#
# check-staged-scope.sh — pre-commit guard against concurrent-lane contamination.
#
# THE FAILURE THIS PREVENTS (2026-06-29): the agentops checkout "runs hot" with
# parallel sessions. A concurrent lane can stage its files in the index between
# your `git add` and your `git commit`; a bare `git commit` then sweeps them in.
# This happened live to a historical landing-review skill commit (GOALS.md/PRODUCT.md/
# .gitignore from another lane rode along) — despite the worktree rule being a
# documented footgun. Advisory knowledge did not prevent it. This is the
# mechanical surface that does.
#
# THE RULE (deterministic, fail-closed): in the CANONICAL checkout (not a linked
# worktree — the worktree IS the safe path), block a commit whose staged set
# MIXES the shared cross-lane files (the high-churn docs/config every lane edits)
# with any other work. A pure-docs commit is fine; a pure-feature commit is fine;
# the MIX is the contamination signature.
#
# OVERRIDE: AGENTOPS_COMMIT_SCOPE_OK=1 git commit ...   (you've confirmed the mix
# is intended — e.g. a feature that legitimately updates README + code together).
#
# Install as a pre-commit hook (chains with any existing one):
#   ln -sf ../../scripts/check-staged-scope.sh .git/hooks/pre-commit
# or call it from an existing .git/hooks/pre-commit.
set -uo pipefail

[ "${AGENTOPS_COMMIT_SCOPE_OK:-0}" = "1" ] && exit 0

# Skip in linked worktrees — committing from a per-bead worktree is the safe path
# this guard is steering you toward, so never block there.
git_dir="$(git rev-parse --git-dir 2>/dev/null || echo)"
case "$git_dir" in
  */worktrees/*) exit 0 ;;
esac

staged="$(git diff --cached --name-only 2>/dev/null || true)"
[ -z "$staged" ] && exit 0

# Shared cross-lane churn files: the docs/config a parallel lane is most likely
# to have staged. Anchored to repo-root paths.
shared_re='^(GOALS\.md|PRODUCT\.md|README\.md|\.gitignore|CLAUDE\.md|AGENTS([-.][A-Za-z0-9]+)*\.md|registry\.json)$'

shared="$(printf '%s\n' "$staged" | grep -E "$shared_re" || true)"
other="$(printf '%s\n' "$staged"  | grep -vE "$shared_re" || true)"

if [ -n "$shared" ] && [ -n "$other" ]; then
  other_n="$(printf '%s\n' "$other" | grep -c . || echo 0)"
  {
    echo "⛔ commit-scope guard: staged set MIXES shared cross-lane files with other work."
    printf '%s\n' "$shared" | sed 's/^/   shared cross-lane: /'
    echo "   + ${other_n} other staged file(s)"
    echo
    echo "This is the concurrent-lane contamination pattern: a parallel session's staged"
    echo "files may have swept into your commit (agentops runs hot)."
    echo "Fix one of:"
    echo "   • commit ONLY your files:   git commit -- <your-paths>"
    echo "   • work in a per-bead worktree:   git worktree add --detach wt-<bead> origin/main"
    echo "Override if the mix is genuinely intended:   AGENTOPS_COMMIT_SCOPE_OK=1 git commit ..."
  } >&2
  exit 1
fi

exit 0
