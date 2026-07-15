# ADR-0003: Executable-Spec Artifact Durability

- **Status:** Accepted, scope reduced by the Cathedral Cut (2026-07-14)
- **Author:** AgentOps maintainers

## Context

Acceptance examples are useful only when their content survives the run that
created them. Repository authors may keep published examples under
`spec/scenarios/` and private evaluation inputs under an ignored local path.
AgentOps no longer creates, promotes, schedules, or closes either kind.

## Decision

- Published scenario artifacts live under `spec/scenarios/` and may be linked
  from an operator-authored `GOALS.md` directive.
- Private holdout scenarios may live under `.agents/holdout/` and remain local.
- Read-only goal inspection resolves a published scenario before a local
  holdout with the same ID.
- `ao goals scenarios` lists or lints existing links. It does not create,
  promote, mutate, schedule, or deliver them.
- Schemas and test fixtures remain tracked repository inputs.

The former domain-slice manifest and phased-RPI machinery are retired. Plan's
`write_scope` and acceptance scenarios are the per-invocation boundary.

## Consequences

Repository authors control their specification format and Git policy. AgentOps
can inspect durable acceptance links without becoming their authoring,
scheduling, or delivery system.

## References

- `schemas/scenario.v1.schema.json`
- `spec/scenarios/`
- `docs/architecture/operating-loop.md`
