# Upgrading

This page contains the action required for the AgentOps 3.3 Cathedral Cut. The
full history remains in [the changelog](CHANGELOG.md); the exact removed-command
map is in [the migration guide](MIGRATION.md).

## Upgrade to 3.3

AgentOps 3.3 removes public command surfaces despite the minor version number.
Three actions prevent broken workflows after upgrading:

1. **Remove the 3.2 pre-push verify hook.** If you ever ran `ao verify init`
   in a repository, delete the `AGENTOPS-VERIFY-RATCHET` block from that
   repository's `.git/hooks/pre-push` (restore `pre-push.agentops-orig` if one
   was set aside). The hook calls removed commands and blocks every push with a
   misleading "ao too old" error; `ao verify init --remove` no longer exists.
   `git push --no-verify` bypasses it once.
2. **Move your config file.** `~/.agentops/config.yaml` and
   `./.agentops/config.yaml` moved to `~/.agents/ao/config.yaml` and
   `./.agents/ao/config.yaml`. Legacy paths are still read as a fallback for
   this release, with a deprecation warning.
3. **Replace plugin installs with source links** (next section).

The semantic loop is now:

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

AgentOps no longer owns retry, queue, work-claim, Git, closure, release, or
delivery state. Plan shapes one behavior, Implement runs one bounded experiment,
Validate judges the exact candidate from fresh context, and RPI reports the
result once.

### Replace plugin installs with source links

Remove a 3.x runtime plugin using the commands in
[Install and day-2 operations](install-day2-ops.md), then install one canonical
checkout:

```bash
git clone https://github.com/boshu2/agentops.git ~/.local/share/agentops
cd ~/.local/share/agentops
ao skills link --dry-run
ao skills link
```

Resolve conflicts deliberately. `ao skills link` never overwrites real skill
directories or foreign symlinks.

### Update skill invocations

- Replace `discovery`, `behavior-first-planning`, and `goal-design` with `plan`.
- Replace `crank` with a caller-selected executor or `swarm`'s optional
  `dispatch_once` adapter.
- Use `premortem` and `postmortem`; hyphenated and underscored variants are gone.
- Invoke `br` or `bv` directly if the caller chooses those trackers; AgentOps no
  longer wraps them.
- Treat `learn` as optional later analysis, not an RPI phase or release receipt.

### Update CLI integrations

Removed lifecycle commands return nonzero, non-mutating tombstones for this
release. They never forward to old implementations. Update integrations now;
the tombstones are removed in the next release.

Use:

- the Validate skill for semantic judgment;
- `ao gate check` for ordinary deterministic repository checks;
- repository-owned Git and CI for delivery;
- caller-owned trackers and runtimes for scheduling, retries, and work state;
- generic `ao provenance` commands only when optional audit records are useful.

### Migrate verdict consumers

Replace `verdict.v1`, Pawl receipts, admission records, and delivery receipts
with `verdict.v2`. A PASS requires distinct declared author and validator context
IDs, explicit freshness attestation, acceptance and subject digests, checked and
unchecked scope, and criterion-level results. Missing identity or incomplete
subject proof produces `NOT_PROVEN`.

Historical lifecycle artifacts may remain as inert evidence. They must not
influence current phase sequencing or verdict validity.

## Older releases

The 3.x and 2.x migration record is historical. Read the versioned entries in
[CHANGELOG.md](CHANGELOG.md) (the standalone 3.0 migration map was retired in
the Cathedral Cut) when maintaining an old installation; do not apply those old
controller, daemon, hook, tracker, or plugin instructions to 3.3.
