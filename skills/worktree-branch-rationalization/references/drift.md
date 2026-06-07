# Worktree↔branch drift + naming conventions

## What drift is

A linked worktree is supposed to track a named branch. Drift is when the two no longer
agree:

- the worktree's branch was **force-pushed/rebased elsewhere**, so the local branch
  and the worktree HEAD diverged;
- the branch was **deleted** while its worktree still exists (orphaned);
- the worktree was **detached** (HEAD points at a SHA, not a branch);
- two worktrees somehow reference work that should be one branch.

Never assume a worktree matches its named branch — verify before any deletion.

## Diagnose

```bash
git worktree list --porcelain          # shows HEAD + branch (or "detached") per worktree
# Compare the worktree HEAD to the branch it claims:
git -C <worktree> rev-parse HEAD
git rev-parse <branch>
# Commits in the worktree but not in the branch (drift content to salvage):
git -C <worktree> log --oneline <branch>..HEAD
```

## Resolve

1. **Salvage first.** Any commit shown by the `<branch>..HEAD` diff is unique to the
   worktree — name it: `git -C <worktree> branch salvage/<name> HEAD`, then harvest per
   `harvest.md`.
2. **Re-point or remove.** Once content is salvaged, either re-point the worktree to the
   reconciled branch (`git -C <worktree> switch <branch>`) or remove it
   (`git worktree remove <worktree>` after confirming clean status).
3. **Prune bookkeeping.** `git worktree prune` clears administrative entries for paths
   that no longer exist.

## Naming conventions (to prevent the next pile-up)

Adopt a prefix scheme so worktrees and branches stay legible and drift is obvious:

| Prefix | Meaning |
|---|---|
| `feat/<topic>` | feature work, one topic per branch |
| `fix/<topic>` | bug fix |
| `agent/<id>-<topic>` | parallel-agent work — the id makes orphans traceable to their run |
| `salvage/<name>` | content rescued during a rationalization (transient) |
| `rationalize/<date>` | the staging branch for a cleanup pass |

Rules that keep the set rationalizable: one worktree per active branch; agent worktrees
carry the agent id in the branch name; delete a worktree as soon as its branch merges;
never reuse a branch name across two live worktrees.
