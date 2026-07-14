# Run Disposition Contract

RPI owns one small decision vocabulary, not an execution controller. After a
wave, check, or review returns evidence, the visible orchestrator classifies
the next move as `NOTE`, `REPAIR`, `REPLAN`, `HOLD`, or `ANDON` and records the
decision in one closed document conforming to
`../schemas/run-disposition.schema.json`.

Crank, Validate, Learn, and delivery adapters return evidence. They do not
authorize work, meter it, maintain attempt counters, grant helpers, or mutate
the disposition. The orchestrator remains responsible for choosing and
executing the legal next move.

## Immutable record

Each record binds:

- `run_id`, which correlates the lifecycle without creating mutable run state;
- the objective identity and SHA-256 digest;
- exactly one canonical disposition and a concrete reason;
- one or more evidence references with their SHA-256 digests;
- an optional `blocker_class` for `HOLD` or `ANDON`; and
- the recording timestamp.

The schema is additional-properties closed. Controller fields such as counters,
allowances, reservations, cost state, or helper state are invalid. Changing the
objective or cited evidence creates a new record; it never rewrites prior proof.

Example:

```json
{
  "schema_version": 1,
  "run_id": "rpi-2026-07-14",
  "objective": {
    "identity": "age-example.1",
    "digest": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  "disposition": "REPAIR",
  "reason": "candidate introduced a failing acceptance example",
  "evidence_refs": [
    {
      "path": ".agents/evidence/acceptance.json",
      "sha256": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
    }
  ],
  "blocker_class": "acceptance",
  "recorded_at": "2026-07-14T12:00:00Z"
}
```

## One disposition language

| Disposition | Meaning | Legal next move |
|---|---|---|
| `NOTE` | Cosmetic, pre-existing, theoretical, or out-of-scope evidence | Record it; do not block the current objective |
| `REPAIR` | Introduced, concrete, verifiable acceptance or correctness defect | Apply one consolidated local repair and recheck affected claims |
| `REPLAN` | Evidence invalidates the slice, dependency shape, or approach | Return to Discovery and Premortem the changed plan |
| `HOLD` | Mutation cannot safely continue without bounded fresh reasoning | Freeze mutation and request one fresh helper consultation |
| `ANDON` | Human authority is genuinely required or a hard external ceiling is spent | Notify the operator with the cited evidence |

A finding blocks only when it is introduced or newly reachable in the current
diff, concrete and verifiable, and breaks acceptance, correctness, safety, or a
claimed contract. Reviewer disagreement, ordinary test failure, `PARTIAL`,
generated drift, and a retry count are evidence for `REPAIR` or `REPLAN`; they
do not manufacture `ANDON`.

## Recovery boundary

Max-attempts, oscillation, or no-progress evidence may produce `HOLD`. The
outer operating-loop policy then permits exactly one bounded fresh-context
consultation for that blocker class. `UNSTUCK` is recorded as `REPAIR` with a
new approach; `ESCALATE` is recorded as `ANDON`. The disposition document does not track helper eligibility or history and cannot become a phase-local retry machine.

A genuinely spent hard time, cost, or quota ceiling may produce `ANDON`
directly. A soft tranche boundary produces resume evidence and `NOTE`, never a
human escalation by itself.

## Deterministic and semantic proof

Deterministic receipts prove identity, syntax, schemas, and executable facts.
Independent Validate judges semantic claims. The disposition cites those facts
but does not reinterpret them, reserve execution, or confer delivery authority.
Repository-selected delivery remains outside RPI.

## Rollback

Reverting a disposition-contract change restores the prior Git tree. Historical
ignored receipts need no migration because they are evidence, not live control
state.
