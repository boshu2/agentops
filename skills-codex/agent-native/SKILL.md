---
name: agent-native
description: Operate explicit orchestrator, implementer
---
# Agent Native

Operate caller-selected agent sessions as explicit roles without turning the
runtime into AgentOps lifecycle authority.

## Roles

- **Orchestrator:** passes explicit packets and reports runtime facts.
- **Implementer:** may modify only its packet's declared subject.
- **Validator:** receives exact candidate content in a fresh, read-only context.
- **Scribe:** records runtime evidence without judging acceptance.

## Contract

1. Require an explicit packet, role, workspace, context identity, and evidence
   destination before starting a worker.
2. Prove runtime readiness and engagement from observable state; a successful
   prompt send is not proof of work.
3. Keep concurrent writers disjoint and isolated. Runtime coordination is not a
   claim, lease, queue, or completion state in AgentOps.
4. Record provider state, transcript references, artifacts, and terminal status.
5. Return runtime evidence to the caller. Do not convert provider retries,
   reconnects, idle states, or failures into Plan, Candidate, or verdict state.
6. A validator session may supply judgment to Validate, but only Validate writes
   `verdict.v2`.

NTM, native processes, Agent Mail, and Gas City are replaceable adapters. Use
them only when the caller selected that execution shape. A single local agent
pays no factory coordination cost.
