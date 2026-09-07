---
last_reviewed: 2026-09-06
---

# AgentOps

AgentOps is the operations layer for agentic engineering: a portable semantic
integration and judgment layer that connects intent, coding agents, software
factories, context sources, and independent validation without taking
ownership of their state or delivery lifecycle. It helps fallible coding
agents produce work that another fresh context can independently judge.

The topology is a federated integration graph — tracker, Git, agents,
factories, checks, and validators stay separate nodes joined by typed
handoffs — and the product boundary is deliberately small. The standard
traversal through the graph is:

```text
RPI -> Plan -> Implement -> fresh Validate -> repair to convergence -> report
```

## Proven floor

AgentOps provides:

- behavior-first intent with normal and edge acceptance examples;
- one bounded RED -> GREEN -> refactor experiment;
- deterministic content identity independent of Git;
- one fresh, author-distinct semantic judgment;
- a `PASS | FAIL | NOT_PROVEN` result with optional content-addressed storage.

No fresh evidence-backed judgment means the experiment is not proven. Context
separation is explicitly attested; it is not claimed as cryptographic proof of
model isolation.

## What AgentOps does not own

AgentOps is not a new GitLab, CI service, tracker, merge queue, delivery system,
release manager, scheduler, or autonomous retry controller. It does not own:

- retries, budgets, queues, claims, leases, work ownership, or next actions;
- Git commits, branches, pushes, PRs, merges, rollback, closure, or release;
- the caller's decision after a validation result;
- mandatory provenance or learning on the validation critical path.

Repositories and callers keep those policies. They may use direct pushes, PRs,
hosted CI, merge queues, cloud agents, or custom release systems without asking
AgentOps for delivery permission.

## Product surfaces

Four load-bearing skills define the core:

| Skill | Responsibility |
|---|---|
| `rpi` | dispatch Plan and Implement once, Validate freshly, repair to convergence under the caller's bound; report |
| `plan` | shape acceptance, evidence, and write scope |
| `implement` | run one bounded experiment and produce a candidate |
| `validate` | independently judge exact content; persist only for a declared consumer |

`learn` remains an optional off-path consumer of verdict collections. Strategy
skills such as Premortem, Postmortem, Council, and idea genies add judgment when
the caller wants it. Factory/runtime adapters such as NTM, Agent Mail, Gas City,
and swarms may provide roles and dispatch. None is a correctness or lifecycle
authority.

The `ao` CLI supplies deterministic repository checks and generic read-only or
record helpers where useful. Semantic validation belongs to the Validate skill,
not a CLI state machine.

## Sovereign evidence

Fresh validation binds acceptance, exact subject content, author and validator
identities, criterion results, evidence, checked scope, and omissions. When a
caller or declared downstream consumer needs durable machine-readable evidence,
the same result is plain `verdict.v2` JSON under caller-controlled storage. A
generic provenance ledger may copy or reference it later, but verdict storage
and ledger availability are never required for validity.

## Selected Context Delivery Lifecycle contract

CDLC means **Context Delivery Lifecycle**: disposable agents improve through
maintained external context and environment. It does not train model weights
or promise deterministic inference. The selected contract adds continuous
learning alongside bounded delivery; it is contract adoption, not a claim that
new CDLC skills or Go evidence operations already ship.

Delivery has three phases: **Discovery** retrieves authorized experience,
resolves intent and uncertainty, and uses Plan to shape one experiment;
**Implement** builds that bounded experiment; **Validate** judges its exact
subject freshly, including admitted bounded repairs. Standalone RPI still
runs with Plan and Implement once. An explicitly selected bounded outer goal
may authorize another experiment within its accepted envelope, including a
return to Discovery. Native work, budgets, stops, queues and delivery remain
with the caller/runtime; AO gains no scheduler or semantic workflow engine.

The selected memory is a caller-owned external reviewed Markdown/OKF bundle.
Protected non-Git drafts and evidence precede exact factual-support review
and distinct destination-disclosure review, both before any Git object, index
or stash. Maintained claims are evidence, never automatic policy. Requested
legacy `.agents/` proof and unique research remain preserved under owner policy.
[ADR-0016](docs/adr/ADR-0016-state-tiers.md) owns placement and confidentiality.

Mechanism correctness and demonstrated net benefit are separate claims.
Factual support, permission to store exact content in a destination, and later
observed usefulness each need their own evidence. The current cold-reuse trial
had no optional knowledge-body use and demonstrates no memory benefit; failed
and null outcomes remain visible. A positive compounding claim needs new
independent task evidence, not citations, stored pages or closed work.

Recall and the Learn extension have later implementation owners; only T25 may
restore the small evolve skill after native stop/continuation is proven.
Existing entrypoint checks stay enforced. The selected runtime target is skills
plus Go `ao` evidence operations, with shared conformance before migrating the
current Python helpers. It does not migrate unrelated grandfathered scripts
or development generators. See [RPI traversal](docs/architecture/rpi-traversal.md)
and [ADR-0017](docs/adr/ADR-0017-loop-as-control-flow-not-knowledge.md).

## Why this shape

Coding agents are stochastic and can overstate completion. More orchestration
does not itself create trust. The smallest useful trust boundary is an explicit
behavior, exact content identity, and a fresh independent judgment whose limits
are recorded. AgentOps packages that boundary without taking over the user's
engineering system.
