# Deep Analysis: "Code as Agent Harness — Toward Executable, Verifiable, and Stateful Agent Systems"

- **Source:** arXiv:2605.18747v1 (cs.CL), published 2026-05-18. HTML: https://arxiv.org/html/2605.18747v1
- **Authors:** Xuying Ning, Katherine Tieu, Dongqi Fu, Tianxin Wei, Zihao Li, Yuanchen Bei, et al. (large UIUC-centered collaboration)
- **Companion repo:** https://github.com/YennNing/Awesome-Code-as-Agent-Harness-Papers
- **Analysis date:** 2026-07-19 (AgentOps research pass)
- **Companion doc:** `code-as-agent-harness-mining-for-agentops.md` (what this paper means for this repo)

## 1. What the paper is

A large survey (literature through early 2026) that reframes code from *the thing agents produce* to *the operational substrate agents run on*. The authors coin **"code as agent harness"**: code as the executable, inspectable, and stateful medium through which agents reason, act, observe feedback, verify progress, and coordinate.

The load-bearing conceptual move is a three-way decomposition of long-running agentic systems:

1. **Model-internal capabilities** — reasoning, perception, planning, evaluation inside the LLM.
2. **System-provided harness infrastructure** — the predefined tools, APIs, sandboxes, memory systems, validators, permission boundaries, telemetry, and workflows that turn a stateless model into a functional agent. This is what the emerging discipline of **harness engineering** optimizes.
3. **Agent-initiated code artifacts** — the underexplored middle: interactive code objects agents *create, execute, observe, revise, persist, and share* inside the task loop. Examples: regression tests, temporary tools, DSL programs, executable workflows, reusable skills, intermediate program states.

The survey's stated thesis: the bottleneck of autonomy is no longer only base-model reasoning but **the reliability of the system that connects model outputs to long-horizon actions and persistent state**. Claude Code, Codex, Copilot coding agents, and DeepAgents are repeatedly cited as the production proof of this shift.

## 2. Structure — three layers

| Layer | Section | Question it answers |
|---|---|---|
| Harness **Interface** | §2 | How does code connect a model to reasoning, action, and environment state? |
| Harness **Mechanisms** | §3 | How do planning, memory, tool use, control loops, and harness self-optimization keep an agent reliable over long horizons? |
| **Scaling** the Harness | §4 | How do multiple agents coordinate over shared code artifacts (repos, tests, traces), and what convergence/consistency problems appear? |

§5 grounds the taxonomy in five application domains (code assistants, GUI/OS agents, embodied agents, scientific discovery, personalization) and closes with seven open problems.

## 3. Section 2 — Harness Interface (code for reasoning, acting, environment)

*(Sections 3–5 synthesized from delegated section reads — see §8 methodology note.)*

**Thesis.** Code is the right medium between a stateless model and its environment because it is simultaneously *executable* (outcomes are formally verifiable), *inspectable* (intermediate computation surfaces as structured traces), and *stateful* (the evolving program persists progress). These are harness-functional properties, not notation properties. Explicit scope boundary: "code" = executable or machine-checkable artifacts (programs, formal specs, proof scripts, tests, repos, simulators, configs, traces); raw perception, physical state, and human intent are *not* code — code makes selected aspects of them checkable but does not replace them.

### 3.1 Code for reasoning (§2.1)

- **Program-delegated reasoning:** model proposes, interpreter executes — PAL, Program-of-Thoughts, Chain-of-Code (with its "LMulator" for semantically simulating non-executable code), MathCoder, CodeI/O (converts programs into I/O-prediction tasks with executable verification).
- **Formal verification interfaces:** SAT/SMT backends (SATLM), proof assistants (ReProver, DeepSeek-Prover-V2, Goedel-Prover), and — frontier — **Lean4Agent**, which uses Lean4 to model and verify *agent workflows and trajectories themselves*. Framing: formal languages as "executable contracts that constrain, certify, and audit agent behavior."
- **Iterative code-grounded reasoning:** closed generate→execute→verify→refine loops with process rewards derived from execution rather than next-token likelihood — NExT (reasoning over traces), CodePRM/ORPS (execution-scored trajectories), RLEF/StepCoder (RL from execution feedback), EG-CFG (execution signals injected *during* generation), FunPRM/ExecVerify (rewards at function/statement/variable granularity).

### 3.2 Code for acting (§2.2)

Central challenge is **grounding**, and the section's most important admission: action failures are often *silent* — invalid state transitions, delayed feedback, no exception raised (a grasp outside the workspace just fails). The verification asymmetry between reasoning (interpreter catches errors) and acting (environment doesn't) is the biggest admitted hole.

- **Grounded skill selection:** SayCan (language × affordance feasibility), KnowNo (conformal-prediction uncertainty → clarify with the human *before* unsafe execution), BOSS (skill-chain bootstrapping).
- **Programmatic policy generation:** Code-as-Policies, RoboCodeX, Code-BT (compile to behavior-tree controllers), NormCode (governed semi-formal interface enforcing auditability).
- **Lifelong code-based agents:** code as "persistent memory substrate" — Voyager (growing executable skill library), LYRA (converts transient human corrections into reusable executable skills), UI-Voyager (failure-driven self-evolution).
- **AutoHarness** pattern: the harness itself is a synthesized artifact — a generated code layer that "filters invalid actions before execution."

### 3.3 Code for environment (§2.3)

World state as executable artifact: verifiable state transitions ("through execution rather than ambiguous natural-language judgment") and persistent, modifiable state.

- **Structured world representations:** WorldCoder (agent writes/updates its world model as editable Python), Code2World (predicts next GUI state as renderable HTML — "make opaque state renderable to make it verifiable"), PoE-World (compositional programmatic experts).
- **Execution-trace world modeling:** SemCoder, CWM (open-weights model trained natively on execution traces).
- **Code-grounded evaluation environments:** InterCode, SWE-bench, LiveCodeBench (continuously refreshed against contamination), Endless Terminals.
- **Verifiable environment construction** (newest): SWE-smith and EnvScaler mass-synthesize environments *together with* their scenarios and rule-based trajectory validators — build the oracle with the fixture, never after.

## 4. Section 3 — Harness Mechanisms (planning, memory, tools, control, optimization)

**Thesis.** Software generation becomes "an interaction among the model, mutable task state, and human-designed harness infrastructure." The model supplies judgment; the harness "turns model decisions into bounded, observable, and revisable changes in an executable environment" and "remains the larger policy-governed system that decides what code may be executed, trusted, persisted, reused, or promoted."

### 4.1 Planning (§3.1)

Four families: **linear decomposition** (ReAct, Self-Planning, Plan-And-Act; industrially, PLAN.md/status logs as "a filesystem-backed control object" — Git-versioned, reviewable, subagent-consumable source of truth); **structure-grounded** (CodePlan's dependency-graph edit obligations, repo-graph localization — plus AGENTS.md/architecture notes as "persistent, inspectable, version-controlled" planning grounding); **search-based** (CodeTree, DARS, SWE-Search MCTS; Meta-Harness searches over harness code itself — search framed as "a harness-level state management problem"); **orchestration-based** (AgentCoder, MapCoder, controller-centric scaffolds; NLAH's harness logic as editable natural language executed under "explicit contracts, budgets, tool interfaces, and environment state").

### 4.2 Memory and context engineering (§3.2)

Memory is "a state-management layer," not a bigger window or a vector DB. Six subtypes: working (SWE-agent — same model, large deltas purely from state organization), semantic (AutoCodeRover structure-aware retrieval; retrieve "evidence aligned with program structure," not more content), experiential (MemGovern: "quality of stored experience matters more than its scale" — ungoverned records cause semantic noise and false retrievals), long-term (governance over "when to write, when to compress, when to retrieve, how to avoid contamination"; MemCoder persists only human-validated fixes), multi-agent (blackboards, pub-sub pools), and **context compaction / state offloading**: agent context gets "compact summaries and resource identifiers rather than raw logs" — canonical example: failing test name + key stack frames + suspect files + *link* to the full log, with full-fidelity artifacts preserved for audit and replay.

### 4.3 Tool use (§3.3)

Tools as "a governed interface between model intent and external systems." Four kinds: function-oriented (API search), environment-interaction (SWE-agent's formalized Agent-Computer Interface), **verification-driven ("tools as deterministic sensors")**, and workflow-orchestration with **lifecycle hooks** — pre-use hooks validate arguments/enforce permissions/block risky commands; post-use hooks sanitize outputs, compact logs, update memory, trigger verification. A tool call becomes "a monitored transition," not a raw model-selected action.

### 4.4 The Plan–Execute–Verify loop (§3.4) — the paper's control-theory core

- **Harness as "cybernetic governor":** observes via deterministic sensors (linters, compilers, type checkers, tests, fuzzers, runtime monitors, CI) and chooses: continue, revise, request context, re-route, *reduce permissions*, or escalate to human.
- **Planning as contract formation (§3.4.2):** a plan is "an explicit contract over the next state transition" — files in scope, expected invariants, validation commands, rollback points, risky operations — "a harness artifact rather than an unobserved reasoning trace." Coupling: "failed verification updates the plan, while the plan determines which verification evidence is meaningful."
- **Sandboxed execution + permissioned transitions (§3.4.3):** three-tier permission model — read-only → sandbox-edit → full-access (network, credentials, deploy, destructive FS ops, git-history mutation), the top tier behind mandatory human-in-the-loop gates. Sandboxes buy *reproducibility*: without them "failures may reflect environment drift rather than program defects." Production governance lives in gateway/policy layers outside the prompt (LiteLLM, Kong, Portkey) producing "falsifiable approval evidence."
- **Verification through deterministic sensors (§3.4.4):** cost-ordered sensor ladder (static → runtime → test → evaluation-harness distributions). The subordination rule: human/agentic critique "should interpret sensor outputs rather than replace them." Reflection "is reliable only when it remains grounded in executable evidence." **Termination is governed by verification state, never model confidence** — stop when required checks pass, attempts plateau, risk tier changes, or human review is required.

### 4.5 Agentic Harness Engineering (§3.5) — the harness as optimization object

Motivation: many failures come from "missing repository context, brittle tool interfaces, weak validators, excessive token cost, poor retry policies, or mismatched permission boundaries rather than from model generation."

- **Deep telemetry (§3.5.1):** shallow logs record pass/fail; deep telemetry records prompts, retrieved context, token cost per stage, tool arguments, permission requests, sandbox snapshots, branch decisions, *rejected alternatives*, human interventions. Turns revision "from anecdotal debugging into comparative diagnosis"; artifact-linked traces are replayable across harness versions for A/B comparison.
- **The Evolution Agent (§3.5.2):** a meta-agent that "edits the operating conditions under which later task agents work." Five-stage loop: observe → diagnose (attribute failures to specific harness components) → propose (tighter tool schema, added validator, changed permission rule, new regression test) → evaluate (held-out tasks or replayed traces) → **promote only changes that improve reliability/cost/safety without regressing previously solved cases**. Related systems: GEPA, EvoMAC, SEW, Live-SWE-agent.
- **Governed harness mutation (§3.5.3):** "AHE should not be confused with unconstrained self-modification." Changes touching permission boundaries, network, credentials, deployment, or HITL requirements require human approval before activation. Closure property: "the Evolution Agent is itself subject to the PEV loop."

**Gaps the authors admit for §3:** planning gains may be evaluation artifacts (benchmarks that miss multi-step coordination errors inflate reported gains); memory gains are inseparable from eval reliability (weak tests/contamination can fake improvements); retrieval-only tooling fails at cross-file reasoning and runtime debugging.

## 5. Section 4 — Scaling the Harness (multi-agent orchestration over code)

**Thesis.** Single-agent harnesses hit three walls: context limits, specialization inefficiency, and *no independent verification channel* (an agent can't reliably detect its own errors). Multi-agent systems distribute work across roles coordinated **through shared code artifacts** — "artifact-mediated communication" (files, diffs, tests, logs, schemas, blackboards) rather than pure message passing.

### 5.1 Roles, interaction modes, topologies (§4.1)

- **Role classes:** synthesis, program understanding, verification, execution, planning, plus EvoMAC's meta-roles (a Gradient Agent attributing failures to *agents* from execution logs, an Updating Agent rewriting prompts and restructuring the workflow DAG — "textual backpropagation").
- **Anti-self-grading designs recur:** AgentCoder's Test Designer generates tests *independently of the code* to avoid the "mode-collapse problem where an agent's biased tests pass its own buggy code"; CANDOR's panelists audit oracle correctness against the NL spec, never the implementation; AgentCoder's Test Executor is a deterministic Python script, not an LLM.
- **Interaction modes:** collaborative synthesis (rare), critique-and-repair (dominant — key axes: simulated vs execution-grounded critique, feedback richness, repair budget before fallback), **adversarial validation** (don't explain what's wrong — *demonstrate a concrete failure*: fuzzer counterexamples, exact failing waveform windows), and reasoning debate (ChatDev's "communicative de-hallucination").
- **Topologies:** predefined (chain/waterfall, cyclic/agile with iteration caps, hierarchical, star) vs objective-driven/adaptive. Notables: L2MAC runs each chain step in a **fresh-context agent** sharing only the external file store; PairCoder detects dead ends (same buggy code or same feedback recurring → switch plan, don't keep repairing); BOAD frames sub-agent team selection as a bandit-optimization problem — **discovered teams beat hand-designed ones**; SEW's evolved workflows converge on two canonical topologies: linear chain and feedback loop.

### 5.2 Execution feedback and synchronization (§4.2)

- **Feedback types**, orderable by information density: compiler/syntax → test pass/fail → fuzzer crash traces (localize to an input category) → static-analysis CWE flags → performance profiles → per-clock-edge simulation snapshots (MAGE, finest granularity surveyed). Structured logs enumerating satisfied/errored/unmet requirements (EvoMAC) beat binary pass/fail as repair signal.
- **QualityFlow's "Imagined Execution":** an LLM simulating the interpreter reaches 98%+ precision/recall on MBPP without running code — raising "when is actual execution necessary?" The survey's answer (pattern 3 below): simulation as fast path, execution as the binding oracle for what simulation "structurally cannot imagine" (crashes, resource exhaustion, boundary errors, perf regressions).
- **Synchronization mechanisms:** sequential handoff (most common; causes "invisible state divergence" under parallel modification), shared blackboards (L2MAC's append-only file store with a mediating Control Unit — "the most principled blackboard"), parallel branches with merge (MAGIS: one Developer per file; HyperAgent: Redis-queue fan-out merged at the planner), structured context scheduling, hierarchical memory, and pool scaling. QualityFlow is the *only* surveyed system managing state history/rollback (never overwrite the initial artifact; revert when the debugging trajectory degrades quality).

### 5.3 The position: a shared code-centric harness substrate (§4.3)

Four formalization levels of shared state: (1) **implicit/file-only** — the majority; state reconstructed from conversation history; belief-vs-truth divergence is invisible and uncorrectable; (2) **repository-based** — structure view (navigation tools, git-diff evolution memory; SyncMind is the only work formally defining ground-truth state S and measuring agent-belief divergence |B−S|); (3) **execution-based** — behavior view ("an objective oracle signal … not subject to hallucination"); (4) **blackboard/shared-state** — the closest to a formal substrate.

**The Central Gap:** "the program, uniquely among multi-agent domains, is an artifact that executes" — yet most systems never exploit this architecturally, "relying on agents to reason about code quality through natural language alone."

**Convergence criteria** (when is a multi-agent run done): correctness/test-gated (most principled — with the trap of converging on LLM-generated tests, i.e. passing your own biased green), security (zero CWE flags AND zero fuzzer crashes), performance thresholds, score-based (candidate-patch ensembles ranked and selected — Trae Agent), consensus (panel majority vote), and **implicit** (fixed iteration budgets, no quality criterion) — the *most prevalent* pattern and, per the authors, the field's most significant gap: "without an objective representation of the program state, systems have no principled criterion for convergence."

### 5.4 Patterns and trends (§4.4) — all seven

1. **The implicit-harness-state constraint** — implicit state is "the technical root of system brittleness rather than a scalability convenience."
2. **Code-mediated channels do not eliminate coordination bottlenecks** — every artifact channel trades fidelity/latency/scope; the real design question is "which artifacts are authoritative, how they are compressed, and how conflicts across channels are resolved."
3. **Execution feedback bridges linguistic and formal reasoning** — execution signals "cannot hallucinate"; mature design = linguistic fast path + execution oracle.
4. **Two complementary representations** — repository view (structure) vs execution view (behavior); *no surveyed system unifies both*.
5. **Topology complexity inversely correlates with harness-state formality** — formal substrate → simple topology (L2MAC); implicit state → elaborate adaptive topologies as workaround. "Topology complexity is partially a symptom."
6. **Context management is the tax of implicit shared state** — control units, pub-sub pools, tiered memories are all workarounds for a missing queryable state representation.
7. **Agent specialization increases the criticality of shared-state metrics** — role proliferation is "a forcing function for developing more mature shared harnesses."

## 6. Section 5 — Applications and Open Problems (read in full)

### 6.1 Code assistants (§5.1.1) — the flagship domain

- **Repository as operational substrate.** Modern assistants (SWE-agent, OpenHands, Claude Code, Codex, Copilot agents, DeepAgents) operate over repositories, not snippets; the hard problem is "constructing a task-specific working view over a large and evolving codebase" (RepoCoder, CodexGraph, AutoCodeRover).
- **Executable development harnesses are the control plane.** The managed loop — repository access, file edits, command execution, approval boundaries, context isolation, logging, validation — is what distinguishes production systems. MCP is cited as the standardization layer for exposing tools/context.
- **The harness itself is becoming an optimization object.** AutoHarness (synthesizes harnesses from environment feedback), Meta-Harness (searches over harness code using execution traces), Agentic Harness Engineering (evolves harness components via observability), Natural-Language Agent Harnesses (externalize roles/contracts/adapters into editable specs), Live-SWE-agent (edits its own scaffolding at runtime).
- **Execution feedback as grounded verification.** Agentless showed a plain fault-localization + patch pipeline guided by test execution is competitive on SWE-bench *without* elaborate agentic control. AlphaCodium: test-driven flow engineering beats single-shot prompting.
- **Latent state: developer intent and conventions.** A patch can pass tests and still be rejected for violating architecture/style — the "organicity" of generated code. Some "solved" SWE-bench issues are solution leakage in issue text, not genuine inference. Coding is framed as a **partially observable program world**: files/tests/tool outputs are observable; design rationale, conventions, and intent are latent state to be inferred.
- **Production datapoint:** Alibaba's LingmaAgent resolves 16.9% of in-house issues fully autonomously, 43.3% with manual intervention.
- **The harness as a distillation surface (2026 development).** Production harnesses are becoming *training-data factories*: Cursor Composer trained via online RL on real usage traces; codex-1 / GPT-5-Codex trained on long-horizon interactions mirroring the Codex harness loop; Anthropic's internal Claude Code dogfooding. "The boundary between the agent and the harness around the agent is becoming a learnable surface."
- **Open challenges specific to code assistants:**
  1. *Verification beyond unit tests* — the oracle-adequacy crisis (PatchDiff, SWE-Bench++), security-correctness gap (Aardvark, Codex Security), organicity gap.
  2. *Failure attribution* — best step-level attribution accuracy across studies is only **14–53%** (Why-do-MAS-fail, Who&When, AgenTracer, AgentDebug); production harnesses lack structured traces for principled debugging.
  3. *Safety governance* — capability-based least-privilege primitives are rare (Aethelgard learned capability governor, fault-tolerant transactional sandboxing, Microsoft Agent Governance Toolkit).
  4. *Harness self-evolution* — stability and rollback questions unanswered.
  5. *Multi-agent state sync on live repos* — humans + agents + CI concurrently mutate shared state (generalizes SyncMind belief-state divergence).
  6. *Trust calibration in pair programming* — when to interrupt, checkpoint, delegate, defer; understudied human-factors problem.

### 6.2 GUI/OS agents (§5.1.2)

Framed as the most literal "program world": every observation is rendered output of code, every action compiles to code. Formalized as a POMDP where the transition function is *executed, not learned* (browser engine / OS is the interpreter). Key pattern: **the evaluator is itself code** interrogating post-action system state (WebArena assertions, OSWorld per-task Python checkers, AndroidWorld adb inspection) — "code generates the environment, code is the agent's action, and code adjudicates the result." Memory appears as persistent programmatic state: skill libraries (Cradle), exploration documents (AppAgent), task-agnostic memory graphs (PlugMem). Sandboxes are "forkable, diffable, version-controlled, reproducible in ways no learned simulator can match."

### 6.3 Embodied agents (§5.1.3)

Code is both **grounding interface** (intent → embodiment-respecting commands) and **safety boundary** (constrains admissible actions at execution time), because physical failures can be silent. Layered harness: semantic reasoning (LLM) / admissibility boundary (typed robot APIs, planners) / perception-state estimation / low-level control. Skills-as-memory: a skill is "not merely something the agent reads, but something the agent re-executes" (Voyager lineage). The frontier problem has shifted "from generating skills to governing the library: forgetting, abstraction, grounding alignment."

### 6.4 Scientific discovery (§5.1.4)

Science as a partially observable program world: hypotheses as programs, protocols as XDL/Opentrons scripts, labs as runtimes ("the agent's policy is the code, the lab is the runtime, and the publication record is the log"). AlphaProof: every reasoning step is a Lean tactic verified before state transition. Self-driving labs (A-Lab: 41 novel compounds in 17 days; Coscientist; Chemputer/XDL as "LLVM IR for chemistry"). Benchmarks: MLE-bench (best scaffold reaches bronze-medal level on 16.9% of Kaggle competitions), ScienceAgentBench, DiscoveryBench (best ~25%).

### 6.5 Personalization (§5.1.5)

Weakest oracle domain: no reliable ground truth for user satisfaction (clicks are misleading proxies). Preference state as an **editable artifact** — structured, inspectable, user-correctable memory beats opaque embeddings. Multi-stakeholder reward conflicts and privacy governance are named open problems.

### 6.6 The seven open problems (§5.2) — the paper's research agenda

1. **Harness-level evaluation and oracle adequacy (§5.2.1).** End-task success conflates model, harness, tools, feedback, and environment difficulty. Proposed harness-level metric dimensions: *(i) trajectory efficiency* (tool calls, tokens, edits, wall-clock); *(ii) verification strength* (coverage, oracle diversity, false-acceptance rate); *(iii) recovery ability*; *(iv) state consistency* (memory/repo/traces/beliefs synchronized); *(v) safety compliance* (permissions, sandboxes, approval gates respected); *(vi) replayability* (trajectory reconstructable and auditable from logs).
2. **Semantic verification beyond executable feedback (§5.2.2).** "A harness can become overconfident precisely because it has executable feedback: the agent sees a green test, but the green test is not the full specification." Missing abstraction: a **verification stack with explicit scope** — each verifier declares *what it verifies, what it cannot verify, and what confidence it provides*. Every accepted action should carry an **evidence bundle**: checks run, assumptions preserved, untested regions, remaining risks. Verification is "not a final gate; it is an evolving, inspectable contract." Feedback should be routed by type (compiler error → syntax repair; test failure → behavioral diagnosis; coverage gap → test generation; reviewer conflict → arbitration) and the harness should be *epistemically aware* — know when a signal is strong enough to act on.
3. **Self-evolving harnesses without regression (§5.2.3).** "A harness mutation should be treated like a code change to a safety-critical runtime." Every proposed edit should carry a **change contract**: component modified, failure mode targeted, predicted improvement, invariants preserved, falsifying evaluation, rollback path. Research agenda: mutation operators, telemetry standards, held-out regression suites, safety invariants, canary deployment, causal evidence for why an edit helped. "The goal is not a harness that changes often, but one that changes only when it can justify the change."
4. **Transactional shared program state and semantic conflict resolution (§5.2.4).** Synchronization mechanisms "synchronize artifacts but not assumptions." Missing abstraction: **transactional shared program state** — each action declares its read set, write set, assumptions, version dependencies, verifier obligations, and conflict policy. Conflicts exist at the level of plans, tests, retrieved evidence, permissions, memory entries, latent user requirements — not just file diffs. Needed: semantic merge, rollback, dependency-aware locking, belief-state reconciliation, re-verification after merge. Metrics: merge success, semantic regression rate, rollback frequency, conflict recurrence, cost of human intervention.
5. **Human-in-the-loop safety and accountability as harness state (§5.2.5).** The harness as **safety governor**: classify actions by risk, enforce permission tiers, deny hard-constraint violations, require human approval for irreversible transitions. Multi-tier permission model (read/inspect → sandboxed edit/exec → network/API → shared repo → production), context-sensitive (same command safe in sandbox, unsafe in prod). Key inversion: human feedback should become **durable harness state** — each approval/rejection/policy exception updates permission rules, escalation policy, verification criteria. High-stakes approvals as auditable state transitions. "Executable accountability."
6. **Multimodal code-harness systems (§5.2.6).** Multimodal observations as persistent, queryable, verifiable state; grounding contracts linking perception → action → verification; each feedback signal exposing scope and uncertainty; skills as (multimodal precondition, executable action, expected postcondition) triples.
7. **Toward a science of harness engineering (§5.2.7).** The object of study is the complete closed-loop system. The four properties of future systems: **executable, inspectable, stateful, governed**.

## 7. Critical assessment

**Strengths.**
- The three-way capability/infrastructure/artifact decomposition is genuinely clarifying — it separates what the model does, what the platform provides, and what the agent builds for itself mid-task.
- §5.2 is the strongest part: it converts vague reliability talk into named missing abstractions (verification stack with scope, evidence bundles, change contracts, transactional shared state, executable accountability) that are directly buildable.
- Honest about negative evidence: failure attribution at 14–53%, SWE-bench leakage, oracle inadequacy, the green-test overconfidence trap.

**Weaknesses.**
- It is a survey-position hybrid: the "position" (§4.3) and open problems are prescriptive design sketches, not validated results. None of the proposed abstractions carry empirical support in the paper itself.
- Taxonomy inflation: many subsections are re-labelings of familiar ideas (memory taxonomies, planning taxonomies) whose boundaries blur in practice.
- The citation set leans heavily on 2025–2026 preprints; several load-bearing systems (AutoHarness, Meta-Harness, Aethelgard, AHE) are cited from anonymous or very recent submissions — treat capability claims as provisional.
- Little engagement with cost: verification stacks, evidence bundles, and transactional state all carry token/latency/complexity budgets the paper does not price.

**Verdict.** As evidence, treat it as a *map of the field's consensus direction*, not proof any mechanism works. As design input, §3.4–3.5, §4.3–4.4, and §5.2 are a high-density source of vocabulary and architecture patterns for exactly the product space AgentOps occupies.

## 8. Methodology note

Full text (261KB) extracted from the arXiv HTML to plain text. Abstract, introduction, §5 (applications + open problems) read directly in the orchestrating context; §2, §3, §4 read by three parallel analyst subagents whose structured notes were synthesized into §§3–5 of this doc. Named systems and quoted phrases were taken from the paper text, not reconstructed from memory.
