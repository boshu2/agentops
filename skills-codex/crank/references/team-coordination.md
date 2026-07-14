# Team Coordination

Crank executes one ready wave of one leaf and returns canonical evidence. The
leaf owner is the direct implementer by default. Trackers identify the work;
Crank does not close them or deliver the repository.

## Direct route

1. Resolve the admitted leaf and exact next failing proof.
2. Give the current owner the bounded write scope, acceptance, and rollback.
3. Implement one wave.
4. Have the lead run targeted deterministic acceptance and commit the accepted
   wave.
5. Return the canonical checkpoint and remaining-plan facts to RPI.

Do not create a runtime task, spawn an agent, or invoke Swarm merely to preserve
a workflow shape.

## Explicit parallel route

Use `/swarm` only when the admitted plan proves at least two lanes have disjoint
source and generated write scopes, separate owners, integration order, and
discard paths. Each writer uses its own worktree. Any shared migration, schema,
contract, CLI, registry, or generated surface serializes the lanes.

## Evidence

For each direct or admitted parallel result, record:

- leaf/wave and owner identity;
- acceptance command, exit status, and exact-input receipt;
- changed files and integrated SHA;
- accepted, failed, or blocked status; and
- whether acceptance, dependencies, write scope, or risk changed.

`DONE`, `PARTIAL`, and `BLOCKED` describe only the Crank wave. They do not mean
the leaf is validated, learned, delivered, or tracker-terminal.

RPI may admit another unchanged wave directly. A material plan change returns to
Discovery/Premortem. A completed leaf freezes for one Validate/Learn transaction;
an incomplete three-wave/90-minute soft boundary writes resume evidence and
stops without proof authorization.
