# AgentOps Daemon Contract

`agentopsd` is the local always-on AgentOps control plane. It turns today's
one-shot RPI, Dream, wiki/forge, and worker orchestration flows into durable
jobs with a stable local API, while preserving foreground CLI fallbacks during
migration.

This document is the architecture ADR for the daemon boundary. Later contracts
add the full field tables, schemas, and per-job truth tables.

## Decision

AgentOps will add a repo-local daemon named `agentopsd`.

The daemon owns:

- a durable run/job event ledger under `.agents/daemon`
- a job queue for RPI, Dream, wiki/forge, and future worker jobs
- a local HTTP API for health, readiness, status, events, and authorized
  mutations
- projection files derived from the ledger for existing readers such as the RPI
  registry, Dream reports, wiki outputs, and OpenClaw snapshots
- local trust enforcement for all mutation endpoints

The daemon does not own:

- the source `.agents` knowledge corpus as an opaque database
- GasCity internals or session provider internals
- OpenClaw UI state
- direct source-code edits outside explicit AgentOps jobs
- legacy one-shot CLI fallbacks until the migration contract retires them

## Source Of Truth

The daemon ledger is the source of truth for daemon-owned runtime state.
Projection files exist for compatibility and consumer ergonomics, but they are
not authoritative.

Authoritative state:

- accepted job requests
- job lifecycle transitions
- worker session references
- terminal results
- request IDs and correlation IDs
- projection rebuild checkpoints

Derived projections:

- RPI run registry status
- Dream daemon-mode run summaries
- wiki/forge job summaries
- OpenClaw consumer snapshots
- status/watch read models

Any implementation that writes a projection must be able to rebuild that
projection from the ledger or mark it stale/degraded.

## Mutation Ack Order

The daemon uses a durable-first mutation ack order:

1. validate the request and local trust policy
2. allocate or reuse `job_id`, `request_id`, and optional idempotency key
3. append the accepted mutation to the daemon ledger
4. fsync or otherwise complete the configured durable write boundary
5. return success to the caller with the accepted IDs
6. update projections asynchronously or synchronously after the durable ledger
   write

If the projection update fails after step 5, the mutation remains accepted and
the affected projection is marked stale until replay rebuilds it. If the ledger
append fails before step 5, the mutation is not accepted.

Submit retry deduplication is keyed by `idempotency_key`; `request_id` is
trace-only. See [Daemon Idempotency](daemon-idempotency.md) for the full
contract.

## HTTP API Semantics

The daemon's local HTTP API is strict about caller-correctable errors:

- `POST /v1/jobs` returns `400 Bad Request` for validation failures such as an
  empty or unknown `job_type`, `503 Service Unavailable` for explicit daemon
  failpoints, and `500 Internal Server Error` only for server-side failures.
- `GET /v1/events` supports `since=<event_id>` and `after_id=<event_id>` cursor
  filters. Unknown cursors return `400 Bad Request` instead of silently
  returning an empty event set.
- `GET /v1/events?limit=N` returns at most `N` events, rejects non-integer or
  negative limits with `400 Bad Request`, and caps very large limits at the
  daemon maximum.
- job cancellation is available at both `POST /v1/jobs/cancel` with `job_id` in
  the body and `POST /v1/jobs/{id}/cancel` with the job id in the path. The
  path form may omit the body; if a body supplies a different `job_id`, the
  daemon returns `400 Bad Request`. Missing jobs return `404 Not Found`.
- schedule mutations are privileged operator actions: `POST /v1/schedules` and
  `DELETE /v1/schedules/{name}` are admin-capability mutation paths, while
  `GET /v1/schedules` remains read-only.

Examples:

- [job request](examples/agentops-daemon/job-request.json)
- [ledger event](examples/agentops-daemon/ledger-event.json)
- [projection manifest](examples/agentops-daemon/projection-manifest.json)

## Status Precedence Truth Tables

The following truth tables are the field-level defaults until later atoms add
per-job refinements.

### Job Status Projection

| Ledger terminal event | Active lease | Projection stale | Reported status | Precedence |
|-----------------------|--------------|------------------|-----------------|------------|
| `job.completed` | any | any | `completed` | terminal ledger event wins |
| `job.failed` | any | any | `failed` | terminal ledger event wins |
| `job.cancelled` | any | any | `cancelled` | terminal ledger event wins |
| none | fresh | false | `running` | active lease wins over queued projection |
| none | expired | false | `retry_waiting` | expired lease and retry budget wins |
| none | none | false | `queued` | accepted ledger event wins |
| none | unknown | true | `degraded` | stale projection cannot claim final state |

### Provider Status Projection

| Daemon ready | GasCity ready | Worker session known | Reported provider state | Precedence |
|--------------|---------------|----------------------|--------------------------|------------|
| false | any | any | `daemon_unavailable` | daemon readiness wins |
| true | false | any | `provider_unreachable` | GasCity readiness wins |
| true | true | false | `session_pending` | accepted job without session |
| true | true | true | `session_bound` | worker session reference wins |

### Snapshot Projection

| Ledger replay status | Projection file exists | Projection version supported | Consumer result |
|----------------------|------------------------|------------------------------|-----------------|
| complete | yes | yes | serve snapshot |
| complete | yes | no | serve compatibility error |
| complete | no | any | rebuild or return `projection_missing` |
| corrupt | any | any | quarantine record and return `projection_degraded` |

These tables are intentionally conservative: a projection never upgrades a job
to success without a terminal ledger event.

## Local Storage

The daemon stores runtime state below `.agents/daemon/`:

```text
.agents/daemon/
  ledger.jsonl             # append-only run/job events
  jobs/                    # optional durable job request envelopes
  projections/             # rebuilt read models
  quarantine/              # corrupt records or invalid worker output
  locks/                   # local daemon and queue coordination locks
  soaks/                   # structured product proof runs
```

Writes that accept or mutate jobs must append the ledger event before returning
success to the caller. Later atoms define the exact mutation ack order and
boundary failpoint tests.

## Job Queue

The daemon job queue is the single submission path for always-on product work.

Initial job families:

- `rpi.run` and `rpi.phase`
- `dream.run` and Dream stage jobs
- `wiki.forge`
- OpenClaw-safe trigger jobs
- `plans.projection` — read-side bd-subscription job that rebuilds the plans
  manifest projection from the shared bushido Dolt source. Read-only HTTP
  surface: `GET /v1/plans/manifest` and `GET /v1/plans/diff?since=<cursor>`.
  See spec at
  `.agents/plans/2026-05-01-daemon-absorption-spec/02-pilot-plans-projection.md`
  and the foundation §6 site 3 (alt) carve-out for read-side endpoints
  (`.agents/plans/2026-05-01-daemon-absorption-spec/00-foundation-contract.md`).

### Read-Side Endpoints — `plans.projection` curl example (F-PM-3)

```sh
# Manifest snapshot (atom-1 stub: empty entries until atom-2 fills the projection)
curl -s http://localhost:7077/v1/plans/manifest | jq .
# {"entries": [{"beads_id": "soc-aaa", "title": "...", "status": "open", ...}],
#  "schema_version": 1}

# Incremental diff since a known cursor (matches /v1/events `since=` convention)
curl -s "http://localhost:7077/v1/plans/diff?since=evt-0042" | jq .
# {"events": [{"event_id": "evt-0043", "event_type": "projection.rebuilt", ...}],
#  "last_event_id": "evt-0099"}
```

Sample bodies above are harvested from the L2 BDD test fixture at
`cli/cmd/ao/plans_bdd_test.go`. The host/port substitution is
configuration-dependent — `ao daemon status` reports the active address.

Queue workers must use leases and heartbeats rather than in-memory ownership.
The queue must tolerate daemon restart and worker crash without losing accepted
jobs.

### Foreground Supervisor

`ao daemon run` may start foreground worker loops with `--workers`.
`--worker-once` exits after each configured worker makes one claim attempt,
which keeps local validation deterministic while exercising the same queue
claim, heartbeat, and terminal event path as the long-running worker loop.

The fake policy supports `openclaw.snapshot`, `wiki.forge`, and `dream.run`.
`wiki.forge` uses the shared `AgentWorker` contract with an in-memory worker.
`dream.run` executes the existing Dream loop, writes terminal artifacts
(`summary_json`, `summary_markdown`, `overnight_log`, and `failure_report` on
failure), and fails the daemon job if the job execution timeout is exhausted.
Product `wiki.forge` execution uses `--executor-policy=gascity` and requires
explicit `--gascity-endpoint` and `--gascity-city` configuration. The daemon
must fail fast when those values are missing instead of inferring API readiness
from the legacy `gc` CLI bridge.

### Product Soak Proofs

`ao daemon soak` writes repeatable proof runs under
`.agents/daemon/soaks/<run-id>/`:

- `scenario.json`
- `events.jsonl`
- `soak-report.json`
- `summary.md`

The `queue-only` scenario proves durable ingestion without claiming terminal
success. `fake-executor --require-terminal` proves the executor path reaches a
terminal ledger event and that `/openclaw/v1/jobs` reports the same terminal
status and artifacts as the daemon queue projection. `dream --require-terminal`
uses the Dream executor path and may complete or fail, but it must produce a
terminal daemon job with artifacts rather than remaining silently queued.

## Local Trust

The default daemon API is local-only.

Trust rules:

- bind to loopback by default
- reject non-local mutation traffic unless explicitly configured
- require a mutation token or equivalent local trust header for writes
- treat browser-origin-style requests as untrusted unless they satisfy the
  mutation policy
- keep read-only health/status routes separate from mutation routes
- surface degraded reasons instead of silently falling back

`local trust` is part of the contract because OpenClaw and future local tools
will consume the daemon without owning `.agents` directly.

## Local HTTP Threat Model

The daemon treats localhost as a constrained trust boundary, not as proof that a
caller is safe. The primary local HTTP risks are:

- accidental bind to `0.0.0.0`, a LAN address, or another non-loopback address
- browser-origin requests reaching mutation endpoints through forms, fetch, or
  preflight-enabled requests
- leaked or world-readable mutation tokens under `.agents/daemon`
- mutation routes accepting paths or methods outside the explicit daemon job
  scope
- confused consumers such as OpenClaw attempting to write `.agents` directly
  instead of enqueueing daemon-owned jobs

The required controls are:

- validate the configured bind address as loopback before starting mutation
  routes
- require `X-AgentOps-Daemon-Token` or `Authorization: Bearer <token>` for
  every mutation
- load token files only when group/other permission bits are not set
- resolve daemon client mutation tokens in this order: explicit `--token`,
  explicit `--token-file`, `AGENTOPSD_TOKEN`, `AGENTOPS_DAEMON_TOKEN`, then
  activation-file token metadata
- reject mutation requests whose method or path is outside the allowlist for
  that route group
- scope accepted tokens by route capability. Plaintext token files remain
  supported as legacy local-only tokens with the current mutation scope.
  JSON token files may provide multiple credentials:

  ```json
  {
    "tokens": [
      {
        "name": "phone-readonly-submit",
        "token": "<secret>",
        "capabilities": ["submit_job", "openclaw_trigger"]
      },
      {
        "name": "mac-executor",
        "token": "<secret>",
        "capabilities": ["submit_job", "cancel_job", "openclaw_trigger"]
      },
      {
        "name": "bushido-admin",
        "token": "<secret>",
        "capabilities": ["admin"],
        "local_only": true
      }
    ]
  }
  ```

  `submit_job` covers `/jobs` and `/v1/jobs`, `cancel_job` covers
  `/jobs/cancel`, `/v1/jobs/cancel`, and `/v1/jobs/{id}/cancel`,
  `openclaw_trigger` covers `/openclaw/v1/triggers/jobs`, and `admin` covers
  schedule mutations such as `POST /v1/schedules` and
  `DELETE /v1/schedules/{name}`. `admin` satisfies all currently allowlisted
  daemon mutation capabilities but does not bypass the path allowlist.
- reject untrusted `Origin` headers and `Sec-Fetch-Site: cross-site` requests
  even when they are sent from a local browser context

Read-only health/status routes may remain easier to inspect locally, but every
route that appends to the ledger must pass this mutation policy before the
append is attempted. Accepted mutation events include the token profile name in
the ledger actor label, e.g. `ao-http:phone-readonly-submit`, so scoped-token
use is auditable during incident review.

## External Systems

GasCity is the preferred session and event substrate for headless workers. The
daemon consumes GasCity through the AgentOps GasCity adapter; it must not import
GasCity internal packages.

OpenClaw is a consumer of daemon-owned projections. It may read snapshots and
call authorized trigger endpoints, but it does not mutate `.agents` directly and
does not own daemon storage.

Claude and Codex worker sessions are represented through the future
`AgentWorker` / `AgentSession` contract. Wiki/forge is the first consumer of
that runtime, not the owner of the abstraction.

## Migration

The first daemon implementation is opt-in. Existing foreground commands remain
valid until the migration contract explicitly changes their default behavior.

Migration phases:

1. contracts and projections exist
2. daemon can run in foreground and report ready
3. RPI and Dream can submit daemon jobs when ready
4. wiki/forge can use daemon-owned worker jobs
5. OpenClaw can consume snapshots and safe triggers
6. doctor, docs, compatibility matrix, and proof gates decide default changes

## Non-Goals

- replacing GasCity
- embedding OpenClaw
- requiring a live external service for normal tests
- routing daemon wiki/forge through Gemma/Ollama except explicit legacy mode
- promising launchd/systemd install before foreground daemon readiness works

## Validation Hooks

At this ADR level, the contract is valid when it names:

- `agentopsd`
- `job queue`
- `.agents/daemon`
- `local trust`
- `ledger` as source of truth
- projections derived from the ledger

Later atoms add JSON examples, truth tables, and executable tests.
