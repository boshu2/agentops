---
last_reviewed: 2026-06-14
---

# PRODUCT.md

## Mission

**AgentOps is autonomous code validation for agentic software development.** It answers the questions that matter before you trust an agent's work: **is this code right, and is this agent output proven enough to receive more autonomy?** The mechanism is an SDLC control-plane shape around a repo-local corpus: markdown in `.agents/` next to your code, produced and consumed by the agents that work there. The product is the loop that produces validated output with proof — it catches the agent saying done when it isn't. **What is proven is the membrane and the record:** independent, fresh-context verification (no verdict = not done) plus a durable, hash-chained provenance ledger that binds every verdict to the work. The durable differentiator is **sovereignty** — that record lives in your repo and outlives any model or vendor. Whether the accumulated corpus *compounds* into a quality edge — the knowledge corpus and the escape-corpus alike — is a named, unproven hypothesis still being measured ([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), [ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)), not the moat. The internal lifecycle is the CDLC: context is developed, tested, delivered, observed, and improved because context is what LLM agents consume.

> The canonical definition of what 3.0 is — the hookless-first CDLC loop and the four-practice waist — lives in [docs/3.0.md](docs/3.0.md). The component and trim/defer routing map that keeps that thesis from sprawling lives in [docs/architecture/component-map.md](docs/architecture/component-map.md). This file is consistent with both.

> **Proven vs still-measuring.** The canonical honesty block is in the [README](README.md#the-honest-version), and every surface points to it. **Proven:** independent verification that records a verdict, plus a durable, tamper-evident record of it — no verdict, not done. **Still measuring:** whether the accumulated corpus makes the next session measurably better; not claimed until the numbers say so ([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), [ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)).

## One self-contained factory; the gate is the in-repo floor

AgentOps is the one self-contained software factory: it researches, plans, validates the plan, implements, and proves work end-to-end, and it carries its own **acceptance gate in-repo**. The gate is the **ratchet pawl-gate** — `docs/contracts/pawls.md` + `scripts/pawl-verdict.sh` + `scripts/reconcile-pr.sh`. At the merge pawl (the one-way door) work reaches *accepted* only through a fresh-context reviewer whose `context_id ≠ author` (model-agnostic by default; ≥2 distinct model families opt-in per pawl), with a verdict that is **evidence-bound and commit-bound** (`head_sha`) and **enforced fail-closed** (no CONFIRMED verdict → HOLD; green CI alone cannot merge), plus auto-redo on REFUTED and circuit-breaker escalation to a human only when a tunable breaker trips. The trust is the **separation of duties**: a stochastic worker never approves its own work, and the author cannot choose, brief, or transcribe its own reviewer. The destination (*autonomous goal → verified done*) lives in [GOALS.md](GOALS.md).

> **There is no separate Mount Olympus product or daemon in the running factory.** The gate is in-repo policy at the pawl, not an external `olympusd` service consuming claims. "Mount Olympus" survives in two reduced forms: (1) **lineage** — the typed-loop, explicit-ratchet-rules work (2026) the pawl-gate descends from; and (2) an optional **high-assurance Linux build** — a binding daemon with OS-account / peercred isolation, for *adversarial multi-tenant* deployments where a hostile worker might forge its own approval. That threat model does not apply to a single operator running their own factory, so the daemon is **parked until earned at scale**; the in-repo pawl-gate is the gate for everyone else.

**The gate is table stakes, not the moat.** Binding cross-vendor pre-merge review with non-author override is *already shipped commercially* — CodeRabbit, Qodo, and GitHub Copilot Code Review all gate merges on independent, often cross-model review (adversarial siege, 2026-06-14). So the fail-closed cross-family gate is the **correctness floor a serious factory needs**, not a differentiator. What AgentOps positions on is **proven**: the membrane verifies independently and the provenance ledger records the verdict — no verdict, not done — and that record is **sovereign**, staying in your repo across any model or vendor. The membrane's *self-improvement from an escape-corpus* is a named, unproven hypothesis still being measured ([ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)), not the moat; the earlier corpus-delta bet was measured and did **not** hold at frontier altitude ([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md)). The differentiator is sovereignty.

## Product Identity

AgentOps is the validation layer for agent teams: software-engineering practice encoded so LLM-written code can be judged, corrected, and trusted under token scarcity.

It gives agents the shared practices humans use to build complex software together: domain context, standards, tests, reviews, issues, handoffs, verdicts, wikis, operating loops, and release discipline.

<!-- agentops:claim:AOP-CLAIM-PRODUCT-CONTEXT-ARTIFACT -->
The highest-leverage input to coding agents is context: what the system knows, what it has tried, what failed, what the codebase decided, and what gates must hold. AgentOps automates the bookkeeping agents do not reliably do for themselves, then turns that record into an engineering artifact: typed, versioned, retrieved, validated, and fed back into the next run.

It is not a packet generator. Packets are one artifact. The public wedge is **autonomous code validation**; the SDLC control-plane shape and the **Context Development Life Cycle (CDLC)** are the mechanism that make validation repeatable. CDLC is the DevSecOps SDLC translated to context, plus the operating practices of multi-agent work: isolated context per worker, stigmergic coordination through a shared corpus, and planner/implementer/validator separation. Software engineering spent decades learning how to get fallible humans to work together in massive codebases. Those practices translate to fallible agents. AgentOps packages Extreme Programming, pragmatic engineering, TDD, DDD, BDD/Gherkin, SRE, DevOps, product discovery, and release discipline into composable skills, gates, standards, artifacts, and CI/substrate adapters. The **RPI workflow** is the canonical instance — `/discovery` produces the planner artifact, `/crank` runs implementer agents in fresh-context waves, `/validate` runs validator agents that have not seen the code. The four layers — validation membrane, evidence trail, context compilation, and knowledge ratchet — are the public product model. Scheduled or unattended compounding belongs to an orchestration substrate.

One factory, two AgentOps operator surfaces, four compounding layers: an **in-harness plugin** (skills for Claude Code, Codex, Cursor, OpenCode) and the **`ao` CLI** (terminal/CI control plane). Out-of-session orchestration is delegated to the substrate boundary (reference: NTM + MCP + managed-agents). AgentOps owns **execution packets, explicit gates, skills, and corpus compounding**; it does not ship a daemon, scheduler, or default hook layer.

The bet is **sovereignty, not features**. Vendors will ship managed memory, councils, and dreaming natively — and lock them to their runtime. Your corpus stays in `.agents/` in your repo, runs on whichever harness you already pay for, and is portable across whichever frontier model wins next quarter. **The model gets smarter. The corpus stays yours.** Humans choose the posture: stay **in the loop** during discovery and validation, or sit **on the loop** while an external substrate dispatches AgentOps loops.

> Canonical contract: [docs/context-lifecycle.md](docs/context-lifecycle.md)
> Internal lineage: the systems-theory work that preceded AgentOps lives in [docs/doctrine/lineage-and-theory.md](docs/doctrine/lineage-and-theory.md), but users do not need that vocabulary to understand the product.

## Vision

The software factory that gets better at proving code with each use. Every session produces code, evidence, decisions, attempts, lessons, and stronger constraints — the next session starts with more knowledge, tighter gates, and less wasted work. The model stays the same; the record stays yours. The proven floor is that every change is independently verified and the verdict is recorded. Whether the accumulated corpus *compounds* into a measurably better next session is the unproven hypothesis under measurement ([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), [ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)).

<!-- agentops:claim:AOP-CLAIM-PRODUCT-FACTORY-GRADE-THROUGHPUT -->
The aspiration is factory-grade confidence for agent-written code first, then throughput: enough structure that agents can run against a defined process, with the operator setting cadence, rigor, and escalation boundaries. Same shape that turned software delivery into an engineering discipline — applied to coding agents.

The thesis is simple: indeterministic workers need disciplined systems. DevOps proved this for engineers. SRE proved it again with SLOs and error budgets. Kubernetes proved it for declarative infrastructure with control loops that reconcile actual state to desired state. Coding agents are the next indeterministic worker class. Same playbook. New substrate. The asset that survives — yours, not ours — is the record the system keeps on your behalf.

### Why it must exist: prove the code before scaling autonomy

AgentOps starts with autonomous code validation, not autonomous orchestration.
The first-order question is: **how do you know the code is right?** The second
is: **how do you know the agent output is right enough to let it run farther
next time?** Autonomy is earned by evidence. When the validation path holds for
a lane, that lane can climb the [Autonomy Ladder](docs/architecture/autonomy-ladder.md):
more steps before review, more repeated ticks, and eventually substrate-driven
execution. The validation boundary does not loosen as autonomy rises; it is the
reason autonomy can rise at all.

### GPS for agentic work, not a workflow

Agentic workers are inherently stochastic, so AgentOps is a goal-directed navigator, not a workflow: you set the destination (the goal), trust the deterministic map and gates over the agent, and let deterministic ground-truth — the test that actually runs — be the windshield that catches the hallucinated road. The full navigator/windshield framing lives in [docs/doctrine/lineage-and-theory.md](docs/doctrine/lineage-and-theory.md#gps-for-agentic-work-not-a-workflow).

## What if the labs ship this natively?

They will. Anthropic's Managed Agents is the first move; others will follow. That's fine — the value isn't in this tool. It's in the corpus you build with it.

AgentOps is bridge infrastructure. Your `.agents/` directory is plain markdown in your repo. If a frontier vendor ships native equivalents in 12 months, your corpus carries forward. If we get acquired or change direction, your corpus is yours. If you outgrow the tool entirely, fork it, customize it, replace it — the corpus is what matters.

Open source forever. Built so you own the asset, not the tool.

## 3.0 Product Posture

AgentOps 3.0 narrows the release wedge without retreating from the software-factory thesis. The primary user is the agent-heavy maintainer with recurring codebase context debt: someone already using coding agents on real repos, already feeling session amnesia, repeated mistakes, low trust in agent output, and scattered validation evidence.

The 3.0 promise is:

> Agent-heavy maintainers can use AgentOps to keep books, compile the right context, validate agent work, curate durable learning, and start the next session smarter without giving the corpus to a hosted control plane.

The hero capability is council inside an agreed engineering domain, not generic multi-model chat. AgentOps lets the operator define the domain through `PRODUCT.md`, `GOALS.md`, standards, skills, issues, test expectations, and evidence rules; then Claude, Codex, and other agents judge product, design, and implementation decisions against that shared operating context. The model can focus on the work because AgentOps carries the process and context boundaries around it.

Bring your agent and bring your harness. If it can consume plugins or skills, AgentOps plugs in. The loop, `ao`, skills, and corpus are the product surfaces operators use to run disciplined agent work. The sharpened posture is that day-one value must not require an unlimited-token cloud factory, hidden runtime hooks, or a fully configured automation lane. Out-of-session compounding is a second-stage accelerator after the user has seen the first evidence trail work, and it runs through an orchestration substrate rather than an AgentOps-shipped daemon.

3.0 public claims are evidence-gated. The release train tracks the PMF scenario in `soc-m6v5.8`; launch claims that cite PMF or productivity evidence must point to exported artifacts under `docs/releases/` or `evals/workbench/results/`, not only local `.agents/` notes.

## Market Convergence

The labs will ship native versions of memory, councils, and dreaming; your record stays in your repo regardless. The detailed concept-by-concept convergence mapping (vs. Anthropic Managed Agents and peers) is an internal positioning note, kept off the public read to avoid defining the product by its enemy: [docs/strategy/market-convergence.md](docs/strategy/market-convergence.md).

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
- **Gap exposure:** Worktree isolation, planner/implementer/validator separation, loop closure, and substrate-driven compounding.

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
5. **Learning compounds under operator control.** `/forge`, `ao flywheel`, and substrate-run maintenance turn completed work into reusable constraints without requiring an AgentOps cloud.
6. **The corpus stays portable.** The same local knowledge base can outlive a model, chat session, harness, or vendor.

## 10-Star Experience

For the 3.0 target audience, a 10-star experience is not "configure every automation lane." It is the first hour proving the factory is worth trusting.

1. **Install fits their runtime.** They can use Claude Code, Codex, OpenCode, or another skills-compatible agent without changing how they work.
2. **The domain packet is visible.** They can see the product, goals, standards, issue context, and test expectations the agents will operate inside.
3. **Council shows the taste layer.** Claude and Codex judge the same product/design/engineering decision against the same domain context and produce a verdict artifact.
4. **A first real repo task leaves evidence.** `/rpi "small goal"`, `/vibe recent`, or `/council validate this PR` produces a verdict and artifact trail the maintainer can inspect.
5. **The next session starts smarter.** The prior evidence, decision, or learning is retrievable and changes what the next agent sees.
6. **The user owns the substrate.** Artifacts are local, grep-able, diff-able, and removable. No AgentOps-hosted control plane is required.
7. **Automation is introduced after trust.** Dream and substrate-driven scheduling are framed as second-stage compounding lanes, not prerequisites for first value.

## What the Product Actually Is

AgentOps is autonomous code validation backed by a wiki for your agents. `.agents/` is markdown in your repo, version-controlled with your code, that agents read, traverse, and contribute to — the kind of wiki your team should already have, except agents do the maintenance. AgentOps automates the discipline of proving work and preserving the proof: capture, retrieval, validation, and compounding all happen mechanically so the wiki stays current instead of bitrotting.

That wiki is the substrate underneath a software factory with three surfaces and four user-facing layers: Validation Membrane and Evidence Trail (proven), plus Context Compiler and Knowledge Ratchet (experimental, gated on ADR-0004/0011). Dream is a bounded compounding skill, not a separate peer product or AgentOps-owned scheduler. The SDLC control plane executes the CDLC — the [Context Development Life Cycle](docs/cdlc.md) — so agent work can move through small, bounded, evidence-bearing vertical slices. The deeper proof-contract framing — identity, reproducibility, evaluation, evidence, recovery — lives in [docs/trust-factory.md](docs/trust-factory.md).

### Three surfaces

The same substrate, reached three ways:

- **In-harness plugin** — skills for Claude Code, Codex, Gemini/Antigravity, Cursor, OpenCode. Context moves through explicit packets and skill workflows first. Runtime hooks are not an AgentOps default; teams that want custom hooks author them separately with the `hooks-authoring` skill. Install via `install-claude.sh`, `install-codex.sh`, `install-agy.sh`, or the skills.sh package.
- **`ao` CLI** — the terminal and CI control plane. `ao context assemble`, `ao lookup`, `ao compile`, `ao goals measure`, `ao flywheel close-loop` — the same compiler, scriptable. Repo-native, with no required AgentOps cloud control plane.
- **Out-of-session substrate** — optional, off-API, off-vendor orchestration outside the AgentOps product core. The reference boundary is NTM + MCP + managed-agents: it dispatches agents that run whole AgentOps loops (`ao rpi`, compile/maturity jobs) while AgentOps stays daemonless and schedulerless.

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

### Four layers: two proven, two experimental

Two layers are proven and carry the product: validation gates enforce judgment, and bookkeeping records the work with its proof. The other two, the context compiler and the knowledge flywheel, are experimental: they run, but whether they make the next session measurably better is still being measured ([ADR-0004](docs/adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), [ADR-0011](docs/adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)). They are labeled below so no reader mistakes the hypothesis for the product.

#### Layer 1: Agent Bookkeeping (proven)

**Problem:** Agents do not keep their own operational memory. They forget what they tried, why they changed course, which warnings mattered, what passed validation, and what should be reused next time.

**What the compiler emits:** File-backed traces of agent work: attempts, decisions, citations, handoffs, findings, retros, post-mortems, council verdicts, and run packets. This is the raw material for context compilation and compounding.

- `.agents/` — repo-local bookkeeping substrate
- `/post-mortem --quick` and `/post-mortem` — capture decisions, lessons, and follow-up work
- `/provenance` and `/trace` — connect artifacts back to their source
- `ao metrics cite` and citation logs — record what knowledge was used
- RPI packets and council verdicts — preserve plan/build/validation evidence

#### Layer 2: Context Compiler (experimental, gated on ADR-0004)

**Problem:** Every session starts from zero. Agents need the right slice of prior work, policy, constraints, and decisions before they can act well.

**What the compiler emits:** Decay-ranked, phase-scoped context packets built from the bookkeeping trail and curated corpus.

- `ao context assemble` — phase-scoped packet assembly with token budgeting
- `ao lookup` — decay-ranked retrieval for on-demand knowledge
- `ao context assemble` — phase-scoped context packets
- `ao compile` — rebuild the knowledge wiki (mine, grow, defrag, lint)
- Reusable skills — generated from `skills/**/SKILL.md` and packaged across Claude Code, Codex, Gemini/Antigravity, and OpenCode
- Runtime install one-liners plus Day-2 operations — install, update, backup, permission repair, recovery, and escalation are product surfaces, not afterthoughts

#### Layer 3: Validation Gates (proven)

**Problem:** Agents ship confident garbage. No review, no second opinion, no gate between "agent thinks this is good" and "this goes into production."

**What the compiler emits:** Multi-model consensus validates plans before build and code before commit. Gates block, not advise. Independent judges debate against the same product/domain context and return one auditable verdict.

- `/pre-mortem` — validate plans before implementation
- `/vibe` — validate code after implementation
- `/council` — multi-model adversarial review (Claude + Codex judges) over agreed product, goals, standards, and issue context
- 63 eval suites + 12-task workbench — deterministic context quality testing
- Baseline A/B — skill-on vs skill-off delta measurement

#### Layer 4: Knowledge Flywheel (experimental, gated on ADR-0004/0011)

**Problem:** Each session ends and the lessons disappear. Same mistakes get made. Same solutions get rediscovered. Nothing compounds.

**What the compiler emits:** Bookkeeping becomes reusable knowledge. Learnings get scored on specificity, actionability, novelty. High-scoring learnings promote to permanent patterns. Patterns become planning rules. Scheduled dream cycles defrag and compound the corpus without competing with foreground engineering. Next session starts loaded.

- `/forge` — extract structured learnings from completed sessions
- `ao flywheel close-loop` — score, promote, curate automatically
<!-- agentops:claim:AOP-CLAIM-PRODUCT-EVOLVE-RECONCILE -->
- `/evolve` — bounded reconciliation: reads goals, targets the worst gap, validates, and can repeat under operator limits
- Substrate-run compounding — bounded private compounding lane
- Out-of-session substrate (reference: NTM + MCP + managed-agents) — operator-owned cadence for unattended rpi, compile, maturity, and related jobs
- `.github/workflows/nightly.yml` — public proof harness for the contracts (not your private runtime)
- MemRL feedback — cited artifacts receive session reward, utility scores update
- Corpus health snapshot — the flywheel reports whether current citation flow is compounding or decaying

### How They Compound

```
Session starts → compiler delivers context → Agent works →
bookkeeping records attempts, decisions, citations, and evidence →
validation gates emit verdicts → Session ends → flywheel promotes learnings →
Dream or substrate-driven maintenance defrags the corpus → Next session starts with better context
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

**Position on the ruler, not the gate.** The fail-closed cross-family gate is table stakes — binding cross-vendor pre-merge review with non-author override already ships (CodeRabbit, Qodo, Copilot). It is a necessary correctness floor, not a wedge, and the messaging does not lean on it. The only *candidate* moat is the **corpus delta**: does the compounding `.agents/` corpus measurably improve agent output versus the same models with no corpus? That is **unproven** — the one A/B on record (ADR-0002, Δ=0) was a hook-layer test, not a corpus A/B, and the corpus delta itself (`ag-8p8o`) is still being measured. So the moat is a *hypothesis gated on a ruler*, not a claim; the load-bearing work is building that ruler. **Fungible dispatch** — identical generalist workers, any-agent-any-bead, mixed families coordinated through the durable board rather than a shared context window — is the *mechanism* that makes the cheap cross-family gate, disposable-context management, and vendor-cost arbitrage possible; it is not itself the moat. The durable, non-quality differentiator is **sovereignty**: the corpus stays in your repo and outlives any model, harness, or vendor.

**When Anthropic ships native scheduling, councils, Skillify, and Dreaming inside Claude Code in the next 6 months, what specifically does AgentOps still do?** The honest answer:

1. **Cross-runtime corpus.** The corpus stays in `.agents/` in your repo. It runs the same way on Claude Code, Codex CLI, Cursor, and OpenCode. When the next frontier model wins next quarter — or when you switch teams or platforms — the corpus comes with you. Vendor-managed memory follows the chat session. AgentOps' corpus follows the team.
2. **Local sovereignty.** The corpus and in-session loops run off-API, off-vendor, against the runtimes you choose. Dream cycles, evolve loops, compile/defrag, and substrate-dispatched rpi jobs do not require an AgentOps cloud and do not ship your codebase to a hosted AgentOps control plane.
3. **Operator workflow encoding.** RPI, planner/implementer/validator separation, fresh-context worker waves, council validation — these are operating practices for multi-agent work, encoded as skills, execution packets, gates, and CI/substrate adapters. Vendors will ship managed agents; they won't ship your operator workflow.
4. **Curated engineering practice.** The operator's preferred practices — pragmatic engineering, XP, TDD, DDD, BDD/Gherkin, SRE, DevOps, product discovery, release gates — become composable context units. You decide when to run them interactively, automate them, or inspect the output.

The model gets smarter. The corpus stays yours. See [docs/doctrine/lineage-and-theory.md](docs/doctrine/lineage-and-theory.md) for how this position was arrived at.

## Assurance Posture

AgentOps is engineered from constrained-environment habits, not consumer-app assumptions. This is not a certification claim; it is the operating posture the system is built toward.

Full profile: [docs/assurance-profile.md](docs/assurance-profile.md).

- **Local-first control.** The corpus lives in `.agents/` beside the code. AgentOps requires no product telemetry, and operators choose which model runtimes, networks, and subscriptions touch the repo.
- **Context as a boundary.** Research, planning, implementation, and validation receive different context. Workers get fresh windows. Validators get evidence packets instead of the implementer's accumulated chat.
- **Bookkeeping by default.** RPI packets, council verdicts, citations, ratchet records, post-mortems, handoffs, and substrate/job outputs leave file-backed traces that can be inspected, diffed, archived, or excluded from source control.
- **Policy gates over advice.** Pre-push checks, CI gates, security scans, goal fitness gates, and pre-mortems encode process as executable constraints instead of relying on the agent to remember a runbook.
- **Variable autonomy.** The same factory can run interactive, supervised, or substrate-scheduled loops. High-risk environments can keep humans in the loop for planning, validation, release, and promotion while still using agents for bounded work.
- **Constrained-network fit.** The design favors repo-local state, explicit artifacts, no required cloud control plane, and operator-owned or substrate-owned scheduling. Formal deployment into classified, export-controlled, or safety-critical environments still requires the local authority's security controls, model approvals, supply-chain process, and accreditation work.

## Evidence

As of 2026-05-10:

**Traction:**

- GitHub repo: 341 stars, 33 forks, 2 open issues, last pushed 2026-05-10T03:24:01Z
- Public surface: GitHub Pages mkdocs site live at boshu2.github.io/agentops/; doctrine site live at 12factoragentops.com
- Distribution/runtime reach: 58 shared skills, 57 checked-in Codex artifacts, and 18 Codex overrides. `/validate` and `/curate` are additive in this train; legacy validation and mining skills remain until their shim/retirement gates are resolved.

**Measured operational proof:**

- `ao doctor --json`: hook coverage and structural gates passing
- Competitive freshness gate: comparison docs maintained within the 45-day target
- Internal assessment: a 2026-05-06 council review found rubric authoring, separate-context grading, and iterate-until-pass present at the capability layer (internal finding; not an external audit or a verified parity claim against Anthropic Managed Agents)
- Empirical workbench A/B (2026-05-06, 12 cases): Δ=+0.0000 — both `skill-on` and `skill-off` legs scored 12/12. Honest read: at workbench v1 difficulty (off-by-one bugs, simple validators, basic SQLi) AgentOps's context layer is non-differentiating because the tasks don't require it. Substrate v2 (realistic agent task difficulty) is roadmap. Source: `evals/workbench/results/2026-05-06-yjzp9-counterstat.json`
- 3.0 PMF scenario evidence is planned but not yet claimed. `soc-m6v5.8` owns the scenario spec, control path, exported evidence, and claim-ledger posture before launch copy uses PMF/productivity claims.

**Maintainer corpus stats** (this repo's `.agents/`, derived by `scripts/corpus-stats.sh` — re-runnable, no fabricated numbers; exported snapshots such as `docs/evidence/2026-04-02-flywheel-case-study.md` when promoted for public citation):

- ~1,842 learnings · ~186 patterns · ~80 planning rules
- ~68 finding markdown files · ~24 registry entries
- ~3,867 citations recorded in `.agents/ao/citations.jsonl`

These are this repo's corpus stats; your own AgentOps install will produce its own. Run `scripts/corpus-stats.sh --table` (or `--json` / `--markdown`) against `$AO_CORPUS_ROOT` to derive yours. The previously-cited "4,940 learnings, 1,195 patterns, 40 planning rules" line was removed because no on-disk source reconciled it; the numbers above are what the tracked source actually returns at the time of writing.

*`.agents/` runtime state was wiped by routine cleanup on 2026-05-07; receipts use 2026-05-04 stable snapshot. Durability fix tracked in soc-rv5p.*

Your corpus grows every session — learnings, patterns, and constraints accumulate in your repo, not ours. The system writes the substrate; you decide whether dream/evolution/compile loops run manually, in session, or through an external orchestration substrate. Scale and compounding follow from the cadence you set, not from a claim we make.

## Desired State vs Current State

`PRODUCT.md` and `GOALS.md` are allowed to outpace the current repo. That is the point of goals: they define the desired state, not a frozen claim that every mechanism is already complete. In the Kubernetes/control-loop lineage, `GOALS.md` is the setpoint, the repo is actual state, `ao goals measure` is the sensor, and `/evolve`, dream, validation gates, and follow-up issues are the reconcile loop. Gaps are not embarrassing when they are named, measured, and queued; they are the worklist that keeps the factory moving toward closure.

## Known Product Gaps

| Gap | Impact | Status |
|-----|--------|--------|
| First-value path is still too diffuse | The current product surface can ask users to understand the whole factory before they feel the first benefit. This most affects the 3.0 PMF wedge: maintainers who need context continuity and trust quickly. | in-progress |
| 3.0 PMF scenario evidence is pending | The release thesis is clear, but the exact scenario with repo/task/control/measures has not yet produced exported proof. Public PMF/productivity claims stay gated on `soc-m6v5.8`. | open |
| Canonical `/validate` and `/curate` consolidation is not release-ready | Additive skills exist in this worktree, but skill-count, registry, and Codex artifact gates are expected to fail until the release train resolves ship/defer and artifact sync. | in-progress |
| Public launch claims need exported proof | Local `.agents/` artifacts are useful operating evidence but are not enough for public claims. 3.0 needs tracked evidence under `docs/releases/` or `evals/workbench/results/` when launch copy cites PMF proof. | planned |
| Compounding autonomy is still maturing | The private compounding lane is substrate-owned after the 3.0 daemon deletion, with bounded compounding, reports, setup guidance, and a separate GitHub nightly proof harness. Remaining work is deeper full-loop autonomy, calibration, and onboarding polish. | in-progress |
| Pattern-to-skill pipeline (synthesis layer) | Detection layer ships in v1 (`.agents/plans/2026-04-23-pattern-to-skill-pipeline-detection.md`); synthesis (LLM-authored draft skill bodies, tier heuristics, on-disk drafts in `.agents/skill-drafts/`) is deferred to v2 after a design council found 8+ blockers. The "self-programming compounding" framing is aspirational, not currently producing on-disk output. | deferred |
| Multi-runtime proof is tiered, not complete | Tier S structural proof is active for all four runtimes. Tier I live inventory proof is partial. Tier E live execution proof remains opt-in / nightly, not a default gate. | in-progress |
| Retrieval and worker knowledge propagation still limit compounding | The flywheel architecture is in place. Retrieval quality and passing prevention/finding context to implement workers remain weaker than the core thesis requires. | open |
| Behavioral eval system needs live agent runtime at scale | Eval workbench shipped: 3 fixture components (Go CLI, Python FastAPI, DevOps), 12 tasks with golden solutions and scoring scripts, behavioral eval suite, agent harness script, eval-skill-delta CI gate, and `--two-pass` head gate. Scoring infrastructure verified (golden 12/12, broken detection 12/12). A/B DeltaScorecard works for deterministic cases. Remaining gap: live agent runtime execution at scale — the harness and gates exist but full skill-on vs skill-off delta across the workbench is not yet a default gate. | in-progress |
| High-assurance profile needs deeper control mapping | The initial [assurance profile](docs/assurance-profile.md) now documents local-first state, evidence packets, policy gates, telemetry boundaries, autonomy modes, and out-of-scope claims. Remaining work is redaction, evidence export, supply-chain inputs, and program-specific control mapping. | in-progress |
| Public messaging is still converging | README, PRODUCT.md, GOALS.md, CDLC, the docs landing page, and the one-page brief are being sharpened around autonomous code validation, with SDLC control-plane/CDLC language as the implementation mechanism. Remaining gap: downstream comparison docs and skill-page intros still need a sweep to match. | in-progress |
| **The moat is unproven — corpus delta not yet measured** | The product's only *candidate* moat (does the compounding corpus measurably improve agent output?) has no A/B proof. ADR-0002's Δ=0 was a hook-layer test, not a corpus A/B. Until `ag-8p8o` produces a measured delta on realistic tasks, the moat is a hypothesis — public quality/moat claims stay gated on it. This is the single highest-leverage gap: the ruler is the product. | open (`ag-8p8o`) |
| **The cross-family gate is table stakes, not a differentiator** | Binding cross-vendor pre-merge review with non-author override already ships commercially (CodeRabbit, Qodo, Copilot — adversarial siege 2026-06-14). The in-repo pawl-gate is a necessary correctness floor; messaging must not position on it as a wedge. | acknowledged |

## Lineage and design principles

The systems-theory lineage (Knowledge OS → Olympus → AgentOps → Mt. Olympus), the Meadows / DevOps-Three-Ways / SRE / Kubernetes / Brownian-ratchet / knowledge-flywheel design pillars, and the parallels AgentOps is *convergent with but not derived from* are internal doctrine — they record where the shape came from, and users do not need that vocabulary to use the product. Full text: [docs/doctrine/lineage-and-theory.md](docs/doctrine/lineage-and-theory.md).

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
