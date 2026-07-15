---
name: using-gc
description: Operate an explicitly selected Gas City as
---
# Using GC

Use Gas City only when the caller explicitly selects it. Treat it as a
replaceable execution adapter, not a completion or correctness boundary.

1. Accept complete explicit packets and a caller-selected city/executor.
2. Map each packet to one role and disjoint workspace.
3. Observe runtime state and return candidate, evidence, or error per packet.
4. Keep GC quests, attempts, stalls, and internal close state inside the
   substrate. They do not become Plan, Candidate, RPI, or verdict state.
5. A fresh GC judge may provide evidence to Validate; only Validate writes
   `verdict.v2`.

This skill performs no automatic selection, retry, semantic validation, Git,
integration, closure, release, or delivery.
