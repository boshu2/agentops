# Component map

| Bounded context | Owns | Does not own |
|---|---|---|
| Intent | Caller-owned bead or issue, derived acceptance digest, write scope | A duplicate AgentOps planning artifact |
| Experiment | One bounded implementation and runtime-derived factual evidence | Model-authored candidate packets, retry, Git, delivery |
| Identity | Deterministic `subject-manifest.v1` | Commit, branch, or tracker authority |
| Judgment | Fresh-context evaluation and `verdict.v2` | Continuation, closure, release |
| Evidence | Atomic content-addressed verdict storage and generic provenance | Admission or lifecycle state |
| Repository checks | Deterministic `ao gate check` execution | Semantic judgment |
| Optional adapters | Explicit `dispatch_once` and runtime execution | Selection, retries, integration, validation |

The only hard skill graph edge is `rpi -> {plan, implement, validate}`. Learn
and every strategy, specialist, runtime, and factory adapter are off-path.
