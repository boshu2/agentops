# OpenClaw Consumer API Contract

OpenClaw is a consumer of AgentOps daemon projections. It does not own `.agents`
storage and must not mutate `.agents` directly.

## Ownership Boundary

AgentOps owns:

- daemon ledger events
- projection rebuilders
- snapshot versions
- read-only resources
- mutation gates for safe trigger endpoints

OpenClaw owns:

- UI presentation
- user interaction state outside `.agents`
- client-side caching

OpenClaw may read AgentOps projections and may call authorized trigger
endpoints. All trigger endpoints enqueue daemon jobs through the ledger-backed
mutation path.

## Read-Only Resources

The first API surface should expose read-only projection resources:

| Resource | Purpose |
|----------|---------|
| `/openclaw/v1/snapshot/latest` | latest aggregate consumer snapshot |
| `/openclaw/v1/runs` | RPI and Dream run summaries |
| `/openclaw/v1/jobs` | daemon job summaries |
| `/openclaw/v1/wiki` | wiki/forge corpus and generation summaries |
| `/openclaw/v1/health` | daemon projection health for the consumer |

Read-only resources must be safe for unauthenticated local inspection only when
the daemon local trust policy allows it.

## Snapshot Versions

Snapshots are versioned projections:

```json
{
  "schema_version": 1,
  "snapshot_id": "snap_20260428_000001",
  "generated_at": "2026-04-28T21:00:00Z",
  "source": {
    "ledger": ".agents/daemon/ledger.jsonl",
    "last_event_id": "evt_20260428_000001"
  },
  "resources": {
    "runs": [],
    "jobs": [],
    "wiki": []
  }
}
```

Readers must tolerate unknown additive fields and reject unsupported
`schema_version` values with a compatibility error.

The v1 schema is represented in code as `openclaw.ConsumerSnapshot`:

| Field | Required | Notes |
|-------|----------|-------|
| `schema_version` | yes | v1 is `1`; unsupported values are rejected |
| `snapshot_id` | yes | stable snapshot identity, usually derived from the latest daemon event |
| `generated_at` | yes | RFC3339/RFC3339Nano timestamp |
| `source.ledger` | yes | authoritative daemon ledger path used to build the projection |
| `source.last_event_id` | no | latest daemon ledger event included in the snapshot |
| `status` | yes | `current`, `stale`, or `degraded` |
| `resources.runs[]` | yes | RPI and Dream run summaries |
| `resources.jobs[]` | yes | daemon job summaries |
| `resources.wiki[]` | yes | wiki/forge generation summaries |

Each resource summary carries `resource_id`, `resource_kind`, `status`, optional
job/run identifiers, request IDs, artifacts, projection targets, timestamps, and
provenance links. `resource_kind` must match the containing list: `run` for
`resources.runs`, `job` for `resources.jobs`, and `wiki` for `resources.wiki`.
For terminal daemon jobs, `/openclaw/v1/jobs` must report the same `status`,
failure summary, artifacts, and latest event provenance as the daemon queue
projection used by `ao daemon jobs show`.

Golden fixture: `cli/internal/openclaw/testdata/consumer_snapshot_v1.json`.

## Product Proof

`ao daemon soak --scenario fake-executor --require-terminal --json` is the
repeatable local proof that OpenClaw can consume terminal daemon state. The soak
writes its scenario, filtered ledger events, JSON report, and markdown summary
under `.agents/daemon/soaks/<run-id>/`; the report includes both daemon queue
jobs and the matching `/openclaw/v1/jobs` resources.

## Mutation Gate

OpenClaw-safe trigger endpoints are optional and must be gated:

- daemon must be ready
- local trust token/header must pass
- request must map to an allowlisted job type
- accepted mutation must append the daemon ledger before success is returned
- response must include `request_id` and `job_id`

Unauthorized mutation must have no enqueue side effect.

## Non-Ownership Of `.agents`

OpenClaw must not write:

- `.agents/daemon`
- `.agents/rpi`
- `.agents/overnight`
- `.agents/wiki`
- `.agents/findings`

If OpenClaw needs to request work, it calls a mutation endpoint and lets
AgentOps own the resulting ledger event, job, and projections.
