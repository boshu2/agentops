# GasCity Integration Contract

AgentOps consumes GasCity through a narrow public API/SSE adapter. The adapter
must treat the published GasCity HTTP API and event streams as the source of
truth. It must not import GasCity `internal/` packages or depend on private
filesystem layouts.

## Source Contract

Authoritative upstream references:

- `/Users/bo/dev/gascity/docs/reference/api.md`
- `/Users/bo/dev/gascity/docs/reference/events.md`
- GasCity OpenAPI schema exposed by the supervisor as `openapi.json`
- GasCity event JSON schema exposed as `events.json`

The first AgentOps adapter is handwritten and intentionally narrow. It covers
only the operations required by RPI, daemon jobs, worker sessions, readiness,
and event replay. Generated clients are deferred until the handwritten DTO
fixtures expose enough stable surface to justify generation.

## Boundary Rules

- AgentOps imports no GasCity `internal/` packages.
- AgentOps records the GasCity adapter contract version in code and tests.
- Every handwritten DTO gets a fixture round-trip test.
- Live GasCity tests are opt-in; fake-server conformance is required in normal
  CI.
- All mutation responses retain the `X-GC-Request-Id` correlation value.

## Required Headers

GasCity requires `X-GC-Request` on every mutation:

- `POST`
- `PUT`
- `PATCH`
- `DELETE`

Any non-empty value is acceptable. Missing `X-GC-Request` must be treated as a
client bug and surfaced without retrying the same request blindly.

Every GasCity response carries `X-GC-Request-Id`. AgentOps must copy this value
into daemon ledger events, RPI registry projections, and user-visible degraded
errors when available.

## Readiness

The adapter must distinguish these states:

| Probe | Success Meaning | Failure Meaning |
|-------|-----------------|-----------------|
| `GET /health` | supervisor process answers | local supervisor reachable |
| `GET /v0/readiness` | supervisor can serve API work | supervisor degraded or unavailable |
| `GET /v0/provider-readiness` | providers can execute sessions | worker provider degraded |
| `GET /v0/city/{cityName}/readiness` | target city is prepared | city not ready for jobs |

Binary discovery is not readiness. `gc` being present on `PATH` is only a
fallback hint.

## Compatibility Matrix

The adapter must make runtime capability explicit instead of collapsing every
state into "GasCity works" or "GasCity missing".

| Runtime Mode | Required Surface | Supported AgentOps Behavior | Not Allowed |
|--------------|------------------|-----------------------------|-------------|
| no-GasCity | no `gc` binary, no API endpoint | foreground non-GasCity commands continue; `ao doctor` reports a non-required GasCity warning; fake-server CI still runs | claiming headless Codex/Claude worker readiness |
| CLI fallback | compatible `gc` binary and `gc status --json`, API/SSE unavailable | legacy bridge commands may run where explicitly selected; diagnostics must say fallback is active | treating CLI polling as equivalent to API/SSE replay |
| API/SSE | GasCity readiness endpoints, mutation headers, sessions, transcripts, event list, and SSE stream | preferred path for RPI phase execution, daemon jobs, AgentWorker sessions, event replay, lost-session classification | importing GasCity `internal/` packages or skipping `X-GC-Request` |
| daemon mode | API/SSE plus `agentopsd` readiness and daemon mutation token | daemon owns accepted RPI, Dream, wiki/forge, and OpenClaw-triggered jobs; session IDs and cursors are persisted through the ledger | returning accepted daemon work before durable ledger append |
| OpenClaw consumer | daemon OpenClaw read endpoints and authorized trigger endpoint | OpenClaw reads projections and asks AgentOps to enqueue work through mutation gates | direct `.agents` writes from OpenClaw |

Normal CI covers no-GasCity, CLI fallback, API/SSE, and daemon mode through
fixtures. Live GasCity checks remain opt-in with `AGENTOPS_LIVE_GASCITY=1`.

## Session Operations

The first adapter must cover:

- list or discover cities
- read city status/readiness
- create or find sessions
- submit/nudge prompts
- read session status
- fetch transcript/result evidence
- cancel or stop where the public API supports it

Session IDs and aliases returned by GasCity must be persisted in daemon ledger
events before AgentOps reports accepted daemon work as running.

## Event And SSE Replay

The adapter must support both list and stream forms:

- supervisor list: `GET /v0/events`
- supervisor SSE: `GET /v0/events/stream`
- city list: `GET /v0/city/{cityName}/events`
- city SSE: `GET /v0/city/{cityName}/events/stream`

Replay requirements:

- support `Last-Event-ID` when using SSE
- support GasCity cursor parameters such as `after_seq` or `after_cursor`
  where the endpoint exposes them
- persist the last consumed cursor with the AgentOps job/session state
- tolerate heartbeat frames without treating them as semantic events
- reconcile through REST list APIs after reconnect

SSE replay is required for daemon restart and RPI phase recovery. A disconnected
stream is degraded, not terminal, until REST reconciliation proves the session
is terminal or unreachable.

## Error Model

GasCity returns RFC 9457 Problem Details bodies for errors. AgentOps must retain:

- HTTP status
- Problem Details `type`, `title`, and `detail`
- semantic code prefixes in `detail`
- `X-GC-Request-Id`

The adapter maps these into typed AgentOps errors:

| GasCity Failure | AgentOps Classification |
|-----------------|-------------------------|
| missing `X-GC-Request` | `client_contract_error` |
| readiness failure | `provider_unreachable` or `city_unready` |
| not found after accepted session | `lost` until reconciliation proves otherwise |
| stream setup problem | `event_stream_unavailable` |
| validation problem | `request_rejected` |

## Versioning

AgentOps pins a local adapter contract version. A future generated client may
replace handwritten DTOs only after:

- fixtures exist for every operation AgentOps uses
- generated DTOs pass the same round-trip tests
- the fallback CLI bridge still works when the API is unavailable
- release notes describe the migration

## Validation Hooks

This contract is valid when it names:

- `X-GC-Request`
- `X-GC-Request-Id`
- SSE replay
- versioning
- no GasCity `internal/` imports
