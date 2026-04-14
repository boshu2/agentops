---
title: "AgentOps vs GSD (Get Shit Done) — Detailed Comparison"
description: "How AgentOps compares to GSD for AI coding agents. GSD optimizes within sessions with fresh context per agent. AgentOps carries knowledge between sessions with a compounding flywheel."
permalink: /comparisons/agentops-vs-gsd
---

# AgentOps vs GSD (Get Shit Done)

> **GSD v1.34** is a full-featured spec-driven development framework for AI coding agents. It installs as slash commands into 7 runtimes (Claude Code, Gemini CLI, OpenCode, Codex, Copilot, Cursor, Antigravity) and solves "context rot" by spawning fresh-context subagents for each task. Trusted by engineers at Amazon, Google, Shopify, and Webflow.
>
> *Comparison updated April 2026. See the [GSD repo](https://github.com/glittercowboy/get-shit-done) for current features.*

---

## At a Glance

| Aspect | GSD | AgentOps |
|--------|-----|----------|
| **Philosophy** | "Ship fast — fresh context per agent" | "Operational layer for coding agents; technically a context compiler" |
| **Core strength** | Multi-agent orchestration with context isolation, multi-runtime support | Cross-session memory, validation gates, knowledge flywheel |
| **GitHub** | glittercowboy/get-shit-done | boshu2/agentops |
| **Latest** | v1.34.2 (April 2026) | v2.37.0 (April 2026) |
| **Scale** | 53 commands, 46 workflows, 16 agents | 50+ skills, compiled CLI, hooks, schemas |
| **Primary use** | Spec-driven development with phased execution | Ongoing codebase work with persistent memory |

---

## What GSD Does Well

### 1. Fresh Context Per Agent

GSD's core innovation. Every spawned agent gets a clean 200K context window. Orchestrators stay thin, agents are disposable. This eliminates context rot — the quality degradation that happens as an AI fills its context window during long sessions.

### 2. Wave-Based Parallel Execution

Plans are grouped into dependency waves. Plans within a wave run in parallel (each with a fresh agent), waves run sequentially. Includes STATE.md file locking with atomic creation and spin-wait jitter.

```
Wave 1: [Plan A, Plan B, Plan C]  ← parallel, fresh 200K each
           ↓ (all complete)
Wave 2: [Plan D, Plan E]          ← parallel, depends on Wave 1
           ↓ (all complete)
Wave 3: [Plan F]                  ← sequential, depends on Wave 2
```

### 3. Model Cost Tiers

Four profiles (quality/balanced/budget/inherit) with per-agent model assignments. Each profile maps agents to opus/sonnet/haiku. This means routine plan-checking can run on budget models while critical execution stays on quality models.

### 4. Auto-Repair on Task Failure

When a task fails during execution, GSD auto-classifies the failure as RETRY (with adjustment), DECOMPOSE (break into sub-steps), or PRUNE (remove and escalate). Budget-controlled with a default of 2 attempts. This is structured recovery, not blind retries.

### 5. Comprehensive Validation Pipeline

Not just "human verification" anymore. GSD v1.27 has:
- 8-dimension plan checker (max 3 iterations)
- Nyquist validation audit (test coverage mapping)
- Post-execution verification agent
- User acceptance testing with auto-diagnosis
- Cross-phase verification debt audit

### 6. Seven Runtime Support

Installs to Claude Code, Gemini CLI, OpenCode, Codex, Copilot, Cursor, and Antigravity. The installer transforms file content per runtime at install time (tool name mapping, agent frontmatter, hook events, path conventions).

### 7. Advisor Mode and Fast Path (New in v1.34)

`/gsd:advisor` provides research-backed discussion without execution. `/gsd:fast` skips planning for trivial tasks. Multi-repo workspace support added for cross-project orchestration.

### 7. Deep State Management

Full `.planning/` directory with 20+ artifact types:
- PROJECT.md, REQUIREMENTS.md, ROADMAP.md, STATE.md
- Per-phase directories with research, plans, summaries, verification, UAT
- Session handoff (HANDOFF.json, continue-here.md)
- Persistent threads, seeds, debug knowledge base, todos

---

## Where GSD Falls Short

### No Cross-Session Learning

GSD has persistence (state files, handoffs, threads) but no knowledge flywheel. There is no mechanism to extract what was learned in one session and inject it into the next. Every session starts with the same agent intelligence — the system does not get smarter over time.

```
┌─────────────────────────────────────────────────────────────────┐
│                         GSD                                     │
│                                                                 │
│  Session 1: discuss → plan → execute → verify → Done            │
│                                                   ↓             │
│                                              (state saved)      │
│                                                   ↓             │
│  Session 2: resume-work → (same state, same intelligence)       │
│                                                                 │
│  Session 100: (agents are no smarter than session 1)            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      AGENTOPS                                   │
│                                                                 │
│  Session 1: research → plan → pre-mortem → crank → vibe → retro │
│                                              ↓                  │
│                                      (learnings extracted)      │
│                                              ↓                  │
│                                      (scored and stored)        │
│                                                                 │
│  Session 2: (inject prior knowledge) → better starting point    │
│                                                                 │
│  Session 100: (agent is a domain expert with scored knowledge)  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### No Strategic Goals or Direction

GSD executes phases within a project but has no mechanism for measuring progress toward higher-level objectives. No equivalent to GOALS.md, `ao goals measure`, or `/evolve`.

### No Pre-Implementation Failure Prevention

GSD validates *after* execution (verify-work, UAT). The plan checker validates plan quality but does not simulate failure modes before implementation begins. AgentOps runs `/pre-mortem` to catch issues before code is written.

### No Issue Graph or Dependency Tracking

GSD has wave-based parallelism for plans within a phase, but no cross-phase issue graph. There is no equivalent to beads (issue tracking with dependencies), which means no mechanical way to track blocked work across phases.

### No Multi-Model Validation Council

GSD's review command supports cross-AI peer review (Gemini, Claude, Codex), but this is a review tool, not a multi-perspective validation council that runs adversarial analysis from different viewpoints simultaneously.

---

## Feature Comparison

| Feature | GSD | AgentOps | Winner |
|---------|:---:|:--------:|:------:|
| Multi-runtime support | ✅ 7 runtimes | ⚠️ Claude Code primary | GSD |
| Fresh context per agent | ✅ Core design | ⚠️ Swarm workers | GSD |
| Model cost tiers | ✅ 4 profiles | ❌ Not yet | GSD |
| Auto-repair on failure | ✅ RETRY/DECOMPOSE/PRUNE | ⚠️ Crank retries | GSD |
| Context rot detection | ✅ Hooks at 35%/25% | ❌ Not yet | GSD |
| Prompt injection defense | ✅ Advisory hook | ❌ Not yet | GSD |
| Wave-based parallelism | ✅ Built in | ✅ Crank waves | Tie |
| Plan validation | ✅ 8-dimension checker | ✅ Pre-mortem + council | Tie |
| Human-in-loop gates | ✅ Configurable gates | ✅ Multiple gates | Tie |
| State persistence | ✅ .planning/ directory | ✅ .agents/ directory | Tie |
| **Cross-session learning** | ❌ No flywheel | ✅ Extract → score → inject | **AgentOps** |
| **Knowledge maturity** | ❌ No scoring | ✅ Maturity tracking + decay | **AgentOps** |
| **Pre-mortem simulation** | ❌ Post-execution only | ✅ Before implementation | **AgentOps** |
| **Multi-model council** | ❌ Sequential review | ✅ Multi-perspective | **AgentOps** |
| **Issue graph execution** | ❌ Phase-scoped waves | ✅ Beads + dependencies | **AgentOps** |
| **Strategic goals** | ❌ No goal tracking | ✅ GOALS.md + evolve | **AgentOps** |
| **Compiled CLI** | ❌ Node.js tools | ✅ Go binary (ao) | **AgentOps** |

---

## Workflow Comparison

### GSD Workflow

```
/gsd:new-project     →  PROJECT.md, REQUIREMENTS.md, ROADMAP.md, config.json
         ↓
/gsd:discuss-phase   →  Capture decisions (CONTEXT.md)
         ↓
/gsd:plan-phase      →  Research → plan → 8-dimension verify (max 3 iterations)
         ↓
/gsd:execute-phase   →  Wave-based parallel execution (fresh agent per plan)
         ↓                  └── Node repair on failure (RETRY/DECOMPOSE/PRUNE)
/gsd:verify-work     →  UAT with auto-diagnosis
         ↓
/gsd:ship            →  Create PR from phase work
         ↓
       [next phase or complete-milestone]
```

### AgentOps Workflow

```
/research     →  Explore codebase + inject prior knowledge
     ↓
/plan         →  Break into dependency-tracked issues (beads)
     ↓
/pre-mortem   →  Simulate failure modes before building
     ↓
/crank        →  Execute unblocked waves → validate → commit
     ↓
/vibe         →  Multi-aspect code validation (council optional)
     ↓
/post-mortem  →  Extract learnings → score → store for next session
```

---

## Architecture Comparison

| Aspect | GSD | AgentOps |
|--------|-----|----------|
| **Commands** | 53 prompt-based slash commands | 50+ skill definitions |
| **Agents** | 16 specialized (fresh context each) | Skill-driven (swarm for parallelism) |
| **CLI tooling** | Node.js (`gsd-tools.cjs`, 15 modules) | Go binary (`ao`, structured subcommands) |
| **Hooks** | 5 JS hooks (statusline, context monitor, prompt guard, workflow guard, update check) | Shell hooks (session lifecycle, tool gates, knowledge injection) |
| **State** | `.planning/` (Markdown + JSON) | `.agents/` (Markdown + JSON) |
| **Config** | `.planning/config.json` (40+ options) | `.agentops.json` + GOALS.md |
| **Install** | npm package, 3000-line installer | Shell script + Go binary |
| **Parallelism** | Wave-based with file locking | Wave-based via crank + swarm |

---

## Overhead Comparison

```
                    SETUP TIME              ONGOING OVERHEAD
                    ══════════              ════════════════

GSD:                ████████░░░░░░░░        ████████░░░░░░░░
                    (npm install + init)    (moderate — .planning/ management)

AgentOps:           ████████░░░░░░░░        ████████░░░░░░░░
                    (install + init)        (moderate — hooks + .agents/)


                    SESSION VALUE           LONG-TERM VALUE
                    ═════════════           ═══════════════

GSD:                ████████████████        ████████░░░░░░░░
                    (strong execution)      (state persists, no learning)

AgentOps:           ████████████████        ████████████████
                    (strong execution)      (knowledge compounds)
```

**Trade-off:** GSD optimizes for execution quality per session. AgentOps optimizes for cumulative intelligence across sessions.

---

## Use Case Fit

### GSD is Best For

| Use Case | Why |
|----------|-----|
| Greenfield projects | Strong project setup + phased execution |
| Multi-runtime teams | 7 runtimes with one install |
| Cost-sensitive work | Model cost tiers control spend |
| Complex single-phase work | Wave parallelism + auto-repair |
| Teams standardizing process | Clear phases with configurable gates |

### AgentOps is Best For

| Use Case | Why |
|----------|-----|
| Long-running codebases | Knowledge flywheel compounds value |
| Repeated maintenance | Learns from past sessions |
| Complex multi-phase work | Issue graph + dependency execution |
| Risk-averse engineering | Pre-mortem + council + vibe gates |
| Strategic direction | GOALS.md + evolve loop |

---

## When to Choose GSD

- You work across **multiple AI runtimes** and need one workflow
- You want **model cost control** at the per-agent level
- Your work is **project-scoped** (clear start and end)
- You value **fresh context per agent** for quality in long sessions
- You want **auto-repair** when tasks fail during execution

## When to Choose AgentOps

- You work on the **same codebase** across many sessions
- You want the system to **get smarter over time**
- You want **failure prevention before implementation**, not just verification after
- You want **dependency-tracked issue execution** across work phases
- You value **strategic goal tracking** and measured progress

---

## Can They Work Together?

**Partially.** GSD and AgentOps both manage state directories and workflow orchestration, so running both simultaneously would create friction. However:

- GSD's fresh-context-per-agent pattern is a technique AgentOps' swarm could adopt
- GSD's model cost tiers solve a problem AgentOps does not yet address
- AgentOps' knowledge flywheel fills GSD's biggest gap (no cross-session learning)

The most practical combination: use GSD for greenfield projects where you need fast phased execution, then bring AgentOps in when the project enters maintenance and long-term development where accumulated knowledge matters.

---

## The Bottom Line

| Dimension | GSD | AgentOps |
|-----------|-----|----------|
| **Philosophy** | Fresh context, fast execution | Knowledge compounds |
| **Overhead** | Moderate | Moderate |
| **Persistence** | State files (no learning) | Knowledge flywheel |
| **Validation** | 8-dimension plan check + UAT | Pre-mortem + council + vibe |
| **Parallelism** | Wave-based, fresh agents | Wave-based, swarm workers |
| **Cost control** | 4-tier model profiles | Not yet |
| **Best for** | Strong execution per session | Cumulative intelligence across sessions |

**GSD is a serious framework for structured AI-assisted development.**
**AgentOps differentiates on the knowledge flywheel — the system that makes every session smarter than the last.**

---

## The Honest Assessment

**GSD is not a lightweight tool anymore.** It is a comprehensive development framework with 53 commands, 16 agents, wave-based parallelism, auto-repair, model cost tiers, and deep state management. Dismissing it as "simple meta-prompting" is inaccurate.

**Where GSD wins:** Execution quality within a session. Fresh context per agent, cost control, auto-repair, and 7-runtime portability.

**Where AgentOps wins:** Intelligence across sessions. The knowledge flywheel (extract, score, inject, decay) has no equivalent in GSD. After 50 sessions on the same codebase, AgentOps is operating with accumulated domain knowledge while GSD agents start fresh every time.

```
Session 1:   GSD and AgentOps roughly equal
Session 10:  AgentOps has a library of scored learnings
Session 50:  AgentOps agents get injected domain expertise; GSD agents do not
Session 100: AgentOps is a domain expert; GSD is still starting from its .planning/ state
```

---

<div align="center">

[← vs. SDD](vs-sdd.md) · [Back to Comparisons](README.md)

</div>
