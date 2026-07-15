# Bounded Work Without Lifecycle Control

AgentOps deliberately keeps its core experiment small:

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

Plan names one active behavior and its write scope. Implement performs one
bounded experiment. Validate independently judges the exact content described
by the candidate manifest. RPI dispatches each phase once and reports the
result without deciding what happens next.

Fresh context is valuable because the author cannot turn its own claim into the
binding semantic verdict. More agents are not automatically better. One fresh
validator is the default; Council, Dueling Idea Genies, Swarm, NTM, Agent Mail,
and other runtimes are optional caller-selected strategies or adapters.

Optional dispatch may execute explicit disjoint packets once. It does not infer
a queue, claim work, retry failures, integrate branches, validate candidates,
or deliver results. The caller or an external software factory owns those
decisions.

Git worktrees, trackers, CI systems, merge queues, and release pipelines may be
useful around an AgentOps experiment. They remain the caller's infrastructure,
not AgentOps lifecycle state.
