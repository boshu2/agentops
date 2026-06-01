---
id: code-map-agentopsd-2026-04-29
type: code-map
date: 2026-04-29
status: initial — to be expanded as agentopsd CLI surface stabilizes (post-Wave 4-5)
---

# agentopsd Codebase Map

> **Status:** Initial draft, modeled on `olympus/docs/code-map/olympus-codebase-map.md`.
> Many sections are placeholders; populate as the agentopsd extraction stabilizes
> (post-Wave 4 / Wave 5 of the `agentops-tqc` epic). The current binary name is
> still `ao` — the rename to `agentopsd` lands later in the epic.
>
> **Primary goal:** Help operators and contributors find the right subsystem quickly.
> **Canonical architecture contract:** TODO — there is no `docs/specs/index.md` yet
> for agentopsd. See `docs/ARCHITECTURE.md`, `docs/agentops-system-map.md`,
> `docs/cli-surface.md`, and `docs/HOOKS.md` until the spec set lands.

## At A Glance

agentopsd is a CLI-first knowledge-flywheel daemon extracted from the legacy
AgentOps tree. The `ao` binary (planned rename: `agentopsd`) drives the RPI
lifecycle (Research → Plan → Implement), runs overnight curation/Dream cycles,
serves a local daemon for hooks and job execution, and harvests learnings
into the `.agents/` flywheel.

Module path: `github.com/boshu2/agentops/cli` (Go 1.26).

## Repository Layout

| Path | Role |
|------|------|
| `cli/cmd/ao/` | CLI entrypoint and command wiring (cobra). 496 files, ~63k LOC of non-test source. The single largest surface in the repo. |
| `cli/cmd/skill-frontmatter-json/` | Auxiliary CLI: emit skill frontmatter as JSON |
| `cli/internal/rpi/` | RPI lifecycle: artifact tracking, cancel, cleanup, executor, phased GC |
| `cli/internal/overnight/` | Overnight curator: checkpoints, Dream stages, ingest, council, morning packets |
| `cli/internal/daemon/` | Long-running daemon: auth, Dream/RPI executors, reconcile, registry, runner |
| `cli/internal/search/` | `.agents/` search: bead context, constraint index, findings, label split |
| `cli/internal/lifecycle/` | Knowledge flywheel close-loop, curate, dedup (extracted from cmd/ao) |
| `cli/internal/ratchet/` | Brownian Ratchet chain log + filelock-protected writes |
| `cli/internal/context/` | Context bundle assembly, brief render, budget |
| `cli/internal/llm/` | LLM client abstraction, chunker, forge tier-1 |
| `cli/internal/vibecheck/` | Code/skill vibe analysis with amnesia/drift/logging detectors |
| `cli/internal/eval/` | Eval engine: baseline, compare, coverage, scorecard, runtime |
| `cli/internal/goals/`, `cli/internal/quality/` | Goals/fitness + repo-quality doctor metrics |
| `cli/internal/storage/`, `cli/internal/types/` | Filesystem helpers + shared type definitions |
| `cli/internal/{agentworker,wikiworker,openclaw,bridge,formatter,gascity,knowledge,corpus,forge,harvest,mine,pool,provenance,parser,plans,resolver,safety,shellutil,state,taxonomy,notebook,bench,cycles,autodev}/` | Smaller subsystems — see Packages table below |
| `cli/pkg/`, `cli/embedded/`, `cli/hooks/`, `cli/bin/` | Public Go API surface, embedded assets, hook scripts, build outputs |
| `docs/`, `docs/code-map/` | This map; `ARCHITECTURE.md`, `cli-surface.md`, `HOOKS.md`, `SCHEMAS.md`, runbooks |
| `skills/`, `skills-codex/`, `skills-codex-overrides/` | AgentOps skill bundles consumed by the CLI |
| `agents/`, `wiki/`, `evals/`, `tests/` | Agent prompts, generated wiki, eval fixtures, integration tests |
| `homebrew-tap/`, `Formula/` | Distribution metadata |

## Runtime Control Flow

> TODO — flesh out once Wave 4-5 stabilizes the entrypoints. Today the typical paths are:

1. `ao rpi <phase>` — execute one RPI phase (research/plan/implement/etc.); state under `.agents/rpi/`.
2. `ao overnight run` — full overnight cycle (close-loop → defrag → metrics → retrieval-bench → knowledge brief → Dream Council → runner passes → synthesis → morning packet → bead sync). Driven by `cli/internal/overnight/`.
3. `ao daemon start|status` — long-running daemon for hook execution, RPI runner, and Dream jobs (`cli/internal/daemon/`).
4. `ao search`, `ao inject`, `ao maturity`, `ao compile` — knowledge-flywheel verbs over `.agents/`.
5. `ao validate`, `ao vibe`, `ao goals`, `ao ratchet` — validation gates.

## Key Entrypoints

| Entrypoint | Why you start here |
|-----------|--------------------|
| `cli/cmd/ao/main.go` | CLI process entry — calls `Execute()` |
| `cli/cmd/ao/app.go` | Cobra root command and global flag wiring (TODO: confirm split with `agentopsd.go`) |
| `cli/cmd/ao/agentopsd.go` | Future-binary command surface (precursor to the `agentopsd` rename) |
| `cli/cmd/ao/daemon_jobs.go` | Daemon job dispatch from the CLI side |
| `cli/internal/rpi/` (multiple) | RPI phase executors, cleanup, registry — wired into `cli/cmd/ao/rpi*.go` |
| `cli/internal/overnight/` (multiple) | Overnight engine — wired into `cli/cmd/ao/overnight*.go` |
| `cli/internal/daemon/auth.go`, `dream_executor.go`, `rpi_runner.go`, `reconcile.go` | Daemon authn, executors, reconcile loop |

## Packages

Top packages by file count (non-test source under `cli/internal/`):

| Package | Files | LOC (src) | One-line purpose |
|---------|------:|----------:|------------------|
| `cli/cmd/ao` | 496 | ~63,137 | All cobra commands, flag wiring, glue between subsystems. Largest single surface; primary refactor target as the daemon extracts. |
| `cli/internal/rpi` | 28 | 4,921 | RPI lifecycle support: artifact paths, cancel/signal handling, stale-run cleanup, phased GC, executor plumbing. |
| `cli/internal/overnight` | 24 | 7,151 | Overnight curator stages, checkpoints (incl. Darwin clonefile path), ingest, Dream Council, morning packet rendering. |
| `cli/internal/daemon` | 17 | 5,816 | Long-running daemon: token-gated auth, Dream/RPI executors, RPI registry/runner, reconcile loop. |
| `cli/internal/search` | 17 | 3,290 | `.agents/` retrieval: bead context, constraint index (`.agents/constraints/index.json`), finding match, label utilities. |
| `cli/internal/vibecheck` | 15 | 1,244 | Vibe analyzer with detector plugins (amnesia, drift, logging, …). |
| `cli/internal/lifecycle` | 11 | 2,680 | In-process knowledge-flywheel close-loop, curate, dedup. Extracted from `cli/cmd/ao` so Dream's REDUCE stage can drive it without shelling out. |
| `cli/internal/ratchet` | 10 | 3,207 | Brownian Ratchet chain log: append-only entries, filelocks, contract validation. |
| `cli/internal/goals` | 10 | 2,670 | GOALS.yaml/MD model, fitness measurement, drift tracking. |
| `cli/internal/context` | 10 | 2,223 | Context-bundle assembly, brief rendering, token-budget enforcement. |
| `cli/internal/llm` | 10 | 2,079 | LLM client abstraction, chunker, forge tier-1 extraction. |
| `cli/internal/eval` | 9 | 3,002 | Eval engine: baseline/compare, coverage, runtime, scorecard. |
| `cli/internal/quality` | 8 | 2,358 | Repo-quality doctor: golden metrics, health/ops metrics, codex-skills lint, stale-refs. |
| `cli/internal/gascity` | 6 | 1,581 | TODO — placeholder summary; gascity is the energy/budget accounting subsystem. |
| `cli/internal/storage` | 6 | 1,224 | Filesystem helpers: locked file IO, search index. |
| `cli/internal/agentworker`, `wikiworker`, `bridge`, `openclaw`, `knowledge`, `formatter`, `types`, … | 3-4 each | 0.4-1.5k each | Smaller leaf packages — TODO: per-package summaries. |

### Top 3 packages — detail

#### `cli/internal/rpi/` (28 files, 4,921 LOC)

- **Purpose:** Support library for the RPI lifecycle (Research → Plan → Implement). Owns artifact paths, cancel-signal parsing, stale-run discovery, phased GC, executor plumbing.
- **Key public types/functions (sample):**
  - `PhaseArtifactNumberPattern` — regex used everywhere RPI artifacts are scanned by phase number.
  - `ProcessInfo` — parsed process metadata from `ps`-style introspection (used by cancel/cleanup).
  - `StaleRunEntry` — describes a stale RPI run discovered during cleanup scanning.
- **Imports from internal:** `cli/internal/types` only — `rpi` is intentionally near the bottom of the dependency graph.
- **Imported by:** `cli/internal/daemon` (rpi_runner, rpi_registry, reconcile), `cli/internal/eval` (runtime), `cli/internal/overnight` (runner passes). Heaviest re-user is `cli/cmd/ao/rpi*.go`.

#### `cli/internal/overnight/` (24 files, 7,151 LOC)

- **Purpose:** Drives the nightly curator/Dream pipeline: checkpoints (with Darwin `clonefile` fast path + cross-platform fallback), ingest, REDUCE/Dream Council stages, morning-packet rendering.
- **Key public types/functions (sample, TODO confirm with API freeze):**
  - Checkpoint clone helpers split by platform (`checkpoint_clone_darwin.go` vs `checkpoint_clone_fallback.go`).
  - Boundary-test harness (`withExecShim`) for swapping `ExecCommand` in tests.
  - `seedAgents` test helper that builds a fake `.agents/` tree (used widely by overnight tests).
- **Imports from internal:** `cli/internal/{corpus, daemon, forge, harvest, lifecycle, mine, pool, provenance, rpi, search}` — overnight is the highest-level orchestrator and the densest internal-import node.
- **Imported by:** `cli/cmd/ao/overnight*.go` and (transitively) the daemon's Dream executor.

#### `cli/internal/daemon/` (17 files, 5,816 LOC)

- **Purpose:** Long-running daemon process. Hosts the Dream executor, RPI runner, RPI registry, reconcile loop, and a token-gated mutation API.
- **Key public types/functions (sample):**
  - Authentication middleware enforcing the mutation-token header (`auth.go` + `TestAuthRequiresMutationTokenHeader`).
  - `DreamRunLoopOptions` and `DreamMode` — typed run-loop config for the Dream executor.
  - RPI registry/runner pair that owns in-flight RPI runs and reconciles their state.
- **Imports from internal:** `cli/internal/{agentworker, gascity, openclaw, rpi, wikiworker}`.
- **Imported by:** `cli/cmd/ao/daemon*.go` and `cli/internal/overnight` (Dream stage hands work to the daemon executor).

## Daemon internals

`cli/internal/daemon/` is the single biggest internal package (69 `*.go` files at
write time, the file-count table above samples non-test source only). It is the
persistent job queue and event store that the RPI runner, Dream cycle, factory
admission, wiki, and plans executors all submit work to.

### Ledger-based event store

The daemon has **no database** — state is a single append-only JSONL event
ledger, replayed into in-memory projections on startup.

- **Path:** `~/<root>/.agents/daemon/ledger.jsonl` (`LedgerFileName = "ledger.jsonl"`,
  see `cli/internal/daemon/store.go`). `NewStore(root)` resolves the directory.
- **Append-only:** each mutation writes a `LedgerEvent` (`cli/internal/daemon/events.go`:
  `NewLedgerEvent`, `NormalizeLedgerEvent`, `ValidateRequestID`). Events carry a
  `RequestID` for idempotency.
- **Replay on startup:** `replayLedger` walks rotated archives **oldest-first**,
  then the live `ledger.jsonl`, reducing events into projections
  (`cli/internal/daemon/projections.go`). Corrupt/over-cap lines are skipped and
  logged (quarantined) rather than aborting the replay.
- **Rotation:** when the live ledger crosses `DefaultLedgerMaxBytes`, `rotateLedger`
  atomically renames it to a timestamped, gzip-compressed archive (archive prefix
  in `store.go`); a fresh `ledger.jsonl` is created lazily on the next write.
  `cli/internal/daemon/ledger_health.go` reports ledger health.

### Job lifecycle

The job queue (`cli/internal/daemon/jobs.go`, `Queue` type) drives every job
through a uniform state machine:

```
SubmitJob ──► ClaimJob (lease) ──► Heartbeat (renew lease) ──► CompleteJob | FailJob
                                                          └──► CancelJob
```

- `Queue.SubmitJob` enqueues a `JobSpec` payload.
- `Queue.ClaimJob` hands a job to an executor under a **lease**; the executor must
  renew with `Queue.Heartbeat` (`applyJobHeartbeat`) or the lease expires and the
  job becomes re-claimable (the `reconcile.go` loop and `supervisor.go` enforce
  lease/heartbeat liveness).
- Terminal transitions: `Queue.CompleteJob` / `Queue.FailJob`; out-of-band
  `Queue.CancelJob`.
- The HTTP server (`cli/internal/daemon/server.go`) is read-only except for
  token-gated mutation handlers (`handleSubmitJob`, `handleCancelJob`); auth lives
  in `auth.go`.

### JobSpec types

Each job type has a typed payload spec serialized into the ledger event:

| Spec | File | Drives |
|------|------|--------|
| `RPIRunJobSpec` | `cli/internal/daemon/rpi_jobs.go` | a multi-cycle RPI run (via `rpi_runner.go`, `rpi_run.go`, `rpi_registry.go`) |
| `RPIPhaseJobSpec` | `cli/internal/daemon/rpi_jobs.go` | a single RPI phase (`rpi_executor.go`) |
| `DreamRunJobSpec` | `cli/internal/daemon/dream_jobs.go` | a full overnight/Dream run (`dream_executor.go`) |
| `DreamStageJobSpec` | `cli/internal/daemon/dream_jobs.go` | a single Dream stage |

Specs carry a `SchemaVersion` (e.g. `DreamJobSpecSchemaVersion`) so older binaries
replaying newer ledgers fail loudly on mismatch (`jobspec_compat_test.go`). Recurring/
scheduled submission is handled by `RecurrenceSupervisor` (`cli/internal/daemon/recurrence.go`,
`recurrence_payload.go`); the schedule file format is parsed by `cli/internal/schedule/parser.go`.
Additional executors (`factory_admission_*`, `wiki_jobs.go`/`wiki_executor.go`,
`plans_*`) follow the same submit→claim→complete contract.

### CLI surface

Wired in `cli/cmd/ao/daemon_jobs.go` (registration: `daemonJobsCmd.AddCommand(...)`):

| Command | Purpose |
|---------|---------|
| `ao daemon jobs submit` | enqueue a job (`SubmitJob`) |
| `ao daemon jobs list` | list queued/active/terminal jobs |
| `ao daemon jobs show <job-id>` | inspect one job's state |
| `ao daemon jobs wait <job-id>` | block until a job reaches a terminal state |
| `ao daemon jobs cancel <job-id>` | request cancellation (`CancelJob`) |
| `ao daemon events tail` | stream the ledger event feed |

(`ao daemon jobs` / `ao daemon events` are the parent groups.) Soak/load testing
lives separately in `cli/cmd/ao/daemon_soak.go` (`ao daemon soak`).

## Cross-references (internal package import graph)

Edges sampled from `grep -h "agentops/cli/internal" cli/internal/<pkg>/*.go`:

```
overnight ──► corpus, daemon, forge, harvest, lifecycle, mine, pool,
              provenance, rpi, search, overnight (intra)
daemon    ──► agentworker, gascity, openclaw, rpi, wikiworker
lifecycle ──► goals, pool, ratchet, storage, types
search    ──► notebook, ratchet, types
context   ──► search
llm       ──► agentworker, parser, types
ratchet   ──► types
eval      ──► rpi
rpi       ──► types
vibecheck ──► (no internal deps; leaf)
```

Observations:

- `cli/internal/types` and `cli/internal/storage` sit at the bottom — leaf packages with broad fan-in.
- `cli/internal/overnight` is the densest aggregator (10 internal imports).
- `cli/internal/daemon` and `cli/internal/overnight` both depend on `cli/internal/rpi`, which is the cross-cutting RPI lifecycle library.
- `cli/cmd/ao/` is the global integration point — every `cli/internal/*` package eventually reaches it through cobra command files (not enumerated here; that's the next pass).

## Persistence Surfaces

| Surface | Path | Notes |
|--------|------|-------|
| RPI runs / artifacts | `.agents/rpi/runs/<id>/`, `.agents/rpi/artifacts/` | Owned by `cli/internal/rpi`; scanned by cleanup + phased GC |
| Overnight checkpoints | `.agents/overnight/<run>/checkpoint*` | Owned by `cli/internal/overnight`; Darwin uses clonefile |
| Knowledge flywheel | `.agents/{learnings,patterns,findings,research,retros,…}/` | Read-many surfaces for `cli/internal/{search,lifecycle,context}` |
| Constraint index | `.agents/constraints/index.json` | `cli/internal/search/constraint.go` schema owner |
| Beads | `.beads/` (bd CLI) | External to this repo's Go code; consumed by `cli/internal/search/bead_context.go` and `cli/cmd/ao` bead helpers |
| Daemon job ledger | `.agents/daemon/ledger.jsonl` | Append-only JSONL event log, replayed into projections on startup, rotated + gzipped at `DefaultLedgerMaxBytes`. Owner: `cli/internal/daemon/store.go`. See "Daemon internals" above. |
| Ratchet chain | TODO — confirm path under `.agents/ratchet/` |
| Goals | `GOALS.yaml`, `GOALS.md` (repo root) + history under `.agents/goals/` |

## Operator Navigation Notes

> TODO — write real runbooks once the agentopsd binary lands. Until then:

When debugging an RPI run:

1. `ao rpi status` (or inspect `.agents/rpi/runs/<id>/`).
2. Read run artifacts under `.agents/rpi/artifacts/`.
3. `cli/internal/rpi/cleanup.go` documents stale-run heuristics.

When debugging an overnight cycle:

1. `ao overnight curator status --json`.
2. Open the most recent `.agents/overnight/latest/` checkpoint directory.
3. Cross-reference with the morning packet (e.g. `D:\vault\dream\YYYY-MM-DD.md` on bushido).

When debugging the daemon:

1. `ao daemon status`, then `ao daemon jobs list` / `ao daemon jobs show <id>` to see queue state.
2. Inspect the event ledger at `.agents/daemon/ledger.jsonl` (append-only; replayed on startup) — the source of truth for all job state.
3. `ao daemon events tail` to stream the live event feed; cross-reference `cli/internal/daemon/reconcile.go` and `rpi_registry.go` for lease/state transitions.

## Scope Notes

- This map describes the **current repository state** as of 2026-04-29 (Wave 4 of `agentops-tqc`).
- The binary is still `ao`; the `agentopsd` rename and corresponding directory split are tracked under the parent extraction epic.
- LOC counts are non-test source only (`*.go` minus `*_test.go`) measured at write time. Recount before any release.
- Cross-reference graph was sampled with `grep`, not built from `go list -deps`. Treat it as a navigation aid, not a complete graph. TODO: replace with a `go list`-driven generator and check it in CI.
- Several `cli/internal/` packages (e.g. `gascity`, `bridge`, `harvest`, `mine`, `pool`, `provenance`, `safety`, `state`, `taxonomy`) carry no per-package summary yet — flagged as TODO above.
