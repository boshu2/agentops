---
last_reviewed: 2026-04-30
---

# PRODUCT.md

## Mission

AgentOps is operational discipline for coding agents. The hard problem: ship reliable code with unreliable agents that don't remember anything. Build the knowledge and memory into the system AND the process — a Meadows compounding system. The moat is the context you, your team, and your business have earned: every landmine, every decision, every scar. Atomic changes. Validation gates. Compounding context. Every session writes a learning. Every learning sharpens the next.

> Canonical contract: [docs/context-lifecycle.md](docs/context-lifecycle.md)

## Vision

The software factory that gets better with each use. Every session produces code, lessons, and stronger constraints — the next session starts with more knowledge, tighter gates, and less wasted work. The model stays the same. The corpus compounds.

The thesis is simple: indeterministic workers need disciplined systems. DevOps proved this for engineers. SRE proved it again with SLOs and error budgets. Kubernetes proved it for declarative infrastructure with control loops that reconcile actual state to desired state. Coding agents are the next indeterministic worker class. Same playbook. New substrate. The asset that survives — yours, not ours — is the corpus the system compounds on your behalf.

## Market Convergence

The April 2026 Claude Code source analysis confirmed that Anthropic's internal tooling follows the same architecture AgentOps implements:

| Anthropic Concept | AgentOps Equivalent | Status |
|---|---|---|
| **Learning Loop** — memory extraction, dream cycle consolidation, future session injection | Knowledge Flywheel — `/retro` → `/forge` → `/harvest` → `ao inject`, tiered promotion (learning → pattern → rule), plus private local Dream via `/dream` and `ao overnight` | Shipped. On-demand capture/promotion is live, and Dream now provides the bounded private overnight compounding lane. GitHub nightly is the public proof harness for the contracts, not the user's private runtime. |
| **Skillify** — AI watches patterns, packages them as reusable skills, compound growth | Skills system — 69 skills, `/heal-skill` audit, `/converter` cross-runtime export, SKILL-TIERS classification | Prototype built. `ao flywheel close-loop` now drafts review-only skills from repeated patterns; promotion polish is the remaining gap. |
| **Verification Agent** — adversarial AI auditing AI, VERDICT system for human review | Council architecture — `/council`, `/pre-mortem`, `/vibe`, `/post-mortem` with multi-model consensus, prediction tracking. Stage 4 behavioral validation adds holdout scenarios + satisfaction scoring in STEP 1.8. | Shipped. On-demand + always-on (STEP 1.8 fires automatically during `/validation`). |

Read the convergence table the right way: AgentOps and every harness like it gets absorbed into the model layer over time. Memory primitives, learning loops, even validation gates — frontier vendors will ship them natively. What stays yours is the corpus. AgentOps is the bridge tool that helps you build the moat *now*, with current models, before the harness layer commoditizes.

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

The bridge tool has three layers. Each smooths a sharp edge of current models so you can build the moat (the corpus) underneath.

### 1. Skills (69 skills across 4 runtimes)

**The discipline layer.**

Markdown-defined primitives and flows that agents load and execute. Atomic, composable, scoped. Engineers recognize the shape: small reviewable units with explicit phase boundaries.

- **Validation primitives** — `/pre-mortem`, `/vibe`, `/council`, `/review`. Multi-model consensus validates plans before build and code before commit. Gates block, not advise.
- **Bookkeeping primitives** — `/retro`, `/forge`, `/inject`, `/flywheel`, `/compile`. Extract, score, curate, and retrieve learnings so solved problems stay solved. The flywheel runs here.
- **Flows** — `/research`, `/implement`, `/validation`, `/rpi`, `/crank`, `/evolve`. Compose primitives into auditable phases. Drop in at any phase. No phase compresses into another.

Skills work across Claude Code, Codex CLI, Cursor, and OpenCode through explicit proof tiers. Tier S structural/install proof is active for all four runtimes; Tier I live inventory proof exists for Claude Code and Codex when local CLIs/auth are available; Tier E live execution proof remains opt-in rather than a default CI gate. Codex-native skills ship alongside Claude-native, and `/converter` exports Cursor rules.

### 2. CLI (`ao`)

**The reliability + autonomy layer.**

A Go binary that provides the repo-native infrastructure skills depend on. Declarative goals, fitness gates, control loops that reconcile.

- **Bookkeeping control plane** — `ao inject`, `ao lookup`, `ao forge`, `ao curate`, `ao defrag`, `ao memory sync` manage learning capture, retrieval, freshness decay, promotion. The flywheel runs here.
- **Goals + reconciliation** — `ao goals measure` runs SLO-shaped fitness gates; `ao goals steer` manages directives; `ao evolve` runs the autonomous reconcile loop that closes the worst fitness gap. SRE error-budget logic, applied to a codebase.
- **Operator surfaces** — `ao context assemble`, `ao rpi`, `ao factory` build phase-appropriate packets and terminal-native flows. Stay in the loop, run on the loop, or drop out entirely. Same machine.

### 3. Hooks

**The always-on layer.**

Session lifecycle hooks that run automatically so the operational layer stays active without agent initiative. The discipline that fires whether the operator remembered or not.

- **SessionStart / SessionEnd / Stop** — stage runtime state, maintain, and close the bookkeeping loop between sessions.
- **PreToolUse / PostToolUse** — nudge toward the right primitives and enforce validation constraints.
- **UserPromptSubmit** — route intent, surface startup guidance, keep the operator on a productive path.

## Core Value Propositions

The three load-bearing claims, expanded:

- **Atomic changes** — every primitive is small enough to be cheap to undo. `/implement` is one scoped task. `/council` is one verdict. `/forge` extracts one learning at a time. Compose them; the work stays auditable end to end.
- **Validation gates** — multi-model consensus (Claude + Codex judges debate independently) validates plans before build and code before commit. Gates block, not advise. The three-gap proof contract — judgment, durable learning, loop closure — defines what reliability means here.
- **Compounding context** — the knowledge flywheel. Each session captures learnings scored on specificity, actionability, novelty, context, and confidence. Learnings promote to patterns; patterns become planning rules. Next session starts loaded, not cold. Escape velocity is a measurable condition: retrieval × usage > decay.
- **Hands-free reconciliation** — `/evolve` reads `GOALS.md`, picks the worst fitness gap, fixes it, validates, records the cycle. SRE error budgets meet Kubernetes control loops. `/dream` runs overnight bookkeeping; source code stays untouched.
- **Multi-runtime, multi-model** — same skills target Claude Code, Codex CLI, Cursor, and OpenCode with documented Tier S/I/E proof levels. `/converter` exports to native formats. Mixed-vendor council judges provide independent perspectives — the discipline lives in the system, not the model.
- **Zero setup, zero telemetry** — all state lives in local `.agents/` directories with no cloud dependency. 69 skills, 12 runtime hook event sections, and the flywheel can operate with no external daemon.

## Strategic Bet

Knowledge is the moat. AgentOps isn't. Every harness — ours included — gets absorbed into the model. Memory primitives, learning loops, validation gates: frontier vendors will ship them natively. What they won't ship is *your* corpus — what your repo learned, what your team scarred, what your codebase decided. AgentOps is the bridge tool: skills, hooks, and a CLI that smooth the sharp edges of current models so you produce reliable output today and build the moat that stays.

## Evidence

As of 2026-04-30:

**Traction:**

- GitHub repo: 320 stars, 34 forks, 10 open issues, last pushed 2026-04-30
- Public surface: GitHub Pages mkdocs site live at boshu2.github.io/agentops/; doctrine site live at 12factoragentops.com
- Distribution/runtime reach: 69 shared skills, 69 checked-in Codex artifacts, and 35 Codex overrides

**Measured operational proof:**

- Knowledge corpus: 4,940 learnings, 1,195 patterns, 40 planning rules — the flywheel is producing
- `ao doctor --json`: hook coverage and structural gates passing
- Competitive freshness gate: comparison docs maintained within the 45-day target

The flywheel numbers (4,940 learnings, 1,195 patterns) are the load-bearing evidence: extracted, scored, promoted artifacts are accumulating at a rate that exceeds decay. The corpus is compounding, not just claiming to. Note: this is the maintainer's corpus. The product's job is to help every user start building their own.

## Known Product Gaps

| Gap | Impact | Status |
|-----|--------|--------|
| Dream autonomy is still maturing | The private local Dream lane runs through `/dream` and `ao overnight`, with bounded compounding, reports, setup guidance, and a separate GitHub nightly proof harness. Remaining work is deeper full-loop autonomy, calibration, and onboarding polish. | in-progress |
| Pattern-to-skill promotion polish remains | The strongest differentiation thesis — self-programming compounding — has review-only draft generation today. Remaining gap: richer synthesis and a clean publish path. | in-progress |
| Multi-runtime proof is tiered, not complete | Tier S structural proof is active for all four runtimes. Tier I live inventory proof is partial. Tier E live execution proof remains opt-in / nightly, not a default gate. | in-progress |
| Retrieval and worker knowledge propagation still limit compounding | The flywheel architecture is in place. Retrieval quality and passing prevention/finding context to implement workers remain weaker than the core thesis requires. | open |
| Public messaging now converged on operational-discipline + moat framing | A 2026-04-30 internal positioning council locked the thesis: *knowledge is the moat; AgentOps is the bridge tool that helps you build it.* Mission, Strategic Bet, README, and mkdocs surfaces aligned in PR #192. Downstream comparison docs and skill-page intros still need a sweep. | in-progress |

## Design Principles

**Theoretical foundation — six pillars:**

1. **[Systems theory (Meadows)](https://en.wikipedia.org/wiki/Twelve_leverage_points)** — Target the high-leverage end of the hierarchy: information flows (#6), rules (#5), self-organization (#4), goals (#3). Changing the loop beats tuning the output. AgentOps is built as a Meadows compounding system around the user's codebase: information flows captured, rules encoded, self-organization through the flywheel, goals declared.
2. **[DevOps Three Ways](docs/the-science.md#part-3-devops-foundation-the-three-ways)** — Flow, feedback, continual learning. The discipline lineage. Applied to the agent loop instead of the deploy pipeline.
3. **SRE (SLOs + error budgets)** — Reliability is a measurable condition, not a vibe. `GOALS.md` carries SLO-shaped fitness gates; `ao goals measure` is the burn-rate equivalent. The reliability lineage. Source: *Site Reliability Engineering* (Beyer, Jones, Petoff, Murphy).
4. **Kubernetes control loops** — Declared state + reconcile loop. `GOALS.md` declares; `/evolve` reconciles. Errors don't crash the loop; they enter the work queue. The self-correction lineage.
5. **[Brownian Ratchet](docs/brownian-ratchet.md)** — Embrace agent variance, filter aggressively, ratchet successes. Chaos + filter + one-way gate = net forward progress. The forward-only-progress lineage.
6. **[Knowledge Flywheel (escape velocity)](docs/the-science.md#the-escape-velocity-condition)** — If retrieval rate × usage rate exceeds decay rate, knowledge compounds. If not, it decays to zero. The compounding-context lineage. *This is the one infrastructure never needed* — software workers persist; agents don't. The corpus is the moat.

**Operational principles:**

1. **Agents are ephemeral; the system carries the state.** Every skill, hook, and flywheel component exists because the agent itself can't remember. Build for amnesia.
2. **The corpus is the user's. The harness is ours.** AgentOps' own commoditization is on the timeline. The user's accumulated knowledge isn't. Optimize the product for what the user keeps.
3. **Context quality determines output quality.** Right context, right window, right time. Phase-specific. Role-scoped. Freshness-weighted.
4. **The cycle is the product.** No single skill is the value. The compounding loop — research, plan, validate, build, validate, learn, repeat — is what makes the system improve.
5. **Two-tier execution.** Orchestrators (`/evolve`, `/rpi`, `/crank`) stay in the main session. Workers fork into subagents where results merge back via the filesystem — never accumulated chat context.
6. **Atomic changes compose.** Every primitive is cheap to undo. The Brownian Ratchet only works if the ratchet step is small.
7. **Reconcile, don't push.** Kubernetes-shaped control loops compare actual state to desired state and fix the gap. They don't fire-and-forget. AgentOps loops do the same.
8. **Dormancy is last resort.** When goals pass and backlog is empty, the system generates productive work from validation gaps, bug hunts, drift detection, and feature suggestions before going dormant.

## Usage

This file enables product-aware council reviews:

- **`/pre-mortem`** — Automatically loads product context when this file exists. Default `--quick` mode includes the context inline; deeper modes add a dedicated `product` perspective alongside plan-review judges.
- **`/vibe`** — Automatically loads developer-experience context when this file exists. Default `--quick` mode includes the context inline; deeper modes add a dedicated `developer-experience` perspective alongside code-review judges.
- **`/council --preset=product`** — Run product review on demand.
- **`/council --preset=developer-experience`** — Run DX review on demand.

Explicit `--preset` overrides from the user skip auto-include (user intent takes precedence).

## See Also

- [Context Lifecycle Contract](docs/context-lifecycle.md) — canonical definition of the three gaps (judgment validation, durable learning, loop closure) with evidence map and mechanism inventory.
- [Scale Without Swarms](docs/scale-without-swarms.md) — why 3-5 focused agents with fresh context and regression gates outperform massive uncoordinated swarms; the AgentOps model of waves, isolation, and gates explained.
- [Brownian Ratchet](docs/brownian-ratchet.md) — the forward-only-progress lineage in detail.
- [The Science](docs/the-science.md) — DevOps Three Ways, the escape velocity condition, and the leverage-points map.
