---
last_reviewed: 2026-05-07
---

# PRODUCT.md

## Mission

**AgentOps automates the discipline of building a wiki for your agents.** The wiki is markdown in `.agents/` next to your code; the agents that use it also produce it. The compounded corpus is the moat.

<!-- agentops:claim:AOP-CLAIM-PRODUCT-CONTEXT-ARTIFACT -->
The highest-leverage input to coding agents is context: what the system knows, what it has tried, what failed, what the codebase decided, and what gates must hold. AgentOps automates the bookkeeping agents do not reliably do for themselves, then turns that record into an engineering artifact: typed, versioned, retrieved, validated, and fed back into the next run.

It encodes the **DevSecOps SDLC** as the **CDLC**, plus the operating practices of multi-agent work: isolated context per worker, stigmergic coordination through a shared corpus, planner/implementer/validator separation. The **RPI workflow** is the canonical instance — `/discovery` produces the planner artifact, `/crank` runs implementer agents in fresh-context waves, `/validation` runs validator agents that have not seen the code. The four layers — bookkeeping, context compilation, validation gates, and knowledge flywheel — are the public product model. Dream is the scheduled overnight mode of the flywheel.

One factory, three operator surfaces, four compounding layers: an **in-harness plugin** (skills for Claude Code, Codex, Cursor, OpenCode), the **`ao` CLI** (terminal/CI control plane), and a **scheduling daemon** (off-API, off-vendor, runs on your hardware against your subscription), with **hooks and gates** wiring policy into the runtime.

The bet is **sovereignty, not features**. Vendors will ship managed memory, councils, and dreaming natively — and lock them to their runtime. Your corpus stays in `.agents/` in your repo, runs on whichever harness you already pay for, and is portable across whichever frontier model wins next quarter. **The model gets smarter. The corpus stays yours.** Humans choose the posture: stay **in the loop** during discovery and validation, or sit **on the loop** while the daemon compounds overnight.

> Canonical contract: [docs/context-lifecycle.md](docs/context-lifecycle.md)
> Lineage: AgentOps positions explicitly against EveryInc's [Compound Engineer](https://github.com/EveryInc/compound-engineering-plugin) — see [docs/comparisons/vs-compound-engineer.md](docs/comparisons/vs-compound-engineer.md) for the in-depth contrast (operator-driven trunk vs. autonomy overlays, capture/scoring/injection, council validation).
> Internal lineage: the systems-theory work that preceded AgentOps lives in the [Lineage](#lineage) section, but users do not need that vocabulary to understand the product.

## Vision

The software factory that gets better with each use. Every session produces code, evidence, decisions, attempts, lessons, and stronger constraints — the next session starts with more knowledge, tighter gates, and less wasted work. The model stays the same. The corpus compounds.

<!-- agentops:claim:AOP-CLAIM-PRODUCT-FACTORY-GRADE-THROUGHPUT -->
The aspiration is factory-grade throughput for code: enough structure that agents can run against a defined process, with the operator setting cadence, rigor, and escalation boundaries. Same shape that turned software delivery into an engineering discipline — applied to coding agents.

The thesis is simple: indeterministic workers need disciplined systems. DevOps proved this for engineers. SRE proved it again with SLOs and error budgets. Kubernetes proved it for declarative infrastructure with control loops that reconcile actual state to desired state. Coding agents are the next indeterministic worker class. Same playbook. New substrate. The asset that survives — yours, not ours — is the corpus the system compounds on your behalf.

## What if the labs ship this natively?

They will. Anthropic's Managed Agents is the first move; others will follow. That's fine — the value isn't in this tool. It's in the corpus you build with it.

AgentOps is bridge infrastructure. Your `.agents/` directory is plain markdown in your repo. If a frontier vendor ships native equivalents in 12 months, your corpus carries forward. If we get acquired or change direction, your corpus is yours. If you outgrow the tool entirely, fork it, customize it, replace it — the corpus is what matters.

Open source forever. Built so you own the asset, not the tool.

## Market Convergence

The April 2026 Claude Code source analysis confirmed that Anthropic's internal tooling follows the same architecture AgentOps implements:

| Anthropic Concept | AgentOps Equivalent | Status |
|---|---|---|
| **Learning Loop** — memory extraction, dream cycle consolidation, future session injection | Knowledge Flywheel — `/retro` → `/forge` → `/harvest` → `ao inject`, tiered promotion (learning → pattern → rule), plus private local Dream via `/dream` and `ao overnight` | Shipped. On-demand capture/promotion is live, and Dream now provides the bounded private overnight compounding lane. GitHub nightly is the public proof harness for the contracts, not the user's private runtime. |
| **Skillify** — AI watches patterns, packages them as reusable skills, compound growth | Skills system — 75 skills, `/heal-skill` audit, `/converter` cross-runtime export, SKILL-TIERS classification | Prototype built. `ao flywheel close-loop` now drafts review-only skills from repeated patterns; promotion polish is the remaining gap. |
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
- **Gap exposure:** All failure modes — bookkeeping (evidence disappears), judgment validation (regressions slip through), durable learning (institutional knowledge lost), and loop closure (completed work doesn't feed back into constraints).

## What the Product Actually Is

AgentOps is a wiki for your agents. `.agents/` is markdown in your repo, version-controlled with your code, that agents read, traverse, and contribute to — the kind of wiki your team should already have, except agents do the maintenance. AgentOps automates the discipline of building one: capture, retrieval, validation, and compounding all happen mechanically so the wiki stays current instead of bitrotting.

That wiki is the substrate underneath a software factory with three surfaces and four user-facing layers: Bookkeeping, Context Compiler, Validation Gates, and Knowledge Flywheel. Dream is the scheduled overnight mode of the flywheel, not a separate peer product. The CDLC — the [Context Development Life Cycle](docs/cdlc.md) — is what the factory executes. The deeper proof-contract framing — identity, reproducibility, evaluation, evidence, recovery — lives in [docs/trust-factory.md](docs/trust-factory.md).

### Three surfaces

The same substrate, reached three ways:

- **In-harness plugin** — skills + hooks for Claude Code, Codex, Cursor, OpenCode. Context loads automatically inside the editor; the agent never has to ask. Install via `claude plugin install`, `install-codex.sh`, or the skills.sh package.
- **`ao` CLI** — the terminal and CI control plane. `ao inject`, `ao compile`, `ao goals measure`, `ao flywheel close-loop` — the same compiler, scriptable. Repo-native, with no required AgentOps cloud control plane.
- **Scheduling daemon** — off-API, off-vendor. `ao schedule` + `ao daemon` run dream, evolve, compile, defrag, forge, and feedback-drain on whatever cadence you set, against whichever local subscription you already pay for. The always-on lane.

### Four layers

The same model used in the README: bookkeeping records the work, the context compiler feeds the next run, validation gates enforce judgment, and the flywheel compounds the corpus.

#### Layer 1: Agent Bookkeeping

**Problem:** Agents do not keep their own operational memory. They forget what they tried, why they changed course, which warnings mattered, what passed validation, and what should be reused next time.

**What the compiler emits:** File-backed traces of agent work: attempts, decisions, citations, handoffs, findings, retros, post-mortems, council verdicts, and run packets. This is the raw material for context compilation and compounding.

- `.agents/` — repo-local bookkeeping substrate
- `/retro` and `/post-mortem` — capture decisions, lessons, and follow-up work
- `/provenance` and `/trace` — connect artifacts back to their source
- `ao metrics cite` and citation logs — record what knowledge was used
- RPI packets and council verdicts — preserve plan/build/validation evidence

#### Layer 2: Context Compiler

**Problem:** Every session starts from zero. Agents need the right slice of prior work, policy, constraints, and decisions before they can act well.

**What the compiler emits:** Decay-ranked, phase-scoped context packets built from the bookkeeping trail and curated corpus.

- `ao inject` — decay-ranked retrieval with token budgeting
- `ao context assemble` — phase-scoped context packets
- `ao compile` — rebuild the knowledge wiki (mine, grow, defrag, lint)
- 75 skills — reusable context packages across Claude Code, Codex, and OpenCode
- 12 lifecycle hooks — context loads automatically without agent initiative
- `bash <(curl -fsSL .../install.sh)` — 30 seconds, zero config

#### Layer 3: Validation Gates

**Problem:** Agents ship confident garbage. No review, no second opinion, no gate between "agent thinks this is good" and "this goes into production."

**What the compiler emits:** Multi-model consensus validates plans before build and code before commit. Gates block, not advise. Independent judges debate and return one auditable verdict.

- `/pre-mortem` — validate plans before implementation
- `/vibe` — validate code after implementation
- `/council` — multi-model adversarial review (Claude + Codex judges)
- 63 eval suites + 12-task workbench — deterministic context quality testing
- Baseline A/B — skill-on vs skill-off delta measurement

#### Layer 4: Knowledge Flywheel

**Problem:** Each session ends and the lessons disappear. Same mistakes get made. Same solutions get rediscovered. Nothing compounds.

**What the compiler emits:** Bookkeeping becomes reusable knowledge. Learnings get scored on specificity, actionability, novelty. High-scoring learnings promote to permanent patterns. Patterns become planning rules. Scheduled dream cycles defrag and compound the corpus without competing with foreground engineering. Next session starts loaded.

- `/forge` — extract structured learnings from completed sessions
- `ao flywheel close-loop` — score, promote, curate automatically
<!-- agentops:claim:AOP-CLAIM-PRODUCT-EVOLVE-RECONCILE -->
- `/evolve` — autonomous reconciliation: reads goals, fixes the worst gap, validates, repeats
- `/dream` and `ao overnight` — bounded private compounding lane
- `ao schedule` + `ao daemon` — operator-owned cadence for dream, evolve, compile, defrag, forge, and feedback-drain
- `.github/workflows/nightly.yml` — public proof harness for the contracts (not your private runtime)
- MemRL feedback — cited artifacts receive session reward, utility scores update
- 1,400+ learnings, 130+ patterns — the corpus is compounding

### How They Compound

```
Session starts → compiler delivers context → Agent works →
bookkeeping records attempts, decisions, citations, and evidence →
validation gates emit verdicts → Session ends → flywheel promotes learnings →
dream daemon defrags overnight → Next session starts with better context
```

That's the CDLC. Generate, compile, test, distribute, deliver, observe, adapt. Same shape as the DevOps SDLC. Different substrate. The model stays the same. The corpus compounds.

### Infrastructure (underneath all four layers)

- **Coordination plane** — `/swarm`, `/crank`, waves, worktree isolation for parallel agents. Scale by adding workers, not overloading context.
- **Temporal compounding** — dream cycles (hours), session forge (minutes), pattern promotion (weeks). Multiple clocks, one flywheel.
- **Multi-runtime** — same skills, same corpus across Claude Code, Codex CLI, and OpenCode. `/converter` exports to Cursor rules. The discipline lives in the system, not the model.
- **Local-first operation** — all AgentOps state lives in local `.agents/` directories. No required AgentOps product telemetry or hosted control plane; operators choose model runtimes, networks, installers, and remotes.

## Strategic Bet

The bet is **sovereignty, not features.** Every harness — ours included — gets absorbed into the model. Memory primitives, learning loops, validation gates: frontier vendors will ship them natively. What they won't ship is *your* corpus — what your repo learned, what your team scarred, what your codebase decided.

**When Anthropic ships native scheduling, councils, Skillify, and Dreaming inside Claude Code in the next 6 months, what specifically does AgentOps still do?** The honest answer:

1. **Cross-runtime corpus.** The corpus stays in `.agents/` in your repo. It runs the same way on Claude Code, Codex CLI, Cursor, and OpenCode. When the next frontier model wins next quarter — or when you switch teams or platforms — the corpus comes with you. Vendor-managed memory follows the chat session. AgentOps' corpus follows the team.
2. **Local sovereignty.** The scheduling daemon runs off-API, off-vendor, against your local subscription. Dream cycles, evolve loops, compile/defrag don't burn your vendor quota and don't ship your codebase to a cloud you don't control.
3. **Operator workflow encoding.** RPI, planner/implementer/validator separation, fresh-context worker waves, council validation — these are operating practices for multi-agent work, encoded as skills and hooks. Vendors will ship managed agents; they won't ship your operator workflow.

The model gets smarter. The corpus stays yours. See the [Lineage](#lineage) section for how this position was arrived at.

## Assurance Posture

AgentOps is engineered from constrained-environment habits, not consumer-app assumptions. This is not a certification claim; it is the operating posture the system is built toward.

Full profile: [docs/assurance-profile.md](docs/assurance-profile.md).

- **Local-first control.** The corpus lives in `.agents/` beside the code. AgentOps requires no product telemetry, and operators choose which model runtimes, networks, and subscriptions touch the repo.
- **Context as a boundary.** Research, planning, implementation, and validation receive different context. Workers get fresh windows. Validators get evidence packets instead of the implementer's accumulated chat.
- **Bookkeeping by default.** RPI packets, council verdicts, citations, ratchet records, post-mortems, handoffs, and schedule outputs leave file-backed traces that can be inspected, diffed, archived, or excluded from source control.
- **Policy gates over advice.** Hooks, pre-push checks, security scans, goal fitness gates, and pre-mortems encode process as executable constraints instead of relying on the agent to remember a runbook.
- **Variable autonomy.** The same factory can run interactive, supervised, scheduled, or unattended loops. High-risk environments can keep humans in the loop for planning, validation, release, and promotion while still using agents for bounded work.
- **Constrained-network fit.** The design favors repo-local state, explicit artifacts, no required cloud control plane, and operator-owned scheduling. Formal deployment into classified, export-controlled, or safety-critical environments still requires the local authority's security controls, model approvals, supply-chain process, and accreditation work.

## Evidence

As of 2026-05-04:

**Traction:**

- GitHub repo: 328 stars, 34 forks, 8 open issues, last pushed 2026-05-04
- Public surface: GitHub Pages mkdocs site live at boshu2.github.io/agentops/; doctrine site live at 12factoragentops.com
- Distribution/runtime reach: 75 shared skills, 74 checked-in Codex artifacts, and 35 Codex overrides

**Measured operational proof:**

- `ao doctor --json`: hook coverage and structural gates passing
- Competitive freshness gate: comparison docs maintained within the 45-day target
- External validation: independent 3-judge audit (council, 2026-05-06) confirmed parity with Anthropic Managed Agents on rubric authoring, separate-context grading, and iterate-until-pass
- Empirical workbench A/B (2026-05-06, 12 cases): Δ=+0.0000 — both `skill-on` and `skill-off` legs scored 12/12. Honest read: at workbench v1 difficulty (off-by-one bugs, simple validators, basic SQLi) AgentOps's hook layer is non-differentiating because the tasks don't require it. Substrate v2 (realistic agent task difficulty) is roadmap. Source: `evals/workbench/results/2026-05-06-yjzp9-counterstat.json`

**Maintainer corpus stats** (this repo's `.agents/`, derived by `scripts/corpus-stats.sh` — re-runnable, no fabricated numbers):

- ~1,842 learnings · ~186 patterns · ~80 planning rules
- ~68 finding markdown files · ~24 registry entries
- ~3,867 citations recorded in `.agents/ao/citations.jsonl`

These are this repo's corpus stats; your own AgentOps install will produce its own. Run `scripts/corpus-stats.sh --table` (or `--json` / `--markdown`) against `$AO_CORPUS_ROOT` to derive yours. The previously-cited "4,940 learnings, 1,195 patterns, 40 planning rules" line was removed because no on-disk source reconciled it; the numbers above are what the tracked source actually returns at the time of writing.

*`.agents/` runtime state was wiped by routine cleanup on 2026-05-07; receipts use 2026-05-04 stable snapshot. Durability fix tracked in soc-rv5p.*

Your corpus grows every session — learnings, patterns, and constraints accumulate in your repo, not ours. The system writes the substrate; you decide on what cadence the dream/evolution/compile loops run via `ao schedule`. Scale and compounding follow from the schedule you set, not from a claim we make.

## Desired State vs Current State

`PRODUCT.md` and `GOALS.md` are allowed to outpace the current repo. That is the point of goals: they define the desired state, not a frozen claim that every mechanism is already complete. In the Kubernetes/control-loop lineage, `GOALS.md` is the setpoint, the repo is actual state, `ao goals measure` is the sensor, and `/evolve`, dream, validation gates, and follow-up issues are the reconcile loop. Gaps are not embarrassing when they are named, measured, and queued; they are the worklist that keeps the factory moving toward closure.

## Known Product Gaps

| Gap | Impact | Status |
|-----|--------|--------|
| Dream autonomy is still maturing | The private local Dream lane runs through `/dream` and `ao overnight`, with bounded compounding, reports, setup guidance, and a separate GitHub nightly proof harness. Remaining work is deeper full-loop autonomy, calibration, and onboarding polish. | in-progress |
| Pattern-to-skill pipeline (synthesis layer) | Detection layer ships in v1 (`.agents/plans/2026-04-23-pattern-to-skill-pipeline-detection.md`); synthesis (LLM-authored draft skill bodies, tier heuristics, on-disk drafts in `.agents/skill-drafts/`) is deferred to v2 after a design council found 8+ blockers. The "self-programming compounding" framing is aspirational, not currently producing on-disk output. | deferred |
| Multi-runtime proof is tiered, not complete | Tier S structural proof is active for all four runtimes. Tier I live inventory proof is partial. Tier E live execution proof remains opt-in / nightly, not a default gate. | in-progress |
| Retrieval and worker knowledge propagation still limit compounding | The flywheel architecture is in place. Retrieval quality and passing prevention/finding context to implement workers remain weaker than the core thesis requires. | open |
| Behavioral eval system needs live agent runtime at scale | Eval workbench shipped: 3 fixture components (Go CLI, Python FastAPI, DevOps), 12 tasks with golden solutions and scoring scripts, behavioral eval suite, agent harness script, eval-skill-delta CI gate, and `--two-pass` head gate. Scoring infrastructure verified (golden 12/12, broken detection 12/12). A/B DeltaScorecard works for deterministic cases. Remaining gap: live agent runtime execution at scale — the harness and gates exist but full skill-on vs skill-off delta across the workbench is not yet a default gate. | in-progress |
| High-assurance profile needs deeper control mapping | The initial [assurance profile](docs/assurance-profile.md) now documents local-first state, evidence packets, policy gates, telemetry boundaries, autonomy modes, and out-of-scope claims. Remaining work is redaction, evidence export, supply-chain inputs, and program-specific control mapping. | in-progress |
| Public messaging shifted to context-compiler + moat framing | CDLC (Context Development Life Cycle) framing landed: Mission, Strategic Bet, README, and mkdocs hero surfaces now use "context compiler" as the primary identity noun. Remaining gap: downstream comparison docs and skill-page intros still need a sweep to match. | in-progress |

## Lineage

The internal lineage that produced this product, and the parallels we are *not* derived from. Users do not need this vocabulary; it records where the shape came from.

### The hierarchy

**Knowledge OS → Olympus → AgentOps → Mt. Olympus.**

- **Knowledge OS** is the systems-theoretic substrate. The dK/dt equation, stigmergy as the multi-agent coordination primitive, Meadows' leverage-point hierarchy as the design discipline. This is the body of theory the rest descends from.
- **Olympus** was the predecessor runtime. Power-user daemon, run ledger, context compilation, constraint injection. Archived as a live system; its patterns survived as skills inside AgentOps.
- **AgentOps** (this repository) is the coding-agent implementation. Skills + hooks + `ao` CLI + scheduling daemon. It applies the context-compounding model to software work.
<!-- agentops:claim:AOP-CLAIM-PRODUCT-MT-OLYMPUS-PROOF -->
- **Mt. Olympus** is the forkable Gas City runtime proof — the empirical demonstration that the substrate runs, autonomously, against a real codebase under operator control.

### Why Meadows, foregrounded

Donella Meadows' *Twelve Leverage Points* ranks intervention points in complex systems from weakest (#12, parameters) to strongest (#1, transcending paradigms). **Changing the loop beats tuning the output.** AgentOps targets the high-leverage end — #4 (self-organization) and #3 (goals) — through the knowledge flywheel and `GOALS.md` reconciliation, rather than #12 (a better prompt). This is the primary organizing principle, not a citation: the entire CDLC is built around moving leverage up Meadows' hierarchy.

### Compound engineering / software factories

The thread-based development pattern — multiple agents working compoundingly, validation gates between phases, learnings extracted into reusable skills — applied via the **software-factory operator pattern**. The lineage runs through Greenfield and Short's *Software Factories: Assembling Applications with Patterns, Models, Frameworks, and Tools* (2003): a factory configures and composes domain-specific assets. AgentOps configures and composes context, skills, and validation gates around an operator's codebase. Direct comparison against EveryInc's Compound Engineer at [docs/comparisons/vs-compound-engineer.md](docs/comparisons/vs-compound-engineer.md).

### Parallel, not derived from

- **Heroku's Twelve-Factor App.** Parallel to, not derived from. The 12-factor app describes stateless web processes managed by a control plane; AgentOps applies the same shape — environment-carried continuity, replaceable workers, explicit control plane — to coding agents. Same operating-style insight, different substrate.
- **Anthropic's Managed Agents** (May 2026), **Cursor agents**, **Factory's *Missions***. Convergent, not derived-from. Multiple teams arriving at planner/implementer/validator separation, dreaming/memory loops, and rubric-graded outcomes is evidence the architecture is correct — not lineage. AgentOps' position is the cross-runtime, repo-native, operator-sovereign substrate.

## Design Principles

**Theoretical foundation — six pillars:**

1. **[Systems theory (Meadows)](https://en.wikipedia.org/wiki/Twelve_leverage_points)** — *The* primary organizing principle, not a citation. **Changing the loop beats tuning the output** — Meadows leverage point #4 (self-organization) and #3 (goals) vs. #12 (parameters). AgentOps is built as a Meadows compounding system around the user's codebase: information flows captured (#6), rules encoded (#5), self-organization through the flywheel (#4), goals declared (#3). Most agent tooling lives at #12; AgentOps lives at #4–#3.
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

- [Context Lifecycle Contract](docs/context-lifecycle.md) — canonical definition of judgment validation, durable learning, and loop closure, with evidence map and mechanism inventory.
- [Trust Factory](docs/trust-factory.md) — how AgentOps maps to the five-step proof contract (identity, reproducibility, evaluation, evidence, recovery).
- [Wiki for your agents](docs/wiki-for-agents.md) — the wiki framing as a standalone document.
- [Scale Without Swarms](docs/scale-without-swarms.md) — why 3-5 focused agents with fresh context and regression gates outperform massive uncoordinated swarms; the AgentOps model of waves, isolation, and gates explained.
- [Brownian Ratchet](docs/brownian-ratchet.md) — the forward-only-progress lineage in detail.
- [The Science](docs/the-science.md) — DevOps Three Ways, the escape velocity condition, and the leverage-points map.
- [vs. Compound Engineer](docs/comparisons/vs-compound-engineer.md) — direct comparison against EveryInc's compound-engineering-plugin, including where AgentOps is in-scope (capture, scoring, injection, council validation, repo-native `ao` workflows) and where it explicitly is not.
