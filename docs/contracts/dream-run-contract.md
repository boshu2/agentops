# Dream Run Contract

`ao overnight` is the headless automation surface for Dream, AgentOps' private overnight operator mode.

This contract defines the minimum behavior required for the shipped Dream surfaces:

- `ao overnight setup`
- `ao overnight start|run`
- `ao overnight report`

## Scope

V1 covers:

- local setup guidance with honest scheduler assistance artifacts
- local-first overnight runs against the real repo-local `.agents` corpus
- machine-readable and markdown morning summaries
- ranked executable morning packets synced into next-work and beads when possible
- optional multimodel Dream Council synthesis over bounded runner reports
- built-in DreamScape terrain rendering inside the report
- explicit process supervision and lock behavior
- optional keep-awake assistance on macOS

V1 does not promise:

- guaranteed scheduled execution on sleeping laptops
- tracked source-code edits overnight
- free-form model-to-model chat overnight
- visualization outside the shared report contract

## Command Surface

Primary commands:

- `ao overnight setup`
- `ao overnight start`
- `ao overnight run`
  `run` is an alias for `start`
- `ao overnight curator status|diagnose|enqueue|compact|event`
- `ao overnight report --from <dir-or-summary.json>`

Required flags for `start`:

- `--goal <text>`
- `--output-dir <path>`
- `--run-timeout <duration>`
- `--keep-awake`
- `--no-keep-awake`
- `--runner <name>` (repeatable)
- `--creative-lane`

Required flags for `setup`:

- `--apply`
- `--scheduler <manual|launchd|cron|systemd|task-scheduler|auto>`
- `--at <HH:MM>`
- `--runner <name>` (repeatable)

Required surfaces for `curator`:

- `status --json` reports configured local curator state, Ollama health, model availability, worker queue depth, status.json content, and Tier 2 runner discovery.
- `diagnose` explains missing worker directories, missing vaults, wrong Ollama endpoint, absent models, stale worker PID when detectable, and unsupported OpenClaw/Morai bridge state.
- `enqueue --kind <lint-wiki|dream-seed|ingest-claude-session>` writes only allowlisted knowledge jobs into the configured worker queue.
- `compact --dry-run|--apply` wraps the existing pending-log compaction contract.
- `event` writes a bounded needs-review event record with source, severity, desired action, escalation target, and event budget.

## Process Model

One Dream run is a single bounded process with a stable output directory.

Defaults:

- output directory: `.agents/overnight/latest`
- lock file: `.agents/overnight/run.lock`
- log file: `<output-dir>/overnight.log`
- timeout: `8h`
- keep-awake: enabled by default, opt-out via config or `--no-keep-awake`

## Locking

Dream must prevent overlapping local runs.

Required behavior:

- acquire an exclusive non-blocking lock on the lock file before running
- fail fast if another run already holds the lock
- release the lock when the process exits

## Step Contract

The first slice runs these steps in order:

1. `ao flywheel close-loop --threshold 0h --json`
2. `ao --dry-run defrag --prune --dedup --oscillation-sweep`
3. `ao metrics health --json`
4. `ao retrieval-bench --live --json`
5. optional: `ao knowledge brief --goal <goal> --json`
6. optional: Dream Council packet generation
7. optional: one bounded runner pass per configured Dream runner
8. optional: Dream Council synthesis
9. Dream morning packet synthesis
10. best-effort bead sync for emitted packets

Hard-fail steps:

- close-loop
- metrics health

Soft-fail steps:

- defrag preview
- retrieval bench
- knowledge brief
- Dream Council runner execution
- Dream Council synthesis when no runner completes
- bead sync when `bd` is unavailable or a packet update fails

Soft failures must degrade the report, not delete it.

### Retrieval-bench determinism

`ao retrieval-bench --live --json` is deterministic by construction and Dream's MEASURE stage relies on this contract for plateau detection between nightly runs. The live query set is a hardcoded slice in `cli/cmd/ao/retrieval_bench.go`; the corpus is resolved from a fixed location (`.agents/learnings/`, `--corpus` fixture, or `~/.agents/learnings/` with `--global`); and the retrieval pipeline (`collectLearnings` → `rankLearnings`) uses a stable sort by `CompositeScore` with no random sampling in either `cli/cmd/ao/` or `cli/internal/bench/`. Dream intentionally does not pass a `--seed` flag — there is no RNG to seed, and adding one would be dead code. If a future change introduces non-determinism in the `--live` path (for example, random tie-breaking, sampled sub-corpora, or time-of-day noise beyond the existing freshness score), it is a contract violation: revert it or make the source of randomness explicitly seedable before Dream depends on the affected metric.

## Crash Behavior

If the Dream process fails after creating the output directory, it must still try to write:

- `summary.json`
- `summary.md`

The report must show:

- `status: failed`
- the last completed step
- degraded or failed steps
- log path

## Keep-Awake Behavior

V1 only auto-manages keep-awake on macOS via `caffeinate`.

Rules:

- default-on for local bedtime runs
- opt-out via config or `--no-keep-awake`
- if `caffeinate` is unavailable, continue and mark the run degraded
- non-macOS platforms must not fake scheduler guarantees

## Output Artifacts

Required artifacts:

- `<output-dir>/close-loop.json`
- `<output-dir>/defrag/latest.json` when the preview succeeds
- `<output-dir>/metrics-health.json`
- `<output-dir>/retrieval-bench.json` when live retrieval succeeds
- `<output-dir>/briefing.json` when a goal briefing succeeds
- `<output-dir>/morning-packets/index.json`
- `<output-dir>/morning-packets/index.md`
- `<output-dir>/morning-packets/<rank>-<slug>-<id>.json` for each emitted packet
- `<output-dir>/council/packet.json` when Dream Council is configured
- `<output-dir>/council/<runner>.json` for each successful Dream Council runner
- `<output-dir>/council/synthesis.json` when Dream Council synthesis succeeds
- `<output-dir>/<runID>/iterations/iter-<N>.json` per-iteration cycle history (Micro-epic 2). Written atomically after each iteration finishes so a crash never truncates history; on resume, `RunLoop` rehydrates prior iterations from this directory rather than starting from memory.
- `<output-dir>/summary.json`
- `<output-dir>/summary.md`

## Relationship To CI

GitHub nightly remains the public proof harness.

`ao overnight` is the private compounding engine.

They may share primitive steps and report shapes, but they are not the same operational surface.

`ao overnight setup` helps persist `dream.*` config and generate host-specific
assistance artifacts. The host scheduler still owns actual scheduling semantics.
On Windows, setup may generate a reviewed Task Scheduler registration script
under `.agentops/generated/dream/`; it must not silently register autonomous
jobs.

## Local Curator Contract

`dream.local_curator.*` describes a Tier 1 local curator. V1 supports
`engine: ollama` with a local Gemma model, a worker directory, a vault directory,
an hourly cap, and an allowlist of job kinds. Autodetection may recognize a local
prototype such as `D:\dream`, `D:\vault`, `http://127.0.0.1:11435`, and
`gemma4:e4b`, but the CLI must continue to work when those machine-local paths
are absent.

The local curator is separate from Dream Council. Gemma can draft, lint, seed,
compact audit logs, and emit needs-review events. Tier 2 runners such as Codex
and Claude can review or synthesize those events. OpenClaw/Morai must remain
unsupported on Windows until a real bridge command such as `openclaw` or
`oc-ask` is discoverable from the AgentOps runtime.

No trigger mesh may be unbounded. Every escalation record needs source, severity,
desired action, escalation target, budget, and a ledger entry. A runner must not
recursively invoke another runner without an explicit remaining budget.

## v2 - Iteration Loop (2026-04-09)

Dream v2 replaces the single-pass 5-step script with a bounded outer loop that iterates INGEST -> REDUCE -> MEASURE until a halt condition fires. Each iteration is atomic: checkpointed on entry so any regression or metadata integrity failure can be rolled back cleanly. The v1 step contract above still describes the primitives; v2 re-uses those primitives inside the iteration body.

### Iteration Structure

Each iteration runs four stages in order (Micro-epic 8 / C1 Option A, 2026-04-11):

1. **INGEST** - harvest new signal into the corpus (absorbs `/harvest` overnight work)
2. **REDUCE** - defrag, dedup, compile, and prune. Mutations are written to the checkpoint's **staging tree** (`cp.StagingDir/.agents/<subpath>/`), NOT the live `.agents/` path.
3. **MEASURE** - `corpus.Compute(cp.StagingDir)` computes fitness against the staged mutations (pre-commit). Under warn-only mode, a regression consumes a rescue and the iteration still commits; under strict mode, a regression (or plateau) triggers `cp.Rollback()` and the iteration halts with status `halted-on-regression-pre-commit` — the live `.agents/` tree is **never mutated**.
4. **COMMIT** - `cp.Commit()` atomically promotes the staging tree into live. This is the first point at which external observers of `~/.agents/` see the new state. A post-commit metadata integrity check (`VerifyMetadataRoundTripPostCommit`) runs here as a ratchet-forward second-stage defence (strip detection).

**Semantic invariant:** No external observer of `~/.agents/` ever sees a fitness-regressed state under strict mode. The pre-commit gate holds the line.

**Historical note:** Before M8, the sequence was REDUCE → COMMIT → MEASURE (Option B). A strict regression halt would fire with status `halted-on-regression-post-commit`, but the live tree was already mutated and had to be rolled back imperfectly. That status is retained in types.go for backward compatibility with persisted iterations from pre-M8 runs.

### Checkpointed Subpaths

Only these paths are mutated and rolled back as a unit:

- `.agents/learnings/`
- `.agents/findings/`
- `.agents/patterns/`
- `.agents/knowledge/`
- `.agents/rpi/next-work.jsonl`

### Halt Conditions

| Reason | Iter status (M8+) | Morning report field |
|--------|-------------------|----------------------|
| Wall-clock budget exhausted | `done` on last successful iter | `budget_exhausted: true` |
| Plateau (K consecutive sub-epsilon deltas), strict mode | `halted-on-regression-pre-commit` | `plateau_reason` |
| Regression beyond per-metric floor, strict mode | `halted-on-regression-pre-commit` | `regression_reason` |
| Warn-only budget exhausted then regression fires | `halted-on-regression-pre-commit` | `regression_reason` + `(warn-only budget exhausted)` suffix |
| Post-commit metadata integrity failure | `degraded` + finding routed to morning report | `post-commit metadata integrity: N stripped field(s)` |
| Crash mid-iteration | recovered via `.agents/overnight/COMMIT-MARKER.*` on next startup | status depends on marker state |

### Anti-Goals

- Dream NEVER mutates source code.
- Dream NEVER invokes `/rpi` or any code-mutating flow.
- Dream NEVER performs git operations (no commits, branches, push, rebase, checkout, etc.).
- Dream NEVER creates symlinks anywhere.
- First-slice scope: no swarm/gc fan-out inside iterations (serial goroutines only).

### New Flags

- `--queue=<file>` - operator-pinned nightly priorities (markdown file; uses evolve pinned-queue format)
- `--max-iterations=<N>` - cap iteration count (0 = budget-bounded only)
- `--plateau-epsilon=<F>` - plateau threshold (default 0.01)
- `--plateau-window=<K>` - plateau window K (default 2, minimum 2)
- `--warn-only` - ratchet mode: warn on plateau/regression instead of halting (default true for first 2-3 production runs)
- `--checkpoint-max-mb=<N>` - max checkpoint storage per run (default 512MB)

### Startup Recovery Protocol

- On each Dream startup, before acquiring the lock, `overnight.RecoverFromCrash` scans `.agents/overnight/COMMIT-MARKER.*` and restores clean state.
- `overnight.LockIsStale` with a 12h threshold + PID liveness check reclaims locks from crashed prior runs.
- `overnight.WriteLockPID` writes the current PID into `run.lock` on acquisition.

### Concurrency Guard

`ao harvest` refuses to run while Dream holds the overnight lock (pm-011). Operators must wait for the lock to release or explicitly stop the Dream run before manual harvest sweeps.
