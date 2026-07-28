# Component map

| System responsibility | Owns | Does not own |
|---|---|---|
| Product and fitness | Caller-visible outcome, product boundary, terminal evidence, read-only measurements | Experiment selection or semantic PASS |
| Campaign | Goal graph, next experiment, cumulative budgets, ratchet, breakers, terminal campaign report | Candidate judgment or verdict mutation |
| Intent | One experiment in a caller-owned bead or issue, single-mint exact snapshot, stable criterion IDs, write scope | A duplicate AgentOps planning artifact or campaign graph |
| Experiment | One bounded candidate, complete actual changed paths, factual checks, and observed effect receipts | Repair loops, Git, delivery, or later work selection |
| Identity | Exact intent, before/final subject manifests, and proof-contract digests | Commit, branch, or tracker authority |
| Judgment | Fresh-context evaluation and `verdict.v2` under an activated proof contract | Candidate edits, self-activation, continuation, closure, or release |
| Evidence | Atomic content-addressed verdict storage, proof transitions, and generic provenance | Admission or lifecycle state |
| Capability evolution | Recurrence, causal, toil, and pattern observations; reusable-capability proposals | Automatic promotion or policy mutation |
| Repository checks | Deterministic `ao gate check` execution | Semantic judgment |
| Optional adapters | Explicit packet execution, runtime coordination, and runtime facts | Selection, retries, integration, or validation authority |

The only hard skill graph edge is `rpi -> {plan, implement, validate}`. A Goal
may invoke several RPIs but remains outside that hard graph. Learn and every
strategy, specialist, runtime, and factory adapter are optional seams described
by [the skill system architecture](../contracts/skill-ports-and-adapters.md).
