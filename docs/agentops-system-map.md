# AgentOps — System Map

```
┌──────────────────────────────────────────────────────────────────┐
│                    AgentOps at a Glance                          │
├───────────────────┬──────────────────────┬───────────────────────┤
│    52  Skills     │  121 CLI Commands    │   13 Hook Entries     │
│  (workflows)      │  (ao binary)         │  (auto-enforcement)   │
└───────────────────┴──────────────────────┴───────────────────────┘
```

---

## The Pipeline — Skills Calling Skills

The top-level skill `/rpi` chains the full pipeline. Each node is a skill. Arrows show calls.

```
                         ┌─────────────┐
                         │   /evolve   │  ← loops /rpi overnight
                         └──────┬──────┘    fitness-gated
                                │ calls
                                ▼
┌───────────────────────────────────────────────────────────────────┐
│                             /rpi                                  │
│                    (full pipeline orchestrator)                   │
└──┬──────────┬───────────┬─────────────┬──────────┬────────────────┘
   │          │           │             │          │
   ▼          ▼           ▼             ▼          ▼
/research   /plan    /pre-mortem     /crank    /post-mortem
   │          │           │             │          │
   │          │      calls /council     │          ├── calls /council
   │          │                         │          └── calls /retro
   │          │                         │
   │          │              ┌──────────┴──────────┐
   │          │              │        /crank       │
   │          │              │  (wave executor)    │
   │          │              └──────────┬──────────┘
   │          │                         │ spawns N parallel
   │          │                         ▼
   │          │                   /implement
   │          │                   /implement    ← one per issue
   │          │                   /implement
   │          │                         │
   │          │                         ▼
   │          │                      /vibe  ←── calls /council
   │          │                             ←── calls /complexity
   │          │                             ←── calls /bug-hunt
   │          │
   └──────────┴──────────────────────────────────────────────────────
```

---

## Judgment Layer — Everything Flows Through Council

`/council` is the core validation primitive. Three skills wrap it:

```
                   ┌──────────────────────────────┐
                   │           /council           │
                   │  (independent judges debate, │
                   │   verdict gates delivery)    │
                   └───────────┬──────────────────┘
                               │ used by
          ┌────────────────────┼────────────────────┐
          ▼                    ▼                    ▼
   /pre-mortem              /vibe              /post-mortem
   (validate plans          (validate code     (wrap-up +
    before building)         before shipping)   learnings)
```

---

## Knowledge Layer — Skills Calling the CLI

Skills hand off to `ao` to persist knowledge across sessions:

```
   SKILL                   ao CLI COMMAND              RESULT
   ─────                   ──────────────              ──────
/research          →    ao lookup                  Prior knowledge loaded into session
/retro             →    ao forge transcript        Learnings extracted from session
/retro             →    ao pool promote            Validated learnings promoted
/evolve            →    ao goals measure           Fitness checked before next cycle
/rpi               →    ao ratchet record          Progress gate checkpointed
/implement         →    ao ratchet check           Gate verified before work starts
/post-mortem       →    finding-compiler.sh        Findings become artifacts, checks, and constraints
/post-mortem       →    ao flywheel close-loop     Citation feedback and lifecycle updates applied
```

---

## Prevention Ratchet

The closed-loop prevention path is file-native:

```
/post-mortem or /pre-mortem
        │
        ▼
.agents/findings/registry.jsonl
        │
        ▼
hooks/finding-compiler.sh
        │
        ├──> .agents/findings/<id>.md
        ├──> .agents/planning-rules/<id>.md
        ├──> .agents/pre-mortem-checks/<id>.md
        └──> .agents/constraints/index.json   (mechanical + active only)
                                              │
                                              ▼
                                   hooks/task-validation-gate.sh
```

`/plan`, `/pre-mortem`, `/vibe`, and `/post-mortem` load compiled planning and review artifacts first, then fall back to the registry when compiled outputs are missing. `task-validation-gate.sh` is the shift-left enforcement surface for active mechanical findings.

---

## CLI Command Families (~85 top-level commands)

The `ao` binary is a Go (1.26) cobra app. Entry point: `cli/cmd/ao/root.go`. Each of the ~85 top-level commands is registered on `rootCmd` from its own `cli/cmd/ao/<command>.go` file. Grouped by what they touch:

```
KNOWLEDGE / FLYWHEEL        VALIDATION / GATES         RPI / EXECUTION
────────────────────        ──────────────────         ───────────────
ao lookup                   ao gate                    ao rpi (phased)
ao search                   ao ratchet                 ao crank
ao inject                   ao constraint              ao discovery
ao forge                    ao vibe                    ao implement
ao pool                     ao validation              ao plan
ao curate                   ao quality                 ao pre-mortem
ao dedup                                                ao autodev
ao contradict               GOALS / FITNESS            ao evolve
ao maturity                 ──────────────             ao swarm
ao harvest                  ao goals
ao flywheel                 ao metrics                 DAEMON / SCHEDULING
ao knowledge                ao badge                   ───────────────────
ao retrieval-bench          ao status                  ao daemon
ao anti-patterns                                        ao overnight
ao provenance               SESSION / MEMORY           ao schedule
                            ────────────────
TEAM (WARMIND)              ao session                 HARNESS / RUNTIME
─────────────               ao context                 ─────────────────
ao warmind                  ao recover                 ao codex
                            ao handoff                 ao gc (Gas City)
UTILITIES                   ao trace                   ao hooks
─────────                   ao notebook                ao config
ao doctor                   ao memory                  ao compile
ao version                  ao mind                    ao quick-start
```

Source of truth for the full surface (every command, flag, and subcommand) is the generated `cli/docs/COMMANDS.md` — do not hand-maintain command lists here.

---

## Skill → ao Integration Surface (ranked by # of skills that invoke each)

~42 of the 77 `skills/**/SKILL.md` shell out to `ao <cmd>`; the rest are pure-prose orchestrators or wrap other CLIs (`bd`, `git`, `gh`, `gemini`, `ntm`). The CLI commands skills lean on most heavily:

```
COMMAND          SKILLS   ROLE
───────          ──────   ────
ao goals           50     fitness / strategic-intent surface (GOALS.md)
ao overnight       29     out-of-session compounding (Dream)
ao lookup          26     prior-knowledge retrieval
ao codex           26     Codex harness dispatch
ao ratchet         24     progress gate / monotonic checkpoint
ao metrics         23     health + flywheel signals
ao knowledge       22     corpus query / activation
ao autodev         11     bounded autonomous dev loops
ao inject          10     context injection + CitationEvent emit
ao search           8     corpus search
ao forge            8     transcript → learnings
ao flywheel         8     close-loop / compounding equation
ao rpi              7     phased orchestrator
ao quick-start      7     bootstrap
ao evolve           7     fitness-gated improvement loop
ao maturity         6     decay / eviction
ao beads            5     issue-tracker bridge
ao pool             4     team staging
ao harvest          4     promote .agents knowledge
ao context          4     session context
ao compile          4     derived-wiki rebuild
ao badge            4     status badges
```

Two cross-cutting clusters:
- **RPI cluster** (`/crank`, `/discovery`, `/implement`, `/plan`, `/pre-mortem`, `/vibe`, `/validation`) all share `lookup` + `metrics` + `ratchet`.
- **Flywheel** is the single broadest consumer — it calls `anti-patterns`, `badge`, `constraint`, `contradict`, `curate`, `dedup`, `flywheel`, `harvest`, `lookup`, `maturity`, `metrics`, `retrieval-bench`, and `status`.

---

## Internal Architecture — Hexagonal (ports & adapters)

`ao` is wired as ports & adapters with **explicit dependency-passing** (no DI container). 14 port interfaces in `cli/internal/ports/` define the seams; concrete `*_adapter.go` files in `cli/cmd/ao/` bind them to the real filesystem/runtime.

```
PORTS (cli/internal/ports/)          ADAPTERS (cli/cmd/ao/*_adapter.go)
──────────────────────────           ──────────────────────────────────
CorpusReaderPort  ─────────┐         corpus_reader_adapter.go
CorpusWriterPort           │         citation_port_adapter.go
CitationPort               ├──bind──▶ gate_runner_adapter.go
ClaimEvidencePort          │         harness_adapter.go
ClaimEvidenceBinderPort    │         finding_compiler_adapter.go
CIStatusPort               │         ... (one adapter per seam)
EventBusPort               │
FactoryAdmissionPort       │         Wiring is hand-passed at command
FindingCompilerPort        │         construction time — read root.go and
GateRunnerPort             │         each cmd/ao/<command>.go to see which
HarnessPort                │         ports a command receives.
LoopReaderPort             │
LoopWriterPort             │
OperatorPort  ─────────────┘
```

Biggest internal packages (file count, largest first): `daemon` (69), `overnight` (48), `ports` (43), `rpi` (41), `goals` (27), `lifecycle` (22), `search` (21), `eval` (21), `vibecheck` (19), `ratchet` (19), `llm` (18), `evalsubstrate` (18), `agentworker` (15), `warmind` (11), `quality` (11), `gascity` (11), `context` (11). Public package: `pkg/vault`.

Core data types all live in `cli/internal/types/types.go`:

```
TYPE              WHAT IT CARRIES
────              ───────────────
Candidate         knowledge item pre-promotion: tier / utility / maturity / supersession
PoolEntry         Candidate + scoring + review + status (team staging record)
CitationEvent     drives flywheel σ (citation rate) and ρ (reuse)
FlywheelMetrics   compounding equation + escape-velocity verdict
GoldenSignals     compounding / accumulating / decaying classification
RubricScores · Scoring · Source · TranscriptMessage
```

The compounding equation behind `FlywheelMetrics`:

```
dK/dt = I − ΔK + σ·ρ·K − B
        │     │      │      └─ B   churn / eviction
        │     │      └─ σ·ρ·K  compounding term (citations × reuse × corpus)
        │     └─ ΔK   decay
        └─ I          new ingest
```

Escape velocity = the compounding term exceeds decay + churn.

---

## Config & Persistence — Files, No Database

`ao` keeps **no database of its own** — persistence is plain files. (Contrast: the separate `bd`/beads CLI uses Dolt.)

```
CONFIG PRECEDENCE (cli/internal/config/)
  CLI flags  >  AGENTOPS_* env  >  project .agentops/config.yaml
             >  ~/.agentops/config.yaml  >  defaults

ON-DISK STATE
  .agents/learnings/         local knowledge corpus (CorpusReaderPort source)
  .agents/findings/          findings + compiled artifacts
  .agents/plans/             plan packets
  .agents/citations.jsonl    CitationEvent log (flywheel feedback)
  .agents/daemon/ledger.jsonl   append-only daemon event store
  .warmind/pool/ → .warmind/learnings/   team staging → team canon
  GOALS.md                   strategic-intent surface (ao goals / ao evolve)
```

Source-of-truth precedence (from CLAUDE.md): **executable** (`cli/**`, `hooks/**`, generated `COMMANDS.md`) > **contracts** (`SKILL.md`, schemas) > **narrative docs**.

---

## Daemon — Persistent Job Queue

`cli/internal/daemon/` (69 files) runs a persistent job queue backed by an append-only, replay-on-startup, size-capped ledger at `.agents/daemon/ledger.jsonl`.

```
JOB LIFECYCLE
  SubmitJob ──▶ ClaimJob(lease) ──▶ Heartbeat ──▶ CompleteJob / FailJob

JOB SPECS                         CLI SURFACE
  RPIRunJobSpec                     ao daemon submit
  RPIPhaseJobSpec                   ao daemon jobs / list / show
  DreamJobSpec                      ao daemon wait / tail / events
  ScheduleConfig                    ao daemon cancel
```

---

## CLI Command Groups (legacy quick view)

```
KNOWLEDGE FLYWHEEL          VALIDATION GATES         SESSION / LIFECYCLE
──────────────────          ────────────────         ───────────────────
ao forge                    ao gate pending          ao session close
ao pool ingest              ao gate approve          ao rpi status
ao pool promote             ao gate reject           ao rpi cancel
ao lookup                   ao ratchet status        ao hooks list
ao lookup                   ao ratchet record        ao config
ao search                   ao ratchet check
ao dedup                    ao ratchet promote       METRICS / HEALTH
ao curate                                            ────────────────
                            GOALS / FITNESS          ao metrics health
MEMORY TOOLS                ───────────────          ao metrics flywheel
────────────                ao goals measure         ao metrics report
ao mind                     ao goals steer           ao flywheel status
ao notebook                 ao goals add             ao maturity
ao memory                   ao goals prune           ao doctor
ao trace                    ao goals history
ao extract                  ao goals drift           UTILITIES
                                                     ─────────
                                                     ao search
                                                     ao constraint
                                                     ao badge
                                                     ao version
```

---

## Three Flagship Data Flows

### 1 — Knowledge Flywheel

```
query
  │
  ▼
CorpusReaderPort.Lookup()  reads .agents/learnings/  ──▶ ranked items
  │
  ▼
ao inject  ──▶ appends CitationEvent to .agents/citations.jsonl

PROMOTION PATH
  .agents/learnings/  ──▶  .warmind/pool/staged/  ──▶  .warmind/learnings/
       (local)               (team staging)             (team canon)

TIERS
  Gold   ≥0.8   auto-promote after 24h
  Silver ≥0.5   needs 1 citation from another engineer
  Bronze        needs 3 citations

ao flywheel close-loop = promote + decay + contradiction-detect + emit FlywheelMetrics
```

### 2 — RPI Phased (`ao rpi phased`)

```
Discovery ──▶ Implementation ──▶ Validation     (fresh context per phase)
    │              │                  │
    └── each phase writes .agents/rpi/phase-N-summary.md

MULTI-CYCLE LOOPS         internal/daemon (rpi_executor.go)
                          gate / landing / kill-switch policy

LIVE SESSIONS            route through the Gas City bridge:
                          cli/cmd/ao/gc_bridge.go
                          cli/cmd/ao/gc_events.go
                          cli/cmd/ao/rpi_phased_gc.go
```

> Current path only. Deprecated, do not document as live: `rpi_loop_supervisor.go`, `rpi_workers.go`, `rpi_parallel.go`, `rpi_c2_events.go`, `rpi_phased_tmux.go`.

### 3 — Overnight / Dream (`ao overnight start` / `report`)

```
Checkpoint ──▶ Measure ──▶ Reduce ──▶ Commit ──▶ [Long-haul] ──▶ Report
              (fitness     (mine                 (optional)
               vs          findings)
               GOALS.md)

Runs as a daemon DreamJobSpec.
Outputs: .agents/overnight/report.json + FlywheelMetrics
```

---

## Hooks — Automatic Enforcement (13 hook entries across 7 trigger points)

Hooks fire without human involvement. The AI cannot bypass them.

```
TRIGGER                   HOOK                        WHAT IT DOES
───────                   ────                        ────────────
Session starts         session-start.sh            Stage runtime state and briefing pointers
Session ends           session-end-maintenance.sh  Harvest learnings, run maintenance
Agent stops            ao-flywheel-close.sh        Close the learning loop
Task completes         task-validation-gate.sh     Execute active compiled constraints and metadata checks
Every tool call        go-complexity-precommit.sh  Block functions over complexity budget
Pre-commit             skill-lint-gate.sh          Reject malformed skills
Pre-commit             dangerous-git-guard.sh      Block force-pushes to main
Pre-commit             pre-mortem-gate.sh          Require pre-mortem for large changes
Worker stop            subagent-stop.sh            Clean up parallel agent state
Worktree created       worktree-setup.sh           Initialize isolated workspace
Worktree merged        worktree-cleanup.sh         Remove stale branches
```

---

## Skill Tiers at a Glance

```
JUDGMENT             EXECUTION              KNOWLEDGE           INTERNAL
────────             ─────────              ─────────           ────────
council              research               retro               inject
vibe                 plan                   forge               extract
pre-mortem           implement              flywheel            ratchet
post-mortem          crank                  goals               standards
                     swarm                                      beads
                     rpi                                        shared
                     evolve
                     release
                     doc
                     status
                     handoff
                     quickstart
                     brainstorm
                     bug-hunt
                     complexity
                     + 14 more
```

---

*52 skills · 121 CLI commands · 13 hook entries · 0 telemetry · everything in plain files*
