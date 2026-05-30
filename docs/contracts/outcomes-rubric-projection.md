# Contract: Outcomes Rubric Projection

> **Status: v1 (ag-hguuf).** Machine-checkable contract for the holdout-safe
> projection of the locked eval substrate into an Outcomes-style grading payload.

## Purpose

[Anthropic Managed Agents Outcomes](https://www.anthropic.com/) grade an agent
run against a **rubric** instead of an exact-match answer. AgentOps already
owns the authoritative grading substrate at `~/.agents/evals/SCHEMA.md` (locked).
This contract defines how a locked `ao eval` Task + its grading criteria
**project** to an Outcomes rubric payload that can cross the cloud boundary.

The projection is a **derived artifact**. `~/.agents/evals/SCHEMA.md` remains the
sole authority; a projected rubric is **never an alternate bar**. The locked
schema is EXTENDED by this projection, never relitigated.

### The load-bearing invariant: holdout isolation

**Managed Agents are not ZDR.** A rubric that crosses the cloud boundary must
NEVER carry a ground-truth answer, a holdout value, a `target`, or an
`expected_output`. A leak is permanent. The contract enforces this three ways:

1. **By construction** — `evalsubstrate.ProjectRubric` copies only allowlisted
   fields and does not accept ground-truth rows, so it *cannot* copy them
   (`cli/internal/evalsubstrate/rubric.go`).
2. **By re-scan** — `Rubric.ContainsAny` is the defense-in-depth guard every
   caller runs against the run's holdout values before the payload leaves the
   boundary (`ao eval outcomes compile` refuses on any hit).
3. **By schema** — `schemas/outcomes-rubric.v1.schema.json` sets
   `additionalProperties: false` at **every nesting level**, so a malformed
   compiler emission that smuggles a leak field **fails schema validation**, not
   merely a code check. This is the surface this contract adds.

## Payload shape

The payload is the JSON encoding of `evalsubstrate.Rubric` — the exact output of
`ao eval outcomes compile`. Schema: [`schemas/outcomes-rubric.v1.schema.json`](../../schemas/outcomes-rubric.v1.schema.json).

```json
{
  "schema_version": 1,
  "source_task_id": "task-go-cc-budget",
  "judge_content_hash": "sha256:…",
  "instructions": "Grade whether the refactor keeps CC under 25.",
  "criteria": [
    { "id": "cc-under-25", "description": "…", "weight": 1.0 }
  ]
}
```

### Field mapping: locked substrate → Outcomes rubric

| Outcomes field        | Source (locked substrate)                              | Notes |
|-----------------------|--------------------------------------------------------|-------|
| `schema_version`      | `evalsubstrate.SchemaVersion`                          | Pins the projected shape (=1). |
| `source_task_id`      | `Task.ID`                                              | Provenance back-pointer; never a ground-truth id. |
| `judge_content_hash`  | judge spec content hash (SCHEMA §rc2 drift key)        | Lets a stale rubric self-invalidate like a stale local judge. |
| `instructions`        | `Task.Description` (optional)                          | Holdout rubrics SHOULD omit when the description could echo a holdout prompt. |
| `criteria[].id`       | judge criterion id                                     | Allowlisted. |
| `criteria[].description` | judge criterion description                         | Allowlisted. |
| `criteria[].weight`   | judge criterion weight                                 | Allowlisted. |

### Deliberately NOT projected (holdout-safety + minimal cloud surface)

The earlier design sketch imagined a richer payload (`suite_id`, `metric`,
`decision_rule`, `split`). The shipped projection deliberately omits them — a
**smaller boundary surface is a smaller leak surface**, and the grader needs
none of them to score a run:

| Field considered          | Why it stays local |
|---------------------------|--------------------|
| `target` / `ground_truth` / `expected_output` / `samples-holdout` | The holdout answer itself. **Never crosses the boundary** — the whole point. Forbidden by `additionalProperties:false`. |
| `metric` / `decision_rule` | Live in the locked `Suite.stats` and the ONE local council verdict. Score aggregation and accept/reject happen **server-side after ingest**, never in the cloud grader. |
| `split` (dev/holdout)     | Tracked by the global Dolt **burn ledger** (`ao eval outcomes` gate#3), not by the payload. The grader is split-agnostic by design. |

If a future Outcomes integration genuinely needs one of these, widen
`evalsubstrate.Rubric` first (the Go schema↔struct drift test will then require
the schema to be updated in lockstep) — never hand-add a property to the schema
alone.

## Validation

- **Standalone validator:** [`scripts/validate-outcomes-rubric.sh`](../../scripts/validate-outcomes-rubric.sh)
  `<payload.json>…` or `--selftest`. Gating structural pass is python
  `jsonschema` (Draft7), mirroring `scripts/validate-next-work.sh`.
- **Acceptance fixtures:** `tests/fixtures/outcomes-rubric/` —
  `valid-dev` (pass), `valid-holdout-criteria-only` (pass, no instructions),
  `invalid-contains-target` (fail — smuggles a `target`). Driven by
  [`tests/scripts/validate-outcomes-rubric.bats`](../../tests/scripts/validate-outcomes-rubric.bats).
- **Schema↔struct drift guard:** `TestOutcomesRubricSchemaMatchesStruct` and
  siblings in `cli/internal/evalsubstrate/rubric_schema_test.go` keep the
  committed schema and the `Rubric`/`Criterion` Go structs in lockstep.

## See also

- [Eval Verdict Pipeline](eval-verdict-pipeline.md) — where ingested Outcomes
  scores land in the one verdict format (Flywheel close).
- `~/.agents/evals/SCHEMA.md` §rc2 (judge content hash / drift key), §4 (verdict
  record) — the locked authority this contract projects from, never replaces.
- `cli/internal/evalsubstrate/rubric.go` — the executable projection.
