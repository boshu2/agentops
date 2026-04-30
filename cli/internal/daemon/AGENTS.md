---
package: cli/internal/daemon
status: active
owner: agentopsd
contract_source: cli/internal/daemon/types.go (JobType, EventType, JobStatus enums)
---

# cli/internal/daemon

`agentopsd` long-running daemon: ledger-backed job queue, event sourcing, and read-only HTTP server for RPI/Dream/Wiki/OpenClaw jobs.

## Ownership

- **Owner:** agentopsd extraction track (epic `agentops-tqc`).
- **Lifecycle:** persistent process. Server (`server.go`) handles HTTP. Supervisor (`supervisor.go`) drives runners. Reconciler (`reconcile.go`) replays the ledger and rebuilds projections at boot.
- **State of truth:** the append-only ledger (`store.go`). Projections (`projections.go`) are a derived view that may be marked stale and rebuilt; never trust them as the primary store.

## Interfaces

- **HTTP surface (read-only by default):** `ReadOnlyServer` in `server.go` exposes `/health`, `/ready`, `/status`, `/events`, plus mutation endpoints when `MutationPolicy` allows. JSON shapes are the public wire contract — change them with care.
- **Job submission:** `SubmitJobRequest` (types.go) plus `JobType` enum:
  - `rpi.run`, `rpi.phase`, `dream.run`, `dream.stage`, `wiki.build`, `wiki.forge`, `openclaw.snapshot`
- **Event stream:** `EventType` enum (`job.accepted`, `job.claimed`, `job.heartbeat`, `job.lease_expired`, `job.completed`, `job.failed`, `job.cancelled`, `projection.marked_stale`, `projection.rebuilt`). Consumers should treat unknown events as forward-compatible.
- **Runner registry:** `rpi_registry.go` plus `rpi_runner.go`, `dream_executor.go` give the daemon pluggable run-handlers per job type.

## Non-obvious rules

- **Read-only is the default posture.** Mutation endpoints require explicit `MutationPolicy` in `ServerOptions`. Do not weaken this without an operator decision recorded in beads.
- **Replay before serve.** On boot the daemon must finish ledger replay (`reconcile.go`) before reporting `ready=true`. `ReadOnlyReadyResponse.LedgerReplayStatus` is the operator-facing signal.
- **Projection lag is exposed, not hidden.** `ProjectionLag` and `degraded_reasons` surface drift; callers use these to decide whether to fail open or wait.
- **Lease expiry is event-driven.** Heartbeats extend the lease; `job.lease_expired` is emitted by the supervisor when they stop. Runners must heartbeat on the cadence configured by `QueueOptions`.
- **No symlinks anywhere in the data dirs.** The repo-wide no-symlink rule applies — copy files instead.

## Cross-references

- Parent epic: `agentops-tqc` (Olympus → agentopsd extraction).
- Pattern source: olympus per-folder `AGENTS.md` ownership convention (root + 6 service folders + 6 role folders).
- Sibling packages: `cli/internal/types` (shared domain types), `cli/internal/openclaw` (snapshot inputs), `cli/internal/overnight` (Dream job type producer).
- Olympus analogue: `~/dev/personal/olympus/services/apollo/` (apollo-style state transitions and ledger replay).
