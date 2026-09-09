# Architecture

AgentOps has a small semantic core and optional adapters around it.

```text
existing bead or caller intent
  -> on-demand planning and bounded implementation
  -> runtime-derived subject-manifest.v1 + check receipts
  -> fresh Validate
  -> PASS | FAIL | NOT_PROVEN
  -> direct repair and fresh revalidation within real bounds when needed
  -> completed acceptance or truthful unfinished result
```

## Core

- **Plan** shapes missing intent and revises disproved approaches under unchanged
  acceptance and scope; clear work need not invoke it.
- **Implement** performs bounded implementation and direct known-defect repair and
  returns runtime-derived subject identity, actual changed paths, and factual
  check receipts.
- **Validate** identifies the exact subject without Git, checks scope and
  acceptance, and obtains one judgment from a distinct declared context. It
  stores a content-addressed verdict atomically only when requested by the
  caller or a declared downstream consumer.
- **RPI** owns the authorized outcome through checks, direct repair and fresh
  final judgment; specialists and planning are on demand (ADR-0017).
- **Memory** optionally recalls reviewed topic pages or separately mines and
  curates supported claims. It cannot change completed product verdicts.

Known findings are repaired within authority and real remaining bounds. A
genuine causal stall admits at most one bounded fresh helper; an unhelpful
answer, cancellation, refusal or a spent real bound ends the attempt. Explicit
repair-round bounds still apply, and no invocation renews an allowance. The
[fixed-dispatch adapter](../skills/rpi/references/bounded-adapter.md) keeps its
narrower optional contract; it does not govern native RPI execution.

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
