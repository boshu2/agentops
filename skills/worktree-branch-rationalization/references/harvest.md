# Harvesting onto the staging branch

The harvest is the whole point: collect the strongest content from every keep-worthy
variant onto one staging branch **before** anything is deleted. Done right, deletion
later is provably lossless.

## Create the staging branch

```bash
git switch main && git pull --ff-only          # canonical tip, current
git switch -c rationalize/$(date +%Y%m%d)      # the staging branch
```

## Ordering rule — newest-first

Process keep-worthy branches and dirty worktrees in **descending last-commit-date**
order. Newer work usually subsumes older variants, so bringing it over first means
older branches frequently turn out fully contained (a clean `git cherry`/`--merged`
result) and need no further work.

## Bringing content over

- **Mergeable branch:** `git merge --no-ff <branch>` (preserves the variant's shape)
  or `git cherry-pick <range>` for surgical pickups.
- **Dirty worktree:** commit it to a salvage branch *inside that worktree first* — the
  uncommitted edits are in no branch yet:
  ```bash
  git -C <worktree> switch -c salvage/<name>
  git -C <worktree> add -A && git -C <worktree> commit -m "salvage: <name> WIP"
  ```
  then harvest `salvage/<name>` like any other branch.
- **Orphaned worktree (branch deleted):** its commits live only at the worktree HEAD —
  `git -C <worktree> branch salvage/<name> HEAD` to give them a name, then harvest.

## Conflict policy

Resolve in favor of the **strongest variant**, not the newest timestamp: the version
that builds, passes tests, and is most complete wins. When two variants both have real
unique work, keep both (separate commits) rather than discarding one. Never resolve a
conflict by blindly taking one side — inspect both.

## Verify the harvest

```bash
git log --oneline main..HEAD          # everything intended-to-keep is here
# build + test the staging branch before moving to containment confirmation
```

The staging branch must build and test clean before Phase 3. A staging branch that
doesn't build is not a safe basis for "this content is captured".
