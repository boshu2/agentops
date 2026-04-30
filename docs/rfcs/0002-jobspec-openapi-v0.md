# RFC 0002: JobSpec OpenAPI v0

Status: draft
Date: 2026-04-30
Companion spec: [`docs/contracts/jobspec-openapi-v0.yaml`](../contracts/jobspec-openapi-v0.yaml)

## Decision Ask

Publish JobSpec OpenAPI v0 as the first daemon conformance contract for
AgentOps runtimes. The v0 contract describes current `agentopsd` behavior only:
durable job submission, cancellation, queue state, ledger replay, projection
readiness, and the OpenClaw consumer surface.

This RFC does not ask to refactor `agentopsd`, add placement, or introduce new
job types. It asks whether the current behavior is stable enough to name,
document, test, and publish.

## Context

The strategic direction is now:

```text
The 12-factor doctrine is the product.
agentops is the implementation.
agentopsd is plumbing.
Be the conformance program.
```

That only works if the public contract names actual behavior. The daemon has a
small useful kernel today:

- loopback-local HTTP routes
- local-token-protected mutations
- append-only daemon ledger under `.agents/daemon/ledger.jsonl`
- durable-first mutation acknowledgement
- idempotent recovery by `job_id` or `idempotency_key`
- queue leasing with claim tokens, lease epochs, heartbeats, retry waiting, and
  terminal completion, failure, or cancellation
- read models rebuilt from ledger replay
- OpenClaw projection resources and allowlisted safe triggers

The OpenAPI document turns that kernel into a portable conformance target. Any
runtime can implement the same contract without adopting AgentOps internals.

## Current Behavior Captured

Source of truth:

- `cli/internal/daemon/server.go` for HTTP routes and response structs
- `cli/internal/daemon/jobs.go` for queue mutation and replay behavior
- `cli/internal/daemon/types.go` for job, status, event, and failure enums
- `cli/internal/daemon/projections.go` for projection names and read models
- `cli/internal/openclaw/api.go` and `cli/internal/openclaw/types.go` for the
  OpenClaw consumer surface

Versioned daemon routes:

| Route | Method | Purpose |
|---|---:|---|
| `/v1/health` | GET | process health |
| `/v1/ready` | GET | ledger replay and projection readiness |
| `/v1/status` | GET | queue snapshot and projections |
| `/v1/events` | GET | replayed ledger events |
| `/v1/jobs` | POST | submit a durable daemon job |
| `/v1/jobs/cancel` | POST | cancel a non-terminal daemon job |

The current router also exposes unversioned aliases for the same daemon routes.

OpenClaw consumer routes:

| Route | Method | Purpose |
|---|---:|---|
| `/openclaw/v1/health` | GET | consumer projection health |
| `/openclaw/v1/snapshot/latest` | GET | aggregate consumer snapshot |
| `/openclaw/v1/runs` | GET | run resources |
| `/openclaw/v1/jobs` | GET | job resources |
| `/openclaw/v1/wiki` | GET | wiki resources |
| `/openclaw/v1/triggers/jobs` | POST | allowlisted job trigger |

## Non-Goals

JobSpec v0 intentionally excludes:

- `placement.affinity`
- node scheduling
- provider capability negotiation
- new worker kinds
- retry policy configuration beyond the current fixed queue options
- distributed lease coordination beyond the current ledger-derived queue
- public browser-safe mutation semantics

Those may belong in later RFCs. They do not belong in a current-behavior v0.

## Contract Shape

The public submission envelope is the current `SubmitJobRequest`:

```json
{
  "request_id": "req_20260430_000001",
  "job_id": "job_rpi_000001",
  "job_type": "rpi.run",
  "idempotency_key": "rpi:demo:cycle-1",
  "payload": {
    "goal": "Draft current-behavior JobSpec v0"
  }
}
```

The accepted response names the durable ids, the queue status, and projection
acknowledgement state:

```json
{
  "accepted": true,
  "request_id": "req_20260430_000001",
  "job_id": "job_rpi_000001",
  "status": "queued",
  "last_event_id": "evt_job.accepted_job_rpi_000001_000001",
  "projection_status": "current",
  "projection_lag": {
    "last_event_id": "evt_job.accepted_job_rpi_000001_000001",
    "event_count": 1,
    "corrupt_record_count": 0,
    "degraded": false
  },
  "idempotency_key": "rpi:demo:cycle-1"
}
```

The queue state machine is intentionally conservative:

```text
queued -> running -> completed
queued -> running -> failed
queued -> running -> retry_waiting -> running
queued -> cancelled
running -> cancelled
```

Terminal statuses are `completed`, `failed`, and `cancelled`. A terminal ledger
event wins over any projection or lease state.

## Conformance Profile

An implementation conforms to JobSpec v0 when it can pass these observable
checks:

1. `GET /v1/health` returns `status=ok`, `daemon=agentopsd`, and a timestamp.
2. `GET /v1/ready` reports replay status, projection status, projection lag, and
   degraded reasons without mutating quarantine state.
3. `POST /v1/jobs` requires local mutation authorization and appends a durable
   accepted event before returning `202`.
4. Retrying a submit with the same `job_id` or `idempotency_key` returns the
   existing job instead of appending a duplicate accepted event.
5. `GET /v1/status` exposes queue jobs rebuilt from the ledger.
6. Running jobs carry a claim token, lease epoch, lease expiry, and attempt
   count.
7. Expired leases become claimable again as `retry_waiting` until retry
   exhaustion.
8. `POST /v1/jobs/cancel` appends a cancellation for non-terminal jobs and
   returns an `already_terminal_*` outcome for terminal jobs.
9. `GET /v1/events` returns replayed ledger events and last event id.
10. OpenClaw read routes report resources from the same daemon projections as
    `/v1/status`.
11. `/openclaw/v1/triggers/jobs` refuses non-allowlisted job types.

The next tactical step is to turn this list into golden compatibility tests for
`/jobs`, `/status`, ledger replay, and queue transitions.

## Open Questions

1. Should invalid `job_type` on `/v1/jobs` become a `400` in v1, or should v0
   preserve the current generic error behavior until a compatibility window is
   announced?
2. Should unversioned route aliases be part of conformance, or documented as
   compatibility conveniences outside the required profile?
3. Should `placement.affinity` be a JobSpec v0 extension field under
   `payload.placement`, or wait for JobSpec v1 after the mt-olympus shim soaks?
4. Should OpenClaw stay inside the JobSpec v0 document, or split into a separate
   consumer API conformance profile?

## Recommendation

Publish the OpenAPI as a draft RFC now. Treat it as a compatibility target, not
as a promise that the daemon API is finished. The value is category ownership:
the schema makes the doctrine executable while preserving the council's scope
discipline.
