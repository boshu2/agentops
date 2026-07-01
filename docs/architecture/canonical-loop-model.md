# The Canonical Loop Model

> Architecture doc. The single statement of how AgentOps' loops relate. Companion to [3.0.md](../3.0.md) (the north star), [Component Map](component-map.md) (product/component routing), [operating-loop.md](operating-loop.md) (the seven moves inside one tick), and [ports-and-adapters.md](ports-and-adapters.md) (the runtime seams). Derivation: [`.agents/discovery/2026-05-24-canonical-loops-ddd.md`](https://github.com/boshu2/agentops/blob/main/.agents/discovery/2026-05-24-canonical-loops-ddd.md).

AgentOps once had roughly twenty surfaces that read like a loop (evolve, rpi, autodev, factory, daemon, crank, dream, swarm, ship-loop, ratchet, flywheel, and more), with no named hierarchy and circular help. This doc collapses that sprawl into one statement.

## The model in one sentence

**One loop body, two drivers, one inner tick, one config.**

- **One loop body**: the same five-beat tick (research, plan, implement, validate, ratchet) at every scale.
- **Two drivers**: an **Evolve** driver (an in-session agent runs the loop, self-paced) and a **Factory** driver (an out-of-session substrate runs the same loop unattended over a queue).
- **One inner tick**: **rpi**, one research-plan-implement-validate cycle over one bead.
- **One config**: **Autodev**, the durable intent layer the loop reads every tick. Autodev is not a loop.

Everything else is a step of the loop, a config source, a runtime adapter, an execution profile, or a read projection. None of them is a peer loop.

AgentOps ships the **Evolve** driver as its product: it runs in a plain session with zero AgentOps-managed daemon. The **Factory** driver is the *substrate's* job — AgentOps deleted its own daemon and delegates out-of-session execution to an adopted substrate (reference: NTM + MCP + managed-agents). See [3.0.md](../3.0.md) and [ADR-0009](../adr/ADR-0009-daemon-deletion-in-session-only.md).

## The picture

```
AUTODEV-CONFIG  (PROGRAM.md / AUTODEV.md + GOALS.md + ADRs)   ← NOT a loop; the intent/spec the loop reads
        │ drives
        ▼
┌─────────────────────────────────────────────────────────────┐
│ ONE LOOP BODY  (the rpi tick: Research → Plan → Implement → Validate → Ratchet)
│                                                               │
│   EVOLVE driver (in session)             FACTORY driver (out of session)
│     an agent runs the loop                 a SUBSTRATE runs the SAME loop
│     self-paced, self-tunable,              unattended over the bead queue;
│     ends with the session                  operator-only stop
│     SHIPPED by AgentOps                     SUBSTRATE-owned (NTM / MCP / managed-agents)
│        │                                          │
│        └──────────── both run ────────────────────┘
│                          │
│                          ▼
│         ┌──────────────────────────────────────────────┐
│         │ RPI TICK  =  inner loop (one cycle)            │
│         │   discovery → plan → crank(wave) → validate    │
│         │   one bead, one behavior, one acceptance proof │
│         └──────────────────────────────────────────────┘
└─────────────────────────────────────────────────────────────┘
```

## The fractal framing

The loop is fractal: the same shape at every layer, run by a human or by a stand-in agent. The only things that change across layers are the **driver** and the **stop policy**.

- **rpi** is one tick.
- **evolve** is N rpi ticks toward a goal: select next-best work, run a tick, post-mortem, repeat.
- **crank / swarm** fan one wave of an rpi tick across an in-session team of agents in isolated worktrees — still in session, still the same five beats per worker.
- A **Factory** is the same loop run unattended over a whole queue by an out-of-session substrate.

Factory and Evolve are not two different loops. They are the **same loop body under two drivers**. Mount Olympus, the full-custom Rust reference implementation, demonstrates this: it folds its in-session evolve cycle into its daemon serve-loop so both paths run the identical tick. The difference is the driver (unattended substrate versus interactive session) and the stop policy (operator-marker-only for the substrate; session-budget allowed for interactive). Olympus keeps a daemon because it is a sovereign product with its own core; AgentOps deletes its daemon and runs the in-session driver only, opting into a substrate for the Factory driver.

Because the shape repeats, the ratchet rules (no self-grade, fresh agent on failure, knowledge becomes constraints) apply identically at every layer. That is what makes the loop compound up the layers instead of repeating flat. The lineage is documented in [`.agents/research/2026-05-24-fractal-orchestration-lineage.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-05-24-fractal-orchestration-lineage.md): the same self-spawning, self-similar loop runs from the Kubernetes-era control plane (2025) through Olympus (2026) to AgentOps 3.0, lifted off any single substrate and given a name.

## Context is the artifact handed off at every edge

A loop tick takes context in and emits context out. The exhaust of one tick is the seed of the next. That handoff is the engineering artifact, and it accumulates in the `.agents/` corpus.

- **In:** `ao inject` compiles a decay-ranked, token-budgeted slice of the corpus for the bead at hand. This is the context compiler doing its job at the start of a tick.
- **Out:** evidence, decisions, citations, and verdicts land in `.agents/` and on the bead, under the promotion ratchet.
- **`ao compile`** rebuilds the corpus periodically so the next inject is fresh.

Context never flows through loop plumbing return values; it flows through the corpus and the bead. That is why the corpus is treated as the central artifact rather than a byproduct. Whether it compounds into a *moat* is an [explicitly unproven hypothesis](../adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md) — the architecture bets on it, but the proven asset is the verification loop, not the moat.

## The DDD seam: what owns the loop versus what orchestrates it

AgentOps owns the in-session loop and the context. An orchestration substrate (reference: NTM + MCP + managed-agents) owns out-of-session execution. The boundary is sharp:

| Domain | Owner | Primitives |
|---|---|---|
| **Orchestration**: when / where / who-supervises / coordination | **Substrate (NTM / MCP / managed-agents)** | a tmux agent swarm (NTM), the MCP tool surface (`ao mcp serve`), managed/agent-SDK drivers (`ao agent`), cron/event triggers (including evolve cadence), the bead queue, human on the loop and merge, runtime providers |
| **The in-session loop + the context**: what the agent does, how context compounds | **AgentOps** | the `/rpi` loop (run as a skill), `/evolve` (work-selection + N rpi), crank/swarm, the ratchet rules, skills, `ao inject` / `compile` / `maturity`, the `.agents/` corpus |

The governing test: is it about *when, where, who supervises, or coordination*? That is the substrate. Is it about *what the loop does or how context compounds*? That is AgentOps.

By that test: rpi's internal steps, the ratchet, `inject`, and `compile` belong to AgentOps; the agent calls them. Evolve's *cadence*, when run unattended, is a substrate cron/trigger; evolve's *logic* (which bead next, N cycles toward a goal) stays in AgentOps. The queue, the agents, and their supervision belong to the substrate.

**The Factory driver is the substrate's job, not an AgentOps-shipped daemon.** AgentOps 3.0 ships no always-on daemon, scheduler, or overnight runner — they were deleted in the rearchitecture. When you want the loop to run unattended over a queue, a substrate drives it. On the reference substrate that dispatch is **swarm-driven**: an NTM tmux swarm (or a lead agent) runs `BEADS_DIR="$(ao beads dir)" br ready` then dispatches the next bead to a worker agent that runs the `/rpi` skill; a managed-agent driver (`ao agent`) or cron handles scheduled cadence, and `ao mcp serve` exposes the tool surface across the seam. AgentOps stays zero-dependency in a plain session through the Evolve driver.

**rpi is never re-expressed in the substrate.** Decomposing the rpi tick into substrate-side workflow steps would duplicate the loop shape (re-introducing the surface-sprawl disease across the seam) and pit the substrate's retry machinery against the ratchet (substrate `max_attempts` retry versus fresh-agent-on-failure; substrate per-step agent assignment versus no-self-grade). The substrate dispatches a whole loop as one unit — an agent running the `/rpi` skill; it never drives the loop's insides.

## Where the family members land

Everything that used to look like a competing loop has a home under this model:

| Surface | What it actually is | Home |
|---|---|---|
| **rpi** | The inner tick | The one inner tick |
| **evolve** | The in-session driver (N rpi cycles) | The Evolve driver |
| **autodev** | The config/intent layer (PROGRAM.md / AUTODEV.md + GOALS.md + ADRs) | One config, NOT a loop |
| **crank** | rpi's wave-executor step (in session) | A loop step, under rpi |
| **swarm** | crank's parallel-dispatch adapter (in session) | A loop step, under crank |
| **ratchet** | The loop's knowledge-capture step | A loop step (move 7) |
| **dream** | The loop run on a knowledge-compounding goal on a schedule | A substrate job profile, not a loop |
| **factory** | The out-of-session driver (substrate-owned) | A driver, not an AgentOps-shipped surface |
| **ship-loop** | A fast-lane evolve/rpi preset | An execution profile, not a loop |
| **flywheel** | A health check on the corpus | Background check, not a loop |
| **goals / council** | Fitness/intent config; a validation step | Config and step, not loops |

## Where this doc lives and what it replaces

This doc lives at `docs/architecture/canonical-loop-model.md` and is linked from the 3.0 north star and the documentation index. It is the canonical statement of the loop hierarchy.

It supersedes the help-string decision-tree approach (the prior attempt to disambiguate evolve, rpi, autodev, and factory as four near-synonyms). Instead of telling agents how to pick between four peers, this model states that they are not peers: they are drivers, an inner tick, and a config of one loop. The vocabulary entries that back this model live in [`skills/domain/references/`](../../skills/domain/references/) (Loop, Factory, Evolve, RPI, Autodev-as-config, Context-Compiler).

## See also

- [the-agent-factory.md](the-agent-factory.md): the control-plane **primitives / citizens** view (roles × primitives × the adapter taxonomy) — the structural counterpart to this loop-hierarchy statement; the unifying entry that cross-links the loop/primitive docs
- [control-loop-model.md](control-loop-model.md): why the loop converges (fast) and self-improves (slow + SPC governor) — the *behavior* of the citizens
- [3.0.md](../3.0.md): the north star this model serves
- [component-map.md](component-map.md): the component routing and trim/defer posture that keeps loop work from sprawling
- [ADR-0009](../adr/ADR-0009-daemon-deletion-in-session-only.md): why AgentOps ships only the in-session driver
- [operating-loop.md](operating-loop.md): the seven moves inside one tick
- [ports-and-adapters.md](ports-and-adapters.md): the runtime seams the loop runs through
- [`skills/domain/references/`](../../skills/domain/references/): the loop-family vocabulary
- [`.agents/discovery/2026-05-24-canonical-loops-ddd.md`](https://github.com/boshu2/agentops/blob/main/.agents/discovery/2026-05-24-canonical-loops-ddd.md): the derivation
- [`.agents/research/2026-05-24-fractal-orchestration-lineage.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-05-24-fractal-orchestration-lineage.md): the lineage
