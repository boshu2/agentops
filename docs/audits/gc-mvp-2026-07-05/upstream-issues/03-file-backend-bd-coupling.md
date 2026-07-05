# File backend: control-dispatcher serve loop and core-pack orders shell out to `bd` and die with "no beads database found"

## Environment

- gascity edge (post-1.3.3), source snapshot `8b17c64`
- macOS (darwin/arm64), `gc` built from source
- `GC_BEADS=file` (file beads backend, `.gc/beads.json`)

## Summary

A file-backend city cannot advance v2 workflows or run the built-in
maintenance orders, because two engine surfaces are hardcoded to the external
`bd` CLI:

1. **Control-dispatcher serve loop.** The core pack's `control-dispatcher`
   agent runs `gc convoy control --serve --follow <agent>`. Its ready-work
   query is a generated shell pipeline of literal
   `bd --readonly --sandbox ready ... --json` invocations. With no bd database
   in the city, every invocation fails (`no beads database found`, emitted by
   `bd`), so the dispatcher lane dies on spawn and control beads
   (step-advance, workflow-finalize) are never processed. Notably the one-shot
   in-process path `gc convoy control <bead-id>` works fine on the file
   backend — it resolves the store through the provider layer.
2. **Core-pack orders.** Most built-in order scripts (`reaper.sh`,
   `gate-sweep.sh`, `orphan-sweep.sh`, `wisp-compact.sh`, `jsonl-export.sh`,
   `cross-rig-deps.sh`, `cascade-nudge-on-blocker-close.sh`,
   `spawn-storm-detect.sh`) call `bd` directly, so they all fail on the file
   backend and `gc doctor`'s `order-firing-current` check reports failures.

Net effect: the file backend works for bead CRUD, sling, and one-shot control
dispatch, but a file-backend city cannot run v2 workflows unattended — we had
to poll `.gc/beads.json` with `jq` and feed `gc convoy control <bead-id>` by
explicit id to get workflows through.

## Minimal repro

1. Create a city with `GC_BEADS=file` and default packs; add any v2 formula.
2. `gc start` — the core `control-dispatcher` session starts
   (`gc convoy control --serve --follow core.control-dispatcher`).
3. Watch the dispatcher session / controller log.
4. Sling the v2 formula so a control bead is created.

**Observed:** the serve loop's ready query fails with
`no beads database found` (from the `bd` subprocess); the dispatcher exits or
idles, the control bead is never claimed, and the workflow never advances.
Running `gc convoy control <that-bead-id>` by hand processes it successfully.
Independently, `gc doctor` shows `order-firing-current` failing because core
order scripts shell to `bd`.

**Expected:** control dispatch and core orders resolve beads through the
configured provider (as the one-shot path does), so a `GC_BEADS=file` city can
advance workflows and fire orders without an external bd installation.

## Code pointers

- `cmd/gc/dispatch_runtime.go:771` — `workflowServeControlReadyQueryForBeads`
  builds the serve-loop ready query; lines 803-824 hardcode
  `emit_ready bd --readonly --sandbox ready ...` (three `bd` invocations plus
  `jq`). Selected for the control dispatcher via `workflowServeWorkQuery` →
  `workflowServeControlReadyQuery` (`dispatch_runtime.go:749-758`) whenever
  the agent has no custom `work_query`.
- `internal/bootstrap/packs/core/agents/control-dispatcher/agent.toml` —
  `start_command` execs `gc convoy control --serve --follow {{.Agent}}`.
- `internal/bootstrap/packs/core/assets/scripts/*.sh` — order scripts calling
  `bd` (e.g. `reaper.sh`, header: "bd close/update commands"; shared tracer
  `_bd_trace.sh`).

## Suggested direction

Same class as the pool/scale probes (filed separately): dispatch these reads
and writes through the configured beads provider instead of a hardcoded `bd`
subprocess — or, at minimum, have the core pack degrade explicitly (skip
bd-only orders, refuse `--serve` with a clear diagnostic) when the provider is
not `bd`.
