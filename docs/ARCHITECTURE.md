# Architecture

AgentOps has a small semantic core and optional adapters around it.

```text
existing bead or caller intent
  -> one bounded implementation experiment
  -> runtime-derived subject-manifest.v1 + check receipts
  -> fresh Validate
  -> PASS | FAIL | NOT_PROVEN
  -> bounded repair under the convergence law (ADR-0017)
  -> RPI report
```

## Core

- **Plan** refines one active behavior, examples, non-goals, required evidence,
  write scope, and the first acceptance check in the existing intent source.
- **Implement** performs one bounded RED → GREEN → refactor experiment and
  returns runtime-derived subject identity, actual changed paths, and factual
  check receipts.
- **Validate** identifies the exact subject without Git, checks scope and
  acceptance, and obtains one judgment from a distinct declared context. It
  stores a content-addressed verdict atomically only when requested by the
  caller or a declared downstream consumer.
- **RPI** invokes Plan and Implement at most once, validates freshly, and on
  `FAIL` or `NOT_PROVEN` with findings repairs and re-validates under the
  convergence law within the caller's `repair_rounds` (ADR-0017).

`FAIL` and `NOT_PROVEN` are terminal when the law stops the repair phase or the
rounds are spent. A caller may update the existing intent source and start a new
invocation. RPI never creates a parallel revision record and never widens the
caller's bound.

## Hexagonal boundary

The core depends on contracts, not substrates. NTM, Agent Mail, Codex Exec,
managed agents, Git metadata, trackers, provenance, and councils are optional
adapters or strategies. Missing or corrupt adapters cannot change a core
outcome.

Generic provenance may record a verdict after one is persisted. Provenance and
artifact storage are never required for a fresh validation result to be valid.

## Repository boundary

`ao gate check` runs ordinary deterministic repository checks. A successful
check says only that those checks passed. Repository policy owns commits,
pushes, pull requests, CI, releases, rollback, and deployment.

See the [component map](architecture/component-map.md), [ports and
adapters](architecture/ports-and-adapters.md), and [public contracts](contracts/index.md).
