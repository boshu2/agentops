---
title: "AgentOps vs Compound Engineer — Detailed Comparison"
description: "How AgentOps compares to Compound Engineer for AI coding agents. Both compound knowledge, but AgentOps automates the flywheel while Compound Engineer requires manual invocation."
permalink: /comparisons/agentops-vs-compound-engineer
last_reviewed: 2026-05-07
---

# AgentOps vs Compound Engineer

> **Compound Engineer** is Every's coding-agent plugin implementing a knowledge-compounding development workflow. The core thesis: "Each unit of engineering work should make subsequent units easier." Current state (May 2026): 37 skills, 51 agents, an explicit 7-phase workflow (ideate → brainstorm → plan → work → review → compound → refresh), and a 10-target cross-runtime conversion CLI. Recent additions: stack-aware reviewer routing, mandatory code review by default, and PR description filtering.
>
> See the [Compound Engineer repo](https://github.com/EveryInc/compound-engineering-plugin) for current features.

---

## At a Glance

| Aspect | Compound Engineer | AgentOps |
|--------|-------------------|----------|
| **Philosophy** | "Each unit of engineering work should make subsequent units easier" | "Operational layer for coding agents; technically a context compiler" |
| **Core strength** | Full ideate-to-compound loop, cross-runtime portability, configurable review agents | Git-tracked memory, validation gates, knowledge flywheel with scoring |
| **GitHub** | EveryInc/compound-engineering-plugin | boshu2/agentops |
| **Latest** | Active development (April 2026) | v2.39.0 (April 2026) |
| **Scale** | 37 skills, 51 agents, 10 runtime targets | 73 skills, compiled CLI, hooks, schemas |
| **Primary use** | Standardized engineering workflow with knowledge capture | Ongoing codebase work with persistent memory and validation |

### Three-Layer Comparison

| Product Layer | Compound Engineer | AgentOps |
|---------------|------------------:|----------|
| **Context Compiler** (L1) | Manual `/refresh` invocation loads context | Automatic injection at session start; phase-scoped, token-budgeted |
| **Validation Gates** (L2) | Configurable review agents (single-model) | Multi-model councils with independent judges and auditable verdicts |
| **Knowledge Flywheel** (L3) | `/compound` captures manually; no scoring or decay | Automated extract → score → promote → inject with decay ranking |

---

## What Compound Engineer Does Well

### 1. The Full Ideate-to-Compound Loop

Compound Engineer's workflow now starts earlier and ends later than just plan/work/review:

```text
Ideate -> Brainstorm -> Plan -> Work -> Review -> Compound -> Refresh
```

- **Ideate** (`/ce:ideate`): Divergent idea generation with parallel subagents using different frames (friction, inversions, leverage, edge cases), then adversarial critique filters to 5-7 survivors. Grounded in codebase scanning and GitHub issue intelligence.
- **Brainstorm** (`/ce:brainstorm`): Collaborative dialogue to define WHAT to build. Produces a requirements doc, not just casual brainstorming. Answers product questions before planning begins.
- **Compound Refresh** (`/ce:compound-refresh`): Systematic maintenance of the `docs/solutions/` knowledge base. Classifies learnings as Keep/Update/Replace/Archive. Has both interactive and autonomous modes with subagent investigation.

### 2. Cross-Runtime Reach Is Excellent

The Bun/TypeScript CLI converts the Claude Code plugin to 10 other formats:

| Target | Notes |
|--------|-------|
| OpenCode | Commands as .md, deep-merged config |
| Codex | Prompt + skill pairs |
| Factory Droid | Tool name remapping |
| Pi | Prompts, skills, extensions, MCPorter interop |
| Gemini CLI | Skills from agents, commands as .toml |
| GitHub Copilot | .agent.md with Copilot frontmatter |
| Kiro | JSON configs + prompt .md |
| OpenClaw | TypeScript skill file |
| Windsurf | Global or workspace scope |
| Qwen Code | Agents as .yaml |

Plus a `sync` command that copies personal `~/.claude/` config to all other runtimes.

### 3. Configurable Review Agents Per Project

The `/setup` skill auto-detects project stack and configures which review agents run during `/ce:review`. Stored in `compound-engineering.local.md`. Includes 15 specialized review agents (security, performance, architecture, Rails patterns, race conditions, schema drift, etc.) that activate based on project type.

### 4. Three-Tier Plan Detail

Plans auto-select Minimal/More/A Lot templates based on complexity. Prevents over-engineering small tasks while allowing depth for large features. Multi-agent research (repo analyst, learnings researcher, best-practices researcher) informs the plan.

### 5. Swarm Orchestration Guide

The `orchestrating-swarms` skill is a 1700+ line reference document covering Claude Code's TeammateTool, Task system, inbox messaging, spawn backends (in-process, tmux, iterm2), and 6 orchestration patterns. This serves as a comprehensive swarm API reference.

### 6. Plugin Marketplace Distribution

Now distributed via Claude Code's plugin marketplace:
```bash
/plugin marketplace add EveryInc/compound-engineering-plugin
```

---

## The Real Difference

Both projects care about compounding. The distinction is **where the system puts the weight** and **how mechanical the compounding is**.

```text
Compound Engineer:
  workflow discipline (ideate → compound → refresh)
  + cross-tool portability (10 runtimes)
  + configurable review agents
  + knowledge capture in docs/solutions/

AgentOps:
  repo-native memory with scoring and injection
  + validation gates (pre-mortem, council, vibe)
  + tracked execution with dependency ordering
  + strategic goals and measured progress
```

<!-- agentops:claim:AOP-CLAIM-COMP-COMPOUND-ENGINEER -->
Compound Engineer captures knowledge manually through the compound step and maintains it through compound-refresh. AgentOps extracts knowledge automatically through session hooks and post-mortems, scores it for maturity, and injects relevant learnings into future sessions mechanically.

---

## Where AgentOps Goes Further

### Automated Knowledge Flywheel, Not Manual Compounding

Both systems compound knowledge. The mechanical difference matters:

| Aspect | Compound Engineer | AgentOps |
|--------|-------------------|----------|
| Knowledge capture | `/ce:compound` (manual trigger) | Post-mortem + session hooks (automatic) |
| Knowledge maintenance | `/ce:compound-refresh` (manual or scheduled) | `ao maturity` + decay (automatic) |
| Knowledge retrieval | `learnings-researcher` agent searches docs/solutions/ | `ao inject` scores and retrieves by relevance |
| Knowledge scoring | Pattern detection at 3+ similar solutions | Maturity scoring with confidence and decay |

AgentOps' flywheel runs without the user remembering to invoke it. Compound Engineer requires the user to run `/ce:compound` after work and `/ce:compound-refresh` periodically.

### More Explicit Failure Prevention

AgentOps adds named gates *before* implementation:

- `/pre-mortem` simulates failure modes before building
- `/council` runs multi-model adversarial validation
- `/vibe` validates across multiple dimensions after building

Compound Engineer has strong review agents but they run *after* implementation. There is no pre-implementation failure simulation.

### Issue Graph and Wave Execution

AgentOps is more opinionated about tracked work:

- `/plan` creates dependency-aware issues through beads
- `/crank` executes unblocked waves with validation after each
- `/evolve` measures goals and repeats the loop automatically

Compound Engineer's `/ce:work` executes plan files with todo tracking and supports swarm mode, but does not model cross-issue dependencies.

### Strategic Goals

AgentOps has GOALS.md, `ao goals measure`, and `/evolve` for measuring progress toward higher-level objectives. Compound Engineer has no equivalent goal-tracking or strategic direction mechanism.

### Model-Independent Phase Routing

AgentOps routes a different model to each phase of one RPI loop, with state preserved across boundaries:

| Phase | Model | Why |
|-------|-------|-----|
| Research / Discovery | Claude | Long-context synthesis over the corpus |
| Plan | Claude | Architectural reasoning |
| Pre-mortem | Codex (or `/council --mixed`) | Adversarial second opinion from a different family |
| Implement | Codex | Code generation throughput |
| Validate | Fresh Claude | Independent reviewer with no prior context bias |

Compound Engineer's 10-target conversion is multi-runtime *distribution* — running the same Claude-shaped workflow on Codex, Gemini, OpenCode, etc. AgentOps does that too via supported runtimes, but additionally swaps models *per phase within a single session* so each phase runs on the model best suited to that phase. CE doesn't do this per-phase model swap; the workflow is single-model end-to-end inside whichever runtime executes it.

---

## Where Compound Engineer Goes Further

### Ideation with Adversarial Filtering

`/ce:ideate` is more structured than AgentOps' `/brainstorm`. It spawns parallel subagents with different analytical frames, then runs adversarial critique. AgentOps has brainstorming but not the structured divergent-then-adversarial pipeline.

### Configurable Review Agents

Compound Engineer's per-project review agent configuration (15 specialized agents, auto-detected from stack) is more flexible than AgentOps' fixed validation pipeline. Different projects get different reviewers automatically.

### Knowledge Maintenance (compound-refresh)

While AgentOps has maturity scoring and decay, Compound Engineer's `/ce:compound-refresh` does systematic investigation of staleness — dispatching subagents to check whether stored solutions are still accurate against the current codebase. AgentOps' decay is time-based; CE's refresh is investigation-based.

### Cross-Runtime Portability

10 runtime targets with a conversion CLI is significantly broader than AgentOps' Claude Code-primary approach. Teams using multiple AI tools benefit directly.

---

## Feature Comparison

| Feature | Compound Engineer | AgentOps | Winner |
|---------|:-----------------:|:--------:|:------:|
| Cross-runtime support | ✅ 10 targets + sync | ⚠️ 4 supported runtime paths, no 10-target converter | CE |
| Configurable review agents | ✅ 15 agents, per-project | ⚠️ Fixed pipeline | CE |
| Ideation + adversarial filtering | ✅ Structured pipeline | ⚠️ Brainstorm skill | CE |
| Knowledge maintenance | ✅ compound-refresh (investigative) | ✅ Maturity + decay (automated) | Tie |
| Plugin marketplace | ✅ Native distribution | ⚠️ Script install | CE |
| Swarm reference guide | ✅ 1700+ line guide | ✅ Swarm skill | Tie |
| Workflow clarity | ✅ Explicit 7-phase loop | ✅ Explicit RPI loop | Tie |
| Planning emphasis | ✅ 3-tier detail levels | ✅ Pre-mortem + plan | Tie |
| Worktree-oriented execution | ✅ Built in | ✅ Built in | Tie |
| **Cross-session learning** | ⚠️ Manual compound step | ✅ Automated flywheel | **AgentOps** |
| **Knowledge scoring** | ⚠️ Pattern detection at 3+ | ✅ Maturity + confidence + decay | **AgentOps** |
| **Pre-mortem simulation** | ❌ Not present | ✅ Before implementation | **AgentOps** |
| **Multi-model council** | ❌ Not present | ✅ Multi-perspective validation | **AgentOps** |
| **Model-independent phase routing** | ❌ Single-model workflow per runtime | ✅ Per-phase model swap (Claude → Codex → fresh Claude) in one RPI loop | **AgentOps** |
| **Issue graph execution** | ⚠️ Plan-based todos | ✅ Beads + dependency waves | **AgentOps** |
| **Strategic goals** | ❌ No goal tracking | ✅ GOALS.md + evolve | **AgentOps** |
| **Compiled CLI** | ❌ No binary | ✅ Go binary (ao) | **AgentOps** |

---

## Workflow Comparison

### Compound Engineer Workflow

```text
/ce:ideate       -> divergent ideation with adversarial filtering
     ↓
/ce:brainstorm   -> collaborative requirements exploration
     ↓
/ce:plan         -> multi-agent research → 3-tier plan
     ↓
/ce:work         -> execute with branching, todos, incremental commits
     ↓                  (supports inline, serial, parallel, or swarm)
/ce:review       -> configurable multi-agent code review
     ↓
/ce:compound     -> document learnings to docs/solutions/
     ↓
/ce:compound-refresh -> maintain knowledge base over time
```

### AgentOps Workflow

```text
/research     -> explore codebase + inject prior knowledge
     ↓
/plan         -> break into dependency-tracked issues (beads)
     ↓
/pre-mortem   -> simulate failure modes before building
     ↓
/crank        -> execute unblocked waves → validate → commit
     ↓
/vibe         -> multi-aspect code validation (council optional)
     ↓
/post-mortem  -> extract learnings → score → store for next session
```

**Key difference:** Compound Engineer starts earlier (ideation, brainstorming) and compounds knowledge through explicit documentation. AgentOps enforces stronger pre-implementation gates and compounds knowledge through automated extraction and scoring.

---

## When to Choose Compound Engineer

- You want a **structured ideation-to-delivery loop** with adversarial idea filtering.
- You work across **multiple AI runtimes** and need config sync.
- You want **project-specific review agents** that auto-detect from your stack.
- Your team wants **plugin marketplace distribution** for easy adoption.
- You prefer **explicit knowledge documentation** over automated extraction.

## When to Choose AgentOps

- You want **automatic learning extraction** without remembering to run a compound step.
- You want **failure prevention before implementation** (pre-mortem, council).
- You want **dependency-tracked issue execution** across complex work.
- You want **strategic goal tracking** and measured progress toward objectives.
- You want a **compiled CLI** with structured operations (search, maturity, config).

---

## Can They Work Together?

**Yes.** This is one of the better pairings.

- Use Compound Engineer for its ideation pipeline, configurable review agents, and cross-runtime sync.
- Use AgentOps for memory scoring and injection, validation gates, and tracked execution.

The knowledge systems are complementary: CE's `docs/solutions/` captures explicit documentation, while AgentOps' `.agents/learnings/` captures automated extractions with maturity scoring. Both could feed into the same codebase without conflict.

---

## The Bottom Line

| Dimension | Compound Engineer | AgentOps |
|-----------|-------------------|----------|
| **Optimizes** | Workflow breadth (ideation to maintenance) | Repo intelligence (learning to injection) |
| **Knowledge model** | Capture → document → refresh | Extract → score → inject → decay |
| **Review model** | 15 configurable agents per project | Fixed gates (pre-mortem, council, vibe) |
| **Runtime reach** | 10 targets + plugin marketplace | 4 supported runtime paths, no 10-target converter |
| **Best fit** | Teams wanting portable, structured workflow | Long-running codebases needing accumulated intelligence |

**Compound Engineer is the closest philosophical neighbor in this comparison set.** Both believe in knowledge compounding. The difference: CE compounds through workflow discipline and explicit documentation. AgentOps compounds through automated extraction, scoring, and mechanical injection.

---

<div align="center">

[← Back to Comparisons](README.md) · [vs. GSD →](vs-gsd.md)

</div>
