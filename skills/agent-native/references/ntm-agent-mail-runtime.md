# NTM + Agent Mail runtime

Use this adapter when persistent pane roles, attachability, or work that outlives
the current session justifies a factory. The lifecycle and authority stay in
`agent-native`; this page only maps that contract onto the reference tools.

## Capability-first startup

1. Inspect `ntm --robot-help` and the narrow robot help for the operation you
   need. Do not assume an older local flag set.
2. Spawn with the discovered structured command, then verify the returned pane
   map, expected working directory, ref, and runtime identity.
3. Tail or snapshot the pane until there is evidence of engagement. A successful
   send is delivery evidence, not work evidence.
4. Apply the portable suspect → bounded nudge → replace rule from
   `agent-lifecycle.md`. Never convert a missing provider into success.

The checked-in adapter is `cli/internal/adapters/agentworker_ntm`. It covers
spawn, send, tail, snapshot, interrupt, and evidence capture without moving
policy into NTM.

## Add Agent Mail only when coordination is real

A single pane needs no mailbox. With multiple live actors, discover the Agent
Mail capabilities before mutation, register identity, and reserve only the
declared write scope. Use `am mail send` for the CLI path (not `am send`), attach
messages to the bead/thread, acknowledge handoffs, and release reservations at
retirement.

Agent Mail does not decide topology, readiness, acceptance, or landing. The
checked-in adapter is `cli/internal/adapters/agentmail_cli`; it implements the
portable port in `cli/internal/ports/agent_mail.go`.

## Factory roles

- Orchestrator: topology, dispatch, breakers, and acceptance routing.
- Worker: one bead and one non-overlapping write scope.
- Verifier: immutable input, fresh context, read-only evidence.
- Scribe: evidence and handoff records without acceptance authority.
- Heartbeat: liveness observation without authorship.

Gas City may consume these same ports when the operator explicitly selects a
city. It is additive; neither NTM nor GC silently falls back to the other.
