# AgentOps Architecture

> Context quality is the primary lever for agent output quality. Orchestrate what enters each window, compound what comes out.

## Overview

AgentOps is a skills plugin that orchestrates context across agent windows and compounds results through a knowledge flywheel — each session is smarter than the last.

The architecture rests on five pillars. Each one is independent — you can use any skill standalone — but together they form a recursive system that gets smarter with every cycle.

```
┌─────────────────────────────────────────────────────────────────┐
│                     DevOps Three Ways                           │
│              (Flow · Feedback · Continual Learning)              │
├─────────────┬─────────────┬─────────────┬───────────────────────┤
│  Brownian   │    Ralph    │  Knowledge  │       Fractal         │
│  Ratchet    │   Wiggum    │  Flywheel   │     Composition       │
│             │   Pattern   │             │                       │
│  chaos →    │  fresh ctx  │  extract →  │  same shape at        │
│  filter →   │  per worker │  score →    │  every scale:         │
│  ratchet    │  disk state │  inject →   │  lead → workers →     │
│             │  lead-only  │  compound   │  validate → next wave │
└─────────────┴─────────────┴─────────────┴───────────────────────┘
```

---

## Design Philosophy

Three principles drive every architectural decision:

**The intelligence lives in the window.** Agent output quality is determined by context input quality. Bad answers mean wrong context was loaded. Contradictions mean context wasn't shared between agents. Hallucinations mean context was too sparse. Drifting means signal-to-noise collapsed. Every failure is a context failure — so every solution is a context solution.

**Least-privilege context loading.** Each agent receives only the context necessary for its task. Research gets prior knowledge. Plan gets a 500-token research summary. Crank workers get fresh context per wave with zero bleed-through. Vibe gets recent changes only. Phase summaries compress output between phases to prevent signal-to-noise collapse. The context window is treated as a security boundary — nothing enters without scoping.

**The cycle is the product.** No single skill is the value. The compounding loop — discovery, implementation, validation, learn, repeat — makes each successive context window smarter than the last. Post-mortem doesn't just extract learnings; it proposes the next cycle's work. The system feeds itself.

---

## Pillar 1: DevOps Three Ways

The meta-framework. [DevOps' Three Ways](https://itrevolution.com/articles/the-three-ways-principles-underpinning-devops/) — Flow, Feedback, Continual Learning — applied to agent orchestration.

**Flow.** Orchestration skills move WIP through the system. Research → plan → validate → build → review → learn — single-piece flow, minimizing context switches. `/rpi` runs all phases end to end. `/crank` executes waves of parallel workers. `/swarm` spawns teams.

**Feedback.** Shorten the feedback loop until defects can't survive it. Multi-model councils (`/council`) catch issues before code ships. Hooks make the rules unavoidable — validation gates, push blocking, regression auto-revert. Problems found Friday don't wait until Monday.

**Continual Learning.** Stop rediscovering what you already know. Every session extracts learnings, scores them, and re-injects them at the next session start. Knowledge compounds when retrieval quality and usage stay ahead of decay and scale friction. Session 50 knows what session 1 learned the hard way.

These three ways aren't aspirational — they're mechanically enforced through skills, hooks, and operational invariants.

Deep dive: [the-science.md](the-science.md) — formal model, decay rates, escape velocity.

---

## Pillar 2: Brownian Ratchet

The execution model. Spawn parallel agents (chaos), validate their output with a multi-model council (filter), merge passing results (ratchet). Progress locks forward — failed agents are discarded cheaply because fresh context means no contamination.

```
        CHAOS                    FILTER                   RATCHET
  ┌───────────────┐      ┌──────────────────┐      ┌──────────────┐
  │ Spawn parallel│      │ Council validates│      │ Merge passing│
  │ agents per    │ ──── │ each result:     │ ──── │ results.     │
  │ wave          │      │ PASS / WARN /    │      │ Progress is  │
  │               │      │ FAIL             │      │ permanent.   │
  └───────────────┘      └──────────────────┘      └──────────────┘
```

### The FIRE Loop

The reconciliation engine that implements the ratchet:

- **F**ind — Read current state: open issues, blocked tasks, completed waves
- **I**gnite — Spawn parallel agents for the next wave of unblocked work
- **R**eap — Harvest results, validate artifacts, lock passing work forward
- **E**scalate — Handle failures: retry (max 3), redecompose, or escalate to human

`/crank` runs the FIRE loop until every issue in an epic is closed. Each wave spawns fresh workers, validates their output, and advances the ratchet.

### Validation Gates

Gates are checkpoints enforced by hooks. They block progress until a condition is met:

| Gate | Blocks | Condition |
|------|--------|-----------|
| Push gate | `git push` | `/vibe` must pass |
| Pre-mortem gate | `/crank` on 3+ issue epics | `/pre-mortem` must pass |
| Task validation | Task completion | Acceptance criteria verified |
| Worker guard | Workers committing | Only lead commits |
| Dangerous git guard | `force-push`, `reset --hard` | Explicit user request required |

### Council (Multi-Model Consensus)

The core validation primitive. Spawns independent judge agents (Claude and/or Codex) that review work from different perspectives, deliberate, and converge on a verdict: PASS, WARN, or FAIL.

Judges write all analysis to output files. Messages to the lead contain only minimal completion signals. This context budget rule prevents N judges from exploding the lead's context window.

Foundation for `/vibe`, `/pre-mortem`, and `/post-mortem`.

Deep dive: [brownian-ratchet.md](brownian-ratchet.md) — full philosophy, economics, FIRE loop details.

---

## Pillar 3: Ralph Wiggum Pattern

The isolation model. Every execution unit gets fresh context — no bleed-through between workers or waves. Named after the [Ralph Wiggum pattern](https://ghuntley.com/ralph/).

### Atomic Work

A unit of work is atomic when it has **no shared mutable state** with concurrent workers. Pure function model:

```
Input:  issue spec + codebase snapshot
Output: patch + verification result
```

This isolation property is what enables parallel wave execution — workers cannot interfere with each other. One task, one worker, one verify cycle.

### Fresh Context Per Wave

Each wave spawns new workers with clean context. No carryover between waves. After a wave completes:

1. Lead validates all worker output
2. Lead commits passing work
3. Resources cleaned up (teams terminated, worktrees removed)
4. Next wave spawned with fresh context that sees the committed changes

### Lead-Only Commits

Workers write files but never commit. Only the lead (the orchestrating session) runs `git add` / `git commit`. This prevents concurrent commits, maintains audit trail, and ensures validation happens before persistence.

### Disk-Backed State

Loop continuity comes from filesystem state, not accumulated chat context:

- **TaskList** tracks work status (what's done, what's blocked)
- **`.agents/`** stores artifacts (research, plans, learnings, council reports)
- **Beads issues** persist across sessions (git-native issue tracking)
- **Backend messaging** carries short coordination signals only (< 100 tokens) — never work details

This is why the system survives context compaction: everything important is on disk.

### Context Boundaries

The system enforces context isolation at three levels:

**Phase boundaries.** Each RPI phase produces a compressed summary (500 tokens max) that feeds the next phase. Raw output never crosses phase boundaries — only distilled signal.

**Worker boundaries.** Each crank worker gets fresh context scoped to its assigned issue. Workers cannot see each other's work-in-progress. Only the lead sees all workers' output and commits.

**Session boundaries.** Each session starts with injected knowledge (freshness-weighted, quality-gated) and ends with extracted learnings. The flywheel bridges sessions without carrying raw context forward.

Deep dive: [how-it-works.md](how-it-works.md) — Ralph Wiggum Pattern, agent backends, hooks, context windowing.

---

<a id="knowledge-flywheel"></a>

## Pillar 4: Knowledge Flywheel

The learning model. Automated extraction → quality gates → tiered storage → retrieval → injection → compounding.

```
  Sessions → Transcripts → Forge → Pool → Promote → Knowledge
       ↑                                               │
       └───────────────────────────────────────────────┘
```

### The RPI Workflow

```
Research → Plan → Implement → Validate
    ↑                            │
    └──── Knowledge Flywheel ────┘
```

Each phase is a context boundary. The output of one phase is compressed and scoped before entering the next — preventing context contamination across phases.

| Phase | Skills | Output |
|-------|--------|--------|
| **Research** | `/research`, `/knowledge` | `.agents/research/` |
| **Plan** | `/pre-mortem`, `/plan` | Beads issues |
| **Implement** | `/implement`, `/crank` | Code, tests |
| **Validate** | `/vibe`, `/retro`, `/post-mortem` | `.agents/learnings/`, `.agents/patterns/` |

Every `/post-mortem` feeds back into the next `/rpi` cycle:

1. Council validates the implementation
2. `/retro` extracts learnings → `.agents/learnings/`
3. Process improvement proposals synthesized from retro findings
4. Next-work items harvested → `.agents/rpi/next-work.jsonl`
   - Each item includes a `target_repo` field: repo name (string) for repo-scoped work, `"*"` for cross-repo items, or omitted for legacy backward compatibility
   - Consumers filter items by matching `target_repo` against the current repo
5. **Suggested `/rpi` command presented** — ready to copy-paste

### Quality Gates

Learnings re-enter future context windows through quality gates: 5-dimension scoring (specificity, actionability, novelty, context, confidence) into gold/silver/bronze tiers. Freshness decay (MemRL two-phase retrieval, delta=0.17/week from [Darr 1995](the-science.md)) ensures stale knowledge loses priority automatically. The flywheel is curation, not just storage.

### Knowledge Artifacts

`.agents/` stores knowledge generated during sessions:

```
.agents/
├── bundles/       # Grouped artifacts
├── council/       # Council/validation reports
├── handoff/       # Session handoff context
├── learnings/     # Extracted lessons
├── patterns/      # Reusable patterns
├── plans/         # Implementation plans
├── pre-mortems/   # Failure simulations
├── reports/       # General reports
├── research/      # Exploration findings
├── retros/        # Retrospective reports
├── specs/         # Validated specifications
└── tooling/       # Tooling documentation
```

Knowledge artifacts are the system's long-term memory. Future `/research` commands discover them via file pattern matching, semantic search (`ao forge`), or Smart Connections MCP (if available). Freshness decay ensures stale artifacts lose priority over time — the system forgets what's no longer relevant. Quality gates prevent low-confidence or context-specific learnings from polluting the shared knowledge base.

Deep dive: [knowledge-flywheel.md](knowledge-flywheel.md) — flywheel mechanics. [the-science.md](the-science.md) — formal model, decay rates, limits to growth, and the scale-aware condition `ρ·σ(K,t) > δ + φ·K - I(t)/K`.

---

## Pillar 5: Fractal Composition

The composition model. The same shape — lead decomposes work → workers execute atomically → validation gates lock progress → next wave — repeats at every scale.

```
Level 0: /implement ─── mini-RPI
│        (explore → build → verify → commit)
│        One worker, one issue, one verify cycle.
│
Level 1: /crank ──────── waves of /implement
│        FIRE loop: Find → Ignite → Reap → Escalate
│        Each wave spawns fresh workers in parallel.
│
Level 2: /rpi ────────── discovery → implementation → validation
│        Full lifecycle. Session IS the lead. Sub-skills manage own teams.
│
Level 3: /evolve ─────── fitness-gated /rpi cycles
         Measure goals → pick worst → run /rpi → re-measure → regress? revert : loop
```

At every level:
- A **lead** decomposes work and validates results
- **Workers** execute atomically with fresh context
- **Validation** gates lock progress forward
- **Next wave** begins with the lead's updated state

The skills compose because they share this shape. `/crank` doesn't know it's inside `/rpi`. `/implement` doesn't know it's inside `/crank`. Each level treats the one below it as a black box that accepts a spec and returns a validated result.

### Backend Selection

The runtime picks the spawning backend by capability detection — not prompt text, not hardcoded tool names:

1. **Codex sub-agents** (`spawn_agent` available) — fastest, native to Codex CLI
2. **Claude native teams** (`TeamCreate` + `SendMessage` available) — tight coordination, debate support
3. **Background tasks** (`Task(run_in_background=true)`) — last-resort fallback
4. **Distributed** (tmux + Agent Mail) — full process isolation, crash recovery for long-running work

The same skill works across all backends. Backend selection is a runtime decision, not an architectural one.

### Complexity Scaling

Gate sizing adapts to epic complexity:

| Complexity | Criteria | Gate Strategy |
|------------|----------|---------------|
| Low | ≤ 2 issues, 1 wave | `--quick` (inline, no spawning) |
| Medium | 3-6 issues, 2 waves | `--quick` (fast default) |
| High | 7+ issues, 3+ waves | Full multi-judge council |

~10% cost for `--quick`, same bug detection class as full council.

---

## Operational Invariants

Cross-cutting rules enforced by hooks — not guidelines, not suggestions. Mechanically enforced.

| Invariant | Enforced By | What It Prevents |
|-----------|-------------|------------------|
| Workers MUST NOT commit | Worker guard hook | Concurrent commits, unvalidated changes |
| Workers MUST NOT race-claim tasks | Pre-assignment before spawn | Race conditions in multi-worker waves |
| Verify THEN trust | Validation contract | False completion claims from agents |
| Push blocked until `/vibe` passes | Push gate hook | Unvalidated code reaching remote |
| `/crank` blocked until `/pre-mortem` passes (3+ issues) | Pre-mortem gate hook | Expensive implementation of flawed plans |
| No destructive git without explicit request | Dangerous git guard | Accidental data loss |
| Mechanical checks override council PASS | Constraint tests | LLMs estimating instead of measuring |
| Max 50 waves per epic | Global wave limit | Infinite execution loops |
| Max 3 retries per gate | Gate retry logic | Infinite retry loops |
| Completion requires explicit marker | Sisyphus rule | Premature completion claims |
| Kill switch checked every cycle | Deploy kill switch | Runaway `/evolve` loops |
| Skip goal after 3 consecutive failures | Strike check | Infinite retry on fundamentally broken goals |

All hooks can be disabled: `AGENTOPS_HOOKS_DISABLED=1` (kill switch) or per-hook variables in [ENV-VARS.md](ENV-VARS.md).

---

## Component Overview

```
.
├── .claude-plugin/
│   └── plugin.json      # Plugin manifest
├── skills/              # 53 skills (43 user-facing, 10 internal)
│   ├── rpi/             # orchestration — Full RPI lifecycle orchestrator
│   ├── council/         # orchestration — Multi-model validation (core primitive)
│   ├── crank/           # orchestration — Autonomous epic execution
│   ├── swarm/           # orchestration — Parallel agent spawning
│   ├── codex-team/      # orchestration — Parallel Codex execution
│   ├── evolve/          # orchestration — Goal-driven fitness loop
│   ├── implement/       # team — Execute single issue
│   ├── research/        # solo — Deep codebase exploration
│   ├── plan/            # solo — Decompose epics into issues
│   ├── vibe/            # solo — Code validation (complexity + council)
│   ├── pre-mortem/      # solo — Council on plans
│   ├── post-mortem/     # solo — Council + retro (wrap up work)
│   ├── shared/          # library — Shared reference docs
│   └── ...              # 39 more skills
├── hooks/               # 12 hook scripts (lifecycle enforcement)
├── lib/                 # Shared code
└── docs/                # Documentation
```

### Skill Tiers

Skills span six tiers. Each level composes the ones below it.

| Tier | Skills | Purpose |
|------|--------|---------|
| **Orchestration** | `/rpi`, `/council`, `/crank`, `/swarm`, `/codex-team`, `/evolve` | Multi-phase workflows |
| **Team** | `/implement` | Single issue, full lifecycle |
| **Solo** | `/research`, `/plan`, `/vibe`, `/pre-mortem`, `/post-mortem`, `/retro`, etc. | Standalone use |
| **Library** | `beads`, `standards`, `shared` | Reference docs loaded by other skills |
| **Background** | `inject`, `extract`, `forge`, `provenance`, `ratchet`, `flywheel` | Hook-triggered, invisible |
| **Meta** | `using-agentops` | Workflow guide, auto-injected |

### Subagents

Subagents are disposable. Each gets fresh context scoped to its role — no accumulated state, no bleed-through. Clean context in, validated output out, then terminate.

Subagent behaviors are defined inline within SKILL.md files. Skills that use subagents (e.g., `/council`, `/vibe`, `/pre-mortem`, `/post-mortem`, `/research`) spawn them via runtime-native backends.

### ao CLI Integration

For full workflow orchestration, skills integrate with the ao CLI:

| Skill | ao Command |
|-------|------------|
| `/research` | `ao forge search` |
| `/retro` | `ao forge index` |
| `/post-mortem` | `ao ratchet record` |
| `/implement` | `ao ratchet claim/record` |
| `/crank` | `ao ratchet verify` |

---

## Session Hook

On session start, `hooks/session-start.sh`:
1. Creates `.agents/` directories if missing
2. Injects `using-agentops` skill content as context
3. Outputs JSON with `additionalContext` for compatible agent runtimes

---

## Installation

```bash
npx skills@latest add boshu2/agentops --all -g
```

Optional:
- [beads](https://github.com/steveyegge/beads) for issue tracking
- [ao CLI](https://github.com/boshu2/ao) for full orchestration
