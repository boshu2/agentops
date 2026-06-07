# Finding large and dangling objects (read-only)

All recipes here are diagnostic — they list and rank, they do not delete. Removing what you find is a
separate, gated decision (large files → [history-rewrite.md](history-rewrite.md); branches → SKILL.md
Phase 2). Run these, present the results, and let the user decide.

## Repo footprint at a glance

```bash
du -sh .git                  # total .git size
git count-objects -vH        # count, size-pack, in-pack, prune-packable, garbage, size-garbage
```

`size-pack` dominated by a few huge blobs ⇒ candidate for history rewrite. `count` (loose objects)
high ⇒ a plain `git gc` will help and is safe.

## Rank the largest blobs in history (the key recipe)

This walks all reachable objects, keeps blobs, sorts by size, and resolves each to a path. Pure read.

```bash
git rev-list --objects --all \
| git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' \
| awk '/^blob/ {print substr($0,6)}' \
| sort --numeric-sort --key=2 \
| tail -n 20 \
| while read -r sha size path; do
    printf '%s\t%s\n' "$(numfmt --to=iec "$size" 2>/dev/null || echo "$size")" "$path"
  done
```

Each line is `human-size  path`. A large blob whose path is a build artifact, dataset, media file, or
`.env`-style file is a removal candidate. A large blob that is a legitimate committed asset (e.g. a
required binary fixture) is NOT — leave it and say so.

## Which commits introduced a given path

```bash
git log --all --oneline --follow -- <path>     # history of that path across renames
```

Use this to understand whether the file was later removed from the tree but still lives in history
(the usual cause of "I deleted it but `.git` is still huge").

## Dangling and unreachable objects

```bash
git fsck --full --dangling           # lists dangling commits / blobs / trees
git fsck --unreachable               # objects not reachable from any ref
```

- A **dangling commit** is often a recoverable lost commit (amended-away, hard-reset, deleted branch).
  Inspect before assuming it is garbage: `git show <sha>` and `git log --oneline <sha> -5`.
- Do NOT `gc --prune=now` to clear these until you have confirmed none represents work the user wants.
  The reflog and these dangling objects are the recovery net; expiring them is the destructive step.

## Recover a lost commit found via fsck

```bash
git show <dangling-sha>                       # confirm it is what you want
git branch recovered/<name> <dangling-sha>    # give it a ref so it survives gc
```

This is the safe response to finding a dangling commit that matters: re-anchor it, don't prune it.

## What "safe" means here

- These commands never write. The risk is only in what you do NEXT with the results.
- Surfacing a large blob is not permission to remove it. Confirm it is genuinely unwanted (Phase 4
  checkpoint in SKILL.md), then proceed to history-rewrite.md with backups in place.
