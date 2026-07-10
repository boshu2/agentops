---
name: agent-native
description: Run persistent software-factory workers
---
# Agent Native

Drive persistent role-shaped workers without making a pane manager, messaging
tool, or city engine the product boundary. The Go `AgentWorker`/`AgentSession`
contract owns start, attach, nudge, cancel, stream, transcript, artifacts, and
terminal state. NTM, native processes, and Gas City are replaceable adapters.

## Route before starting a factory

Use persistent workers only when attachability, role continuity, or work that
outlives the current session materially helps. Otherwise choose one native
Codex agent, a bounded in-session fanout, or a sequential bead chain. Refuse a
factory when work is one-shot, shared write scopes collide, or the operator did
not choose an out-of-session substrate.

The stable role vocabulary is:

- **Orchestrator** owns topology, dispatch, breakers, and acceptance routing.
- **Worker** owns one bead and one write scope.
- **Verifier** is fresh, read-only, and owns no production fix.
- **Scribe** records evidence and handoff without deciding acceptance.
- **Heartbeat** observes liveness and engagement without authoring work.

A pane may fill one role at a time. Role names do not create new authority; the
bead, port, and membrane contracts do.

## Portable lifecycle

1. **Contract:** bind bead id, Given/When/Then, role, workspace/write scope,
   evidence path, timeout, nudge ceiling, and replacement ceiling.
2. **Orient:** load repository rules, source precedence, relevant whole-loop
   skill, and acceptance before delivering work.
3. **Ready:** prove the runtime is reachable and the worker is at the expected
   workspace/ref. A successful send is not readiness.
4. **Engage:** deliver one whole AgentOps loop skill or bead slice. Observe a
   transcript/state change that proves engagement.
5. **Observe:** use provider state, transcript deltas, artifacts, and terminal
   status. Do not infer completion from prompt acknowledgement or an idle pane.
6. **Recover:** first quiet observation marks suspect; the next receives one
   bounded nudge; another quiet observation replaces the worker. Progress
   resets suspicion. Provider loss never becomes success.
7. **Prove:** require usable persisted artifacts and the slice's deterministic
   acceptance. Reviewer lanes hand evidence to `pawl-review`; workers do not
   self-grade.
8. **Handoff/retire:** acknowledge ownership transfer, release reservations,
   record evidence, and explicitly stop or retain the attachable session.

NTM mechanics live in [`ntm`](../ntm/SKILL.md) and the real adapter at
`cli/internal/adapters/agentworker_ntm`. Agent Mail mechanics live in
[`agent-mail`](../agent-mail/SKILL.md) and `agentmail_cli`. Use Agent Mail only
for multiple live actors that need identity, file reservations, messages,
acknowledgements, or handoffs. A single worker pays no coordination tax.
The CLI adapter discovers its surface first, then uses `am mail send` (there is
no flat `am send`) when the MCP tool surface is unavailable.

Gas City remains an explicit operator choice. Its driver may delegate a bounded
role through the same ports; neither substrate is an automatic fallback for the
other, and GC's membrane remains its close door.

Read [the lifecycle contract](references/agent-lifecycle.md) before implementing
another adapter. For the reference factory route, use the
[NTM + Agent Mail runtime](references/ntm-agent-mail-runtime.md).
