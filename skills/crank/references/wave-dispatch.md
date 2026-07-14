# Wave Dispatch

Dispatch one selected wave of one accepted behavioral leaf. The direct
single-writer path is default; parallelism is an explicit optimization for
disjoint lanes.

## 1. Pin the selected wave

Before mutation, record:

```bash
: "${RPI_RUN_ID:?RPI run id is required for evidence correlation}"
WAVE_START_SHA="$(git rev-parse HEAD)"
```

The dispatch packet must bind leaf and wave identity, accepted plan and
Premortem, write scope, next failing acceptance proof, rollback, and base SHA.
Missing or mismatched input returns `BLOCKED` evidence. Crank does not choose a
different objective or create control state around dispatch.

## 2. Build the minimum worker packet

Include:

- leaf and wave identity;
- exact next failing proof and immutable tests in GREEN mode;
- write scope and rollback;
- `metadata.issue_type`;
- executable acceptance and any surface-specific checks; and
- relevant standards or prior evidence already cited by the plan.

Do not inject broad knowledge dumps, duplicate shared-note archives, or every
available standard.

## 3. Execute directly by default

Use one direct `/implement` worker, or let the current leaf owner implement the
wave. The lead runs external acceptance and is the lead-only committer after the
result passes. The implementer never self-issues the final semantic verdict.

For test-first work:

1. establish the contract and right-reason RED proof;
2. make the smallest implementation turn GREEN; and
3. refactor under unchanged green tests.

These may be distinct waves inside the same leaf. They do not each receive
Validate or Learn.

## 4. Parallel dispatch only for proven disjoint lanes

Use `/swarm` or another runtime-native multi-worker transport only when at least
two lanes have disjoint source and generated write scopes, explicit owners,
integration order, and discard paths. Each lane uses an isolated worktree.
Serialize any shared migration, schema, contract, CLI, registry, or generated
surface. Availability of NTM, panes, or subagents is not permission to use them.

## 5. Return evidence

After the lead integrates the wave, run the targeted acceptance once and follow
[wave-completion.md](wave-completion.md). Return canonical checkpoint identity,
introduced/base-attributed failures, material plan deltas, and remaining work.
Crank does not invoke Validate, Learn, Premortem, delivery, or tracker closeout.
Only RPI records the next disposition.
