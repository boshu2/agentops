# Component map

AgentOps is the operations layer for agentic engineering. Its architecture is
a federated integration graph: caller-owned intent, context sources, coding
agents, software factories, deterministic checks, and independent judgment
stay separate nodes joined by typed handoffs. AgentOps supplies the semantic
work-and-proof protocol between the nodes; it absorbs none of their state.

## Graph nodes and typed handoffs

| Node | Owned by | Handoff toward AgentOps | Handoff away from AgentOps |
|---|---|---|---|
| Caller | The requesting human or system | One resolved intent (bead, issue, or conversation snapshot) | The RPI report; every later decision |
| Context source (tracker, Git, CASS, CM) | Its own system | Cited evidence with source identity and freshness | Nothing — AgentOps never writes its authority |
| Coding agent | The selected runtime | One bounded candidate plus factual receipts | The same candidate, judged, never edited by the judge |
| Software factory / execution orchestrator | The selected factory | Native runtime facts (completion, failure, logs) | Verdicts as read-only evidence; never dispatch or repair |
| Deterministic checks | The executable that runs them | Factual check receipts | Nothing — checks prove facts, not meaning |
| Fresh validator | A context distinct from the author | One `PASS \| FAIL \| NOT_PROVEN` over exact content | Optional `verdict.v2` for a declared consumer |

The standard traversal across these nodes is the
[RPI traversal](rpi-traversal.md): Plan -> Implement -> fresh Validate ->
report and stop. Vocabulary is fixed in
[the ubiquitous language](../contracts/ubiquitous-language.md).

## Responsibilities

| System responsibility | Owns | Does not own |
|---|---|---|
| Product and fitness | Caller-visible outcome, product boundary, terminal evidence, read-only measurements | Experiment selection or semantic PASS |
| Campaign | Goal graph, next experiment, cumulative budgets, ratchet, breakers, terminal campaign report | Candidate judgment or verdict mutation |
| Intent | One experiment in a caller-owned bead or issue, single-mint exact snapshot, stable criterion IDs, write scope | A duplicate AgentOps planning artifact or campaign graph |
| Experiment | One bounded candidate, complete actual changed paths, factual checks, and observed effect receipts | Repair loops, Git, delivery, or later work selection |
| Identity | Exact intent, before/final subject manifests, and proof-contract digests | Commit, branch, or tracker authority |
| Judgment | Fresh-context evaluation over exact intent and subject; optional `verdict.v2` persistence | Candidate edits, self-activation, continuation, closure, or release |
| Evidence | Optional atomic content-addressed verdict storage and generic provenance | Admission or lifecycle state |
| Capability evolution | Recurrence, causal, toil, and pattern observations; reusable-capability proposals | Automatic promotion or policy mutation |
| Repository checks | Deterministic `ao gate check` execution | Semantic judgment |
| Optional adapters | Explicit packet execution, runtime coordination, and runtime facts | Selection, retries, integration, or validation authority |

The only hard skill graph edge is `rpi -> {plan, implement, validate}`. A Goal
may invoke several RPIs but remains outside that hard graph. Learn and every
strategy, specialist, runtime, and factory adapter are optional seams described
by [the skill system architecture](../contracts/skill-ports-and-adapters.md).
