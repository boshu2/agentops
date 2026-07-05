# File backend: pool sessions never spawn after startup — reconciler demand probes are hardcoded to the `bd` CLI

## Environment

- gascity edge (post-1.3.3), source snapshot `8b17c64`
- macOS (darwin/arm64), `gc` built from source
- `GC_BEADS=file` (file beads backend, `.gc/beads.json`)

## Summary

On a file-backend city, pool sessions are only spawned during the startup
reconcile. After startup, routing new work to a pool agent (`gc sling`, API
routes) never wakes a pool session — the work sits routed and ready
indefinitely until the controller is restarted, at which point the startup
reconcile picks it up.

The poke path itself is wired (sling calls `PokeController`, the controller
run loop drains `pokeCh` and runs event-driven ticks — `cmd/gc/city_runtime.go`,
`run()` select loop), but the tick's pool demand evaluation cannot succeed on
the file backend: the default `scale_check` / pool demand query is a shell
pipeline that invokes the literal `bd` CLI, which has no database in a
file-backend city and fails with `no beads database found`.

## Minimal repro

1. Create a city with `GC_BEADS=file` and one pool agent (e.g. a codex pool
   with `max_active_sessions > 0`, no custom `work_query`/`scale_check`).
2. `gc start` the city. Note the startup reconcile completes.
3. Create a bead and route it to the pool:
   `gc sling <pool-agent> <bead-id>` (or any route that stamps
   `gc.routed_to=<pool>` on an unassigned ready bead).
4. Wait several patrol intervals.

**Observed:** no pool session is ever spawned; the bead stays ready/unassigned.
Restarting the controller spawns the session (startup path works).

**Expected:** the next reconciler tick (poke-driven or patrol) sees the routed
demand and scales the pool from zero, same as startup does.

## Code pointers

- `internal/config/config.go:3511` — `bdReadyPoolDemandShell` returns a literal
  `bd ready --metadata-field "gc.routed_to=$target" ... --json` command string.
- `internal/config/config.go:3617` — `poolDemandCountShell` wraps it in
  `sh -c` (also requires `jq`); this is the default `EffectivePoolDemandQuery`
  (`config.go:3945`) used when an agent declares no `scale_check`.
- `cmd/gc/pool.go:105` — `shellScaleCheck` executes that command as a
  subprocess; there is no file-backend translation of the `bd` invocation.
- `internal/config/config.go:3763` — `EffectiveWorkQuery` has the same
  hardcoded `bd ready` shape (see `TestEffectiveWorkQueryDefault`).

So the reconciler's spawn decision (and the worker's claim query) both depend
on a `bd` subprocess that structurally cannot work when the beads provider is
`file`, even though the store itself is a first-class provider.

## Suggested direction

Route the demand/work queries through the configured beads provider (the
in-process store, or a provider-dispatched `gc bd`-equivalent shim) instead of
a hardcoded `bd` subprocess, or document that pools require the bd/Dolt
backend.

Related: the control-dispatcher serve loop and core-pack order scripts have
the same `bd` coupling — filed separately.
