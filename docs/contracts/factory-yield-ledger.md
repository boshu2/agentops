# Factory Yield Ledger Contract

> **Status:** Draft
> **Decision:** Factory yield is recorded as schema-backed ledger events,
> not as retrospective throughput notes.
> **Consumers:** `agentopsd` factory projections, routing promotion gates,
> bounded pilot scorecards, operator review

The yield ledger compares baseline and treatment lanes after review,
validation, recovery, merge conflicts, escaped defects, interventions, and
model/API cost are counted. It answers whether a lane produces accepted,
validated work per wall-clock hour, not whether it ran more agents.

## Schema

Machine-readable schema:

- `schemas/factory-yield.v1.schema.json`
- `docs/contracts/factory-yield-ledger.schema.json`

Validated fixture:

- `docs/contracts/factory-yield-ledger.example.json`

## Event Shape

Every yield event uses `event_type: "factory.yield_observation"` and
`schema_version: 1`.

Required correlation fields:

| Field | Purpose |
|---|---|
| `observation_id` | Stable yield event identity. |
| `run_id`, `job_id`, `task_id` | Factory run and task correlation. |
| `lane_id`, `provider`, `runtime`, `model`, `authority` | Routing lane identity. |
| `routing_decision_id`, `routing_policy_id` | Route and policy evidence. |
| `validation_id`, `validation_status` | Validation outcome. |
| `merge_decision_id`, `merge_status` | Manual merge outcome. |
| `artifact_refs` | Routing, validation, merge, diff, and transcript pointers. |

Required yield fields:

- `baseline_or_treatment`;
- `accepted_patches`;
- `wall_clock_minutes`;
- `review_minutes`;
- `recovery_minutes`;
- `model_cost_usd`;
- `conflict_count`;
- `defect_count`;
- `operator_interventions`;
- `sidecar_consumed_by`;
- `decision_used_for`.

Optional timing fields such as `queue_wait_minutes` and `execution_minutes`
make latency decomposable without changing the core accepted-patches/hour
calculation.

## Promotion Criteria

Local lanes start at `OBSERVE` or `ADVISORY`. They do not own code, routing,
or merge decisions in milestone 1.

Promotion from `OBSERVE` to `ADVISORY` requires all of:

- at least 20 yield observations for the lane or sidecar class;
- consumed sidecar output in at least 50 percent of reviewed factory jobs;
- no increase in escaped `defect_count` against the baseline lane;
- positive or neutral accepted patches per hour after review and recovery
  minutes are included;
- explicit `decision_used_for` evidence naming the review or routing decision.

Promotion beyond `ADVISORY` is out of scope for milestone 1. Any future
`DELEGATED` or `AUTHORITATIVE` promotion requires a new routing policy with
`promotion_gate.requires_yield_evidence: true`, a non-zero sample size, and a
manual operator approval record.

## Baseline Versus Treatment

The baseline for milestone 1 is the default cloud/frontier coding lane with
manual merge. A treatment may add a second cloud/frontier worker or an advisory
sidecar. GasCity / Mt. Olympus production coding lanes remain disabled.

Compare lanes with:

```text
accepted_patches_per_hour =
  accepted_patches / ((wall_clock_minutes + review_minutes + recovery_minutes) / 60)
```

Treat zero-conflict, zero-defect, low-cost output as better only when validation
passed and the merge status reached `manual_pending` or `manual_merged`.

## Failure Semantics

Validation failure, merge rejection, retained failed worktrees, and recovery
minutes must remain visible in the yield event. Omitting failed observations
invalidates the comparison because it hides recovery cost.

