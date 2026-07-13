# FIRE — One-Wave Execution

FIRE is Crank's phase-local execution pattern. It produces evidence for one
bounded wave; it is not a repository integration loop or a tracker-closing loop.

```text
FIND -> IGNITE -> REAP -> ESCALATE -> RETURN
```

## FIND

Read the accepted plan and current tracker or TaskList state. Select only ready
slices whose wave-validity rows pass. Reading tracker state does not authorize
Crank to make it terminal.

## IGNITE

Dispatch the selected slices through `/swarm`, with explicit write ownership and
fresh worker context. A conflicting write scope makes the affected slices
sequential.

## REAP

Collect worker output and run each slice's deterministic acceptance. Preserve:

- the task identifier;
- acceptance command and exit status;
- changed files and write-scope result;
- evidence artifact location;
- accepted, failed, or blocked status.

Reap ratchets knowledge about the wave. It does not push, merge, submit to a Git
queue, close tracker state, or declare repository delivery.

## ESCALATE

Classify failures and return their evidence to the orchestrator. Bounded
within-wave recovery may repair a transient worker execution, but Crank never
starts a changed wave, silently replans, or crosses into another lifecycle
phase.

## RETURN

Return exactly one of `DONE`, `PARTIAL`, or `BLOCKED` for this invocation, plus
the wave evidence. `DONE` means the selected wave has accepted implementation
evidence. It does not mean the epic, tracker, Git history, or delivery workflow
is complete.

The caller sends the evidence to Validate, then Learn. Only the orchestrator may
choose another wave, apply tracker updates, or invoke repository-selected
delivery afterward.
