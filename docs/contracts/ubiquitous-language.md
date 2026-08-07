# Ubiquitous language

## Product architecture

| Term | Meaning |
|---|---|
| Operations layer | The product category: the portable layer that makes heterogeneous agentic engineering systems interoperate semantically. |
| Federated integration graph | The topology: caller-owned intent, source systems, agents, factories, checks, and judgments remain separate nodes joined by typed handoffs. |
| Semantic work-and-proof protocol | The interoperability contract: exact intent, exact subject, evidence, fresh judgment, and honest outcomes. |
| RPI traversal | One standard path through the graph: Plan -> Implement -> fresh Validate -> report and stop. |

The traversal is RPI. The graph is the topology. The protocol defines
interoperability. The operations layer is the product.

`loop` may describe a local control structure inside one tool or a caller's own
campaign. It is not the product category and not the global architecture.

## Supporting terms

| Term | Meaning |
|---|---|
| Context source | A system that owns queryable evidence — the tracker (Beads), Git, CASS, CM, or another caller-selected store. AgentOps cites it with source identity and freshness; it never absorbs its authority. |
| Execution orchestrator | A caller-selected system that schedules, runs, and retries work — a Goal, Mayor, factory, CI, or merge queue. It owns execution lifecycle and never semantic judgment. |
| Software factory | An execution orchestrator with persistent internal sessions and roles (Gas City, NTM rigs, swarms). Operated only through its own doors. |
| Projection | A rebuildable derived view with a named consumer, generator, inputs, exact source identities, and freshness. Deletable without changing semantic behavior; never authority. |
| Fresh judgment | One `PASS | FAIL | NOT_PROVEN` issued over exact content by a context distinct from the candidate author. |
| Runtime completion | A runtime or factory's own report that execution finished. A fact about execution, never validation or delivery proof. |
| Repository check | A deterministic command outcome (`ao gate`, tests, linters). It proves facts; a fresh context judges meaning. |

## Experiment terms

| Term | Meaning |
|---|---|
| Intent | The caller's requested behavior and constraints. |
| Intent source | The caller-owned bead, issue, or conversation containing behavior, acceptance, and scope. |
| Intent digest | A runtime-derived digest used to detect acceptance drift; not a model-authored artifact. |
| Subject manifest | Deterministic content identity independent of Git. |
| Fresh validator | A declared context identity distinct from the candidate author's identity. |
| Verdict | `PASS`, `FAIL`, or `NOT_PROVEN` over one acceptance digest and one subject digest. |
| RPI report | `NOT_PLANNED`, `NOT_BUILT`, or the semantic verdict, followed by stop. |
| Revision | A change to the caller-owned intent source followed by a new invocation. |
| Strategy | Optional advice such as premortem, postmortem, council, or an idea genie. |
| Adapter | Optional transport or runtime that cannot change core semantics. |

## Forbidden conflations

- An **operations layer is not an execution orchestrator**: AgentOps connects
  systems semantically; it does not schedule, retry, or own their work.
- An **RPI result is not a factory result**: a traversal's verdict and a
  factory's completion report are different facts from different owners.
- **Check success is not semantic PASS**: deterministic green proves facts;
  only a fresh validator judges meaning.
- A **projection is not source authority**: derived views cite their sources
  and can always be deleted and rebuilt.

Avoid using admission, claim, lease, queue, close, land, release, or delivery as
AgentOps lifecycle states. Repositories and callers may use those words in their
own systems; AgentOps does not own those transitions.
