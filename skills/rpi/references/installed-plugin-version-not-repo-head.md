# /rpi Runs The Installed Plugin's Skills, Not The Repo Working Tree

Before metering, testing, or measuring skill behavior via `/rpi` (or any
`Skill()` call), **verify which plugin version is actually active**. The
harness loads skills from `~/.claude/plugins/cache/agentops-marketplace/agentops/<version>/`,
which can lag the repo and — when multiple cached versions are present —
can resolve to a stale one even though `installed_plugins.json` pins the
latest.

A measurement that assumes "repo HEAD has the new path, therefore `/rpi`
runs the new path" will silently measure the old path.

## When This Fires

- Token-budget measurement comparing skill paths
- Behavior test of a recent skill edit before reinstall
- Any `Skill(skill="x")` invocation expected to use post-edit code
- Plugin cache contains 2+ versions of `agentops-marketplace`
- `installed_plugins.json` was updated but plugins were not pruned

## The Diagnostic

```bash
# 1. What's in the cache?
ls ~/.claude/plugins/cache/agentops-marketplace/agentops/
# Expect ONE version. Multiple = drift risk.

# 2. What does installed_plugins.json pin?
jq '."agentops-marketplace".version' ~/.claude/installed_plugins.json

# 3. What does Skill resolution actually load?
# Read the "Base directory for this skill:" line in the skill-load output.
# Compare to repo:
wc -l skills/<n>/SKILL.md
# vs the loaded base dir's SKILL.md.
```

If the loaded base dir's SKILL.md line count doesn't match the repo,
the harness is reading a stale cache version.

## Evidence (anchored)

> "During the soc-etwf.6 clean-room RPI token measurement, the
> `agentops:rpi` Skill loaded its machinery from
> `~/.claude/plugins/cache/agentops-marketplace/agentops/2.41.0/` — the
> **pre-#275 baseline** (crank 689 lines, swarm 783, discovery 241) —
> even though the repo HEAD (`f77e8b22`) carried the post-#275 billboard
> skills (crank 210, swarm 292, discovery 121). The plugin cache held
> four versions (`2.38.0`, `2.39.0`, `2.41.0`, `2.41.1`).
> `installed_plugins.json` pinned `2.41.1`, yet the Skill resolver
> loaded `2.41.0`. Pruning the cache to `2.41.1`-only forced the correct
> resolution."
— `.agents/learnings/2026-05-15-rpi-loads-installed-plugin-not-repo-head.md`
(soc-etwf.6)

The mismatch: cache pinned `2.41.1`, but the resolver picked `2.41.0`.
A measurement run with this drift will label "new path" results that
are actually the baseline. Pruning the cache was the only reliable fix.

## How To Apply

### Before any skill-behavior measurement

1. **Verify the active plugin version.** Read the "Base directory for
   this skill:" line in the skill-load output (it's the first line of
   the skill content delivered to the agent).
2. **Compare against the repo.** `wc -l <base-dir>/SKILL.md` vs
   `wc -l skills/<n>/SKILL.md`. Mismatch → cache drift.
3. **Prune the plugin cache.** Keep only the version pinned in
   `installed_plugins.json`:
   ```bash
   PIN=$(jq -r '."agentops-marketplace".version' ~/.claude/installed_plugins.json)
   find ~/.claude/plugins/cache/agentops-marketplace/agentops/ -mindepth 1 -maxdepth 1 -type d \
     | grep -v "/$PIN$" \
     | xargs -r rm -rf
   ```
4. **Re-verify.** Reload the skill; the base dir should now point at
   the pinned version.

### To measure the repo working tree specifically

Two options:

1. **Reinstall from the repo first.** Make the cache point at the
   working tree's state, then run via `Skill()`.
2. **Read the repo files directly.** Follow the repo `SKILL.md` files
   inline rather than via the installed Skill tool. Loses the
   harness-mediated execution but isolates the test from cache state.

## Failure Mode

A measurement claims "new path performs X" but actually measured the
old path. The fix isn't to re-run the measurement; it's to verify the
cache state, prune, then re-measure. Otherwise the published number
is wrong with a green-looking process.

## See Also

- `~/.claude/installed_plugins.json` — the version pin source of truth
- `~/.claude/plugins/cache/agentops-marketplace/agentops/<version>/` —
  where skills are loaded from at runtime
- `references/autonomous-execution.md` — broader autonomous-loop rules
  that interact with skill versioning
