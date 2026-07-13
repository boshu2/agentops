# Team Coordination

Crank executes exactly one ready wave and returns evidence. Trackers are inputs
to wave selection and identifiers in the result; tracker closeout and repository
delivery remain caller-owned after Validate and Learn return.

## Beads Mode

1. Read the current wave's ready issues without changing their terminal state.
2. Create one runtime task per issue and preserve its issue identifier.
3. Copy declared dependencies into the runtime task graph.
4. Invoke `/swarm` once for the bounded wave.

Example runtime task:

```text
TaskCreate(
  subject="<issue-id>: <issue-title>",
  description="Implement <issue-id> using /implement. Return acceptance evidence.",
  activeForm="Implementing <issue-id>"
)
```

## TaskList Mode

Use the already-created pending, unblocked tasks and invoke `/swarm` once. The
same evidence and one-wave return boundary applies.

## Collect Wave Evidence

For every worker result, record:

- issue or task identifier;
- acceptance command and exit status;
- changed-file boundary and result artifact;
- worker status: accepted, failed, or blocked;
- remaining-work summary.

Do not translate accepted worker evidence into a tracker close, push, merge,
queue submission, or delivery verdict. Those are separate caller decisions.
Crank may report the tracker mutations that appear appropriate, but it does not
apply them.

## Return One Wave

- `DONE`: every slice in this wave has accepted implementation evidence.
- `PARTIAL`: some slices returned evidence and work remains.
- `BLOCKED`: no safe wave result can be produced within the bounded recovery.

These markers describe only this Crank invocation. They do not mean that an epic
is closed, a tracker is terminal, or work reached a repository destination.

The mandatory next handoff is the evidence packet to Validate. Validate returns
immutable proof to Learn; the orchestrator alone decides whether to continue,
retry, update tracking, or select a repository delivery mechanism.
