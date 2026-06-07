# Rewriting history to remove a file or secret (DESTRUCTIVE — gated)

> This is the single most dangerous operation in the skill. Rewriting history changes every commit
> SHA from the touched point forward, breaks every other clone, and — once reflogs expire — has no
> in-repo undo. Do not enter this path unless ALL preconditions below hold. When any is uncertain,
> STOP and report; do not guess.

## Preconditions (ALL required)

1. **Clean working tree.** `git status --short` is empty. Uncommitted work must be committed or
   stashed-and-noted first.
2. **Shared-history consent.** The repo is solo, OR the owner explicitly confirms collaborators have
   agreed to re-clone after the rewrite. Never rewrite published history unilaterally.
3. **Full backup exists.** A fresh mirror clone is the undo:
   ```bash
   git clone --mirror . ../$(basename "$PWD").backup.git
   ```
   Confirm it completed and record its path in the cleanup report.
4. **The target is genuinely unwanted.** Confirmed via the Phase 4 diagnosis, not assumed.

If you cannot establish all four, the correct action is to stop and hand the decision back.

## Tool choice

| Tool | Use when | Notes |
|---|---|---|
| `git filter-repo` | Default. Removing files/paths or replacing secret text. | Not bundled with git; check `git filter-repo --help`. Refuses to run on a non-fresh clone unless `--force` — that refusal is a safety feature, respect it. |
| BFG Repo-Cleaner | Very large repos, simple "delete these blobs / files" jobs. | Java jar; faster on huge histories; less flexible than filter-repo. |
| `git filter-branch` | Avoid. | Slow, error-prone, officially discouraged. Prefer filter-repo. |

Verify the chosen tool is actually installed before promising it:

```bash
git filter-repo --version    # or: java -jar bfg.jar --version
```

## Procedure — remove a path from all history (git-filter-repo)

```bash
# Backup first (see preconditions). Then:
git filter-repo --path path/to/bigfile.zip --invert-paths      # drop one path everywhere
# or remove by glob:
git filter-repo --path-glob '*.zip' --invert-paths
```

## Procedure — scrub a secret value from all history

```bash
# Put the literal secret(s) in a replacements file, one 'OLD==>REMOVED' per line.
printf 'sk_live_REDACTED==>REMOVED\n' > /tmp/secrets.txt
git filter-repo --replace-text /tmp/secrets.txt
rm -f /tmp/secrets.txt
```

**A scrubbed secret is still a compromised secret.** Always tell the user to rotate/revoke it at the
provider. The value may already exist in other clones, forks, CI caches, or backups; rotation is the
only real remediation. Rewriting history is necessary but NOT sufficient.

## After the rewrite

```bash
git reflog expire --expire=now --all      # drop old refs to the pre-rewrite objects
git gc --prune=now                        # reclaim the space (now irreversible — backup is your undo)
```

Then propagate, if the repo is shared and consent was given:

```bash
git push --force-with-lease --all         # safer than --force; aborts if remote moved unexpectedly
git push --force-with-lease --tags
```

Tell every collaborator to re-clone (or hard-reset to the new history). Old clones still contain the
old objects and can re-introduce them on a careless push.

## If something goes wrong

Restore from the mirror backup made in the preconditions:

```bash
git remote add backup ../<repo>.backup.git    # or re-clone from it entirely
```

This is exactly why the mirror backup is mandatory — it is the only undo once `gc --prune=now` has run.
