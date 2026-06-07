---
name: worktree-branch-rationalization
description: >-
  Rationalize a repo's accumulated git worktrees and local branches down to a
  canonical line plus a small set of protected branches, harvesting the strongest
  content from every variant onto a staging branch BEFORE any destructive
  cleanup. Built for the aftermath of parallel-agent (multi-worktree) work, where
  dozens of linked worktrees and stale branches pile up and drift apart from the
  branches they were created for.
  Triggers: "rationalize my branches", "clean up git worktrees", "213 branches /
  47 worktrees", "kill all worktrees save what's worth saving", "merge what's
  worth merging and delete the rest", "branch archaeology", "worktree drift",
  "orphaned worktree", "prune stale branches", "parallel-agent repo cleanup".
skill_api_version: 1
user-invocable: false
practices:
- continuous-delivery
- trunk-based-development
hexagonal_role: domain
consumes:
- git
produces:
- git-branches
- .agents/worktree-rationalization/report.md
context:
  window: inherit
metadata:
  tier: execution
  stability: experimental
  dependencies:
  - git
output_contract: >-
  a staging branch carrying the harvested best content, a worktree+branch
  inventory, and a written rationalization report at
  .agents/worktree-rationalization/report.md — destructive removal happens only
  after the harvest is committed and confirmed
---

# worktree-branch-rationalization — collapse the sprawl, lose nothing worth keeping

> Harvest first. Delete last. The canonical line plus a few protected branches is the destination.

## ⚠️ Critical Constraints

- **Harvest before you prune — always.** Capture the best content onto a staging branch and commit it *before* removing any worktree or deleting any branch. Deletion is the last phase, never an early shortcut.
  - **Why:** a deleted branch's unmerged commits are only recoverable from reflog for a limited window; an orphaned-worktree branch may have the *only* copy of real work. Pruning first turns recoverable into lost.
  - **WRONG:** `git worktree prune && git branch -D feature-x` (delete, then hope it merged).
  - **CORRECT:** confirm `feature-x` is contained in staging (`git branch --merged staging`), *then* `git branch -d feature-x` (lowercase `-d` refuses to drop unmerged work).
- **Never remove a worktree with a dirty or locked working tree.** Run `git -C <worktree> status --porcelain` first; uncommitted changes there are not in any branch.
  - **Why:** `git worktree remove --force` on a dirty tree silently discards uncommitted edits — the exact agent-swarm output you are trying to save.
- **Never touch the protected set.** The default branch, release/maintenance branches, and any branch with an open PR are off-limits to deletion regardless of merge state.
  - **Why:** "merged into staging" is not the same as "safe to delete from origin"; protected branches may carry history other people depend on.
- **Use `-d` not `-D` as the default.** Reserve `-D` (force) for branches you have *individually* confirmed are fully harvested.
  - **Why:** `-D` skips the "is this merged?" safety check; defaulting to it defeats the whole skill.

## Why This Exists

Parallel-agent work multiplies worktrees and branches faster than anyone reconciles them. Each agent spins up a linked worktree on its own branch; runs end, sessions crash, branches get half-merged or abandoned. Weeks later the repo has dozens of linked worktrees — some pointing at deleted branches, some dirty, some duplicating work already on `main` — and a branch list nobody can reason about. Naive cleanup (`git worktree prune`, bulk `git branch -D`) is fast and destructive: it throws away the one copy of real work hiding in an orphaned worktree. This skill makes the safe path the *default* path: inventory everything, harvest the strongest content onto one staging branch, confirm containment, then collapse the rest.

## Quick Start

```bash
# From the repo root. Inventory only — no mutations.
bash {baseDir}/scripts/audit.sh

# After reviewing the audit, run the guided rationalization (harvest → confirm → prune).
bash {baseDir}/scripts/rationalize.sh --staging rationalize/$(date +%Y%m%d)
```

`audit.sh` is **read-only** (run it first, every time). `rationalize.sh` mutates the repo and **pauses for confirmation** before any destructive step.

## Workflow

### Phase 1 — Inventory (read-only)
Build the ground truth. `git worktree list --porcelain` for every linked worktree; `git for-each-ref refs/heads` for every local branch with its upstream and last-commit date. Classify each into the categories in [references/taxonomy.md](references/taxonomy.md): canonical, protected, active, stale, orphaned-worktree, drifted, duplicate.

**Checkpoint:** every worktree and branch lands in exactly one category, and the protected set is explicit and written down. Do not proceed until the inventory is complete.

### Phase 2 — Harvest onto staging
Create one staging branch off the canonical tip. Walk the *keep-worthy* branches and dirty worktrees newest-first; bring their unique content over by cherry-pick, merge, or — for dirty worktrees — committing the working tree to a salvage branch first. Resolve conflicts in favor of the strongest variant. See [references/harvest.md](references/harvest.md) for the ordering rule and conflict policy.

**Checkpoint:** `git log --oneline canonical..staging` shows every piece of work you intended to keep, and the staging branch builds/tests clean.

### Phase 3 — Confirm containment
For each branch you plan to delete, prove its content is reachable: `git branch --merged staging` lists the safe ones; for the rest, `git cherry staging <branch>` shows any commit *not* yet in staging. Anything with un-harvested commits goes back to Phase 2 — it is not eligible for deletion.

**Checkpoint:** the delete list contains only branches whose every commit is contained in staging (or is intentionally abandoned and recorded as such in the report).

### Phase 4 — Prune (destructive, last)
Remove worktrees with `git worktree remove` (only after confirming clean status), then `git worktree prune` for stale administrative entries. Delete harvested branches with `git branch -d`. Reconcile worktree↔branch drift per [references/drift.md](references/drift.md). Write the report.

**Checkpoint:** `git worktree list` shows only the canonical + intentionally-kept worktrees; `git branch` shows the canonical line plus the protected set; the report is written.

## Output Specification

Write the rationalization report to `.agents/worktree-rationalization/report.md` containing:
- the pre-state counts (worktrees, branches) and the post-state counts;
- the protected set (verbatim);
- the staging branch name and the harvested commit list;
- every deleted branch + worktree, with the reason and the SHA it was contained at;
- any branch *intentionally abandoned* (unmerged content deliberately dropped), named explicitly.

## Quality Rubric

- **No silent loss:** every deleted branch is provably contained in staging OR named in the report's "intentionally abandoned" list — never neither.
- **Protected set untouched:** the default, release, and open-PR branches still exist and point at the same SHAs they did before.
- **Reproducible state:** after the run, `git worktree list` and `git branch` match the post-state counts recorded in the report.
- **Dirty work captured:** no worktree was removed while `git status --porcelain` showed uncommitted changes.

## Examples

**47 worktrees after an agent swarm.** `audit.sh` flags 31 as duplicates of `main`, 9 as orphaned (branch already deleted), 5 dirty, 2 canonical. Harvest the 5 dirty + any unique commits from the 9 orphaned onto `rationalize/20260606`, confirm containment, then `git worktree remove` the 45 non-canonical and `git branch -d` the harvested branches. Report records all 45.

**Worktree↔branch drift.** A worktree's branch was force-pushed and rebased elsewhere, so the local branch and the worktree's HEAD diverged. `drift.md` resolves it: salvage the worktree's unique commits to a branch, then re-point or remove. Never assume the worktree matches its named branch.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `git worktree remove` fails: "contains modified or untracked files" | dirty working tree | commit to a salvage branch first, then remove; only `--force` after the salvage commit |
| `git branch -d` fails: "not fully merged" | branch has commits not in staging | go back to Phase 2 and harvest, or `git cherry staging <branch>` to see exactly what is missing |
| worktree listed but path gone | stale administrative entry | `git worktree prune` (safe — only removes bookkeeping for deleted paths) |
| branch shows merged but a worktree still references it | drift | resolve per [references/drift.md](references/drift.md) before deleting |

## See Also

| I need to… | Reference |
|---|---|
| Classify each worktree/branch | [references/taxonomy.md](references/taxonomy.md) |
| Order the harvest + resolve conflicts | [references/harvest.md](references/harvest.md) |
| Fix worktree↔branch drift + naming conventions | [references/drift.md](references/drift.md) |
