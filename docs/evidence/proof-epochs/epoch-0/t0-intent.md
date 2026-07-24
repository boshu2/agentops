# T0 exact intent

Freeze an honest, recoverable starting state for the skill-system overhaul
without overwriting the preserved original worktree or claiming that later
tranches are complete.

## Required criteria

- **T0-1 — Ownership:** no pre-existing caller-owned path is overwritten,
  restored from Git, or ambiguously assigned to the landing candidate.
- **T0-2 — Inventory:** exactly 49 canonical skill sources have unique paths
  and exact-byte digests.
- **T0-3 — Live checks:** every T0 load-bearing local check is GREEN and has a
  seeded negative witness that fails for the intended reason.
- **T0-4 — Regeneration:** the owning regeneration entrypoint is fail-fast,
  byte-idempotent, and leaves generated views current.
- **T0-5 — Proof chain:** every transitive proof edge is classified; no
  `UNKNOWN` or `USED_UNSOUND` edge is represented as PASS support.
- **T0-6 — Routing:** at least 30 realistic current routing scenarios are
  replayable, with abstention and forbidden-authority expectations separate
  from observations.
- **T0-7 — Reconciliation:** commit `16d764b5a`, `craft-goal`, the origin-based
  landing tree, and the preserved stale worktree have explicit dispositions.
- **T0-8 — Bootstrap proof:** epoch 0 freezes exact component bytes, modes, and
  qualification corpus; a standalone, hostile-tested CAS recorder outside the
  T1 subject is the only bootstrap activation path.
- **T0-9 — Safe pause:** installed delivery channels, landed/in-flight state,
  authority, and gaps are durable enough for a fresh context to resume without
  campaign memory.

## Declared exclusions

- Remote branch-protection and required-job policy.
- Historical routing observations while CASS reports
  `checkpoint_incomplete`.
- External plugin installations absent from this host.
- T1 through T8 implementation and Go CLI G0 through G2 implementation.
- Changing installed skill links, credentials, release channels, or external
  systems.

These exclusions are not required criteria and cannot be used to weaken any
criterion above.
