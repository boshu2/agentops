# The AgentOps Seed

The product seed is the smallest useful trust boundary for agent-created work:

1. Plan refines one active behavior, acceptance examples, non-goals, evidence,
   and write scope in the caller-owned intent source.
2. Implement performs one bounded experiment while the runtime derives the
   subject manifest, changed paths, and check receipts.
3. Validate independently judges the exact content manifest once.
4. The durable `verdict.v2` artifact records what was and was not proven.

RPI composes those phases exactly once and reports the result. A repository may
add its own tracker, Git workflow, CI, release policy, factory runtime, or
learning process around that seed. None is required for AgentOps correctness.

The seed works in a non-Git directory without the `ao` binary. Its only durable
core state is the resolved intent, derived content identity, and
content-addressed verdict.

See [the operating loop](architecture/operating-loop.md) and
[product boundary](../PRODUCT.md).
