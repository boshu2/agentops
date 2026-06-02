# The Canonical Loop Model

> Architecture doc. The single statement of how AgentOps' sessions, legacy loops, and background-agent substrate relate. Companion to [3.0.md](../3.0.md) (the north star), [operating-loop.md](operating-loop.md) (the practice sequence inside a work session), and [ports-and-adapters.md](ports-and-adapters.md) (the runtime seams). Derivation: [`.agents/discovery/2026-05-24-canonical-loops-ddd.md`](https://github.com/boshu2/agentops/blob/main/.agents/discovery/2026-05-24-canonical-loops-ddd.md), updated by the NTM background-agent rescope.

AgentOps once had roughly twenty surfaces that read like a loop (evolve, rpi, autodev, factory, daemon, crank, dream, swarm, ship-loop, ratchet, flywheel, and more), with no named hierarchy and circular help. This doc now collapses that sprawl into one operational statement for the current product direction: **skills run the work; NTM keeps background agents ready.**

## The model in one sentence

**One skill-session contract, two placements, one background substrate.**

- **One skill-session contract**: a Claude/Codex session loads skills, pulls context, reserves scope, does work, validates, and records provenance.
- **Two placements**: interactive sessions started by the operator, or background sessions kept warm by NTM.
- **One background substrate**: NTM supervises Claude/Codex tmux sessions; mcp-agent-mail coordinates claims, reservations, and handoff; MCP exposes tools.
- **One durable intent layer**: beads, GOALS, ADRs, and docs provide the work and constraints the session reads.

Everything else is a skill, a config source, a runtime adapter, an execution profile, or a read projection. None of them is a peer daemon.

AgentOps ships the skills, `ao` support surface, validation gates, and provenance/corpus machinery. It does **not** ship a daemon. The Factory driver is the NTM substrate's job: keep background Claude/Codex sessions ready, coordinate through mcp-agent-mail, and let those sessions use the same skills an interactive agent would use. See [3.0.md](../3.0.md) and [ADR-0009](../adr/ADR-0009-daemon-deletion-in-session-only.md).

## The picture

```
INTENT-CONFIG  (beads + GOALS.md + ADRs + docs)   ← NOT a loop; the work/spec the session reads
        │ drives
        ▼
┌─────────────────────────────────────────────────────────────┐
│ ONE SKILL-SESSION CONTRACT                                  │
│                                                               │
│   INTERACTIVE placement                 BACKGROUND placement
│     operator starts a session              NTM keeps Claude/Codex sessions
│     agent loads skills                     ready; mcp-agent-mail assigns,
│     validates + records evidence           reserves, and coordinates
│        │                                          │
│        └──────────── both use ────────────────────┘
│                          │
│                          ▼
│         ┌──────────────────────────────────────────────┐
│         │ SKILLS + AO SUPPORT                            │
│         │   bootstrap → inject → skill-guided work       │
│         │   validate → provenance → handoff/PR           │
│         └──────────────────────────────────────────────┘
└─────────────────────────────────────────────────────────────┘
```

## The fractal framing

The session contract is fractal: the same skill-guided shape works whether a human starts the session or NTM keeps it warm. The only things that change are **placement** and **supervision**.

- **Skill sessions** are the active unit.
- **rpi** and **evolve** are legacy compatibility wrappers around earlier loop shapes; do not use them as the background-agent execution path.
- **crank / swarm** fan a scoped wave across in-session or NTM-supervised agents in isolated worktrees.
- A **Factory** is a set of background skill sessions supervised by NTM, not an AgentOps daemon.

Mount Olympus, the full-custom Rust reference implementation, still demonstrates the alternative: it folds a typed work loop into its daemon serve-loop. Olympus keeps a daemon because it is a sovereign product with its own core. AgentOps deletes its daemon and uses NTM for background placement while keeping the work contract in portable skills.

Because the shape repeats, the ratchet rules (no self-grade, fresh agent on failure, knowledge becomes constraints) apply identically at every layer. That is what makes the loop compound up the layers instead of repeating flat. The lineage is documented in [`.agents/research/2026-05-24-fractal-orchestration-lineage.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-05-24-fractal-orchestration-lineage.md): the same self-spawning, self-similar loop runs from the Kubernetes-era control plane (2025) through Olympus (2026) to AgentOps 3.0, lifted off any single substrate and given a name.

## Context is the artifact handed off at every edge

A loop tick takes context in and emits context out. The exhaust of one tick is the seed of the next. That handoff is the engineering artifact, and it accumulates in the `.agents/` corpus.

- **In:** `ao inject` compiles a decay-ranked, token-budgeted slice of the corpus for the bead at hand. This is the context compiler doing its job at the start of a tick.
- **Out:** evidence, decisions, citations, and verdicts land in `.agents/` and on the bead, under the promotion ratchet.
- **`ao compile`** rebuilds the corpus periodically so the next inject is fresh.

Context never flows through loop plumbing return values; it flows through the corpus and the bead. That is why the corpus is the moat: it is the compounding artifact, not a byproduct.

## The DDD seam: what owns the loop versus what orchestrates it

AgentOps owns skill sessions and the context they compound. The NTM background-agent substrate owns out-of-session supervision. The boundary is sharp:

| Domain | Owner | Primitives |
|---|---|---|
| **Orchestration**: when / where / who-supervises / coordination | **NTM + mcp-agent-mail substrate** | long-lived Claude/Codex tmux sessions, mailbox threads, file reservations/locks, worker check-ins, bead queue polling, human on the loop and merge |
| **Skill sessions + context**: what the agent does, how context compounds | **AgentOps** | skills, `ao session bootstrap`, `ao inject` / `compile` / `maturity`, validation helpers, provenance capture, the `.agents/` corpus |

The governing test: is it about *when, where, who supervises, or coordination*? That is the substrate. Is it about *what the loop does or how context compounds*? That is AgentOps.

By that test: skills, `inject`, validation, provenance, and corpus compilation belong to AgentOps; the agent session calls them. The queue, the long-lived agents, their worktrees, mailboxes, and supervision belong to the NTM substrate.

**The Factory driver is the substrate's job, not an AgentOps-shipped daemon.** AgentOps 3.0 ships no always-on daemon, scheduler, or overnight runner — they were deleted in the rearchitecture. When you want agents ready in the background, NTM supervises Claude and Codex tmux sessions. A lead agent or operator reads `bd ready`, assigns work through mcp-agent-mail, workers reserve files, load the appropriate skills, and execute as normal sessions. `ao mcp serve` exposes the tool surface across the seam when a session needs MCP.

**AgentOps practice is never re-expressed as substrate workflow steps.** NTM does not own "research/plan/implement/validate" logic, and it does not drive deprecated `ao rpi`/`ao evolve` wrappers. It owns session lifecycle and coordination. The worker session owns the skill-guided work and evidence.

## Where the family members land

Everything that used to look like a competing loop has a home under this model:

| Surface | What it actually is | Home |
|---|---|---|
| **rpi** | Legacy inner-loop wrapper | Compatibility surface; not the background-agent execution path |
| **evolve** | Legacy autonomous-loop wrapper | Compatibility surface; not the background-agent execution path |
| **autodev** | The config/intent layer (PROGRAM.md / AUTODEV.md + GOALS.md + ADRs) | Config, NOT a loop |
| **crank** | A wave-execution skill pattern | Skill-guided execution profile |
| **swarm** | Parallel-dispatch adapter for scoped waves | In-session or NTM-supervised agent team pattern |
| **ratchet** | Knowledge-capture/promotion rule set | Session closeout and corpus discipline |
| **dream** | Historical scheduled-compounding name | Retired pointer; background checks belong to NTM session profiles |
| **factory** | The out-of-session driver (NTM substrate-owned) | A driver, not an AgentOps-shipped daemon |
| **ship-loop** | A fast-lane shipping preset | An execution profile, not a loop |
| **flywheel** | A health check on the corpus | Background check, not a loop |
| **goals / council** | Fitness/intent config; a validation step | Config and step, not loops |

## Where this doc lives and what it replaces

This doc lives at `docs/architecture/canonical-loop-model.md` and is linked from the 3.0 north star and the documentation index. It is the canonical statement of the loop hierarchy.

It supersedes the help-string decision-tree approach (the prior attempt to disambiguate evolve, rpi, autodev, and factory as four near-synonyms). Instead of telling agents how to pick between four peers, this model states that the active unit is a skill session and the out-of-session placement is NTM. The vocabulary entries that back this model live in [`skills/domain/references/`](../../skills/domain/references/).

## See also

- [3.0.md](../3.0.md): the north star this model serves
- [ADR-0009](../adr/ADR-0009-daemon-deletion-in-session-only.md): why AgentOps ships only the in-session driver
- [operating-loop.md](operating-loop.md): the practice sequence a skill session can follow
- [ports-and-adapters.md](ports-and-adapters.md): the runtime seams sessions run through
- [`skills/domain/references/`](../../skills/domain/references/): the loop-family vocabulary
- [`.agents/discovery/2026-05-24-canonical-loops-ddd.md`](https://github.com/boshu2/agentops/blob/main/.agents/discovery/2026-05-24-canonical-loops-ddd.md): the derivation
- [`.agents/research/2026-05-24-fractal-orchestration-lineage.md`](https://github.com/boshu2/agentops/blob/main/.agents/research/2026-05-24-fractal-orchestration-lineage.md): the lineage
