# Primitive Chains

> AgentOps is not a flat skill catalog. It is a set of compiled primitive chains that turn disposable agent sessions into a ratcheting development system.

## Why This Doc Exists

The product story has drifted between "Research-Plan-Implement," "five commands," "shift-left validation," and "knowledge flywheel." All of those describe part of the system, but the executable shape is now clearer:

- `12-Factor AgentOps` provides the operating conditions
- the `Stigmergic Spiral` describes the macro lifecycle for stateless builders
- the `Brownian Ratchet` explains how noisy parallel work still becomes forward progress
- the skills, hooks, `ao` commands, and `.agents/` artifacts are the concrete primitives that implement that model

This document maps the live primitives to the chains they form.

## Primitive Matrix

| Layer | Primitive | Primary Surfaces | What It Contributes |
|------|-----------|------------------|---------------------|
| Mission / fitness | explicit goals | `GOALS.md`, `GOALS.yaml`, `ao goals`, `/evolve` | Mission-type orders, measurable fit, next-gap selection |
| Discovery | understand before acting | `/brainstorm`, `/research`, `ao search`, `ao lookup`, `.agents/research/` | Observe, orient, and scope using repo-native context |
| Risk / prevention | confront what can fail | `/plan`, `/pre-mortem`, findings registry, planning rules, pre-mortem checks | Risk-first scoping and reusable failure prevention |
| Execution | fresh-context implementation | `/crank`, `/swarm`, `/implement`, worktrees, beads waves | Parallel OODA loops with bounded scope and retries |
| Validation | judgment before closure | `/vibe`, `/validation`, `/council`, task-validation gate | Detect defects and regressions before accepting completion |
| Learning | extract and reinforce | `/post-mortem`, `/retro`, `/forge`, `ao flywheel`, `ao maturity` | Convert completed work into reusable knowledge |
| Ratchet / provenance | lock progress | `ao ratchet`, commits, `.agents/ao/chain.jsonl` | Ensure accepted work becomes the new baseline |
| Continuity | survive context loss | `/handoff`, `/recover`, phased manifests, session hooks, `.agents/rpi/` | Disk-backed continuity when sessions compact or die |

## Chain 1: Macro Lifecycle

This is the executable replacement for the older "five commands" story.

```text
Discovery -> Implementation -> Validation
    |             |                 |
    v             v                 v
scope/risk    validated build   learn + next work
```

| Phase | Primary Skills | Durable Outputs |
|------|----------------|-----------------|
| Discovery | `/brainstorm` -> `/research` -> `/plan` -> `/pre-mortem` | research artifacts, beads graph, execution packet, known risks |
| Implementation | `/crank` -> `/swarm` -> `/implement` | closed issues, code, tests, ratchet checkpoints |
| Validation | `/validation` -> `/vibe` -> `/post-mortem` -> `/retro` -> `/forge` | findings, learnings, promoted constraints, next-work queue |

`/rpi` is the orchestrator that routes across those phases. The historical acronym remains, but the current runtime shape is phased.

## Chain 2: Discovery

```text
Mission -> brainstorm -> search / lookup -> research -> plan -> pre-mortem
```

What happens:

1. `GOALS.md` or the user goal defines the mission, not the exact procedure.
2. `/brainstorm` separates the problem from the implementation habit.
3. `ao search` and `ao lookup` pull prior repo knowledge and nearby precedents.
4. `/research` synthesizes the current state into `.agents/research/*.md`.
5. `/plan` decomposes the work, loading prior findings and planning rules before it does.
6. `/pre-mortem` pressure-tests the plan and can promote reusable findings back into prevention surfaces.

This chain is the "scope + risk" half of the Stigmergic Spiral.

## Chain 3: Implementation

```text
execution packet -> crank -> wave selection -> swarm -> implement -> verify -> ratchet
```

What happens:

1. `/crank` reads the epic or execution packet.
2. Ready work is selected in waves from beads or TaskList.
3. `/swarm` spawns fresh workers using the runtime-native backend.
4. `/implement` executes one issue with bounded context and local validation.
5. The lead verifies the wave, records ratchet state, and retries `PARTIAL` or `BLOCKED` work.
6. The loop stops only at `<promise>DONE</promise>`, `<promise>BLOCKED</promise>`, or hard limits.

This chain is the micro-tempo engine. The worker set is disposable. The accepted output is not.

## Chain 4: Validation and Learning

```text
vibe -> post-mortem -> retro -> forge -> flywheel close-loop
```

What happens:

1. `/vibe` validates the produced system against code quality, architecture, security, and intent.
2. `/post-mortem` captures what changed, what failed, and what should become reusable.
3. `/retro` provides quick-capture learning when full wrap-up is unnecessary.
4. `/forge` turns transcripts or markdown artifacts into structured knowledge.
5. `ao flywheel close-loop` records session closure so the next run starts with better retrieval.

This is where the repo becomes more capable than it was before the session started.

## Chain 5: Compiled Prevention

```text
finding -> registry -> compiler outputs -> planning / validation / task-complete gates
```

| Step | Surface | Effect |
|------|---------|--------|
| Capture | `.agents/findings/registry.jsonl` | Normalized reusable findings ledger |
| Compile | `.agents/pre-mortem-checks/`, `.agents/planning-rules/`, `.agents/constraints/index.json` | Findings become advisory or enforceable artifacts |
| Reuse | `/plan`, `/pre-mortem`, `/vibe`, `task-validation-gate.sh` | Future work starts with prior failures already loaded |

This is the contract ratchet: surprises are expected once, then compiled into the environment.

## Chain 6: Continuity and Recovery

```text
session start -> phased handoff -> handoff / recover -> session end -> next session
```

| Continuity Surface | Role |
|--------------------|------|
| `hooks/session-start.sh` | Injects lightweight repo context and points at durable artifacts |
| `ao rpi phased` + phase manifests | Keeps each phase context-bounded and disk-backed |
| `/handoff` | Leaves a structured continuation packet for the next operator |
| `/recover` | Rehydrates in-progress work after compaction or interruption |
| `hooks/session-end-maintenance.sh` | Extracts and curates end-of-session knowledge |
| `hooks/ao-flywheel-close.sh` | Closes the loop at stop time |

This is the stigmergic memory layer. No agent has to remember yesterday if the environment was updated correctly.

## How the Chains Bind to `ao` Internals

The chains above are skill-level stories. Underneath, `ao` (the Go binary at `cli/cmd/ao/`) implements them through a hexagonal architecture, not ad-hoc command code.

- **Ports, not direct calls.** `cli/internal/ports/` declares ~14 port interfaces (`CorpusReaderPort`, `CorpusWriterPort`, `CitationPort`, `ClaimEvidencePort`, `EventBusPort`, `GateRunnerPort`, `FindingCompilerPort`, `FactoryAdmissionPort`, `LoopReaderPort`, `LoopWriterPort`, `HarnessPort`, `OperatorPort`, `CIStatusPort`, `ClaimEvidenceBinderPort`) across five bounded contexts (Corpus, Validation, Loop, Factory, Runtime). Concrete adapters live as `cli/cmd/ao/*_adapter.go` (~14 production adapters, each paired with an in-memory adapter for tests). Wiring is explicit dependency-passing — there is no DI container.
- **Discovery / Learning** ride the Corpus context: `ao lookup` and `ao inject` read prior knowledge through `CorpusReaderPort` and append a `CitationEvent` to `.agents/ao/citations.jsonl`, which feeds decay-ranked retrieval on the next query.
- **Validation** rides the Validation context (`GateRunnerPort`, `FindingCompilerPort`): `/vibe` and the compiled-prevention gates run through these ports rather than calling tools directly.
- **Continuity / multi-cycle loops** ride the Loop and Runtime contexts. `ao rpi phased <goal>` runs Discovery → Implementation → Validation with fresh context per phase, each writing `.agents/rpi/phase-N-summary.md` as the bounded carry-forward for the next phase. Multi-cycle supervision lives in `cli/internal/daemon/rpi_executor.go` (gate / landing / kill-switch); live sessions bridge through Gas City (`cli/cmd/ao/gc_bridge.go`, `rpi_phased_gc.go`). Older `rpi_loop_supervisor.go`, `rpi_parallel.go`, `rpi_c2_events.go`, and `rpi_phased_tmux.go` still exist on disk but are deprecated — do not treat them as the current path.

### Persistence: files, not a database

`ao` keeps no database of its own. Every durable surface in the chains above is a plain file:

| Surface | Path | Written by |
|---------|------|------------|
| Citations / retrieval feedback | `.agents/ao/citations.jsonl` | `ao inject`, `ao lookup` |
| Ratchet / provenance chain | `.agents/ao/chain.jsonl` | `ao ratchet`, commits |
| Daemon event store | `.agents/daemon/ledger.jsonl` | the daemon (append-only JSONL, replayed on startup, corrupt-line quarantine, gzip rotation) |
| Findings registry | `.agents/findings/registry.jsonl` | `/post-mortem`, `/pre-mortem` |
| Phase carry-forward | `.agents/rpi/phase-N-summary.md` | `ao rpi phased` |
| Team pool → canon | `.warmind/pool/staged/` → `.warmind/learnings/` | `ao warmind` / `ao flywheel close-loop` |
| Strategic intent | `GOALS.md` | `/goals`, `/evolve` |

The daemon (`cli/internal/daemon/`, the largest internal package) drives out-of-session chains through `JobSpec`s — `RPIRunJobSpec` / `RPIPhaseJobSpec` (`rpi_jobs.go`) and `DreamRunJobSpec` / `DreamStageJobSpec` (`dream_jobs.go`) — with a job lifecycle of `SubmitJob → ClaimJob (lease) → Heartbeat → CompleteJob / FailJob`. Recurring jobs are scheduled by the `RecurrenceSupervisor` (`recurrence.go`). The compounding equation behind flywheel metrics (`dK/dt = I − ΔK + σρK − B`) is documented canonically in [The Science](../the-science.md); cite it there rather than restating it.

Config precedence for all of the above: CLI flags > `AGENTOPS_*` env > project `.agentops/config.yaml` > `~/.agentops/config.yaml` > defaults. The separate `bd` (beads) CLI uses Dolt; `ao` itself does not.

## Terminology Drift Ledger

| Term | Historical Meaning | Current Executable Meaning |
|------|--------------------|----------------------------|
| `RPI` | Research -> Plan -> Implement | Historical product name for the full lifecycle; the runtime now executes `Discovery -> Implementation -> Validation` |
| `five commands` | `/research`, `/plan`, `/pre-mortem`, `/crank`, `/post-mortem` | Useful legacy teaching aid, but incomplete because it omits `/brainstorm`, `/validation`, `/vibe`, `/retro`, `/forge`, and continuity surfaces |
| `knowledge injection` | startup context loading | Now broader: `lookup`, `search`, notebooks, handoffs, findings, and phase manifests assemble context together |
| `three hooks` | session start/end/stop | The runtime currently declares 7 hook event sections, with three lifecycle anchors plus prompt/tool/task guardrails |
| `Research-Plan-Implement` | product slogan | Still appears in names and legacy docs, but phased execution and validation are first-class now |
| `orchestrators never fork` | architectural rule of thumb | Desired direction, but some live skill contracts still declare `context.window: fork`; trust `SKILL.md` until the contracts are fully harmonized |
| `ao database` / `the store` | implied a backing DB inside `ao` | `ao` persists to plain files only (`.agents/**`, `.warmind/**`, `GOALS.md`); the only Dolt-backed tracker is the separate `bd` CLI |
| `commands wrap tools` | one `ao <cmd>` per skill action | Skills carry judgment; only ~42 of 77 skills shell out to `ao` at all, and `ao` itself routes through ~14 ports + adapters rather than direct calls |

## Audit Snapshot

The current repo state behind this document:

- `77` source skills in `skills/` (`68` user-facing and `9` internal)
- roughly `85` top-level `ao` commands (one `cmd/ao/<command>.go` each, entry `cmd/ao/root.go`); only ~42 of the 77 skills shell out to `ao`, the rest are pure-prose orchestrators or wrap other CLIs (`bd`, `git`, `gh`, `gemini`, `ntm`)
- `7` runtime hook event sections recorded in the current CLI/skills map
- prevention is partly `ao`-mediated and partly file-native through the finding registry and compiled constraints

The split is load-bearing: a `SKILL.md` carries judgment and orchestration; `ao` carries durable state. The single broadest `ao` consumer is the `flywheel` skill; the discovery/implementation/validation (RPI) cluster shares `lookup` + `metrics` + `ratchet`.

For the current command-to-skill matrix, see [CLI ↔ Skills/Hooks Map](../cli-skills-map.md).

## See Also

- [How It Works](../how-it-works.md)
- [Knowledge Flywheel](../knowledge-flywheel.md)
- [The Science](../the-science.md) — canonical home of the `dK/dt` compounding equation
- [Context Lifecycle Contract](../context-lifecycle.md)
- [Brownian Ratchet](../brownian-ratchet.md)
- [CLI ↔ Skills/Hooks Map](../cli-skills-map.md)
