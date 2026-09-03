# ADR-0018: Retire the goals, shared, and scope skills

- **Status:** Accepted (2026-09-03)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0017](ADR-0017-loop-as-control-flow-not-knowledge.md) (the loop restored; its cathedral-cut gate is where retirements are tombstoned)
- **Origin:** the 2026-09-02 field audit of the shipped skill inventory (findings F5 to F9) and the Train 2 legibility run that acted on it; `docs/plans/2026-09-02-legible-membrane-plan.md` explicitly deferred these retirements to Train 2

## Context

Three shipped skills carried no contract of their own:

- `goals` was a pure `alias-of fitness` whose body delegated verbatim, so a
  probe of it would have measured `fitness` and filed the verdict under the
  wrong name; the probe-coverage denominator already excluded it for that
  reason.
- `shared` was a tombstone with no consumer, kept only so old links resolved.
- `scope` held five write-scope checks that every Plan already had to run
  before an experiment could start, so callers reached it only by knowing the
  name.

## Decision

Delete all three. `scope`'s five checks move into `plan`'s Workflow step 3
and `plan`'s description carries `scope`'s former triggers so a caller's own
words still land on the kernel that owns them (pinned by the routing golden
`rq-09-scope-to-plan`). `goals` callers use `fitness` directly.

The cathedral-cut conformance gate tombstones the three names in
`REMOVED_SKILLS`: reintroducing any of them, in `skills/` or a generated
projection, fails the gate. The Codex package count and golden count pins move
with the deletions.

## Consequences

- Fifty-four shipped skills instead of fifty-seven; no capability is lost,
  because none of the three owned a behavior of its own.
- `plan`'s description grew by three triggers, which raised the routing golden
  `rq-02` pack ceiling from 90 to 100 tokens (observed 96, disclosed in the
  golden's notes).
- Historical prose that mentions the three names stays as history; only
  active listings and machine inventories were pruned.
