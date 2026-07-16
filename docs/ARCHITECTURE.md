# Architecture

AgentOps has a small semantic core and optional adapters around it.

```text
existing bead or caller intent
  -> one bounded implementation experiment
  -> runtime-derived subject-manifest.v1 + check receipts
  -> fresh Validate
  -> verdict.v2
  -> RPI report and stop
```

## Core

- **Plan** refines one active behavior, examples, non-goals, required evidence,
  write scope, and the first acceptance check in the existing intent source.
- **Implement** performs one bounded RED → GREEN → refactor experiment and
  returns runtime-derived subject identity, actual changed paths, and factual
  check receipts.
- **Validate** identifies the exact subject without Git, checks scope and
  acceptance, obtains one judgment from a distinct declared context, and stores
  a content-addressed verdict atomically.
- **RPI** invokes each core phase at most once and reports the result.

`FAIL` and `NOT_PROVEN` are terminal results for that invocation. A caller may
update the existing intent source and start a new invocation. RPI never creates
a parallel revision record or revises the subject automatically.

## Hexagonal boundary

The core depends on contracts, not substrates. NTM, Agent Mail, Codex Exec,
managed agents, Git metadata, trackers, provenance, and councils are optional
adapters or strategies. Missing or corrupt adapters cannot change a core
outcome.

Generic provenance may record a verdict after it is written. Provenance
availability is never required for verdict validity.

## Repository boundary

`ao gate check` runs ordinary deterministic repository checks. A successful
check says only that those checks passed. Repository policy owns commits,
pushes, pull requests, CI, releases, rollback, and deployment.

See the [component map](architecture/component-map.md), [ports and
adapters](architecture/ports-and-adapters.md), and [public contracts](contracts/index.md).
