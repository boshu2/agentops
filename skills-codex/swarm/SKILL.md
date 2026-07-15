---
name: swarm
description: Dispatch explicit disjoint packets exactly
---
# Swarm

Swarm exposes one optional factory port:

```text
dispatch_once(explicit_disjoint_packets, executor)
  -> per-packet candidate | evidence | error
```

The caller supplies every complete packet, proves their write scopes disjoint,
and chooses the executor. Swarm dispatches each packet once, preserves packet and
context identities, collects results, and stops.

The reference implementation is [`scripts/dispatch_once.py`](scripts/dispatch_once.py).
It validates the entire explicit batch before the first call, invokes the supplied
executor exactly once for each packet, and returns executor exceptions as factual
per-packet errors.

Swarm does not select work, create packets, schedule from a backlog, persist a
queue, claim ownership, retry, validate, integrate, close, use Git, or deliver.
Executor failures remain executor evidence and cannot become core phase or
verdict state.
