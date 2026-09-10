---
last_reviewed: 2026-09-10
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
Accepted intent -> native implementation and checks -> fresh independent judgment -> finish
```

## Proven floor

AgentOps provides:

- behavior-first intent with normal and edge acceptance examples;
- bounded implementation and direct repair under the caller's accepted intent;
- deterministic content identity independent of Git;
- one fresh, author-distinct semantic judgment;
- a `PASS | FAIL | NOT_PROVEN` result with optional content-addressed storage.

No fresh evidence-backed judgment means the experiment is not proven. Context
separation is explicitly attested; it is not claimed as cryptographic proof of
model isolation.

## What AgentOps does not own

AgentOps is not a new GitLab, CI service, tracker, merge queue, delivery system,
release manager, scheduler, or autonomous retry controller. It does not own:

- aggregate retry controllers, budgets, queues, claims, leases, or work ownership;
- Git commits, branches, pushes, PRs, merges, rollback, closure, or release;
- the caller's decision after a validation result;
- mandatory provenance or learning on the validation critical path.

Repositories and callers keep those policies. They may use direct pushes, PRs,
hosted CI, merge queues, cloud agents, or custom release systems without asking
AgentOps for delivery permission.

## Product surfaces

The default uses the coding agent and shell with zero mandatory AgentOps skills.
The repository brief supplies acceptance, source boundaries and required checks.
`ao` supplies deterministic tools on demand. A native goal can maintain
continuity; the caller's tracker keeps work and handoffs.

The existing skill library remains optional. Select guidance when a task needs
it; an explicit RPI workflow packages these operations:

| Skill | Responsibility |
|---|---|
| `rpi` | own the authorized outcome through direct repair, checks and fresh final judgment |
| `plan` | shape missing intent or revise a disproved approach within accepted scope |
| `implement` | implement, check and repair known defects within real bounds |
| `validate` | independently judge exact content; persist only for a declared consumer |
| `memory` | recall reviewed context or separately mine and curate topic pages |

`learn` is a compatible off-path entrypoint for Memory's mining operation over
authorized episodes, corrections and other evidence. Strategy
skills such as Premortem, Postmortem, Council, and idea genies add judgment when
the caller wants it. Factory/runtime adapters such as NTM, Agent Mail, Gas City,
and swarms may provide roles and dispatch. None is a correctness or lifecycle
authority.

The `ao` CLI supplies deterministic repository checks and generic read-only or
record helpers where useful. A fresh native reviewer owns semantic validation;
the optional Validate skill packages that method. A CLI cannot issue semantic
acceptance from successful checks alone.

## Sovereign evidence

Fresh validation binds acceptance, exact subject content, author and validator
identities, criterion results, evidence, checked scope, and omissions. When a
caller or declared downstream consumer needs durable machine-readable evidence,
the same result is plain `verdict.v2` JSON under caller-controlled storage. A
generic provenance ledger may copy or reference it later, but verdict storage
and ledger availability are never required for validity.

## Selected Context Delivery Lifecycle contract

Native execution owns the authorized outcome through finish. The optional
[RPI charter](skills/rpi/SKILL.md) supplies on-demand Plan, Implement, Validate
and Memory when selected. Known
failures get direct repair; evidence can revise an approach within unchanged
acceptance and scope. A genuine causal stall admits at most one bounded helper,
never a helper chain. Cheap checks precede fresh author-distinct final judgment;
reserve capacity for finishing and respect real caller/native bounds.

[Memory](skills/memory/SKILL.md) supplies optional recall and separately budgeted
mine/learn and curate/qualify/retire operations. BD owns work/status/handoffs,
Git content, native/CASS systems episodes, and caller-selected reviewed external
Markdown topic pages reusable claims. Update a topic page rather than one file
per session; preserve applicability, action, support, limits and invalidation.
A single incident supports a narrow observation. Learning can remove rules;
rare useful constraints and no-change remain valid. No blind TTL/deletion.

Exact factual support, permission to store content and later usefulness are
separate claims. Protected external non-Git drafts and fresh support/disclosure
review precede any Git object/index/stash/import. Preserve legacy `.agents/`
evidence under [ADR-0016](docs/adr/ADR-0016-state-tiers.md). This lean path uses
public or already-cleared trial inputs only; native restricted-source isolation
is not supplied by a skill or same-user process. No new scheduler or AO command
is needed, and no evolve root is restored.

Saved pages, retrieval and closed work cannot prove benefit or compounding.
The prior cold-reuse trial demonstrated no memory benefit and its failed/null
outcomes remain visible. Positive usefulness needs later task evidence. These
skills maintain external context; they do not train weights or promise
deterministic inference. [RPI traversal](docs/architecture/rpi-traversal.md) and
[ADR-0017](docs/adr/ADR-0017-loop-as-control-flow-not-knowledge.md) own the amended
behavior and the explicitly optional fixed-dispatch reference adapter.

## Why this shape

Coding agents are stochastic and can overstate completion. More orchestration
does not itself create trust. The smallest useful trust boundary is an explicit
behavior, exact content identity, and a fresh independent judgment whose limits
are recorded. AgentOps packages that boundary without taking over the user's
engineering system.
