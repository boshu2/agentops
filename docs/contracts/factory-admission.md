# Factory Admission Contract

> **Status:** Draft
> **Consumers:** `agentopsd`, factory schedules, `ao factory` operator
> commands, RPI daemon jobs, morning digest renderers

Factory admission is the daemon-owned gate between a proposed autonomous
factory work order and source-mutating execution. It answers one question:

> May this work order enqueue code-changing work now?

The answer is a typed `AdmissionDecision`. A blocked decision is not an
implementation failure. It is a successful control-plane stop.

## Scope

This contract covers:

- the factory work-order input shape;
- the factory admission decision output shape;
- freshness, blocker, CI, landing, and digest fields required before mutation;
- daemon job specs for `factory.admission` and `factory.local-pilot`;
- schedule payload defaults and validation expectations.

This contract does not cover:

- automatic merge;
- distributed schedulers;
- host service installation;
- full RPI execution internals after an admitted handoff.

## Job Types

| Job type | Purpose |
|---|---|
| `factory.admission` | Evaluate one work order and emit an admission decision. |
| `factory.local-pilot` | Evaluate a local pilot work order and, when the daemon policy has RPI execution enabled, enqueue an admitted `rpi.run` child job. |

Both job types use schema version `1` and fail closed on malformed specs.

## Work Order

Schema: [`schemas/factory-work-order.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/factory-work-order.v1.schema.json)

Required fields:

| Field | Meaning |
|---|---|
| `schema_version` | Must be `1`. |
| `work_order_id` | Stable operator or schedule-generated work order id. |
| `generated_at` | RFC3339 timestamp for when the evidence was collected. |
| `expires_at` | RFC3339 timestamp after which source mutation must block. |
| `base_sha` | Git commit SHA the evidence applies to. |
| `target` | Selected goal, bead, or execution packet. |
| `allowed_files` | Files or path prefixes the admitted work may touch. |
| `validation_commands` | Commands required before manual review or handoff completion. |
| `landing_policy` | `off` or `manual_pr`; automatic merge is not allowed. |
| `digest_policy` | Must be `required` for source-mutating work. |
| `open_pr_blockers` | Open PR overlap evidence, empty when none known. |
| `main_ci_baseline` | Latest main CI evidence, including `green`, `red`, or `unknown`. |

`allowed_files` must be relative repository paths. Absolute paths, empty
strings, and parent-directory escapes are invalid.

## Admission Decision

Schema: [`schemas/factory-admission.v1.schema.json`](https://github.com/boshu2/agentops/blob/main/schemas/factory-admission.v1.schema.json)

Required fields:

| Field | Meaning |
|---|---|
| `schema_version` | Must be `1`. |
| `work_order_id` | Work order that was evaluated. |
| `run_id` | Factory run id. |
| `evaluated_at` | RFC3339 evaluation time. |
| `allowed` | `true` only when the work order may proceed. |
| `reasons` | Empty or informational on allow; required on block. |
| `landing_policy` | Effective landing policy. |
| `digest_policy` | Effective digest policy. |
| `evidence` | Snapshot summary for blockers and main CI. |

Optional fields:

| Field | Meaning |
|---|---|
| `child_job_id` | Enqueued child `rpi.run` job id when a handoff happens. |
| `artifact_refs` | Durable refs for decision, blocker matrix, CI baseline, and digest. |

## Event And Artifact Location

Daemon-owned factory admission artifacts live under:

```text
.agents/daemon/factory/runs/<run_id>/
  work-order.json
  admission.json
  blocker-matrix.json
  main-ci-baseline.json
```

The executor records one additive lifecycle event:

| Event | Required payload |
|---|---|
| `factory.admission_decided` | `run_id`, `work_order_id`, `allowed`, `reasons`, `landing_policy`, `digest_policy`, `artifact_refs` or `artifacts` |

When admission allows an RPI handoff, the same event may include
`child_job_id`. The child job is still a normal `rpi.run` queue job; admission
does not become the execution loop.

Daemon executor policy determines how that child job runs:

- `fake` completes deterministic CI-safe phase artifacts.
- `gascity` delegates phases to the GasCity API executor.
- `cli-fallback` runs one safe local cycle in-process via `RPIRunExecutor`
  (`cli/internal/daemon/rpi_run.go`) with `landing_policy=off`. The previous
  shell-out wrapper under `scripts/` was retired in soc-bcrn.3.7.

## Blocked Vs Malformed

Malformed requests are daemon job failures:

- invalid schema version;
- missing work order;
- invalid enum value;
- invalid path;
- non-RFC3339 timestamps.

Valid-but-unsafe requests complete with `allowed=false`:

- stale or expired work order;
- base SHA mismatch;
- dirty source tree;
- tracked repo-root `.agents`;
- open PR overlap;
- red unrelated main CI;
- unknown GitHub evidence in source-mutating mode;
- missing RPI handoff capability.

This distinction keeps control-plane stops visible without pretending
implementation work failed.

Blocked admission is not yield evidence. Yield observations are emitted only
after execution, validation, and manual review state exist.

## Landing Policy

Allowed values:

- `off` — admission may produce evidence but must not land code.
- `manual_pr` — code may be prepared only for manual PR review.

Forbidden values:

- `auto_merge`
- `sync_push`
- `main`
- any direct push to the default branch

## Schedule Defaults

Factory schedules may omit daemon mechanics that the recurrence supervisor can
derive:

- `schema_version`
- `job_type`
- `run_id`
- `mode`

Factory schedules may not omit the work order itself. A schedule without
`work_order` is malformed because the daemon cannot infer target, allowed
files, validation commands, CI evidence, or landing policy safely.

## Mechanical Verification

Minimum gates:

```bash
python3 -m json.tool schemas/factory-work-order.v1.schema.json >/dev/null
python3 -m json.tool schemas/factory-admission.v1.schema.json >/dev/null
python3 tests/scripts/test-factory-admission-contracts.py
cd cli && go test ./internal/daemon -run 'FactoryAdmission|FactoryLocalPilot|JobType|Recurrence'
```
