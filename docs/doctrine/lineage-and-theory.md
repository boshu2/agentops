# Lineage and theory (doctrine)

> **Internal doctrine — not a public surface.** Relocated out of `PRODUCT.md` (bead
> `age-focus-membrane-bookkeeper-m1wg.9`) so the public product read keeps Mission,
> what-it-does, personas, honest evidence, and gaps — while the systems-theory lineage,
> the design-principle pillars, and the navigator/windshield framing live here for the
> reader who wants where the shape came from. Users do not need this vocabulary to use
> the product. Canonical product positioning: [PRODUCT.md](../../PRODUCT.md).

## GPS for agentic work, not a workflow

The deepest "why" under the corpus and the gates is this: **agentic workers are inherently stochastic, so AgentOps is a goal-directed navigator, not a workflow.** A deterministic worker — compiled code — takes rails: the route *is* the track, and a script, DAG, or pipeline is the right shape. An agent has no rails. Told "turn left," it has real probability of confidently driving into the lake. So the orchestration paradigm **inverts**:

- **Orchestration determinism runs inverse to worker determinism.** Deterministic worker → script / DAG / pipeline (rails). Stochastic worker → a navigator that *assumes* deviation and self-corrects continuously.
- **You don't script the route — you set the destination.** A goal is a destination — acceptance, "done" — not a turn-by-turn workflow. Linear `crank` / `swarm` waves are rails: the brittle special case for when scopes happen not to collide. The general shape is a **goal-directed traversal of a deterministic role-topology with live re-routing**: the *map* (roles, stages, legal transitions, gates) is fixed and trusted; the *route* (the path a given goal takes) is dynamic — chosen at each node, recalculated on failure.
- **Trust the environment, not the agent.** Trust the GPS — the deterministic map plus the gates — not the driver. The environment supplies the reliability the agent structurally cannot.
- **The windshield is non-negotiable.** The agent's signature failure is not a wrong turn on a real map; it *hallucinates roads that do not exist* — invents an API, a file, a fact, and drives toward it confidently. Re-routing cannot save you from a road that was never there. Only **deterministic ground-truth** — the gate, the eval, the test that actually runs — is the windshield that catches the lake. This is why the validation-gate / evidence layer is the load-bearing floor, not an optional add-on.

This is the reliability-into-stochastic-systems thesis stated as architecture: you do not make the *agent* reliable — stochastic is what it is — you build the navigator that makes the *work* reliable in spite of it.

## Lineage

The internal lineage that produced this product, and the parallels we are *not* derived from. Users do not need this vocabulary; it records where the shape came from.

### The hierarchy

**Knowledge OS → Olympus → AgentOps → Mt. Olympus.**

- **Knowledge OS** is the systems-theoretic substrate. The dK/dt equation, stigmergy as the multi-agent coordination primitive, Meadows' leverage-point hierarchy as the design discipline. This is the body of theory the rest descends from.
- **Olympus** was the predecessor runtime. Power-user daemon, run ledger, context compilation, constraint injection. Archived as a live system; its patterns survived as skills inside AgentOps.
- **AgentOps** (this repository) is the coding-agent implementation. Skills + execution packets + `ao` CLI + explicit validation gates. It applies the context-compounding model to software work; always-on scheduling is delegated to a substrate.
<!-- agentops:claim:AOP-CLAIM-PRODUCT-MT-OLYMPUS-PROOF -->
- **Mt. Olympus** is not a separate running product. It is (1) the **lineage** of the in-repo ratchet pawl-gate — the typed-loop / explicit-ratchet-rules work (2026) the gate descends from — and (2) a parked, optional **high-assurance Linux build** (binding daemon, OS-account isolation) for adversarial multi-tenant settings. The acceptance gate it pioneered now ships inside AgentOps as the in-repo pawl-gate; there is no `olympusd` service in the running factory.

### Why Meadows, foregrounded

Donella Meadows' *Twelve Leverage Points* ranks intervention points in complex systems from weakest (#12, parameters) to strongest (#1, transcending paradigms). **Changing the loop beats tuning the output.** AgentOps targets the high-leverage end — #4 (self-organization) and #3 (goals) — through the knowledge flywheel and `GOALS.md` reconciliation, rather than #12 (a better prompt). This is the primary organizing principle, not a citation: the entire CDLC is built around moving leverage up Meadows' hierarchy.

### Compound engineering / software factories

The thread-based development pattern — multiple agents working compoundingly, validation gates between phases, learnings extracted into reusable skills — applied via the **software-factory operator pattern**. The lineage runs through Greenfield and Short's *Software Factories: Assembling Applications with Patterns, Models, Frameworks, and Tools* (2003): a factory configures and composes domain-specific assets. AgentOps configures and composes context, skills, and validation gates around an operator's codebase. Direct comparison against EveryInc's Compound Engineer at [docs/comparisons/vs-compound-engineer.md](../comparisons/vs-compound-engineer.md).

### Parallel, not derived from

- **Heroku's Twelve-Factor App.** Parallel to, not derived from. The 12-factor app describes stateless web processes managed by a control plane; AgentOps applies the same shape — environment-carried continuity, replaceable workers, explicit control plane — to coding agents. Same operating-style insight, different substrate.
- **Anthropic's Managed Agents** (May 2026), **Cursor agents**, **Factory's *Missions***. Convergent, not derived-from. Multiple teams arriving at planner/implementer/validator separation, dreaming/memory loops, and rubric-graded outcomes is evidence the architecture is correct — not lineage. AgentOps' position is the cross-runtime, repo-native, operator-sovereign substrate.

## Design Principles

**Theoretical foundation — six pillars:**

1. **[Systems theory (Meadows)](https://en.wikipedia.org/wiki/Twelve_leverage_points)** — *The* primary organizing principle, not a citation. **Changing the loop beats tuning the output** — Meadows leverage point #4 (self-organization) and #3 (goals) vs. #12 (parameters). AgentOps is built as a Meadows compounding system around the user's codebase: information flows captured (#6), rules encoded (#5), self-organization through the flywheel (#4), goals declared (#3). Most agent tooling lives at #12; AgentOps lives at #4–#3.
2. **[DevOps Three Ways](../the-science.md#part-3-devops-foundation-the-three-ways)** — Flow, feedback, continual learning. The discipline lineage. Applied to the agent loop instead of the deploy pipeline.
3. **SRE (SLOs + error budgets)** — Reliability is a measurable condition, not a vibe. `GOALS.md` carries SLO-shaped fitness gates; `ao goals measure` is the burn-rate equivalent. The reliability lineage. Source: *Site Reliability Engineering* (Beyer, Jones, Petoff, Murphy).
4. **Kubernetes control loops** — Declared state + reconcile loop. `GOALS.md` declares; `/evolve` reconciles. Errors don't crash the loop; they enter the work queue. The self-correction lineage.
5. **[Brownian Ratchet](../brownian-ratchet.md)** — Embrace agent variance, filter aggressively, ratchet successes. Chaos + filter + one-way gate = net forward progress. The forward-only-progress lineage.
6. **[Knowledge Flywheel (escape velocity)](../the-science.md#the-escape-velocity-condition)** — If retrieval rate × usage rate exceeds decay rate, knowledge compounds. If not, it decays to zero. The compounding-context lineage. *This is the one infrastructure never needed* — software workers persist; agents don't. The corpus is the asset that stays yours, and the candidate moat — unproven until the delta is measured (see [Strategic Bet](../../PRODUCT.md#strategic-bet)).

**Operational principles:**

1. **Agents are ephemeral; the system carries the state.** Every skill, hook, and flywheel component exists because the agent itself can't remember. Build for amnesia.
2. **The corpus is the user's. The harness is ours.** AgentOps' own commoditization is on the timeline. The user's accumulated knowledge isn't. Optimize the product for what the user keeps.
3. **Context quality determines output quality.** Right context, right window, right time. Phase-specific. Role-scoped. Freshness-weighted.
4. **The cycle is the product.** No single skill is the value. The compounding loop — research, plan, validate, build, validate, learn, repeat — is what makes the system improve.
5. **Two-tier execution.** Orchestrators (`/evolve`, `/rpi`, `/crank`) stay in the main session. Workers fork into subagents where results merge back via the filesystem — never accumulated chat context.
6. **Atomic changes compose.** Every primitive is cheap to undo. The Brownian Ratchet only works if the ratchet step is small.
7. **Reconcile, don't push.** Kubernetes-shaped control loops compare actual state to desired state and fix the gap. They don't fire-and-forget. AgentOps loops do the same.
8. **Dormancy is last resort.** When goals pass and backlog is empty, the system generates productive work from validation gaps, bug hunts, drift detection, and feature suggestions before going dormant.
