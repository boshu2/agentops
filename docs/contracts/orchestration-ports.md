# Optional dispatch adapter

AgentOps has one optional factory port:

```text
dispatch_once(explicit_disjoint_packets, executor)
  -> per-packet candidate | evidence | error
```

The caller supplies every packet and the executor. Each packet is dispatched at
most once. Parallel dispatch is legal only when the caller has supplied disjoint
write scopes.

The adapter owns no selection, queue, persistence, retry, validation, Git,
integration, closure, release, or delivery. A runtime attempt count, pane state,
message acknowledgement, or substrate error never becomes RPI or verdict state.

NTM, Agent Mail, Codex Exec, managed agents, and other runtimes are driven
adapters behind this boundary. None is required for Plan, Implement, Validate,
or RPI.
