# AgentOpsd Control Plane Contract

> **Status:** Draft
> **Decision:** `agentopsd` is the production control plane for safe,
> observable parallel coding-agent work. Mt. Olympus and GasCity are reference
> or backend lanes only in milestone 1.
> **Consumers:** `agentopsd`, RPI daemon jobs, operator status surfaces,
> routing-policy validation, future factory pilots

This contract defines the first dark-factory control-plane slice. It turns
parallel coding work into daemon-owned resources and telemetry before adding
more worker count, live backend lanes, or automatic merge authority.

## North Star

Optimize for accepted, validated work per wall-clock hour after review time,
recovery time, merge conflicts, defects, interventions, and model/API cost are
counted. Raw agent count is not a success metric.

## Architectural Split

- AgentOps / `agentopsd` owns production queueing, routing, worker slots,
  worktree allocation, lifecycle telemetry, validation gates, yield ledger
  events, and operator projections.
- Mt. Olympus and GasCity are implementation backends or reference runtimes.
  They are not production-critical for milestone 1.
- Local compute starts as `OBSERVE` or `ADVISORY`. It may scout, retrieve,
  summarize, classify, preflight, or critique, but it does not own code,
  merge decisions, or routing authority until measured yield gates promote it.
- Cloud/frontier models are the default coding brains until another lane wins
  measured yield gates.

## Control-Plane Resources

### Worker Slot

A worker slot is a daemon-owned unit of execution capacity. It is not a
goroutine count or an implicit CLI process.

Required fields:

| Field | Meaning |
|---|---|
| `slot_id` | Stable daemon slot identity. |
| `run_id` | Factory, RPI, or pilot run identity. |
| `job_id` | Daemon job that owns the slot lease. |
| `task_id` | Bead, shard, or task assigned to the slot. |
| `lane_id` | Routing policy lane selected for the task. |
| `provider` | Provider selected by routing. |
| `runtime` | Runtime selected by routing. |
| `model` | Model selected by routing. |
| `authority` | `OBSERVE`, `ADVISORY`, `DELEGATED`, or `AUTHORITATIVE`. |
| `branch` | Branch or detached source identity. |
| `worktree_path` | Isolated worktree path for code-owning lanes. |
| `resource_policy` | Timeout, memory cap, host preference, and concurrency class. |
| `lease_epoch` | Queue/slot lease epoch. |
| `status` | Slot state. |

Slot statuses:

- `idle`
- `allocated`
- `running`
- `blocked_validation`
- `awaiting_manual_merge`
- `terminal`
- `retained_failed`

Slot invariants:

- `max_total_concurrency` and lane `max_concurrency` are enforced before a job
  starts.
- A slot cannot start until routing and worktree allocation events are durable.
- A slot cannot be reused until terminal state and retention or merge
  disposition are recorded.
- `DELEGATED` lanes may produce scoped code changes in owned worktrees, but
  they cannot merge automatically.
- `AUTHORITATIVE` lanes are not used in milestone 1.

### Worktree Ownership

Worktrees are evidence-bearing artifacts. Cleanup authority must be explicit.

Required fields:

| Field | Meaning |
|---|---|
| `worktree_id` | Stable daemon worktree identity. |
| `owner_run_id` | Factory or RPI run that created the worktree. |
| `owner_job_id` | Daemon job that owns cleanup authority. |
| `owner_slot_id` | Worker slot that wrote to the worktree. |
| `base_commit` | Clean source commit used for allocation. |
| `branch` | Branch or detached ref used by the worker. |
| `path` | Worktree path. |
| `created_at` | Allocation timestamp. |
| `dirty_state` | `clean`, `dirty`, or `unknown`. |
| `retention_policy` | Retention rule. |
| `merge_disposition` | Merge/review state. |

Retention policies:

- `retain_on_failure`
- `retain_until_manual_merge`
- `delete_after_verified_merge`

Merge dispositions:

- `not_requested`
- `manual_pending`
- `manual_merged`
- `rejected`
- `abandoned`

Worktree invariants:

- Clean-base preflight must pass before allocation.
- Worktree paths must live under the daemon-owned factory worktree root.
- Destructive cleanup requires matching `owner_run_id` and `owner_job_id`.
- Failed workers and failed validations retain worktrees and artifacts by
  default.
- Same-wave file overlap detection runs before dispatch. Overlap blocks
  parallel execution unless the tasks are explicitly serialized.

## Routing Policy

Routing policy is schema-backed by
[`routing-policy.md`](routing-policy.md) and
`schemas/routing-policy.v1.schema.json`.

Milestone 1 routing requirements:

- `manual_merge_by_default` is `true`.
- The default production coding lane is a cloud/frontier `DELEGATED` lane.
- Local lanes are limited to `OBSERVE` or `ADVISORY`.
- GasCity / Mt. Olympus production coding lanes are disabled.
- Unknown authorities, duplicate lanes, missing default lanes, and unsupported
  task classes fail closed.

## Lifecycle Telemetry

Lifecycle events are appended to the daemon ledger before projections claim a
worker is active, blocked, retained, or ready for review.

Additive factory event types:

| Event | Required payload |
|---|---|
| `factory.job_submitted` | `job_id`, `run_id`, `task_id`, `requested_by`, `objective` |
| `factory.job_claimed` | `job_id`, `run_id`, `slot_id`, `worker_id` |
| `factory.job_started` | `job_id`, `run_id`, `slot_id`, `worker_id` |
| `factory.admission_decided` | `job_id`, `run_id`, `work_order_id`, `allowed`, `reasons`, `landing_policy`, `digest_policy`, `artifact_refs` |
| `factory.routing_decided` | `job_id`, `run_id`, `lane_id`, `provider`, `runtime`, `model`, `authority`, `reason` |
| `factory.slot_allocated` | `slot_id`, `job_id`, `lane_id`, `max_concurrency_snapshot` |
| `factory.worktree_allocated` | `worktree_id`, `slot_id`, `path`, `base_commit`, `branch`, `owner_job_id` |
| `factory.validation_started` | `validation_id`, `job_id`, `commands`, `level` |
| `factory.validation_completed` | `validation_id`, `status`, `artifacts`, `duration_ms` |
| `factory.merge_decision` | `job_id`, `decision`, `decider`, `reason`, `conflicts`, `manual_command` |
| `factory.job_terminal` | `job_id`, `status`, `artifact_refs`, `transcript_ref`, `retained_worktree` |

Optional pointer fields accepted on factory events:

- `logs`, `log_refs`, and `log_ref`;
- `artifact_refs` in addition to `artifacts`;
- `transcript_refs` and `transcript_ref`;
- `diff_refs` and `diff_ref`.

Projection additions:

`ProjectionSet.factory` is the status projection for these events. Its top-level
fields are:

- `admissions`;
- `jobs`;
- `active_workers`;
- `slots`;
- `queue_lanes`;
- `model_lanes`;
- `validations`;
- `blocked_validations`;
- `worktrees`;
- `retained_failed_worktrees`;
- `merge_decisions`;
- `pending_manual_merges`;
- `terminal_jobs`;
- `recent_events`;
- `logs`;
- `artifacts`;
- `transcripts`;
- `diffs`;
- `last_routing_decision`.

Enum-like projected fields use explicit allowlists:

- `authority`: `OBSERVE`, `ADVISORY`, `DELEGATED`, or `AUTHORITATIVE`;
- validation `status`: `running`, `passed`, `failed`, `blocked`, or
  `cancelled`;
- merge `decision`: `not_requested`, `manual_pending`, `manual_merged`,
  `rejected`, or `abandoned`;
- terminal job `status`: existing terminal job statuses `completed`, `failed`,
  or `cancelled`.

## Validation And Merge Gates

Required validation levels:

| Level | Purpose | Example command |
|---|---|---|
| L0 | Static contract and schema checks | `python3 -m json.tool schemas/routing-policy.v1.schema.json` |
| L1 | Unit validation for policy and worktree safety | `cd cli && go test ./internal/daemon ./cmd/ao -run 'RoutingPolicy|ParallelWorktree'` |
| L2 | Daemon queue/projection integration | `cd cli && go test ./internal/daemon -run 'Queue|Supervisor|Projection|Routing|Factory'` |
| L3 | Bounded local fake/frontier pilot | `scripts/pre-push-gate.sh --fast` |

Merge gate invariants:

- Required validation must finish before merge review.
- Failed validation blocks merge.
- Wave-level gates aggregate slot validation states.
- Milestone 1 uses manual merge first.
- Automatic merge is disabled by default, including legacy `ao rpi parallel`.

## Yield Ledger

Yield observations are ledger events, not retrospective prose. They are emitted
per run and aggregated by lane, provider, runtime, model, task class, and
authority.

The machine-readable event contract is
[`factory-yield-ledger.md`](factory-yield-ledger.md) and
`schemas/factory-yield.v1.schema.json`.

Required event:

```json
{
  "event_type": "factory.yield_observation",
  "schema_version": 1,
  "run_id": "factory-2026-05-03-001",
  "lane_id": "frontier-codex",
  "task_class": "code_change",
  "baseline_or_treatment": "treatment",
  "accepted_patches": 1,
  "wall_clock_minutes": 42,
  "review_minutes": 8,
  "recovery_minutes": 0,
  "model_cost_usd": 3.25,
  "validation_status": "passed",
  "merge_status": "manual_pending",
  "conflict_count": 0,
  "defect_count": 0,
  "operator_interventions": 1,
  "sidecar_consumed_by": [],
  "decision_used_for": ["manual_merge_review"],
  "artifact_refs": {
    "diff": ".agents/factory/runs/factory-2026-05-03-001/diff.patch",
    "validation": ".agents/factory/runs/factory-2026-05-03-001/validation.json"
  }
}
```

Yield gates must account for:

- accepted patches per hour;
- model/API cost;
- queue wait and execution latency;
- review and recovery minutes;
- merge conflicts;
- validation failures and escaped defects;
- operator interventions;
- whether advisory sidecars were consumed and which decision used them.

## Operator Status

`ao daemon status` and future factory status views should expose:

- active slots and worker identities;
- queue depth by lane;
- default lane and disabled lanes with reasons;
- blocked validations;
- retained failed worktrees;
- pending manual merges;
- recent lifecycle events;
- artifact, log, transcript, and diff pointers.

Status must not hide retained failure evidence to make throughput look better.

## First Implementation Milestone

Milestone 1 is safe observed parallelism:

- schema-backed routing policy;
- worker slot and worktree ownership contract;
- clean-base and overlap preflight;
- lifecycle event vocabulary and projection shape;
- validation-required manual merge gates;
- yield ledger event shape;
- bounded 1-vs-2 pilot runbook using cloud/frontier lanes.

Explicit non-goals:

- no broad daemon queue rewrite;
- no automatic merge to `main` or the source branch by default;
- no destructive cleanup of failed worker worktrees;
- no distributed multi-daemon election;
- no Mt. Olympus or GasCity production routing;
- no local model promotion beyond `OBSERVE` or `ADVISORY`;
- no raw-agent-count optimization.

## Beads

The discovery packet decomposes this contract under epic `soc-dpci`:

| Bead | Scope |
|---|---|
| `soc-dpci.2` | Agentopsd control-plane architecture contract. |
| `soc-dpci.3` | Routing policy schema and fixtures. |
| `soc-dpci.4` | Worker slot and worktree ownership contract. |
| `soc-dpci.5` | Lifecycle telemetry and status projection events. |
| `soc-dpci.6` | Validation and manual merge gate policy. |
| `soc-dpci.7` | Factory yield ledger schema. |
| `soc-dpci.8` | Bounded cloud/frontier pilot command. |
| `soc-dpci.9` | Retire unsafe `ao rpi parallel` production path. |

## Pre-Mortem Controls

Worktree destruction:

- no force cleanup of unowned/stale worktrees;
- failed worktrees are retained;
- cleanup requires matching ownership metadata.

Over-abstract routing policy:

- policy is backed by JSON Schema, fixtures, and daemon validation code;
- GasCity production routing fails closed in milestone 1.

Missing telemetry:

- lifecycle events are defined before throughput or yield claims;
- status must show blocked and retained states.

Mt. Olympus scope creep:

- GasCity / Mt. Olympus lanes stay disabled for production coding work;
- backend health is not a prerequisite for milestone 1.
