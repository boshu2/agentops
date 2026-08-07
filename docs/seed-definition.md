# The AgentOps Seed

The product seed is the smallest useful trust boundary for agent-created work:

1. Plan refines one active behavior, acceptance examples, non-goals, evidence,
   and write scope in the caller-owned intent source.
2. Implement performs one bounded experiment while the runtime derives the
   subject manifest, changed paths, and check receipts.
3. Validate independently judges the exact content manifest once and reports
   what was and was not proven.
4. A caller or declared downstream consumer may request a durable `verdict.v2`
   representation of that result.

RPI composes those phases exactly once and reports the result. A repository may
add its own tracker, Git workflow, CI, release policy, factory runtime, or
learning process around that seed. None is required for AgentOps correctness.

The seed works in a non-Git directory without the `ao` binary. The resolved
intent and derived content identity can be passed directly between contexts;
content-addressed verdict storage is an optional evidence surface.

See [the RPI traversal](architecture/rpi-traversal.md) and
[product boundary](../PRODUCT.md).
