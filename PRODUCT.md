---
last_reviewed: 2026-05-06
---

# PRODUCT.md

## Mission

AgentOps is a context compiler + validation harness + cross-runtime skill packaging for coding agents, with a schedulable always-on dream/evolution daemon. It assembles, validates, and delivers **the right context at the right time** so every session reads from **the corpus** on the way in and writes back on the way out — typed, versioned, validated, decay-ranked. Your agent's context becomes an engineering artifact, not chat history. Vendor memory follows the chat. The corpus follows the team.

The corpus is the descendant of the wiki (Ward Cunningham, 1995), the runbook, the postmortem, the toil budget — every prior generation's codified team knowledge, made agent-readable and maintained like code. Same lineage. New substrate.

Compounding is something the user **schedules**, not something the system magic-claims. `ao schedule` + `ao daemon` run dream, evolve, compile, defrag, forge, and feedback-drain on whatever cadence the user sets — that's the always-on lane. The three-gap model (judgment validation, durable learning, loop closure) names the failure modes; today **Gap 1 is mechanically enforced** via hooks, static analysis, and CI gates. **Gaps 2 and 3 are roadmap** — the gates are declared, the substrate is maturing, and the empirical proof artifacts (eval workbench, MemRL outcome-grounded reward, flywheel-proof) are tracked work, not shipped claims.

> Canonical contract: [docs/context-lifecycle.md](docs/context-lifecycle.md)

## Vision

The software factory that gets better with each use. Every session produces code, lessons, and stronger constraints — the next session starts with more knowledge, tighter gates, and less wasted work. The model stays the same. The corpus compounds.

The thesis is simple: indeterministic workers need disciplined systems. DevOps proved this for engineers. SRE proved it again with SLOs and error budgets. Kubernetes proved it for declarative infrastructure with control loops that reconcile actual state to desired state. Coding agents are the next indeterministic worker class. Same playbook. New substrate. The asset that survives — yours, not ours — is the corpus the system compounds on your behalf.

## Market Convergence

The April 2026 Claude Code source analysis confirmed that Anthropic's internal tooling follows the same architecture AgentOps implements:

| Anthropic Concept | AgentOps Equivalent | Status |
|---|---|---|
| **Learning Loop** — memory extraction, dream cycle consolidation, future session injection | Knowledge Flywheel — `/retro` → `/forge` → `/harvest` → `ao inject`, tiered promotion (learning → pattern → rule), plus private local Dream via `/dream` and `ao overnight` | Shipped. On-demand capture/promotion is live, and Dream now provides the bounded private overnight compounding lane. GitHub nightly is the public proof harness for the contracts, not the user's private runtime. |
| **Skillify** — AI watches patterns, packages them as reusable skills, compound growth | Skills system — 73 skills, `/heal-skill` audit, `/converter` cross-runtime export, SKILL-TIERS classification | Prototype built. `ao flywheel close-loop` now drafts review-only skills from repeated patterns; promotion polish is the remaining gap. |
| **Verification Agent** — adversarial AI auditing AI, VERDICT system for human review | Council architecture — `/council`, `/pre-mortem`, `/vibe`, `/post-mortem` with multi-model consensus, prediction tracking. Stage 4 behavioral validation adds holdout scenarios + satisfaction scoring in STEP 1.8. | Shipped. On-demand + always-on (STEP 1.8 fires automatically during `/validation`). |
| **Managed Agents Dreaming** (May 2026) — scheduled session review, pattern extraction, memory curation between sessions | `/dream` + `ao overnight` + `cli/cmd/ao/dream_executor.go` + `.github/workflows/nightly.yml` dream-cycle proof job | Shipped. Bounded private overnight compounding lane runs the harvest → forge → close-loop → defrag chain unattended, off the API and against any model. |
| **Managed Agents Outcomes** (May 2026) — rubric-driven separate-context grader with iterate-until-pass | Shipped at three scopes: project — `GOALS.md` (rubric) + `ao goals measure` (each gate runs as separate subprocess; `cli/internal/goals/measure.go:132-164`) + `/evolve` (iterates worst-failing gate until pass; `skills/evolve/SKILL.md:379-388`); plan — `/pre-mortem` council judges as separate-context graders; code — `/vibe` council judges. Independent 3-judge audit confirmed parity on rubric authoring, separate-context grading, iterate-until-pass, and pinpoint-what-changed. | Shipped at the capability layer. Empirical workbench A/B (2026-05-06): Δ=+0.0000 across 12 cases at v1 difficulty (both legs 12/12) — task difficulty floor exhausted; v2 substrate (realistic agent tasks where the hook layer differentiates) is roadmap. Counter-stat artifact: `evals/workbench/results/2026-05-06-yjzp9-counterstat.json`. |

Read the convergence table the right way: AgentOps and every harness like it gets absorbed into the model layer over time — Anthropic's 2026-05-06 Managed Agents launch is the textbook example. Memory primitives, learning loops, even validation gates — frontier vendors will ship them natively. What stays yours is the corpus. AgentOps is the bridge tool that helps you build the moat *now*, with current models, before the harness layer commoditizes.

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

Three layers. Each solves a different problem. All three compound through the CDLC — the [Context Development Life Cycle](docs/cdlc.md).

### Layer 1: The Context Compiler

**Problem:** Every agent session starts from zero. No memory of what worked, what failed, or what the codebase expects.

**What it does:** Assembles the right context for the right phase. Research gets different context than implementation. Scores knowledge by utility and freshness. Trims to the token budget. Delivers at session start automatically.

- `ao inject` — decay-ranked retrieval with token budgeting
- `ao context assemble` — phase-scoped context packets
- `ao compile` — rebuild the knowledge wiki (mine, grow, defrag, lint)
- 73 skills — reusable context packages across Claude Code, Codex, and OpenCode
- 12 lifecycle hooks — context loads automatically without agent initiative
- `bash <(curl -fsSL .../install.sh)` — 30 seconds, zero config

### Layer 2: The Validation Gates

**Problem:** Agents ship confident garbage. No review, no second opinion, no gate between "agent thinks this is good" and "this goes into production."

**What it does:** Multi-model consensus validates plans before build and code before commit. Gates block, not advise. Independent judges debate and return one auditable verdict.

- `/pre-mortem` — validate plans before implementation
- `/vibe` — validate code after implementation
- `/council` — multi-model adversarial review (Claude + Codex judges)
- 63 eval suites + 12-task workbench — deterministic context quality testing
- Baseline A/B — skill-on vs skill-off delta measurement

### Layer 3: The Knowledge Flywheel

**Problem:** Each session ends and the lessons disappear. Same mistakes get made. Same solutions get rediscovered. Nothing compounds.

**What it does:** Every session extracts learnings. Learnings get scored on specificity, actionability, novelty. High-scoring learnings promote to permanent patterns. Patterns become planning rules. Next session starts loaded. The flywheel runs overnight unattended.

- `/forge` — extract structured learnings from completed sessions
- `ao flywheel close-loop` — score, promote, curate automatically
- `/evolve` — autonomous reconciliation: reads goals, fixes the worst gap, validates, repeats
- `/dream` — overnight compounding: full extract→score→promote→inject cycle unattended
- MemRL feedback — cited artifacts receive session reward, utility scores update
- 1,400+ learnings, 130+ patterns — the corpus is compounding

### How They Compound

```
Session starts → Layer 1 delivers compiled context → Agent works →
Layer 2 validates the output → Session ends → Layer 3 extracts learnings →
Next session starts with better context (back to Layer 1)
```

That's the CDLC. Generate, compile, test, distribute, deliver, observe, adapt. Same shape as the DevOps SDLC. Different substrate. The model stays the same. The corpus compounds.

### Infrastructure (underneath all three layers)

- **Coordination plane** — `/swarm`, `/crank`, waves, worktree isolation for parallel agents. Scale by adding workers, not overloading context.
- **Temporal compounding** — dream cycles (hours), session forge (minutes), pattern promotion (weeks). Multiple clocks, one flywheel.
- **Multi-runtime** — same skills, same corpus across Claude Code, Codex CLI, and OpenCode. `/converter` exports to Cursor rules. The discipline lives in the system, not the model.
- **Zero telemetry** — all state lives in local `.agents/` directories. No cloud dependency.

## Strategic Bet

Knowledge is the moat. AgentOps isn't. Every harness — ours included — gets absorbed into the model. Memory primitives, learning loops, validation gates: frontier vendors will ship them natively. What they won't ship is *your* corpus — what your repo learned, what your team scarred, what your codebase decided. AgentOps is the bridge tool: skills, hooks, and a CLI that smooth the sharp edges of current models so you produce reliable output today and build the moat that stays.

## Evidence

As of 2026-05-04:

**Traction:**

- GitHub repo: 328 stars, 34 forks, 8 open issues, last pushed 2026-05-04
- Public surface: GitHub Pages mkdocs site live at boshu2.github.io/agentops/; doctrine site live at 12factoragentops.com
- Distribution/runtime reach: 73 shared skills, 73 checked-in Codex artifacts, and 35 Codex overrides

**Measured operational proof:**

- `ao doctor --json`: hook coverage and structural gates passing
- Competitive freshness gate: comparison docs maintained within the 45-day target
- External validation: independent 3-judge audit (council, 2026-05-06) confirmed parity with Anthropic Managed Agents on rubric authoring, separate-context grading, and iterate-until-pass
- Empirical workbench A/B (2026-05-06, 12 cases): Δ=+0.0000 — both `skill-on` and `skill-off` legs scored 12/12. Honest read: at workbench v1 difficulty (off-by-one bugs, simple validators, basic SQLi) AgentOps's hook layer is non-differentiating because the tasks don't require it. Substrate v2 (realistic agent task difficulty) is roadmap. Source: `evals/workbench/results/2026-05-06-yjzp9-counterstat.json`

Your corpus grows every session — learnings, patterns, and constraints accumulate in your repo, not ours. The system writes the substrate; you decide on what cadence the dream/evolution/compile loops run via `ao schedule`. Scale and compounding follow from the schedule you set, not from a claim we make.

## Known Product Gaps

| Gap | Impact | Status |
|-----|--------|--------|
| Dream autonomy is still maturing | The private local Dream lane runs through `/dream` and `ao overnight`, with bounded compounding, reports, setup guidance, and a separate GitHub nightly proof harness. Remaining work is deeper full-loop autonomy, calibration, and onboarding polish. | in-progress |
| Pattern-to-skill pipeline (synthesis layer) | Detection layer ships in v1 (`.agents/plans/2026-04-23-pattern-to-skill-pipeline-detection.md`); synthesis (LLM-authored draft skill bodies, tier heuristics, on-disk drafts in `.agents/skill-drafts/`) is deferred to v2 after a design council found 8+ blockers. The "self-programming compounding" framing is aspirational, not currently producing on-disk output. | deferred |
| Multi-runtime proof is tiered, not complete | Tier S structural proof is active for all four runtimes. Tier I live inventory proof is partial. Tier E live execution proof remains opt-in / nightly, not a default gate. | in-progress |
| Retrieval and worker knowledge propagation still limit compounding | The flywheel architecture is in place. Retrieval quality and passing prevention/finding context to implement workers remain weaker than the core thesis requires. | open |
| Behavioral eval system needs live agent runtime at scale | Eval workbench shipped: 3 fixture components (Go CLI, Python FastAPI, DevOps), 12 tasks with golden solutions and scoring scripts, behavioral eval suite, agent harness script, eval-skill-delta CI gate, and `--two-pass` head gate. Scoring infrastructure verified (golden 12/12, broken detection 12/12). A/B DeltaScorecard works for deterministic cases. Remaining gap: live agent runtime execution at scale — the harness and gates exist but full skill-on vs skill-off delta across the workbench is not yet a default gate. | in-progress |
| Public messaging shifted to context-compiler + moat framing | CDLC (Context Development Life Cycle) framing landed: Mission, Strategic Bet, README, and mkdocs hero surfaces now use "context compiler" as the primary identity noun. Remaining gap: downstream comparison docs and skill-page intros still need a sweep to match. | in-progress |

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
