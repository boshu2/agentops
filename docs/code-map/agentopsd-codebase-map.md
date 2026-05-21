---
id: code-map-agentopsd-2026-05-20
type: code-map
date: 2026-05-20
status: current navigation aid
---

# agentopsd Codebase Map

> **Primary goal:** Help operators and contributors find the right subsystem quickly.
> **Canonical architecture contracts:** Start with `docs/ARCHITECTURE.md`,
> `docs/agentops-system-map.md`, `docs/contracts/agentops-daemon.md`,
> `docs/contracts/agentopsd-control-plane.md`, and `docs/contracts/context-map.md`.

## At A Glance

agentopsd is a CLI-first knowledge-flywheel daemon extracted from the legacy
AgentOps tree. The `ao` binary drives the RPI lifecycle
(Research → Plan → Implement), runs overnight curation/Dream cycles,
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

Typical operator paths:

1. `ao rpi <phase>` — execute one RPI phase (research/plan/implement/etc.); state under `.agents/rpi/`.
2. `ao overnight run` — full overnight cycle (close-loop → defrag → metrics → retrieval-bench → knowledge brief → Dream Council → runner passes → synthesis → morning packet → bead sync). Driven by `cli/internal/overnight/`.
3. `ao daemon start|status` — long-running daemon for hook execution, RPI runner, and Dream jobs (`cli/internal/daemon/`).
4. `ao search`, `ao inject`, `ao maturity`, `ao compile` — knowledge-flywheel verbs over `.agents/`.
5. `ao validate`, `ao vibe`, `ao goals`, `ao ratchet` — validation gates.

## Key Entrypoints

| Entrypoint | Why you start here |
|-----------|--------------------|
| `cli/cmd/ao/main.go` | CLI process entry — calls `Execute()` |
| `cli/cmd/ao/root.go` | Cobra root command, command groups, and version wiring |
| `cli/cmd/ao/agentopsd.go` | AgentOps daemon/control-plane command surface |
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
| `cli/internal/gascity` | 6 | 1,581 | GasCity remote-compute integration types, compatibility checks, and adapter budget/accounting support. |
| `cli/internal/storage` | 6 | 1,224 | Filesystem helpers: locked file IO, search index. |
| `cli/internal/agentworker`, `wikiworker`, `bridge`, `openclaw`, `knowledge`, `formatter`, `types`, … | 3-4 each | 0.4-1.5k each | Focused adapters and leaf libraries for worker sessions, wiki/OpenClaw integration, knowledge parsing, formatting, and shared structs. |

### Top 3 packages — detail

#### `cli/internal/rpi/` (28 files, 4,921 LOC)

- **Purpose:** Support library for the RPI lifecycle (Research → Plan → Implement). Owns artifact paths, cancel-signal parsing, stale-run discovery, phased GC, executor plumbing.
- **Key public types/functions (sample):**
  - `PhaseArtifactNumberPattern` — regex used everywhere RPI artifacts are scanned by phase number.
  - `ProcessInfo` — parsed process metadata from `ps`-style introspection (used by cancel/cleanup).
  - `StaleRunEntry` — describes a stale RPI run discovered during cleanup scanning.
- **Imports from internal:** `cli/internal/types` only — `rpi` is intentionally near the bottom of the dependency graph.
- **Used by:** `cli/internal/daemon` (rpi_runner, rpi_registry, reconcile), `cli/internal/eval` (runtime), `cli/internal/overnight` (runner passes). Heaviest re-user is `cli/cmd/ao/rpi*.go`.

#### `cli/internal/overnight/` (24 files, 7,151 LOC)

- **Purpose:** Drives the nightly curator/Dream pipeline: checkpoints (with Darwin `clonefile` fast path + cross-platform fallback), ingest, REDUCE/Dream Council stages, morning-packet rendering.
- **Key public types/functions (sample):**
  - Checkpoint clone helpers split by platform (`checkpoint_clone_darwin.go` vs `checkpoint_clone_fallback.go`).
  - Boundary-test harness (`withExecShim`) for swapping `ExecCommand` in tests.
  - `seedAgents` test helper that builds a fake `.agents/` tree (used widely by overnight tests).
- **Imports from internal:** `cli/internal/{corpus, daemon, forge, harvest, lifecycle, mine, pool, provenance, rpi, search}` — overnight is the highest-level orchestrator and the densest internal-import node.
- **Used by:** `cli/cmd/ao/overnight*.go` and (transitively) the daemon's Dream executor.

#### `cli/internal/daemon/` (17 files, 5,816 LOC)

- **Purpose:** Long-running daemon process. Hosts the Dream executor, RPI runner, RPI registry, reconcile loop, and a token-gated mutation API.
- **Key public types/functions (sample):**
  - Authentication middleware enforcing the mutation-token header (`auth.go` + `TestAuthRequiresMutationTokenHeader`).
  - `DreamRunLoopOptions` and `DreamMode` — typed run-loop config for the Dream executor.
  - RPI registry/runner pair that owns in-flight RPI runs and reconciles their state.
- **Imports from internal:** `cli/internal/{agentworker, gascity, openclaw, rpi, wikiworker}`.
- **Used by:** `cli/cmd/ao/daemon*.go` and `cli/internal/overnight` (Dream stage hands work to the daemon executor).

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
| Daemon state | `.agents/daemon/` plus per-user token material under `~/.agents/daemon/` when configured | Owned by `cli/internal/daemon` and `cli/cmd/ao/daemon*.go` |
| Ratchet chain | `.agents/ao/chain.jsonl` | Loaded and updated by `cli/internal/ratchet` and `ao ratchet *` commands |
| Goals | `GOALS.yaml`, `GOALS.md` (repo root) + history under `.agents/goals/` |

## Operator Navigation Notes

When debugging an RPI run:

1. `ao rpi status` (or inspect `.agents/rpi/runs/<id>/`).
2. Read run artifacts under `.agents/rpi/artifacts/`.
3. `cli/internal/rpi/cleanup.go` documents stale-run heuristics.

When debugging an overnight cycle:

1. `ao overnight curator status --json`.
2. Open the most recent `.agents/overnight/latest/` checkpoint directory.
3. Cross-reference with the morning packet (e.g. `D:\vault\dream\YYYY-MM-DD.md` on bushido).

When debugging the daemon:

1. `ao daemon status`, then inspect daemon state and event output with `ao daemon events tail`.
2. Inspect `cli/internal/daemon/reconcile.go` and `rpi_registry.go` for state transitions.

## Scope Notes

- This map describes the **current repository state** as of 2026-05-20.
- The user-facing binary is `ao`; agentopsd is the daemon/control-plane surface behind that CLI.
- LOC counts are non-test source only (`*.go` minus `*_test.go`) measured at write time. Recount before any release.
- Cross-reference graph was sampled with `grep`, not built from `go list -deps`. Treat it as a navigation aid, not a complete graph.
