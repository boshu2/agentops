---
last_reviewed: 2026-09-16
---

# AgentOps

**Agent work you can verify and build on.**

**AgentOps means agent operations:** applying years of DevOps experience to how
coding agents plan, implement, validate, and hand off work.

AgentOps helps developers using Claude Code and Codex turn a requested behavior
into an independently checked change. It applies established engineering
practices through optional skills and supporting CLI tools: behavior-driven
planning, shared domain language, implementation checks, and fresh review of
the actual result.

The product aims to make engineering effort leave reusable improvements behind.
A repair can add a regression test; a tool can remove repeated work; a recorded
decision can preserve context for the next agent. Those assets stay with the
caller's engineering system across sessions and model changes.

AgentOps is the **operations layer for agentic engineering**. It connects intent,
coding agents, checks, and independent judgment while existing tools retain
work, execution, and delivery. The standard path is:

```text
Accepted intent -> native implementation and checks -> fresh independent judgment -> finish
```

## Engineering practices in the workflow

Planning uses **behavior-driven development (BDD)**. State what the caller can
observe before implementation, including consequential boundaries. Given/When/Then
is useful for branching behavior; a short example in the existing conversation
or issue is enough. Implementation and validation use that same accepted example.
Tests written after coding may extend the evidence but cannot redefine the promise.

**Domain-driven design (DDD)** supplies shared language and rule ownership.
Use the caller's established terms across acceptance examples, code, tests, and
review. A bounded context identifies where a term has an agreed meaning; when
terms cross contexts, make the translation explicit. The Domain skill helps
resolve ambiguity without requiring a new glossary for every task.

The [Practice Registry](PRACTICE-REGISTRY.md) traces these choices to established
engineering practices, including design by contract, testing, refactoring, and
delivery. AgentOps' approach developed independently from applying DevOps
experience to agents. Other projects have converged on similar practices; their overlap
makes them useful peers in the ecosystem. A practice's usefulness for an agent
still needs evidence from the work.

## What the product provides

AgentOps provides:

- behavior-first intent with normal and edge acceptance examples in the caller's
  domain language;
- bounded implementation and direct repair under the caller's accepted intent;
- deterministic content identity independent of Git;
- one fresh, author-distinct semantic judgment;
- a `PASS | FAIL | NOT_PROVEN` result with optional content-addressed storage.

No fresh evidence-backed judgment means the experiment is not proven. Context
separation is explicitly attested; it is not claimed as cryptographic proof of
model isolation.

Planning, implementation, and validation are responsibilities that can travel
between native sessions and selected factories. Final judgment must come
from a fresh context distinct from the author. Three permanently separate
agents are not required. Review defaults to the author's model family;
cross-model review is an explicit choice. Model diversity alone proves neither
independence nor correctness, and a verdict does not authorize a merge.

## Engineering that subsequent work can build on

The goal is durable progress in the caller's system. For example, an accepted
behavior that a completed Job must not repeat its side effect can lead to a
repair and a regression test. Later changes inherit that check. A record of
the request, change, and validation lets another engineer or agent understand
what was established and what remains unresolved.

Git owns source and history; the caller's tracker owns work and handoffs.
Requested review evidence uses caller-selected durable storage. The topology
is a federated integration graph: these systems remain separate authorities
connected by references and typed handoffs. Durability depends on their actual
protection and recovery controls, not on choosing Git for every record.

Reusable assets are the mechanism for engineering to compound. Demonstrated
benefit requires subsequent work to use them successfully, with maintenance
and unnecessary review costs included. Commit counts, saved pages, and verdict
counts alone do not measure that benefit. There is no mandatory lesson or new
artifact for every completed task.

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
| `domain` | resolve domain meanings and rule ownership used throughout the change |
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

Saved pages, retrieval and closed work cannot prove memory benefit or general
compounding. The prior cold-reuse trial demonstrated no memory benefit and its failed/null
outcomes remain visible. Positive usefulness needs later task evidence. These
skills maintain external context; they do not train weights or promise
deterministic inference. [RPI traversal](docs/architecture/rpi-traversal.md) and
[ADR-0017](docs/adr/ADR-0017-loop-as-control-flow-not-knowledge.md) own the amended
behavior and the explicitly optional fixed-dispatch reference adapter.

## Evidence and claim limits

The [September coding pilot](https://github.com/boshu2/agentops/pull/1125) retained
24 starts and produced eight comparable pairs across six task families, with
no observed paired endpoint difference. Its separate eight-start memory
experiment did not demonstrate incremental benefit. Those bounded results
inform selective use of guidance; they establish neither equivalence nor
general productivity improvement.

[Independent review caught incomplete acceptance coverage](https://github.com/boshu2/agentops/pull/1129)
in the trial readout, leading to a repair and regression test. That is a concrete
example of review producing a reusable check. It does not establish the net
time or cost benefit of the whole workflow.

## Why this shape

The [12 Factor AgentOps principles](https://www.12factoragentops.com/factors)
are the original inspiration: preserve the operating properties that make
agent work reliable as tools and runtimes change. The Practice Registry gives
the broader engineering lineage behind the selected techniques.

Coding agents are stochastic and can overstate completion. More orchestration
does not itself create trust. The smallest useful trust boundary is an explicit
behavior in shared domain language, exact content identity, and a fresh
independent judgment whose limits are recorded. AgentOps packages those
practices so the resulting improvements remain usable in the caller's
engineering system.
