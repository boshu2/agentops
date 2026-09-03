---
last_reviewed: 2026-08-17
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

## Learning thesis

The long-term hypothesis remains that recurring validated mistakes can become
better context, tests, or deterministic checks. That compounding claim is not
the core completion boundary. Learning runs off-path, cites distinct verdicts
and findings, and proposes evidence for later evaluation; it does not silently
promote policy or control another experiment.

## Why this shape

Coding agents are stochastic and can overstate completion. More orchestration
does not itself create trust. The smallest useful trust boundary is an explicit
behavior, exact content identity, and a fresh independent judgment whose limits
are recorded. AgentOps packages that boundary without taking over the user's
engineering system.
