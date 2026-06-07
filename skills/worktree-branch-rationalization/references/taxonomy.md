# Worktree + branch taxonomy

Classify **every** worktree and **every** local branch into exactly one bucket. The
inventory in Phase 1 is incomplete until each item has a category and the protected
set is explicit.

## Categories

| Category | Definition | Default disposition |
|---|---|---|
| **canonical** | the default branch (`main`/`master`/`trunk`) and its worktree | keep — never delete |
| **protected** | release/maintenance branches; any branch with an open PR; anything in the repo's protected-branch policy | keep — never delete, regardless of merge state |
| **active** | branch with commits in the last ~14 days and a live owner/agent | keep; harvest only if the owner agrees to retire it |
| **stale** | no new commits in a long window, no open PR, fully merged into canonical | harvest (usually already contained) → delete |
| **orphaned-worktree** | linked worktree whose branch was deleted, or whose path is gone | salvage unique commits, then `git worktree remove` / `prune` |
| **drifted** | worktree HEAD and its named branch have diverged (force-push/rebase elsewhere) | resolve per `drift.md` before any deletion |
| **duplicate** | branch/worktree whose every commit is already on canonical | confirm containment → delete |

## How to gather the raw data

```bash
# Worktrees: path, HEAD, branch, locked/prunable flags
git worktree list --porcelain

# Branches: name, upstream, last-commit date, last SHA — newest first
git for-each-ref --sort=-committerdate \
  --format='%(refname:short)|%(upstream:short)|%(committerdate:short)|%(objectname:short)' \
  refs/heads/

# Is a branch contained in canonical already?
git branch --merged main
```

## Deciding the protected set

Write it down verbatim before Phase 2. Include, at minimum: the default branch, any
branch matching the repo's release pattern (e.g. `release/*`, `v*`, `maintenance/*`),
and every branch that currently has an open PR. When unsure whether a branch is
protected, treat it as protected — false positives cost nothing here; false negatives
lose history.
