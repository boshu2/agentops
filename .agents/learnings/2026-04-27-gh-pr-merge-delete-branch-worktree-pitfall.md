---
id: learning-2026-04-27-gh-pr-merge-delete-branch-worktree-pitfall
type: learning
date: 2026-04-27
category: tooling
confidence: high
maturity: provisional
utility: 0.5
source_epic: ag-3lx
helpful_count: 0
harmful_count: 0
reward_count: 0
---

# Learning: `gh pr merge --delete-branch` Aborts Mid-Step If A Local Worktree Holds The Branch

## What We Learned

`gh pr merge --merge --delete-branch` performs three operations in sequence:

1. POST merge to GitHub (success)
2. `git branch -d` locally (FAILS if a worktree holds the branch)
3. DELETE remote branch via API (NEVER REACHED if step 2 fails)

Result: the PR is merged but the remote branch survives, requiring a follow-up `git push origin --delete <branch>`. Operators expecting one-shot cleanup are left with inconsistent state.

In this epic, sibling sessions had checkout worktrees at `/Users/bo/.codex/worktrees/pr171-finish` for `feat/eval-canaries-closeout`. The merge succeeded; local branch deletion failed; remote branch deletion was skipped; manual cleanup was required.

## Why It Matters

If you assume `--delete-branch` was atomic, you'll think the cleanup is done. The remote branch dangling can confuse later searches (`git ls-remote`), `gh pr list --search`, and any tooling that enumerates "active branches".

## When To Apply

- You're in a multi-session or multi-worktree setup where sibling agents may hold checkouts of feature branches
- You're scripting PR merges and need predictable post-state

## How To Apply

Either:
1. Confirm no worktrees hold the branch before running `--delete-branch`:
   ```
   git worktree list | grep <branch>
   ```
2. Or skip `--delete-branch` and do it explicitly in two steps:
   ```
   gh pr merge <N> --merge
   git push origin --delete <branch>
   ```

The second pattern is more robust under sibling-session parallelism.

## Source

Session resumption on epic ag-3lx, 2026-04-27. PR #171 merge with `--delete-branch` aborted on local-cleanup; required `git push origin --delete feat/eval-canaries-closeout` and `feat/eval-cli-integration` to finish remote cleanup.
