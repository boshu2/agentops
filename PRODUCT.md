---
last_reviewed: 2026-05-15
---

# PRODUCT.md

## Mission

**AgentOps is an SDLC control plane for agentic software development.** It automates the discipline of building a wiki for your agents: markdown in `.agents/` next to your code, produced and consumed by the agents that work there. The compounded corpus is the moat. The internal lifecycle is the CDLC: context is developed, tested, delivered, observed, and improved because context is what LLM agents consume.

## Product Identity

AgentOps is the SDLC control plane for agent teams: software-engineering practice encoded for LLM agents under token scarcity.

It gives agents the shared practices humans use to build complex software together: domain context, standards, tests, reviews, issues, handoffs, verdicts, wikis, operating loops, and release discipline.

<!-- agentops:claim:AOP-CLAIM-PRODUCT-CONTEXT-ARTIFACT -->
The highest-leverage input to coding agents is context: what the system knows, what it has tried, what failed, what the codebase decided, and what gates must hold. AgentOps automates the bookkeeping agents do not reliably do for themselves, then turns that record into an engineering artifact: typed, versioned, retrieved, validated, and fed back into the next run.

It is not a packet generator. Packets are one artifact. The public category is **SDLC control plane**; the internal mechanism is the **Context Development Life Cycle (CDLC)**. CDLC is the DevSecOps SDLC translated to context, plus the operating practices of multi-agent work: isolated context per worker, stigmergic coordination through a shared corpus, and planner/implementer/validator separation. Software engineering spent decades learning how to get fallible humans to work together in massive codebases. Those practices translate to fallible agents. AgentOps packages Extreme Programming, pragmatic engineering, TDD, DDD, BDD/Gherkin, SRE, DevOps, product discovery, and release discipline into composable skills, gates, standards, artifacts, and schedules. The **RPI workflow** is the canonical instance — `/discovery` produces the planner artifact, `/crank` runs implementer agents in fresh-context waves, `/validation` runs validator agents that have not seen the code. The four layers — bookkeeping, context compilation, validation gates, and knowledge flywheel — are the public product model. Dream is the scheduled overnight mode of the flywheel.

One factory, three operator surfaces, four compounding layers: an **in-harness plugin** (skills for Claude Code, Codex, Cursor, OpenCode), the **`ao` CLI** (terminal/CI control plane), and a **scheduling daemon** (off-API, off-vendor, runs on your hardware against your subscription), with **execution packets, explicit gates, and optional hooks** wiring policy into the runtime.

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

## 3.0 Product Posture

AgentOps 3.0 narrows the release wedge without retreating from the software-factory thesis. The primary user is the agent-heavy maintainer with recurring codebase context debt: someone already using coding agents on real repos, already feeling session amnesia, repeated mistakes, low trust in agent output, and scattered validation evidence.

The 3.0 promise is:

> Agent-heavy maintainers can use AgentOps to keep books, compile the right context, validate agent work, curate durable learning, and start the next session smarter without giving the corpus to a hosted control plane.

The hero capability is council inside an agreed engineering domain, not generic multi-model chat. AgentOps lets the operator define the domain through `PRODUCT.md`, `GOALS.md`, standards, skills, issues, test expectations, and evidence rules; then Claude, Codex, and other agents judge product, design, and implementation decisions against that shared operating context. The model can focus on the work because AgentOps carries the process and context boundaries around it.

Bring your agent and bring your harness. If it can consume plugins or skills, AgentOps plugs in. The daemon, Loop, `ao`, optional hooks, skills, and corpus all stay in the product for operators who want to automate the pipeline, run scheduled reviews, or build factories. The sharpened posture is that day-one value must not require an unlimited-token cloud factory, hidden runtime hooks, or a fully configured overnight automation lane. Scheduled compounding is a second-stage accelerator after the user has seen the first evidence trail work.

3.0 public claims are evidence-gated. The release train tracks the PMF scenario in `soc-m6v5.8`; launch claims that cite PMF or productivity evidence must point to exported artifacts under `docs/releases/` or `evals/workbench/results/`, not only local `.agents/` notes.

## Market Convergence

The April 2026 Claude Code source analysis confirmed that Anthropic's internal tooling follows the same architecture AgentOps implements:

| Anthropic Concept | AgentOps Equivalent | Status |
|---|---|---|
| **Learning Loop** — memory extraction, dream cycle consolidation, future session context | Knowledge Flywheel — `/retro` → `/forge` → `/harvest` → `ao lookup` / `ao context assemble`, tiered promotion (learning → pattern → rule), plus private local Dream via `/dream` and `ao overnight` | Shipped. On-demand capture/promotion is live, and Dream now provides the bounded private overnight compounding lane. GitHub nightly is the public proof harness for the contracts, not the user's private runtime. |
| **Skillify** — AI watches patterns, packages them as reusable skills, compound growth | Skills system — 77 skills, `/heal-skill` audit, `/converter` cross-runtime export, SKILL-TIERS classification | Prototype built. `ao flywheel close-loop` now drafts review-only skills from repeated patterns; promotion polish is the remaining gap. |
| **Verification Agent** — adversarial AI auditing AI, VERDICT system for human review | Council architecture — `/council`, `/pre-mortem`, `/vibe`, `/post-mortem` with multi-model consensus, prediction tracking. Stage 4 behavioral validation adds holdout scenarios + satisfaction scoring in STEP 1.8. | Shipped. On-demand + always-on (STEP 1.8 fires automatically during `/validation`). |
| **Managed Agents Dreaming** (May 2026) — scheduled session review, pattern extraction, memory curation between sessions | `/dream` + `ao overnight` + `cli/cmd/ao/dream_executor.go` + `.github/workflows/nightly.yml` dream-cycle proof job | Shipped. Bounded private overnight compounding lane runs the harvest → forge → close-loop → defrag chain unattended, off the API and against any model. |
| **Managed Agents Outcomes** (May 2026) — rubric-driven separate-context grader with iterate-until-pass | Shipped at three scopes: project — `GOALS.md` (rubric) + `ao goals measure` (each gate runs as separate subprocess; `cli/internal/goals/measure.go:132-164`) + `/evolve` (iterates worst-failing gate until pass; `skills/evolve/SKILL.md:379-388`); plan — `/pre-mortem` council judges as separate-context graders; code — `/vibe` council judges. Independent 3-judge audit confirmed parity on rubric authoring, separate-context grading, iterate-until-pass, and pinpoint-what-changed. | Shipped at the capability layer. Empirical workbench A/B (2026-05-06): Δ=+0.0000 across 12 cases at v1 difficulty (both legs 12/12) — task difficulty floor exhausted; v2 substrate (realistic agent tasks where the hook layer differentiates) is roadmap. Counter-stat artifact: `evals/workbench/results/2026-05-06-yjzp9-counterstat.json`. |

Read the convergence table the right way: AgentOps and every harness like it gets absorbed into the model layer over time — Anthropic's 2026-05-06 Managed Agents launch is the textbook example. Memory primitives, learning loops, even validation gates — frontier vendors will ship them natively. What stays yours is the corpus. AgentOps is the bridge tool that helps you build the moat *now*, with current models, before the harness layer commoditizes.

## Target Personas

### Persona 1: The Agent-Heavy Maintainer
- **Goal:** Keep a real codebase moving with coding agents while preserving context, evidence, and release judgment across sessions.
- **Pain point:** Each agent session starts cold. The maintainer knows there were prior attempts, warnings, decisions, and fixes, but they are scattered across chats, commits, notes, and memory.
- **Gap exposure:** Bookkeeping, context compilation, judgment validation, and durable learning. This is the 3.0 PMF wedge.

### Persona 2: The Quality-First Maintainer
- **Goal:** Ship fewer but higher-confidence releases while using agents for more of the work.
- **Pain point:** Agents can produce coherent-looking changes that miss edge cases, violate repo conventions, or leave no reviewable proof trail.
- **Gap exposure:** Judgment validation, claim governance, release readiness, and evidence export.

### Persona 3: The Agent Orchestrator
- **Goal:** Run multiple agents or repeated agent loops on a shared codebase without losing coordination or repeating mistakes.
- **Pain point:** Parallel and repeated agent work creates file conflicts, stale assumptions, duplicated investigations, and context drift.
- **Gap exposure:** Worktree isolation, planner/implementer/validator separation, loop closure, and scheduled compounding.

### Anti-Personas

- **One-off prompt users** who only need a single answer and do not care whether the repo remembers it.
- **Cloud-control-plane buyers** who want a hosted autonomous factory more than local corpus ownership.
- **Teams that will not inspect artifacts** and only want an agent to say "done."
- **New agent users with no repeated workflow yet**; they may benefit later, but the first 3.0 wedge is users who already feel agent-session context debt.

## Core Value Propositions

1. **Repo-local memory for agent work.** Attempts, decisions, citations, verdicts, handoffs, findings, retros, and run packets become artifacts the next session can use.
2. **Context starts warm.** AgentOps compiles prior work into phase-scoped context instead of asking every agent to rediscover the repo from scratch.
3. **Judgment becomes a gate.** `/pre-mortem`, `/vibe`, and `/council` add fresh-context review before risky plans and code ship.
4. **Engineering practice becomes executable.** Pragmatic engineering, XP, TDD, DDD, BDD/Gherkin, SRE, DevOps, and product-management practices become reusable skills, standards, gates, issue flows, and acceptance proofs the operator can jump into, automate, or review.
5. **Learning compounds under operator control.** `/forge`, `/harvest`, `/dream`, `ao flywheel`, and schedules turn completed work into reusable constraints without requiring an AgentOps cloud.
6. **The corpus stays portable.** The same local knowledge base can outlive a model, chat session, harness, or vendor.

## 10-Star Experience

For the 3.0 target audience, a 10-star experience is not "configure every automation lane." It is the first hour proving the factory is worth trusting.

1. **Install fits their runtime.** They can use Claude Code, Codex, OpenCode, or another skills-compatible agent without changing how they work.
2. **The domain packet is visible.** They can see the product, goals, standards, issue context, and test expectations the agents will operate inside.
3. **Council shows the taste layer.** Claude and Codex judge the same product/design/engineering decision against the same domain context and produce a verdict artifact.
4. **A first real repo task leaves evidence.** `/rpi "small goal"`, `/vibe recent`, or `/council validate this PR` produces a verdict and artifact trail the maintainer can inspect.
5. **The next session starts smarter.** The prior evidence, decision, or learning is retrievable and changes what the next agent sees.
6. **The user owns the substrate.** Artifacts are local, grep-able, diff-able, and removable. No AgentOps-hosted control plane is required.
7. **Automation is introduced after trust.** Daemon/schedule/Dream is framed as a second-stage compounding lane, not a prerequisite for first value.

## What the Product Actually Is

AgentOps is an SDLC control plane backed by a wiki for your agents. `.agents/` is markdown in your repo, version-controlled with your code, that agents read, traverse, and contribute to — the kind of wiki your team should already have, except agents do the maintenance. AgentOps automates the discipline of building one: capture, retrieval, validation, and compounding all happen mechanically so the wiki stays current instead of bitrotting.

That wiki is the substrate underneath a software factory with three surfaces and four user-facing layers: Bookkeeping, Context Compiler, Validation Gates, and Knowledge Flywheel. Dream is the scheduled overnight mode of the flywheel, not a separate peer product. The SDLC control plane executes the CDLC — the [Context Development Life Cycle](docs/cdlc.md) — so agent work can move through small, bounded, evidence-bearing vertical slices. The deeper proof-contract framing — identity, reproducibility, evaluation, evidence, recovery — lives in [docs/trust-factory.md](docs/trust-factory.md).

### Three surfaces

The same substrate, reached three ways:

- **In-harness plugin** — skills for Claude Code, Codex, Cursor, OpenCode, with optional hook adapters for runtimes that want them. Context moves through explicit packets and skill workflows first. Install via `claude plugin install`, `install-codex.sh`, or the skills.sh package.
- **`ao` CLI** — the terminal and CI control plane. `ao context assemble`, `ao lookup`, `ao compile`, `ao goals measure`, `ao flywheel close-loop` — the same compiler, scriptable. Repo-native, with no required AgentOps cloud control plane.
- **Scheduling daemon** — off-API, off-vendor. `ao schedule` + `ao daemon` run dream, evolve, compile, defrag, forge, and feedback-drain on whatever cadence you set, against whichever local subscription you already pay for. The always-on lane.

### Domain and practice layer

Before the four layers run, AgentOps helps the operator define the domain the agents operate inside. This is the pragmatic-engineering, DDD, TDD, and BDD-shaped part of the product: product docs define intent, goals define fitness, issues define current work, standards define style and constraints, tests or Gherkin-like scenarios define expected behavior, and skills encode the process.

The point is to take process burden off the model. The LLM still researches, plans, implements, reviews, and explains, but AgentOps curates the context in and out of each phase so the model does not have to invent the development methodology while doing the work.

The narrow waist is BDD/Gherkin + DDD + Hexagonal + TDD:

| Practice | Product role |
|---|---|
| **BDD / Gherkin** | Express intent as observable behavior and acceptance examples |
| **DDD** | Keep human and agent inside the same bounded vocabulary |
| **Hexagonal architecture** | Keep adapters, tools, runtimes, and vendors outside the core loop |
| **TDD** | Give every slice a first failing test and executable done condition |

All other practices attach there: CI/CD reruns the proof, SRE/DORA measures fitness, ADRs and provenance preserve decision memory, wikis and ratchets preserve learning, and Agile/XP keeps work atomic instead of waterfall-shaped.

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

- `ao context assemble` — phase-scoped packet assembly with token budgeting
- `ao lookup` — decay-ranked retrieval for on-demand knowledge
- `ao context assemble` — phase-scoped context packets
- `ao compile` — rebuild the knowledge wiki (mine, grow, defrag, lint)
- 77 skills — reusable context packages across Claude Code, Codex, and OpenCode
- Optional lifecycle hooks — adapter profiles for teams that want runtime automation after the hookless path is proven
- `bash <(curl -fsSL .../install.sh)` — 30 seconds, zero config

#### Layer 3: Validation Gates

**Problem:** Agents ship confident garbage. No review, no second opinion, no gate between "agent thinks this is good" and "this goes into production."

**What the compiler emits:** Multi-model consensus validates plans before build and code before commit. Gates block, not advise. Independent judges debate against the same product/domain context and return one auditable verdict.

- `/pre-mortem` — validate plans before implementation
- `/vibe` — validate code after implementation
- `/council` — multi-model adversarial review (Claude + Codex judges) over agreed product, goals, standards, and issue context
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

That's the CDLC inside the SDLC control plane. Generate, compile, test, distribute, deliver, observe, adapt. Same shape as the DevOps SDLC. Different substrate. The model stays the same. The corpus compounds.

The CDLC is not a packet pipeline. It is the operating discipline that ensures every high-value context token carries intent, boundary, evidence, decision, constraint, or next action.

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
3. **Operator workflow encoding.** RPI, planner/implementer/validator separation, fresh-context worker waves, council validation — these are operating practices for multi-agent work, encoded as skills, execution packets, gates, and optional hooks. Vendors will ship managed agents; they won't ship your operator workflow.
4. **Curated engineering practice.** The operator's preferred practices — pragmatic engineering, XP, TDD, DDD, BDD/Gherkin, SRE, DevOps, product discovery, release gates — become composable context units. You decide when to run them interactively, automate them, or inspect the output.

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

As of 2026-05-10:

**Traction:**

- GitHub repo: 341 stars, 33 forks, 2 open issues, last pushed 2026-05-10T03:24:01Z
- Public surface: GitHub Pages mkdocs site live at boshu2.github.io/agentops/; doctrine site live at 12factoragentops.com
- Distribution/runtime reach: 77 shared skills, 77 checked-in Codex artifacts, and 35 Codex overrides. `/validate` and `/curate` are additive in this train; legacy validation and mining skills remain until their shim/retirement gates are resolved.

**Measured operational proof:**

- `ao doctor --json`: hook coverage and structural gates passing
- Competitive freshness gate: comparison docs maintained within the 45-day target
- External validation: independent 3-judge audit (council, 2026-05-06) confirmed parity with Anthropic Managed Agents on rubric authoring, separate-context grading, and iterate-until-pass
- Empirical workbench A/B (2026-05-06, 12 cases): Δ=+0.0000 — both `skill-on` and `skill-off` legs scored 12/12. Honest read: at workbench v1 difficulty (off-by-one bugs, simple validators, basic SQLi) AgentOps's hook layer is non-differentiating because the tasks don't require it. Substrate v2 (realistic agent task difficulty) is roadmap. Source: `evals/workbench/results/2026-05-06-yjzp9-counterstat.json`
- 3.0 PMF scenario evidence is planned but not yet claimed. `soc-m6v5.8` owns the scenario spec, control path, exported evidence, and claim-ledger posture before launch copy uses PMF/productivity claims.

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
| First-value path is still too diffuse | The current product surface can ask users to understand the whole factory before they feel the first benefit. This most affects the 3.0 PMF wedge: maintainers who need context continuity and trust quickly. | in-progress |
| 3.0 PMF scenario evidence is pending | The release thesis is clear, but the exact scenario with repo/task/control/measures has not yet produced exported proof. Public PMF/productivity claims stay gated on `soc-m6v5.8`. | open |
| Canonical `/validate` and `/curate` consolidation is not release-ready | Additive skills exist in this worktree, but skill-count, registry, and Codex artifact gates are expected to fail until the release train resolves ship/defer and artifact sync. | in-progress |
| Public launch claims need exported proof | Local `.agents/` artifacts are useful operating evidence but are not enough for public claims. 3.0 needs tracked evidence under `docs/releases/` or `evals/workbench/results/` when launch copy cites PMF proof. | planned |
| Dream autonomy is still maturing | The private local Dream lane runs through `/dream` and `ao overnight`, with bounded compounding, reports, setup guidance, and a separate GitHub nightly proof harness. Remaining work is deeper full-loop autonomy, calibration, and onboarding polish. | in-progress |
| Pattern-to-skill pipeline (synthesis layer) | Detection layer ships in v1 (`.agents/plans/2026-04-23-pattern-to-skill-pipeline-detection.md`); synthesis (LLM-authored draft skill bodies, tier heuristics, on-disk drafts in `.agents/skill-drafts/`) is deferred to v2 after a design council found 8+ blockers. The "self-programming compounding" framing is aspirational, not currently producing on-disk output. | deferred |
| Multi-runtime proof is tiered, not complete | Tier S structural proof is active for all four runtimes. Tier I live inventory proof is partial. Tier E live execution proof remains opt-in / nightly, not a default gate. | in-progress |
| Retrieval and worker knowledge propagation still limit compounding | The flywheel architecture is in place. Retrieval quality and passing prevention/finding context to implement workers remain weaker than the core thesis requires. | open |
| Behavioral eval system needs live agent runtime at scale | Eval workbench shipped: 3 fixture components (Go CLI, Python FastAPI, DevOps), 12 tasks with golden solutions and scoring scripts, behavioral eval suite, agent harness script, eval-skill-delta CI gate, and `--two-pass` head gate. Scoring infrastructure verified (golden 12/12, broken detection 12/12). A/B DeltaScorecard works for deterministic cases. Remaining gap: live agent runtime execution at scale — the harness and gates exist but full skill-on vs skill-off delta across the workbench is not yet a default gate. | in-progress |
| High-assurance profile needs deeper control mapping | The initial [assurance profile](docs/assurance-profile.md) now documents local-first state, evidence packets, policy gates, telemetry boundaries, autonomy modes, and out-of-scope claims. Remaining work is redaction, evidence export, supply-chain inputs, and program-specific control mapping. | in-progress |
| Public messaging is still converging | README, PRODUCT.md, GOALS.md, CDLC, the docs landing page, and the one-page brief now lead with "SDLC control plane" and explain CDLC as the internal context lifecycle. Remaining gap: downstream comparison docs and skill-page intros still need a sweep to match. | in-progress |

## Lineage

The internal lineage that produced this product, and the parallels we are *not* derived from. Users do not need this vocabulary; it records where the shape came from.

### The hierarchy

**Knowledge OS → Olympus → AgentOps → Mt. Olympus.**

- **Knowledge OS** is the systems-theoretic substrate. The dK/dt equation, stigmergy as the multi-agent coordination primitive, Meadows' leverage-point hierarchy as the design discipline. This is the body of theory the rest descends from.
- **Olympus** was the predecessor runtime. Power-user daemon, run ledger, context compilation, constraint injection. Archived as a live system; its patterns survived as skills inside AgentOps.
- **AgentOps** (this repository) is the coding-agent implementation. Skills + execution packets + optional hooks + `ao` CLI + scheduling daemon. It applies the context-compounding model to software work.
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

- [PRACTICE-REGISTRY.md](PRACTICE-REGISTRY.md) — practice lineage and canonical `practices: [slug]` registry used by skills, hooks, evals, schemas, scripts, and CLI code.
- [Context Lifecycle Contract](docs/context-lifecycle.md) — canonical definition of judgment validation, durable learning, and loop closure, with evidence map and mechanism inventory.
- [Trust Factory](docs/trust-factory.md) — how AgentOps maps to the five-step proof contract (identity, reproducibility, evaluation, evidence, recovery).
- [Wiki for your agents](docs/wiki-for-agents.md) — the wiki framing as a standalone document.
- [Scale Without Swarms](docs/scale-without-swarms.md) — why 3-5 focused agents with fresh context and regression gates outperform massive uncoordinated swarms; the AgentOps model of waves, isolation, and gates explained.
- [Brownian Ratchet](docs/brownian-ratchet.md) — the forward-only-progress lineage in detail.
- [The Science](docs/the-science.md) — DevOps Three Ways, the escape velocity condition, and the leverage-points map.
- [vs. Compound Engineer](docs/comparisons/vs-compound-engineer.md) — direct comparison against EveryInc's compound-engineering-plugin, including where AgentOps is in-scope (capture, scoring, injection, council validation, repo-native `ao` workflows) and where it explicitly is not.
