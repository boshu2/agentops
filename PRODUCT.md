---
last_reviewed: 2026-04-09
---

# PRODUCT.md

## Mission

AgentOps is the operational layer for coding agents. Publicly, it gives agents **bookkeeping**, **validation**, **primitives**, and **flows** so every session starts where the last one left off. Technically, it uses a context-compiler architecture: raw session signal becomes reusable knowledge, compiled prevention, and better next work.

The older three-gap model remains the internal proof contract: judgment validation, durable learning, and loop closure are how we verify that the product actually compounds.

> Canonical contract: [docs/context-lifecycle.md](docs/context-lifecycle.md)

## Vision

The software factory that gets better with each use. Every session produces code, lessons, and stronger constraints — so the next session starts with more knowledge, tighter gates, and less wasted work. The model stays the same; the environment around it compounds.

This is the direction the industry is converging on. Anthropic's internal Claude Code architecture validates the same three primitives AgentOps shipped months earlier: a learning loop (memory extraction → off-session consolidation → future injection), self-programming skills (pattern extraction into reusable capabilities), and adversarial verification (independent agents auditing other agents' output). AgentOps is already there — the work now is deepening the flywheel and making it autonomous.

## Market Convergence

The April 2026 Claude Code source analysis confirmed that Anthropic's internal tooling follows the same architecture AgentOps implements:

| Anthropic Concept | AgentOps Equivalent | Status |
|---|---|---|
| **Learning Loop** — memory extraction, dream cycle consolidation, future session injection | Knowledge Flywheel — `/retro` → `/forge` → `/harvest` → `ao inject`, tiered promotion (learning → pattern → rule) | Shipped. On-demand today; dream cycle (automated nightly consolidation) is the next step. |
| **Skillify** — AI watches patterns, packages them as reusable skills, compound growth | Skills system — 68 skills, `/heal-skill` audit, `/converter` cross-runtime export, SKILL-TIERS classification | Shipped. Manual authoring today; pattern-to-skill pipeline is the next step. |
| **Verification Agent** — adversarial AI auditing AI, VERDICT system for human review | Council architecture — `/council`, `/pre-mortem`, `/vibe`, `/post-mortem` with multi-model consensus, prediction tracking. Stage 4 behavioral validation adds holdout scenarios + satisfaction scoring in STEP 1.8. | Shipped. On-demand + always-on (STEP 1.8 fires automatically during `/validation`). |

The gap between "architecture exists for compound growth" (what others describe) and "compound growth is actually happening" (what AgentOps delivers with harvest/forge/evolve) is the moat.

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

AgentOps has three layers:

### 1. Skills (68 skills across 4 runtimes)

Markdown-defined primitives and flows that agents load and execute:

- **Validation primitives** — `/pre-mortem`, `/vibe`, `/council`, `/review`. Multi-model consensus validates plans before build and code before commit.
- **Bookkeeping primitives** — `/retro`, `/forge`, `/inject`, `/flywheel`, `/compile`. Extract, score, curate, and retrieve learnings so solved problems stay solved.
- **Flows** — `/research`, `/implement`, `/validation`, `/rpi`, `/crank`, `/evolve`. Compose single actions into repeatable paths that can run manually or end to end.

Skills work across Claude Code, Codex CLI, Cursor, and OpenCode. Each runtime has native format support (`/converter` exports between them). Codex-native skills ship alongside Claude-native.

### 2. CLI (`ao`)

A Go binary that provides the repo-native infrastructure skills depend on:

- **Bookkeeping control plane** — `ao inject`, `ao lookup`, `ao forge`, `ao curate`, `ao defrag`, and `ao memory sync` manage learning capture, retrieval, freshness decay, and promotion.
- **Goal and issue orchestration** — `ao goals measure` runs fitness gates, `ao goals steer` manages directives, and `bd` provides git-native issue tracking with dependency graphs and epic management.
- **Context assembly and operator surfaces** — `ao context assemble`, `ao rpi`, and `ao factory` build phase-appropriate packets and terminal-native flows.

### 3. Hooks

Session lifecycle hooks that run automatically so the operational layer stays active without agent initiative:

- **SessionStart / SessionEnd / Stop** — inject, maintain, and close the bookkeeping loop between sessions.
- **PreToolUse / PostToolUse** — nudge toward the right primitives and enforce validation constraints.
- **UserPromptSubmit** — route intent, surface startup guidance, and keep the operator on a productive path.

## Core Value Propositions

The public value proposition should now map to the category we are actually selling:

- **Bookkeeping That Compounds** — Agent knowledge is managed like code: version-controlled, reviewed, promoted, and decayed instead of trapped in ephemeral chat history or a proprietary store. Each session captures learnings scored on specificity, actionability, novelty, context, and confidence. Learnings promote to patterns; patterns become planning rules.
- **Validation Before Ship** — Multi-model consensus (Claude + Codex judges debate independently) validates plans before build and code before commit. Validation gates block, not advise.
- **Primitives + Flows** — Skills are standalone building blocks. Use one (`/council validate this PR`), compose several (`/research` → `/plan` → `/council validate`), or run the full lifecycle (`/rpi`). The same recursive shape repeats at every scale.
- **Hands-Free Execution** — `/evolve` and `/crank` spawn agents that work toward goals autonomously. Cycle state is disk-based, regression gates are hard-gated, and every cycle writes a verifiable audit trail.
- **Multi-Runtime, Multi-Model** — Same skills work across Claude Code, Codex CLI, Cursor, and OpenCode. `/converter` exports to native formats. Mixed-vendor council judges provide independent perspectives.
- **Zero Setup, Zero Telemetry** — All state lives in local `.agents/` directories (git-ignored by default; opt in to commit with `AGENTOPS_GITIGNORE_AUTO=0`) with no cloud dependency. 69 skills, 3 hooks, and the flywheel can operate with no external daemon.

## Strategic Bet

AgentOps bets that the durable advantage in AI coding will come from compounding context between sessions, not from packing more prompts, more agents, or more context into a single session. The winning layer is the bookkeeping/context-compiler layer: raw session signal becomes reusable learnings, compiled prevention, and better next work.

## Evidence

As of 2026-04-09:

**Traction:**

- GitHub repo: 265 stars, 24 forks, 2 open issues, last pushed 2026-04-08
- Public surface: GitHub Pages comparison site and search metadata are live
- Distribution/runtime reach: 66 shared skills, 66 checked-in Codex artifacts, and 35 Codex overrides

**Measured operational proof:**

- Knowledge corpus: 163 learnings, 13 planning rules, 12 patterns
- `ao doctor --json`: 10 of 12 checks passing, with full 7/7 hook coverage
- Competitive freshness gate passing: all 5 comparison docs are within the 45-day target

## Known Product Gaps

| Gap | Impact | Status |
|-----|--------|--------|
| Full dream-cycle automation is incomplete | The product promise says compounding should happen between sessions, but nightly automation does not yet run the full harvest -> forge -> inject -> report loop. | open |
| Pattern-to-skill pipeline is not built | The strongest differentiation thesis, self-programming compounding, is still manual at the last mile. | open |
| Multi-runtime proof is still partial | README and PRODUCT promise parity across Claude Code, Codex, Cursor, and OpenCode, but live runtime verification and Codex parity still cost ongoing maintenance. | in-progress |
| Messaging is not yet fully unified | Public surfaces should now converge on "operational layer," while technical docs still need a clean split between the public category and the internal "context compiler" framing. | open |
| Retrieval and worker knowledge propagation still limit compounding | The flywheel architecture is in place, but retrieval quality and passing prevention/finding context to implement workers remain weaker than the core thesis requires. | open |

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
