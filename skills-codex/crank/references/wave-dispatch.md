# Wave Dispatch

Dispatch one admitted wave of one behavioral leaf. The direct single-writer path
is default; parallelism is an explicit optimization for disjoint lanes.

## 1. Atomically admit the wave

RPI owns the one persistent run governor. Crank receives its stable identity and
measured charges and must obtain a durable admission before mutation:

```bash
: "${RPI_RUN_ID:?RPI run id is required}"
: "${RPI_GOVERNOR_STATE_DIR:?persistent governor state dir is required}"
: "${RPI_REVIEWER_TOKENS:?reviewer-token meter is required}"
: "${RPI_ELAPSED_SECONDS:?elapsed-time meter is required}"
: "${RPI_REVIEW_CONTEXTS:?review-context meter is required}"
: "${RPI_DETERMINISTIC_EXECUTIONS:?deterministic-execution meter is required}"

ADMISSION_JSON="$(python3 skills/rpi/scripts/run-governor.py admit \
  --state-dir "$RPI_GOVERNOR_STATE_DIR" \
  --run-id "$RPI_RUN_ID" \
  --action crank-wave \
  --reviewer-tokens "$RPI_REVIEWER_TOKENS" \
  --elapsed-seconds "$RPI_ELAPSED_SECONDS" \
  --review-contexts "$RPI_REVIEW_CONTEXTS" \
  --deterministic-executions "$RPI_DETERMINISTIC_EXECUTIONS")" || exit 1

test "$(jq -r '.authorized' <<<"$ADMISSION_JSON")" = true || exit 1
RPI_ADMISSION_ID="$(jq -r '.admissions[-1].id' <<<"$ADMISSION_JSON")"
WAVE_START_SHA="$(git rev-parse HEAD)"
```

Do not reset the run, create a local counter, or turn a soft tranche boundary
into HOLD/ANDON.

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

## 4. Parallel dispatch only when admitted

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
