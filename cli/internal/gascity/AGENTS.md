---
package: cli/internal/gascity
status: active
owner: agentopsd
contract_source: GasCity OpenAPI fixtures (gascity-openapi-2026-04-28)
---

# cli/internal/gascity

Narrow handwritten HTTP adapter for the public GasCity supervisor API. The
agentopsd lane uses GasCity to manage cities/sessions/events; this package
is the only place that speaks GasCity-over-HTTP from inside `ao`.

## Ownership

- Owned by the agentopsd extraction track. Per-folder ownership pattern
  ported from olympus's service-level AGENTS.md files (e.g. olympus's
  `delphi/`, `argus/`, `hades/` per-service docs).
- The adapter is intentionally **handwritten + narrow**. Generated clients
  are deferred until the GasCity DTO fixtures stabilize — see
  `AdapterStrategy = "handwritten-narrow"` in `types.go`.
- Contract is pinned by `AdapterContractVersion = "gascity-openapi-2026-04-28"`.
  `MinSupportedSupervisorVersion = "0.13.0"`. Bumping either requires a
  fixture refresh + coordinated change in callers.

## Public interfaces

| Symbol | Purpose |
|---|---|
| `NewClient(cfg Config) (*Client, error)` | Construct a configured client; validates endpoint URL |
| `Client.Health(ctx) / Ready(ctx)` | GET /health, GET /readiness probes |
| `Client.CreateCity / ListCities / GetCity` | City lifecycle |
| `Client.CreateSession / ListSessions / SendMessage / TailEvents` | Session lifecycle + event streaming |
| `Client.Fallback*` | Degraded-mode helpers when supervisor is offline (see `fallback.go`) |
| `ValidateContractVersion(string) error` | Reject fixture sets the adapter wasn't written against |
| DTOs: `HealthResponse`, `CityCreateRequest/Response`, `SessionCreateRequest/Response`, `EventEnvelope`, etc. | Stable JSON wire types |

## Non-obvious rules

- **`X-GC-Request` (`MutationHeader`) is required on all mutations.** The
  client adds it automatically using `Config.MutationToken` (default
  `"agentops"`). Don't bypass — supervisor will reject.
- **`X-GC-Request-Id` (`RequestIDHeader`) is round-tripped on every response**
  into `ResponseMeta.RequestID` for correlation. Surface it in error paths.
- **No retries, no rate-limiting, no circuit-breaker here.** The adapter is
  a thin wrapper; resilience policy lives in callers.
- **Endpoint must include scheme + host** (`NewClient` rejects bare paths).
- **Fallback path is intentional, not a hack.** When the supervisor is down,
  callers can switch to `fallback.go` helpers that surface a typed
  unavailable error rather than network noise.

## Cross-references

- `cli/cmd/ao/gc_bridge.go`, `gc_events.go`, `rpi_phased_gc.go` — CLI bridge
  that holds `Client` instances. CLAUDE.md flags these as the live wiring;
  deprecated tmux-mode files (`rpi_phased_tmux.go`, `rpi_loop_supervisor.go`,
  etc.) must NOT be revived.
- `cli/internal/daemon/` — agentopsd daemon owns lifecycle and may invoke
  this client.
- External: GasCity supervisor (Go service, not in this repo).
