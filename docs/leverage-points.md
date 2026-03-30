# Leverage Points: Meadows' 12 Mapped to AgentOps

> Donella Meadows ranked the places to intervene in a complex system from least to most powerful. AgentOps concentrates on the high-leverage end because changing the loop beats tuning the output.

**Reference:** Meadows, D. H. (1999). "Leverage Points: Places to Intervene in a System." *Sustainability Institute*.

---

## The 12 Leverage Points

### #12 — Constants, Parameters, Numbers

**Meadows:** The numerical values that define the system -- rates, quantities, thresholds. The knobs everyone reaches for first and the ones that matter least.

**AgentOps implementation:**

| Parameter | Value | Where defined |
|-----------|-------|---------------|
| Literature decay constant | 0.17/week | Darr (1995); motivates the escape threshold |
| Operational delta | avg_age_days / 100 | `ao metrics health` (`cli/cmd/ao/metrics_health.go`) |
| Retrieval target (sigma) | 0.7 | `ao metrics health` escape velocity threshold |
| Citation target (rho) | 0.3 | `ao metrics health` escape velocity threshold |
| Context load ceiling | 40% of window | Hook enforcement (35% warn, 40% hard stop) |
| Summary budget | 500 tokens | Briefing packet assembly (`ao context assemble`) |
| Max waves per epic | 50 | `/crank` FIRE loop global limit |
| Max retries per gate | 3 | Gate retry logic in validation hooks |
| Confidence decay | 10%/week | Learning freshness scoring in `ao lookup` |
| Circuit breaker | 60 minutes | `/evolve` stops if no productive cycle in 60 min |

**dK/dt mapping:** These tune `delta`, `phi`, and the operating bounds of `sigma`. Changing them shifts the curve; it does not change the shape of the system.

**Status:** Implemented. All values are configurable but defaults are evidence-based.

---

### #11 — Buffer Sizes

**Meadows:** The sizes of stabilizing stocks relative to their flows. Buffers absorb shocks. Too small and the system oscillates; too large and it becomes sluggish.

**AgentOps implementation:**

- **Context guard** — 35% warn threshold, 40% hard stop. Prevents the "lost in the middle" retrieval collapse (Liu et al. 2023). Implemented in session-start hook and context assembly.
- **Knowledge tiering** — Gold/silver/bronze tiers in `.agents/learnings/`. Gold learnings are always injected; bronze are available but not proactively loaded. Controls the hot-set size.
- **Idle streak detection** — `/evolve` tracks consecutive idle cycles from `cycle-history.jsonl`. At threshold, the system stops rather than wasting cycles. This is a buffer against runaway autonomous loops.
- **`.agents/` corpus size** — The physical K stock. Tiering and pruning (`ao maturity --expire`) prevent the buffer from growing past the point where retrieval degrades.

**dK/dt mapping:** Bounds `K` to prevent `sigma(K,t)` collapse. The context guard specifically prevents the buffer from becoming so large that information is lost in the middle.

**Status:** Implemented. Context guard and tiering are active. Corpus size monitoring via `ao metrics health` knowledge_stock.

---

### #10 — Stock-and-Flow Structure

**Meadows:** The physical arrangement of stocks, flows, and their interconnections. The plumbing of the system. Hard to change once built.

**AgentOps implementation:**

The knowledge stock `K` lives in `.agents/`. Its structure:

```
.agents/
  learnings/     -- I(t) deposits here via ao forge
  patterns/      -- Reusable solutions extracted from learnings
  constraints/   -- Compiled rules (constraint-compiler.sh)
  retros/        -- Retrospective summaries
  council/       -- Validation verdicts
  research/      -- Exploration findings
  plans/         -- Decomposed epics
  ao/            -- Session index, citations, metrics (JSONL)
```

**Flows:**
- **Inflow:** `ao forge` (session learnings), `/retro`, `/post-mortem` deposit into `I(t)`
- **Outflow (decay):** `ao maturity --expire` removes stale artifacts, freshness scoring deprioritizes old knowledge
- **Reinforcement:** `ao lookup` retrieves from stock on demand, citation tracking records usage, MemRL utility scoring adjusts future retrieval priority
- **Friction:** As `K` grows, retrieval quality degrades without active scale controls (tiering, pruning, re-indexing)

**dK/dt mapping:** This IS the physical equation. `K` = `.agents/` corpus. `I(t)` = forge inflow. `delta * K` = expiry outflow. `sigma * rho * K` = retrieval-citation compounding. `phi * K^2` = scale friction.

**Status:** Implemented. The stock-and-flow structure is the architectural foundation.

---

### #9 — Delays

**Meadows:** The lengths of time relative to the rates of change. Delays in feedback loops cause oscillation. If information about the state of a stock takes too long to reach the decision point, the system overshoots.

**AgentOps implementation:**

- **Phase boundaries (R to P to I to V)** — The RPI lifecycle enforces sequential phases: Research, Plan, Implement, Validate. Each phase must complete before the next begins. This is a deliberate delay that prevents premature implementation. The cost of finding a bug increases 10x at each stage it survives.
- **Circuit breaker (60 min)** — `/evolve` stops if no productive cycle occurred in the last 60 minutes. This prevents the system from oscillating between idle cycles indefinitely. Implemented as a timestamp check against `cycle-history.jsonl`.
- **Confidence decay (10%/week)** — Learning freshness scores decay over time, creating a delay between when knowledge was created and when it becomes effectively invisible to retrieval. This matches Ebbinghaus's forgetting curve.
- **Stale run TTL** — Sessions that do not close cleanly have their state cleaned up by the pending-cleaner hook after a timeout. Prevents stale state from contaminating future sessions.
- **Maturity lifecycle** — `ao maturity --expire` implements time-delayed eviction. Knowledge that is not retrieved within its TTL decays out of the active set.

**dK/dt mapping:** Controls the lag between `I(t)` and usable `K`. Phase boundaries create healthy delays (validation before deployment). Confidence decay and maturity lifecycle create the `delta * K` drain term.

**Status:** Implemented. Phase boundaries, circuit breaker, and confidence decay are all active.

---

### #8 — Balancing Feedback Loops

**Meadows:** Negative feedback loops that keep stocks within bounds. Thermostats. The strength of these loops relative to the impacts they are trying to correct determines whether the system stays stable.

**AgentOps implementation:**

| Loop | Mechanism | What it balances | Files/commands |
|------|-----------|------------------|----------------|
| **B1: Freshness decay** | Knowledge decays at ~17%/week without retrieval | Prevents stale knowledge from polluting decisions | `ao maturity --expire`, freshness scoring in `ao lookup` |
| **B2: Scale friction** | As K grows, retrieval quality degrades and governance cost rises | Prevents corpus bloat from collapsing sigma | Tiering, pruning, MemRL utility scoring (`ao feedback`) |
| **Regression gates** | `/evolve` snapshots fitness before each cycle; regression = automatic revert | Prevents improvement cycles from making things worse | `ao goals measure`, fitness snapshot comparison |
| **Council FAIL** | Multi-model council returns FAIL verdict; blocks merge | Prevents bad code from locking into ratchet | `/vibe`, `/council` verdicts in `.agents/council/` |
| **Push gate** | Hook blocks `git push` if `/vibe` has not passed | Prevents unvalidated code from reaching main | `hooks/push-gate.sh` |
| **Pre-mortem gate** | Hook blocks `/crank` if `/pre-mortem` has not passed | Prevents implementation of unvetted plans | `hooks/pre-mortem-gate.sh` |

**dK/dt mapping:**
- B1 creates the `delta * K` term — the constant drain that R1 must overcome.
- B2 creates the `phi * K^2` term — the scale friction that grows superlinearly with knowledge stock.
- Regression gates, council FAIL, and push gate prevent negative `dK/dt` spikes (regressions that would destroy validated knowledge).

**How `ao metrics health` maps:** `delta` (average learning age) measures B1 drain pressure. When `delta` is high, more knowledge is old and decay is winning. The `loop_dominance` field shows `B1` (decayed/session) directly. When `B1 > R1`, the system reports `dominant: "B1"` — balancing loops are winning.

**Status:** Implemented. All six balancing loops are active and mechanically enforced.

---

### #7 — Reinforcing Feedback Loops

**Meadows:** Positive feedback loops that amplify change. Compound interest. Vicious and virtuous cycles. The strength of the gain around the loop determines how fast the system grows or collapses.

**AgentOps implementation:**

**R1: The Knowledge Flywheel**

```
retrieve (ao lookup --query "topic")
    |
    v
use in session (citation)
    |
    v
stronger priors (reinforced knowledge survives decay)
    |
    v
better future retrieval (higher utility scores)
    |
    +---> ao forge extracts new learnings
    |         |
    v         v
retrieve (ao lookup) ... [loop repeats]
```

This is the `sigma * rho * K` compounding term. Each retrieval-and-use cycle:
1. Reinforces the retrieved knowledge (Ebbinghaus — retrieval slows decay)
2. Creates new knowledge from the application
3. Improves utility scores for retrieved items (MemRL — `ao feedback`)
4. Makes future retrieval more effective (higher sigma)

The `* K` multiplier means it is proportional to existing stock. More knowledge, more compounding — until scale friction (B2) intervenes.

**How `ao metrics health` maps:**
- `sigma` (retrieval effectiveness) measures R1's input quality — are you finding what you need?
- `rho` (citation rate) measures R1's conversion — are you using what you find?
- `sigma * rho` is the compound rate. Target: 0.21 (0.7 x 0.3).
- `escape_velocity` = `sigma * rho > delta/100` — true means R1 dominates B1.
- `loop_dominance.R1` (new/session) shows R1's output rate directly.
- `loop_dominance.dominant` shows which loop is currently winning.

When `dominant: "R1"`, the flywheel is spinning faster than decay can drain it. This is the system's primary health indicator.

**Status:** Implemented. The flywheel is the core product mechanism. `ao metrics health` provides real-time R1/B1 visibility.

---

### #6 — Information Flows

**Meadows:** Who has access to what information. A new information flow — delivering information to a place where it was not going before — is a powerful intervention, often more effective than adjusting parameters or even strengthening feedback loops.

**AgentOps implementation:**

| Flow | From | To | Mechanism | Why it matters |
|------|------|----|-----------|----------------|
| Knowledge injection | `.agents/learnings/` | Session context | `ao lookup` (freshness-weighted, utility-scored, on demand) | Session N knows what session 1 learned |
| Knowledge extraction | Session output | `.agents/learnings/` | `ao forge` (hook-enforced at session end) | Experience survives session death |
| Briefing packets | Prior research/plans | Agent context | `ao context assemble` (500-token summaries) | Right information, right phase, right agent |
| Least-privilege loading | Full knowledge stock | Filtered subset | Phase-based and role-based filtering | Prevents lost-in-the-middle; context as security boundary |
| Ralph Wiggum | Previous wave state | New wave workers | Fresh context per wave (zero bleed-through) | Workers reason from clean state, not accumulated garbage |
| Hook nudges | System state | Agent prompt | PostToolUse/UserPromptSubmit hooks | "Run /vibe before pushing" — invisible except when needed |
| Finding registry + compiled prevention | `.agents/findings/registry.jsonl`, `.agents/planning-rules/*.md`, `.agents/pre-mortem-checks/*.md`, `.agents/constraints/index.json` | `/plan`, `/pre-mortem`, `/vibe`, `/post-mortem`, `task-validation-gate.sh` | skill contracts + `hooks/finding-compiler.sh` + `docs/contracts/finding-registry.md` | Reusable failures become earlier planning/review checks, and mechanically detectable ones can become active validation blockers |

**The 40% Rule as an information flow control:** The context guard is not just a buffer control (#11). It is fundamentally an information flow decision: what gets loaded and what does not. At 40% capacity, the system must choose. That choice — freshness-weighted, utility-scored, phase-scoped — determines sigma.

**dK/dt mapping:** Directly increases `sigma` by getting the right knowledge to the right window at the right time. Also increases `rho` by making retrieved knowledge more relevant to the current task (phase scoping reduces noise).

**Status:** Implemented. All seven information flows are active. The prevention-oriented seventh flow now spans registry intake and compiled prevention outputs: planning and judgment load compiled artifacts first, and task validation consumes active constraint index entries for mechanically detectable findings. `ao context assemble` and `ao lookup` remain the primary CLI delivery mechanisms for broader knowledge retrieval.

---

### #5 — Rules

**Meadows:** The incentives, punishments, and constraints that govern the system. Rules change behavior without changing the physical structure. They are powerful because they shape every transaction within the system.

**AgentOps implementation:**

AgentOps has 3 active runtime hooks in `hooks/hooks.json` that enforce the core knowledge lifecycle mechanically. The agent does not decide whether to follow them. The system enforces them.

| Rule | Enforcement | What it prevents |
|------|-------------|------------------|
| Session start hook | Runs `hooks/session-start.sh` on `SessionStart` | Cold starts without prior knowledge |
| Session end maintenance hook | Runs `hooks/session-end-maintenance.sh` on `SessionEnd` | Lost session learnings and stale pools |
| Stop close-loop hook | Runs `hooks/ao-flywheel-close.sh` on `Stop` | Unclosed flywheel feedback cycles |
| Task validation constraint hook | Runs `hooks/task-validation-gate.sh` on `TaskCompleted` | Mechanically detectable failures escaping validation despite prior findings |

Guardrail hook scripts (push gate, worker guard, pre-mortem gate, etc.) remain in the repo and can be re-enabled by manifest policy, but they are not part of the active runtime contract by default.

**Additional rules:**
- **Sisyphus rule** — Completion requires an explicit marker. The agent cannot claim "done" without the system agreeing. Prevents premature completion claims.
- **Max 50 waves** — Global wave limit prevents infinite execution loops in `/crank`.
- **Strike check** — Skip goal after 3 consecutive failures. Prevents infinite retry on fundamentally broken goals.
- **Kill switches** — `AGENTOPS_HOOKS_DISABLED=1` (all hooks), deploy kill file (stops `/evolve`). Every autonomous loop has a manual override.

**dK/dt mapping:** Rules do not appear directly in the equation, but they prevent catastrophic dK/dt events — regressions that would send K to zero. They also enforce the information flows (#6) that keep sigma high. Without rules, the flywheel depends on agent memory. Agents forget. Rules do not.

**Status:** Implemented. The session-boundary hooks and task-validation constraint hook are enforced. All kill switches tested.

---

## The Organizing Insight: #5 to #4 (Rules to Self-Organization)

This is the product insight.

Simple rules produce complex behavior that evolves. The seed encodes rules (#5). Emergence produces self-organization (#4). This transition — from mechanical enforcement to adaptive behavior — is what separates AgentOps from a checklist.

Consider what happens:
1. **Rules** (#5): Hooks enforce the extract-inject cycle. Every session deposits learnings. Every session retrieves them. The agent has no choice.
2. **Information flows** (#6): The flywheel delivers the right knowledge to the right context. Sigma increases.
3. **Reinforcing feedback** (#7): Retrieved knowledge that is used gets reinforced. Utility scores rise. Future retrieval improves. The flywheel accelerates.
4. **Self-organization** (#4): `/evolve` measures fitness, picks the worst gap, fixes it, validates nothing regressed, extracts what it learned. The system's rules change based on its own experience.

The rules do not specify what the system should build. They specify how it should learn. The fitness landscape (GOALS.md) determines what gets built. The rules determine that whatever gets built also produces knowledge that compounds.

This is why the product is the seed, not the tree. The same 6 seed elements — GOALS.md, `.agents/`, hooks, CLAUDE.md section, core skills, bootstrap learning — planted in different repos produce different systems. The rules are identical. The emergent behavior differs because the goals differ.

Fractal composition is the structural manifestation of this insight:

```
attempt -> validate -> learn -> constrain
```

The same shape at every scale (function, issue, epic, repository) means rules at one level produce self-organization at the level above. `/implement` follows rules. `/crank` orchestrates `/implement` waves and the orchestration itself evolves. `/evolve` runs `/crank` cycles and the cycle selection itself adapts to measured fitness.

---

### #4 — Self-Organization

**Meadows:** The ability of a system to add to, change, or evolve its own structure. The most powerful property of living systems. Self-organization produces complexity from simplicity.

**AgentOps implementation:**

| Mechanism | What self-organizes | How |
|-----------|--------------------|----|
| `/evolve` fitness loop | Goal pursuit strategy | Measures all goals, selects worst by severity weight, fixes it, validates no regression, learns. Next cycle's choice depends on this cycle's result. |
| Finding registry ratchet | Planning, review, and validation context | Structured findings enter through `.agents/findings/registry.jsonl`, compile into promoted artifacts plus planning/pre-mortem outputs, and mechanical active findings feed `.agents/constraints/index.json` for `task-validation-gate.sh`. |
| `/forge` pattern extraction | Knowledge taxonomy | Extracts reusable patterns from sessions. The pattern library grows and changes shape based on what the system encounters. |
| Skill composition | Capability surface | Skills chain: `/research` -> `/plan` -> `/pre-mortem` -> `/crank` -> `/vibe` -> `/post-mortem`. The chain is fixed but each skill adapts its behavior to its inputs. |
| Progressive skill revelation | User-visible surface | New users see 8 starter skills. The remaining skills reveal as the user grows. The system's visible complexity adapts to the user's readiness. |
| Severity-weighted goal selection | Priority ordering | `ao goals measure` scores all goals by weight. `/evolve` works the highest-weight failure first. The priority order changes every cycle based on measurement. |

**The finding registry deserves emphasis.** It is the mechanism that converts #7 (reinforcing feedback - learnings) into earlier-stage prevention. When a failure is normalized into a structured finding, it no longer stays as prose: the compiler promotes it into reusable planning/review artifacts, and mechanically detectable active findings can also become task-validation constraints. The important limit is not "registry versus enforcement" anymore; it is "advisory for ambiguous findings, enforced for the mechanical subset."

**dK/dt mapping:** Self-organization does not appear in the equation because it changes the equation itself. When the finding registry promotes a failure into reusable planning/review artifacts, it changes the system's `sigma` by improving retrieval focus and its `rho` by making reuse happen earlier in the lifecycle. When the same finding becomes an active mechanical constraint, it also reduces `phi` by preventing repeated validation misses from compounding into governance friction.

**Status:** Implemented in part. `/evolve` is production-tested (116 cycles, ~7 hours continuous). Registry intake, compiled planning/review reuse, and active task-validation enforcement for mechanically detectable findings are all live. The remaining limit is selectivity, not absence: many findings will always stay advisory because they cannot be compiled safely.

---

### #3 — Goals

**Meadows:** The purpose or function of the system. What the system is trying to achieve. Changing the goal of a system changes everything about its behavior, even if every parameter, feedback loop, and rule stays the same.

**AgentOps implementation:**

`GOALS.md` in the repo root. Mechanically verifiable. No subjective scoring.

```markdown
## Directives
### 1. Increase test coverage
### 2. Reduce complexity hotspots

## Gates
| ID | Check | Weight | Description |
|----|-------|--------|-------------|
| tests-pass | cd src && make test | 8 | All tests pass |
| coverage-floor | ./scripts/check-coverage.sh --min=70 | 6 | Coverage above 70% |
```

**Key properties:**
- Gates have shell commands that exit 0 (pass) or non-zero (fail). Binary, not subjective.
- Weight determines severity. Higher weight = higher priority when multiple gates fail.
- `ao goals measure` runs all gates, reports status, feeds `/evolve` cycle selection.
- `ao goals steer add/remove/prioritize` manages directives.
- The meta-goal `sigma * rho > delta/100` is the operational escape velocity condition — the system's implicit goal across all repos.

**The dormancy-is-success property:** A well-evolved system has all gates passing. `/evolve` finds nothing to fix. This is not stagnation — it is the designed end state. The system worked itself out of a job for the current goal set. New goals restart the cycle.

**dK/dt mapping:** Goals define what `I(t)` should target. Without goals, forge input is random — every session produces knowledge, but no session produces knowledge that serves a coherent direction. Goals make the input term directional.

**Status:** Implemented. `ao goals init`, `ao goals measure`, `ao goals steer` are all active.

---

### #2 — Paradigms

**Meadows:** The shared assumptions, beliefs, and mental models out of which the system arises. The deepest source of system behavior. Paradigms are harder to change than anything else about a system, and nothing else produces such broad change.

**AgentOps embodies 6 paradigm shifts:**

| # | From | To | Lever |
|---|------|----|-------|
| 1 | Reduce variance | Harness variance (Brownian Ratchet) | Spawn parallel attempts, filter aggressively, ratchet successes |
| 2 | Context is infinite | Context is scarce (40% Rule) | Treat context as a security boundary, least-privilege loading |
| 3 | Validation is post-hoc | Validation is preventive (Shift-Left) | `/pre-mortem` before implementation, hooks at every gate |
| 4 | Rules are guidelines | Rules are structural (Hooks) | 3 active runtime hooks that cannot be forgotten, skipped, or rationalized away |
| 5 | Knowledge is hoarded | Knowledge is flowing (Flywheel) | Extract, score, tier, decay, re-inject across sessions |
| 6 | Designed systems | Evolved systems (The Seed) | Define starting conditions, let the system evolve toward goals |

**The 6th Paradigm Shift: From Designed to Evolved**

This is the foundational shift. The others are consequences of it.

A designed system is specified upfront and built to spec. An evolved system is given starting conditions and a fitness function, then adapts. The difference:

- **Designed:** Someone decides the order of operations. Someone decides when to test. Someone decides what to refactor.
- **Evolved:** Severity-weighted goal selection naturally produces the correct sequence. The system builds its own safety net first (tests pass before refactoring begins), then uses that safety net to move aggressively. Nobody tells it the order.

The seed does not design outcomes. It creates conditions for emergence:
- GOALS.md defines the fitness landscape
- Hooks enforce the rules of engagement
- The flywheel provides the memory mechanism
- `/evolve` provides the selection pressure

Given these conditions, the system produces different outcomes in different repos — just as the same genetic machinery produces different organisms in different environments.

This is why the product is 6 seed elements, not 53 skills. The skills are the phenotype. The seed is the genotype.

**Status:** Implemented. All 6 paradigm shifts are embedded in the architecture. The 6th (designed to evolved) is realized through `/evolve` and the seed definition (see [seed-definition.md](seed-definition.md)).

---

### #1 — Transcending Paradigms

**Meadows:** The ability to step outside all paradigms and see them as mental models, not truth. The realization that no paradigm is "correct" — each is a limited way of seeing. This is the highest leverage point because it frees the system from being trapped in any single worldview.

**AgentOps implementation:**

Two mechanisms approach this level:

**Meta-observer pattern** — Autonomous workers coordinate through shared memory (stigmergy) without central orchestration. The meta-observer watches from outside the system, synthesizes findings across workers, and intervenes only when workers are blocked. The observer is not part of the work — it observes the work happening and learns from the pattern of coordination itself. See [meta-observer-pattern.md](workflows/meta-observer-pattern.md).

**Stigmergy coordination** — Workers leave traces in shared state (`.agents/`) that influence other workers' behavior without direct communication. No worker knows the full system state. No coordinator directs the full system. The system's behavior emerges from local interactions with shared artifacts — the same principle that governs ant colonies.

**The honest assessment:** Level #1 is aspirational. True paradigm transcendence would mean the system can recognize when its own organizing metaphors (the seed, the flywheel, the ratchet) are limiting and replace them. AgentOps does not do this yet. The finding registry (#4) can now promote learnings into reusable advisory checks and active task-validation constraints, and `/evolve` can change what it works on, but neither can change the fundamental assumptions of the system itself.

What exists is the infrastructure for it: append-only logs that let a future meta-observer analyze whether the system's paradigms are serving it.

**Status:** Partial. Meta-observer pattern and stigmergy are implemented. True paradigm transcendence is a research direction.

---

## The dK/dt Equation Through the Lens of Leverage Points

```
dK/dt = I(t) - delta * K + sigma(K,t) * rho * K - phi * K^2
```

Each term maps to specific leverage points:

### I(t) — Input Rate

| Leverage Point | How it affects I(t) |
|----------------|---------------------|
| #12 (Parameters) | Token budgets and summary lengths bound how much can be forged per session |
| #7 (R1 loop) | The flywheel's reinforcing loop means each retrieval-use cycle generates new learnings, increasing I(t) |
| #6 (Info flows) | `ao forge` extracts knowledge at session end; `ao context assemble` ensures research findings reach planners |

### delta * K — Decay Drain

| Leverage Point | How it affects decay |
|----------------|---------------------|
| #12 (Parameters) | delta = 0.17/week (Darr 1995), confidence decay = 10%/week |
| #8 (B1 loop) | Freshness decay is the primary balancing loop draining the stock |
| #9 (Delays) | Maturity lifecycle and expiry TTL control how quickly decay acts |

### sigma(K,t) × rho × K — Compounding Term

| Leverage Point | How it affects compounding |
|----------------|---------------------------|
| #6 (Info flows) | Least-privilege loading, phase scoping, and context assembly determine what gets retrieved (sigma) |
| #8 (B2 via MemRL) | Utility scoring (`ao feedback`) adjusts retrieval priority, preventing sigma collapse at scale |
| #7 (R1 loop) | This IS the R1 loop. Retrieval × usage × existing stock = compound growth |
| #5 (Rules) | Hooks enforce the extract-inject cycle that keeps sigma and rho nonzero |

### phi * K^2 — Scale Friction

| Leverage Point | How it affects friction |
|----------------|------------------------|
| #8 (B2 loop) | Tiering, pruning, and re-indexing are the active controls against B2 |
| #11 (Buffers) | Context guard prevents the buffer from growing past retrieval collapse |
| #12 (Parameters) | Tier thresholds and pruning aggressiveness are tunable parameters |

---

## How `ao metrics health` Maps to #7 and #8

The `ao metrics health` command (implemented in `cli/cmd/ao/metrics_health.go`) is a direct instrument panel for leverage points #7 and #8.

```
$ ao metrics health

Flywheel Health
===============

RETRIEVAL:
  sigma (retrieval effectiveness): 0.700    <-- R1 input quality
  rho   (citation rate):           0.300    <-- R1 conversion rate
  delta (avg learning age, days):   14.2    <-- B1 drain pressure

ESCAPE VELOCITY:
  sigma * rho = 0.2100                      <-- R1 compound rate
  delta / 100 = 0.1420                      <-- B1 threshold
  status:       COMPOUNDING [+]             <-- R1 > B1

KNOWLEDGE STOCK:
  learnings:   156                          <-- K (physical stock)
  patterns:    23
  constraints: 8
  total:       187

LOOP DOMINANCE:
  R1 (new/session):     2.40               <-- R1 output rate
  B1 (decayed/session): 1.10               <-- B1 drain rate
  dominant:             R1                  <-- Which loop is winning
```

| Metric | Leverage Point | What it tells you |
|--------|---------------|-------------------|
| `sigma` | #7 (R1 input) | Are you finding what you need? Low sigma = information flow problem (#6) |
| `rho` | #7 (R1 conversion) | Are you using what you find? Low rho = relevance problem or citation friction |
| `delta` | #8 (B1 drain) | How old is your knowledge? High delta = decay is winning |
| `escape_velocity` | #7 vs #8 | Is R1 stronger than B1? The single most important system health indicator |
| `R1 (new/session)` | #7 (R1 output) | How much new knowledge per session? |
| `B1 (decayed/session)` | #8 (B1 drain) | How much knowledge lost per session? |
| `dominant` | #7 vs #8 | Which loop is winning right now? |

**The operating rule:** When `dominant: "B1"`, the system needs intervention. Either increase input (more forge), improve retrieval (fix information flows), increase usage (fix relevance), or reduce drain (prune stale knowledge and re-index).

---

## Gaps

What is missing from the Meadows mapping and what would close each gap.

| # | Leverage Point | Gap | What would close it |
|---|----------------|-----|---------------------|
| 12 | Parameters | Parameter sensitivity analysis not performed | Run controlled experiments varying delta, sigma, rho targets; measure effect on dK/dt |
| 11 | Buffers | No dynamic buffer sizing | Adaptive context guard that adjusts the 40% threshold based on measured retrieval accuracy |
| 10 | Stock-and-flow | No real-time flow visualization | A `ao metrics flow` command showing I(t), decay rate, and compound rate as a time series |
| 9 | Delays | Phase boundary delays are fixed | Adaptive delays — skip pre-mortem for trivial changes, enforce deeper review for high-risk ones |
| 8 | B loops | B2 (scale friction) is monitored but not auto-controlled | Auto-trigger pruning when precision@k drops below threshold |
| 7 | R1 loop | No cross-project R1 | Knowledge compounding across repos (not just within one). Transfer learning for agent knowledge. |
| 6 | Info flows | No feedback on information flow quality | Measure whether injected knowledge actually influenced decisions (beyond citation counting) |
| 5 | Rules | Rules are additive only | No mechanism to deprecate or remove rules that are no longer serving the system |
| 4 | Self-organization | Registry/compiler promotion is one-way (finding to stronger prevention) | Reverse path: detect stale checks or rules and demote them back to learnings |
| 3 | Goals | Goals are human-authored | Goal suggestion based on measured system state ("your coverage is declining — add a coverage goal?") |
| 2 | Paradigms | Paradigm shifts are documented, not measured | Paradigm adherence metrics — is the team actually practicing "harness variance" or falling back to sequential? |
| 1 | Transcending | No true self-reflection on paradigms | A meta-evolve loop that questions whether the organizing metaphors (seed, flywheel, ratchet) are still serving |

---

## See Also

- [strategic-direction.md](strategic-direction.md) — Consolidation decision, summary Meadows table, 6 paradigm shifts
- [seed-definition.md](seed-definition.md) — The 6 seed elements with Meadows mapping per element
- [the-science.md](the-science.md) — The dK/dt equation, escape velocity, limits to growth
- [how-it-works.md](how-it-works.md) — Hooks, ratchet, Ralph Wiggum, compaction resilience
- [workflows/meta-observer-pattern.md](workflows/meta-observer-pattern.md) — Stigmergy coordination pattern
