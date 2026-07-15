---
title: "AgentOps as a Trust Boundary"
description: "How AgentOps identifies and independently judges one bounded agent-created change."
permalink: /trust-factory
last_reviewed: 2026-07-14
---

# AgentOps as a Trust Boundary

Generation is stochastic. AgentOps adds a small, inspectable boundary between
an agent's claim and a caller's decision to rely on it.

The boundary has four facts:

- the requested behavior and acceptance are explicit;
- the exact subject is identified by content, not by a mutable branch name;
- a distinct fresh context judges that subject against the acceptance;
- the result is written as a standalone content-addressed `verdict.v2`.

This is evidence, not permission to ship. AgentOps does not own Git, CI,
tracking, retries, queues, release, or recovery. The consumer system decides
what to do after PASS, FAIL, or NOT_PROVEN.

Optional Premortem, Council, and multi-model strategies can strengthen the
judgment selected by the caller. Optional Learn analysis can study collections
of past verdicts later. Neither changes the core lifecycle.

See [PRODUCT.md](../PRODUCT.md) and the
[operating loop](architecture/operating-loop.md).
