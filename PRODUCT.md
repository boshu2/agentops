---
last_reviewed: 2026-03-15
---

# PRODUCT.md

## Mission

AgentOps closes the three gaps that prevent coding agents from compounding: **judgment validation**, **durable learning**, and **loop closure**. It is the local DevOps layer that validates the plan and code, captures what was learned, and feeds it into the next session so every cycle produces better work than the last.

> Canonical contract: [docs/context-lifecycle.md](docs/context-lifecycle.md)

## Vision

Make coding agents feel like a real engineering organization: validated work, institutional memory, and continuous improvement by default.

## Target Personas

### Persona 1: The Solo Developer
- **Goal:** Ship features faster while maintaining code quality — without manual code review or multi-person coordination overhead.
- **Pain point:** Each agent session starts from scratch. There's no memory of what worked, what failed, or what the codebase expects. Validation is manual or skipped entirely.
- **Gap exposure:** Judgment validation (no review before commit) and durable learning (session amnesia).

### Persona 2: The Agent Orchestrator
- **Goal:** Run multiple agents in parallel on a shared codebase without conflicts, with visibility into what each agent is doing and what the system learned.
- **Pain point:** Parallel agents create cascading blockers — file conflicts, violated constraints, repeated mistakes. No coordination layer exists between sessions. Manual ticket grooming and post-mortems burn cycles that agents should handle.
- **Gap exposure:** Loop closure (completed work doesn't inform next work) and durable learning (agents repeat each other's mistakes).

### Persona 3: The Quality-First Maintainer
- **Goal:** Ship fewer but higher-confidence releases. Prevent regressions. Maintain institutional knowledge across team and agent turnover.
- **Pain point:** Design decisions get lost in commit messages. Agents repeat mistakes because knowledge isn't captured. Test coverage stalls because writing tests is slower than writing features.
- **Gap exposure:** All three gaps — judgment validation (regressions slip through), durable learning (institutional knowledge lost), and loop closure (completed work doesn't feed back into constraints).

## What the Product Actually Is

AgentOps has three layers, each designed to close one or more of the three gaps:

### 1. Skills (54 skills across 4 runtimes)

Markdown-defined workflows that agents load and execute. Organized by which gap they close:

- **Judgment validation** — `/pre-mortem`, `/vibe`, `/council`, `/review`. Multi-model consensus validates plans before build and code before commit.
- **Durable learning** — `/retro`, `/forge`, `/inject`, `/flywheel`, `/athena`. Extract, score, curate, and retrieve learnings so solved problems stay solved.
- **Loop closure** — `/post-mortem`, `/evolve`, `/rpi`, `/crank`. Turn completed work into better next work, better rules, and better future context.

Skills work across Claude Code, Codex CLI, Cursor, and OpenCode. Each runtime has native format support (`/converter` exports between them). Codex-native skills ship alongside Claude-native.

### 2. CLI (`ao`)

A Go binary that provides the repo-native infrastructure skills depend on:

- **Knowledge flywheel** (durable learning) — `ao inject`, `ao lookup`, `ao forge`, `ao curate`, `ao defrag` manage the learning lifecycle with quality scoring, freshness decay, and deduplication.
- **Goals** (loop closure) — `ao goals measure` runs fitness gates, `ao goals steer` manages strategic directives, `/evolve` uses goals as its objective function.
- **Context assembly** (judgment validation) — `ao context assemble` builds phase-appropriate context packets. `ao inject` loads relevant learnings into the current session.
- **Issue tracking** (loop closure) — `bd` (beads) provides git-native issue tracking with dependency graphs, wave decomposition, and epic management.

### 3. Hooks

Session lifecycle hooks that run automatically, closing gaps without agent initiative:

- **SessionStart** — injects relevant knowledge (durable learning), checks for stale state, loads prior handoffs (loop closure).
- **PreToolUse / PostToolUse** — nudges toward structured workflows, enforces constraints (judgment validation).
- **UserPromptSubmit** — pre-mortem reminders, stall detection (judgment validation).

## Core Value Propositions

Each value proposition maps to one or more of the three gaps it closes:

- **Ship With Confidence** (judgment validation) — Multi-model consensus (Claude + Codex judges debate independently) validates plans before build and code before commit. Validation gates block, not advise.
- **Compound Intelligence Across Sessions** (durable learning) — Each session captures learnings scored on specificity, actionability, novelty, context, and confidence. Freshness decay ensures recent insights outweigh stale patterns. The flywheel compounds when retrieval rate x usage rate exceeds decay rate.
- **Hands-Free Execution** (loop closure) — `/evolve` and `/crank` spawn agents that work toward goals autonomously. Cycle state is disk-based (survives context compaction), regression gates are hard-gated, and every cycle writes a verifiable audit trail. Completed work produces better next work automatically.
- **Compose What You Need** (all three gaps) — Skills are standalone building blocks. Use one (`/council validate this PR`), chain several (`/plan` -> `/pre-mortem` -> `/crank`), or run the full pipeline (`/rpi`). The same recursive shape — lead decomposes, workers execute, gates lock — repeats at every scale.
- **Multi-Runtime, Multi-Model** (judgment validation) — Same skills work across Claude Code, Codex CLI, Cursor, and OpenCode. `/converter` exports to native formats. Mixed-vendor council judges (Claude + Codex) provide independent perspectives.
- **Zero Setup, Zero Telemetry** (all three gaps) — All state lives in git-tracked `.agents/` directories with no cloud dependency. 63 skills, 3 hooks, and the knowledge flywheel work independently with no external daemon. Install is one command per runtime.

## Design Principles

**Theoretical foundation — four pillars:**

1. **[Systems theory (Meadows)](https://en.wikipedia.org/wiki/Twelve_leverage_points)** — Target the high-leverage end of the hierarchy: information flows (#6), rules (#5), self-organization (#4), goals (#3). Changing the loop beats tuning the output.
2. **[DevOps (Three Ways)](docs/the-science.md#part-3-devops-foundation-the-three-ways)** — Flow, feedback, continual learning — applied to the agent loop instead of the deploy pipeline.
3. **[Brownian Ratchet](docs/brownian-ratchet.md)** — Embrace agent variance, filter aggressively, ratchet successes. Chaos + filter + one-way gate = net forward progress.
4. **[Knowledge Flywheel (escape velocity)](docs/the-science.md#the-escape-velocity-condition)** — If retrieval rate x usage rate exceeds decay rate, knowledge compounds. If not, it decays to zero. The flywheel exists to stay above that threshold.

**Operational principles:**

1. **Context quality determines output quality.** Every skill, hook, and flywheel component exists to ensure the right context is in the right window at the right time.
2. **Least-privilege loading.** Agents receive only the context necessary for their task — phase-specific, role-scoped, freshness-weighted.
3. **The cycle is the product.** No single skill is the value. The compounding loop — research, plan, validate, build, validate, learn, repeat — is what makes the system improve.
4. **Two-tier execution.** Orchestrators (`/evolve`, `/rpi`, `/crank`) stay in the main session. Workers fork into subagents where results merge back via the filesystem — never accumulated chat context.
5. **Dormancy is last resort.** When goals pass and backlog is empty, the system generates productive work from validation gaps, bug hunts, drift detection, and feature suggestions before going dormant.

## Usage

This file enables product-aware council reviews:

- **`/pre-mortem`** — Automatically includes `product` perspectives (user-value, adoption-barriers, competitive-position) alongside plan-review judges when this file exists.
- **`/vibe`** — Automatically includes `developer-experience` perspectives (api-clarity, error-experience, discoverability) alongside code-review judges when this file exists.
- **`/council --preset=product`** — Run product review on demand.
- **`/council --preset=developer-experience`** — Run DX review on demand.

Explicit `--preset` overrides from the user skip auto-include (user intent takes precedence).

## See Also

- [Context Lifecycle Contract](docs/context-lifecycle.md) — canonical definition of the three gaps (judgment validation, durable learning, loop closure) with evidence map and mechanism inventory.
- [Scale Without Swarms](docs/scale-without-swarms.md) — why 3-5 focused agents with fresh context and regression gates outperform massive uncoordinated swarms; the AgentOps model of waves, isolation, and gates explained.
