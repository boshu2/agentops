# Persistent Pull-Flow Governor

This is the sole admission, budget, breaker, helper, and disposition contract
for one RPI run. Discovery, Crank, Validate, Learn, and delivery adapters may
request or report work, but they do not create private wave, retry, attempt, or
helper counters.

## Persistent run identity

Every run has a stable `run_id` and one state file under
`.agents/rpi/run-governor/<run_id>.json`. The state conforms to
`../schemas/run-governor.schema.json`. A fresh process resumes that file; it
must not initialize a replacement when the run already exists.

Initialize one run before dispatching any work:

```bash
python3 skills/rpi/scripts/run-governor.py init \
  --state-dir .agents/rpi/run-governor \
  --run-id "$RPI_RUN_ID" \
  --max-waves 3 \
  --max-reviewer-tokens "$RPI_MAX_REVIEWER_TOKENS" \
  --max-elapsed-seconds "$RPI_MAX_ELAPSED_SECONDS" \
  --max-review-contexts "$RPI_MAX_REVIEW_CONTEXTS" \
  --max-deterministic-executions "$RPI_MAX_DETERMINISTIC_EXECUTIONS"
```

Three waves or 90 minutes, whichever comes first, is the routine bounded
tranche ceiling. The token, elapsed-time, review-context, and
deterministic-execution ceilings are always declared; the governor does not
invent them. Missing or invalid ceilings are non-authorizing.

## Admit before dispatch

Crank waves, semantic reviews, and deterministic proof runs all use the same
inbound port:

```bash
python3 skills/rpi/scripts/run-governor.py admit \
  --state-dir .agents/rpi/run-governor \
  --run-id "$RPI_RUN_ID" \
  --action "$RPI_ACTION" \
  --reviewer-tokens "$RPI_REVIEWER_TOKENS" \
  --elapsed-seconds "$RPI_ELAPSED_SECONDS" \
  --review-contexts "$RPI_REVIEW_CONTEXTS" \
  --deterministic-executions "$RPI_DETERMINISTIC_EXECUTIONS"
```

The command takes an exclusive per-run lock, validates the complete prior
state, checks the projected charge, appends the admission, fsyncs a temporary
file, atomically replaces the run state, fsyncs the directory, and only then
returns `authorized:true`. Complete validation means every declared schema
constraint plus semantic consistency: exact object keys, types, admission
sequence and identity, action-to-wave charge, accumulated usage, helper
history, protected-disposition metadata, and authorization state. Dispatch is
illegal without that durable receipt. Concurrent or fresh-process callers
therefore cannot oversubscribe the run.

All four meters are mandatory on every request, including explicit zeroes.
Missing state, corrupt state, a missing meter, an unknown action, or malformed
control input returns nonzero with `authorized:false` and a neutral `NOTE`
response; it does not mutate the run or manufacture `ANDON`. A meter blocks an
action only when that action has a positive projected charge and the resulting
usage would exceed its ceiling. Saturating the wave meter therefore blocks
another Crank wave but not a zero-wave semantic review. A caller must stop
admitting new waves before a soft tranche ceiling is spent, persist resume
state, and return `PARTIAL`; that boundary is neither HOLD nor ANDON. A
genuinely exceeded hard wave, time, token, context, deterministic-execution,
cost, or quota ceiling sets `ANDON` and `helper.allowed:false`; a spent ceiling
never buys recovery work.

## One disposition language

Only these run dispositions exist:

| Disposition | Meaning | Legal next move |
|---|---|---|
| `NOTE` | Nonblocking evidence or a recorded admission | Continue within the admitted action |
| `REPAIR` | Concrete local defect or a helper-provided new approach | Return to the earliest repairable move |
| `REPLAN` | Evidence invalidates the slice or approach | Return to Discovery, then Premortem the changed plan |
| `HOLD` | A stuckness breaker tripped while recovery budget remains | Request the one helper for that blocker class |
| `ANDON` | Operator authority is required or a hard ceiling is spent | Stop and present the smallest evidence-backed decision |

`REFUTED` is review evidence, not a disposition; it becomes `REPAIR` or
`REPLAN`. `UNSTUCK` and `ESCALATE` are helper results, not dispositions.
`UNSTUCK` must name a new approach and resumes as `REPAIR`. `ESCALATE` becomes
`ANDON`.

## Bounded tranche and receipt reuse

One behavioral leaf is the bounded tranche and the implementation, acceptance,
semantic-proof, and delivery unit. One writer mutates it through one to three
sequential low-risk waves. Every wave receives targeted deterministic acceptance,
and the Premortem-bound acceptance, dependencies, write scope, and risk class
must remain unchanged. No second writer or second leaf is implicit.

Intermediate leaf edits occur before freeze and do not invalidate a verdict
because no semantic verdict exists yet. At the tranche boundary, freeze the
complete candidate once. Any edit after freeze invalidates the semantic verdict.

An exact-input deterministic receipt is reusable when candidate/tree inputs,
command and mode, registry/toolchain identity, and relevant environment match.
Reuse has zero deterministic-execution charge. Missing, stale, suspicious, or
invalidated receipts are rerun fail-closed. Validation may verify receipt identity;
it must not rerun the same broad suite by habit.

One initial semantic blocker set may receive one consolidated repair batch and
one affected-claim closure. A second distinct repair need is `REPLAN`; phases do
not create another local review loop.

## Breaker and helper rules

Max-attempts, oscillation, and no-progress are stuckness signals. They enter
`HOLD` with matching reason, blocker class, and `helper.allowed:true`, and
authorize exactly one bounded fresh-context helper consultation per blocker
class. The helper advises; it does not dispatch, validate, or authorize
delivery. `UNSTUCK` plus a nonempty new approach is the explicit exit to
`REPAIR`; `ESCALATE` is the explicit exit to `ANDON`. A malformed, mismatched,
or repeated helper request is refused without changing the protected state. If
the same blocker trips again after its helper was consumed, the breaker enters
`ANDON`.

Human-only judgment reaches `ANDON` directly. An explicit `human` command with
a reason may move an operator-owned stop to `NOTE`, `REPAIR`, or `REPLAN`; the
generic transition command cannot do so. Human authority cannot clear an
exceeded hard ceiling. An ordinary failed check, a review refutation, or a
retry count by itself never reaches `HOLD` or `ANDON`.

Generic `transition` records only `NOTE`, `REPAIR`, or `REPLAN`, and only when
the current state is not protected. It can neither create nor clear `HOLD` or
`ANDON`; `break`, `helper`, and `human` are the explicit authority-bearing
ports for those transitions.

The command surface is:

- `init` — create the one run state; refuses replacement.
- `admit` — atomically charge and record work before dispatch.
- `transition` — record an ordinary `NOTE`, `REPAIR`, or `REPLAN` disposition.
- `break` — classify stuckness as `HOLD` or human-only work as `ANDON`.
- `helper` — consume the one helper as `UNSTUCK` or `ESCALATE`.
- `human` — explicitly release a non-ceiling `ANDON` under operator authority.

## Pull-flow boundary

The goal remains aggregate demand. One behavioral leaf occupies WIP and is the
bounded tranche. A completed leaf freezes once, Validates once, Learns once, and
hands the same candidate to repository delivery. If three waves or 90 minutes
arrives while the leaf is incomplete, persist `PARTIAL` resume evidence and stop
without proof, Learn, or delivery authorization. Exact remote verification and
the report release the WIP slot; only then may another leaf begin. The governor
controls admission and recovery only; it does not become a planner, implementer,
semantic judge, tracker, or delivery adapter.
