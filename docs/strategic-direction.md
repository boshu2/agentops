# Strategic Direction

> One binary, one equation, one recursive shape. Everything else is implementation.

## Consolidation Decision

Three repositories converge into a single product surface:

| Repository | Role | Status |
|------------|------|--------|
| **AgentOps** (`ao`) | One binary. Skills, hooks, knowledge flywheel, RPI orchestration, goal-driven evolution. The product. | Active |
| **Gas Town** (`gt`) | Upstream workspace manager. Multi-agent coordination, rig registry, dispatch. Consumes `ao` as a tool. | Active (upstream) |

Olympus (`ol`) was the power-user daemon predecessor, archived. Its patterns (context compilation, constraint injection, run ledger) survive as features inside `ao`. No live integration exists.

---

## The Organizing Equation

Everything in AgentOps serves one inequality:

```
dK/dt = I(t) - d*K + s*r*K - f*K^2
```

| Symbol | Meaning | AgentOps mechanism |
|--------|---------|-------------------|
| `K` | Knowledge stock (validated learnings, patterns, decisions) | `.agents/` corpus |
| `I(t)` | Input rate (new knowledge per cycle) | `ao forge`, `/retro`, `/post-mortem` |
| `d` | Decay rate (~17%/week without reinforcement, Darr 1995) | `ao maturity --expire` |
| `s` | Retrieval effectiveness (do you find what you need?) | `ao lookup` freshness-weighted scoring, `ao search` |
| `r` | Citation rate (do you use what you find?) | Knowledge reuse in research/plan phases |
| `f` | Scale friction (indexing overhead, noise, governance cost) | Tiering, pruning, utility scoring (MemRL) |

**Escape velocity:** When `s * r > d` (retrieval times usage exceeds decay), knowledge compounds. When it does not, growth stalls regardless of input volume.

Every feature, skill, hook, and CLI command exists to keep the system above that threshold. See [the-science.md](the-science.md) for the full formal model with limits-to-growth analysis.

---

## Meadows' 12 Leverage Points Mapped to AgentOps

Donella Meadows ranked intervention points in complex systems from least to most powerful. AgentOps concentrates on the high-leverage end (#6 through #1) because changing the loop beats tuning the output.

| # | Leverage Point (Meadows) | AgentOps Implementation | Effect on dK/dt |
|---|--------------------------|------------------------|-----------------|
| 12 | Constants, parameters, numbers | Token budgets, timeout values, decay rate (0.17/week) | Tunes `d`, `f` |
| 11 | Buffer sizes | `.agents/` corpus size, context window capacity (40% rule) | Bounds `K`, prevents `s` collapse |
| 10 | Material stocks and flows | Knowledge artifacts flowing through extract-score-inject-compound | The physical `K` stock |
| 9 | Delays | Freshness decay intervals, maturity lifecycle (expire/evict), stale run TTL | Controls lag between `I(t)` and usable `K` |
| 8 | Balancing feedback loops | Regression gates auto-revert bad cycles, council FAIL blocks merge, push gate blocks unvalidated code | Prevents `K` regression |
| 7 | Reinforcing feedback loops | Knowledge flywheel (session N learnings feed session N+1), citation-based utility scoring (MemRL) | The `s*r*K` compounding term |
| 6 | Information flows | `ao lookup` (knowledge into context on demand), `ao forge` (experience out of sessions), hook nudges, briefing packets | Increases `s` by getting right knowledge to right window |
| 5 | Rules | Hooks (3 active lifecycle events in `hooks/hooks.json`), validation gates, worker-guard (lead-only commit), dangerous-git guard, pre-mortem gate | Structural enforcement. Rules cannot be forgotten or ignored. |
| 4 | Self-organization | `/evolve` fitness loop (measure-fix-validate-learn-repeat), constraint compiler (learnings become structural rules), progressive skill revelation | The system improves its own rules based on experience |
| 3 | Goals | `GOALS.md` with mechanically verifiable gates, `ao goals measure`, severity-weighted selection, North Stars and Anti Stars | System intent. What the system optimizes toward. |
| 2 | Mindset/paradigm | The 6 paradigm shifts below | How the builder thinks about agent systems |
| 1 | Transcending paradigms | The seed itself. Same starting conditions produce different systems depending on the fitness landscape. | The product is the seed, not the tree. |

**The bet:** Most agent tooling operates at levels 12-10 (tuning parameters, managing buffers, moving data). AgentOps operates primarily at levels 6-3 (information flows, rules, self-organization, goals). The claim is that structural changes to how the loop works produce more leverage than incremental tuning of what flows through it.

---

## The 6 Paradigm Shifts

These are the mental model changes that distinguish AgentOps from conventional agent tooling:

### 1. From "reduce variance" to "harness variance" (Brownian Ratchet)

Traditional: minimize variance. One developer, one approach, careful sequential steps.
AgentOps: maximize *controlled* variance. Spawn parallel attempts, filter aggressively with councils and gates, ratchet successes. Failed attempts are cheap (~10K tokens); shipped bugs are expensive (hours of debugging). The economics favor more chaos with stronger filters.

### 2. From "context is infinite" to "context is scarce" (40% Rule)

Traditional: stuff the context window with everything possibly relevant.
AgentOps: treat context as a security boundary. Each agent gets only the information necessary for its task, freshness-weighted, within 40% of window capacity. Liu et al. (2023) showed LLMs lose retrieval accuracy in crowded contexts ("lost in the middle"). Least-privilege loading prevents this.

### 3. From "validation is post-hoc" to "validation is preventive" (Shift-Left)

Traditional: review code after it is written.
AgentOps: validate at every stage. `/pre-mortem` catches spec failures before implementation. Hooks enforce gates mechanically (push gate, pre-mortem gate, worker guard). Councils validate before *and* after code ships. The cost of finding a bug increases 10x at each stage it survives.

### 4. From "rules are guidelines" to "rules are structural" (Hooks)

Traditional: coding standards in a wiki that agents may or may not read.
AgentOps: 12 hook lifecycle events that fire automatically on session start, tool use, push, compaction, and stop. Rules enforced by hooks cannot be forgotten, skipped, or rationalized away. The agent does not decide whether to follow them -- the system enforces them.

### 5. From "knowledge is hoarded" to "knowledge is flowing" (Flywheel)

Traditional: knowledge lives in individual context windows and dies when the session ends.
AgentOps: knowledge is extracted (`ao forge`), quality-gated (specificity, actionability, novelty scoring), tiered (gold/silver/bronze), freshness-decayed, and retrieved on demand (`ao lookup`). The flywheel makes session 50 know what session 1 learned. Knowledge that is not retrieved and used decays. Knowledge that compounds survives.

### 6. From "designed systems" to "evolved systems" (The Seed)

Traditional: design the complete system upfront, then build it.
AgentOps: define minimal starting conditions (GOALS.md + hooks + core skills + flywheel bootstrap), plant them in a repo, and let the system evolve toward whatever that repo's goals are. `/evolve` measures fitness, fixes the worst gap, validates nothing regressed, extracts what it learned, and repeats. The system builds its own safety net first (tests), then uses that safety net to refactor aggressively. Nobody tells it the order -- severity-based goal selection naturally produces the correct sequence.

This is the deepest shift. The product is not 53 skills. The product is the seed that, given a fitness landscape, produces the right system.

---

## Agent-First Design Principles

These principles govern how skills, hooks, and the CLI present information to agents:

### Briefing Packets

Every agent receives a structured briefing packet scoped to its role and phase. Research agents get prior learnings. Plan agents get a 500-token research summary. Crank workers get fresh context per wave with zero bleed-through. Vibe judges get recent changes only. No agent sees everything. Context is a security boundary.

### Chain Intelligence

Intelligence lives in the chain of skills, not in any individual skill. `/research` alone is exploration. `/plan` alone is decomposition. But `/research` -> `/plan` -> `/pre-mortem` -> `/crank` -> `/vibe` -> `/post-mortem` is a self-correcting pipeline where each phase validates and constrains the next. The post-mortem proposes the next cycle's work. The chain feeds itself.

### Progressive Disclosure

New users see 8 starter skills: `/quickstart`, `/research`, `/council`, `/vibe`, `/rpi`, `/implement`, `/retro`, `/status`. The remaining 45 skills reveal themselves as the user grows. Verb aliases let users type what they mean ("review this code" triggers `/vibe`). The system is approachable at the surface and deep underneath.

### Fractal Composition

The same shape repeats at every scale: lead decomposes work, workers execute atomically, validation gates lock progress, next wave begins.

```
/implement  -- one worker, one issue, one verify cycle
    /crank  -- waves of /implement (FIRE loop)
        /rpi    -- research -> plan -> crank -> validate -> learn
            /evolve -- fitness-gated /rpi cycles
```

Each level treats the one below as a black box: spec in, validated result out. This is Meadows' self-organization (#4) -- the same pattern produces different outcomes depending on the goals and constraints it operates under.

---

## Sources

This document synthesizes:

- Strategic direction council (2026-02-21): 4-judge unanimous WARN, feature saturation reached
- The science (formal knowledge model): dK/dt equation, escape velocity, limits to growth
- Brownian ratchet philosophy: chaos + filter + ratchet execution model
- Architecture (5 pillars): Three Ways, Ratchet, Ralph Wiggum, Flywheel, Fractal Composition
- PRODUCT.md: mission, vision, design principles, Meadows foundation
- 2026 roadmap: 4 epics (multi-runtime, autonomous hardening, adoption, bridge)
- Pre-mortem for The Seed (2026-02-24): 6 findings, all addressed

## See Also

- [seed-definition.md](seed-definition.md) -- the minimal starting conditions
- [the-science.md](the-science.md) -- formal knowledge model
- [brownian-ratchet.md](brownian-ratchet.md) -- execution philosophy
- [ARCHITECTURE.md](ARCHITECTURE.md) -- system design
- [how-it-works.md](how-it-works.md) -- operational mechanics
