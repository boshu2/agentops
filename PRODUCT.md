---
last_reviewed: 2026-07-14
---

# AgentOps

AgentOps helps fallible coding agents produce work that another fresh context
can independently judge.

The product boundary is deliberately small:

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

## Proven floor

AgentOps provides:

- behavior-first intent with normal and edge acceptance examples;
- one bounded RED -> GREEN -> refactor experiment;
- deterministic content identity independent of Git;
- one fresh, author-distinct semantic judgment;
- a standalone, content-addressed `PASS | FAIL | NOT_PROVEN` verdict.

No fresh evidence-backed verdict means the experiment is not proven. The
verdict's context separation is explicitly attested; it is not claimed as
cryptographic proof of model isolation.

## What AgentOps does not own

AgentOps is not a new GitLab, CI service, tracker, merge queue, delivery system,
release manager, scheduler, or autonomous retry controller. It does not own:

- retries, budgets, queues, claims, leases, work ownership, or next actions;
- Git commits, branches, pushes, PRs, merges, rollback, closure, or release;
- the caller's decision after a verdict;
- mandatory provenance or learning on the validation critical path.

Repositories and callers keep those policies. They may use direct pushes, PRs,
hosted CI, merge queues, cloud agents, or custom release systems without asking
AgentOps for delivery permission.

## Product surfaces

Four load-bearing skills define the core:

| Skill | Responsibility |
|---|---|
| `rpi` | dispatch Plan, Implement, and Validate once; report and stop |
| `plan` | shape acceptance, evidence, and write scope |
| `implement` | run one bounded experiment and produce a candidate |
| `validate` | independently judge exact content and persist the verdict |

`learn` remains an optional off-path consumer of verdict collections. Strategy
skills such as Premortem, Postmortem, Council, and idea genies add judgment when
the caller wants it. Factory/runtime adapters such as NTM, Agent Mail, Gas City,
and swarms may provide roles and dispatch. None is a correctness or lifecycle
authority.

The `ao` CLI supplies deterministic repository checks and generic read-only or
record helpers where useful. Semantic validation belongs to the Validate skill,
not a CLI state machine.

## Sovereign evidence

The durable verdict is plain JSON under caller-controlled storage. It binds
acceptance, exact subject content, author and validator identities, criterion
results, evidence, checked scope, and omissions. A generic provenance ledger
may copy or reference it later, but ledger availability is never required for
validity.

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
