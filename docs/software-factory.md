# Optional software-factory adapters

AgentOps defines portable packets and role boundaries. It does not run a
software factory or own its queue.

## Roles

| Role | Input | Output |
|---|---|---|
| Planner | caller intent | one PlanPacket |
| Implementer | exact PlanPacket | one CandidatePacket |
| Validator | exact Plan and Candidate in a fresh context | one durable verdict |

One runtime may fill the roles in separate contexts. PASS still requires
distinct nonempty author and validator context IDs plus an explicit freshness
attestation.

## Optional runtimes

Native Codex, NTM panes, Agent Mail, managed agents, cloud workers, and Gas City
may host these roles when the caller selects them. They may provide process
durability, isolation, or messages; their internal retries, pane state, queues,
and budgets never become AgentOps result state.

`dispatch_once(explicit_disjoint_packets, executor)` is the complete factory
adapter contract. The caller supplies every packet and the executor. The
adapter checks that write scopes are provably disjoint, dispatches each packet
once, returns candidate/evidence/error per packet, and stops. It does not
select, persist, retry, validate, integrate, commit, close, release, or deliver.

## Integration boundary

A factory may combine returned candidates using its own repository policy. Each
semantic candidate still needs exact identity and a fresh Validate verdict.
AgentOps does not convert factory completion, worker success, or deterministic
checks into PASS.

Git, trackers, pull requests, merge queues, CI, deployment, and release remain
owned by the caller's environment.

## Related contracts

- [Operating loop](architecture/operating-loop.md)
- [Optional dispatch](contracts/orchestration-ports.md)
- [Agent workflow](agent-workflow-reference.md)
