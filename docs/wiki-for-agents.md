---
title: "Evidence your agents can read"
description: "AgentOps packets and verdicts are portable files, not a hosted control plane."
permalink: /wiki-for-agents
last_reviewed: 2026-07-14
---

# Evidence your agents can read

AgentOps produces ordinary structured files: plans, candidates, content
manifests, evidence references, and verdicts. They can be inspected by people,
passed to another fresh context, retained under a repository's policy, or
exported into an external assurance system.

The core does not maintain an automatic wiki or knowledge corpus. Optional
specialist tools may search prior material, and an explicitly invoked Learn
step may analyze verdict collections later. Those capabilities remain outside
RPI correctness and never decide whether current work is valid.

This boundary keeps the durable asset portable without turning AgentOps into a
hosted control plane, tracker, queue, or delivery system.

See [the trust boundary](trust-factory.md) and [PRODUCT.md](../PRODUCT.md).
