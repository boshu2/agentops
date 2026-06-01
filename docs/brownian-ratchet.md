# The Brownian Ratchet: AI-Native Development Philosophy

> **Chaos + Filter + Ratchet = Progress**

> **Architectural home:** This is the deep dive for **[Pillar 2: Brownian Ratchet](ARCHITECTURE.md#pillar-2-brownian-ratchet)** — the execution model in the AgentOps four-pillar architecture (Brownian Ratchet · Ralph Wiggum · Knowledge Flywheel · Fractal Composition). ARCHITECTURE.md carries the canonical short form; this page carries the full philosophy, economics, and FIRE-loop details.
>
> **Vocabulary note (2026-05):** Examples below that use `gt …` commands and "Gas Town / the Mayor / polecats" reflect an earlier orchestration surface. The current dispatch backend is **Gas City** (`gc …` commands; bridge at `cli/cmd/ao/gc_bridge.go`, `gc_events.go`, `rpi_phased_gc.go`). The *pattern* (parallel chaos → filter → ratchet) is unchanged; treat the specific `gt`/Mayor/polecat callsites as illustrative, not literal. See `skills/using-gc/SKILL.md` and the repo `CLAUDE.md` "Gas City Dispatch" section for the live command surface.

---

## The Metaphor

A Brownian ratchet is a thermodynamic thought experiment: random molecular motion (chaos) passes through a one-way gate (filter) to produce net forward movement (ratchet). The gate only allows motion in one direction—progress accumulates while regression is blocked.

In AI-native development, we face similar dynamics:
- **Agents produce variance** - multiple attempts, different approaches, occasional failures
- **Validation filters quality** - tests, CI, code review catch bad attempts
- **Merge is permanent** - once code lands in main, progress is locked

The insight: instead of fighting variance, **harness it**. Spawn parallel attempts, filter aggressively, ratchet the successes.

---

## The Three Components

### 1. Chaos (Embrace Variance)

Traditional development minimizes variance: one developer, one approach, sequential execution. This is safe but slow.

AI-native development **maximizes controlled variance**:
- Spawn multiple polecats working in parallel
- Each takes a slightly different path
- Some fail, some succeed—that's expected
- More attempts = more chances to find the optimal solution

**The economics**: 4 polecats × 30% failure rate still yields ~3 successes per wave. Sequential execution with 30% failure rate means constant restarts.

```
Traditional:  ───────────────────────────────────► (slow, fragile)
AI-native:    ═══╦═══╦═══╦═══╗
              ═══╬═══╬═══╬═══╬═══════════════════► (fast, resilient)
              ═══╩═══╩═══╩═══╝
                 ↑
              parallel attempts (some fail, most succeed)
```

### 2. Filter (Validate Aggressively)

Chaos without filtering produces garbage. The filter is what makes the ratchet work.

**Filtering happens at multiple levels**:

| Level | Filter | What Gets Blocked |
|-------|--------|-------------------|
| Pre-implementation | `/pre-mortem` | Bad specs, missing requirements |
| During implementation | CI, tests, lint | Broken code, regressions |
| Post-implementation | `/vibe` | Quality issues, security flaws |
| Human gate | PR review | Architectural mistakes |

**The key insight**: filters are cheap, rework is expensive. Front-load validation.

A polecat that fails CI costs ~10K tokens. A bug that ships to production costs days of debugging. Aggressive filtering is economically rational.

### 3. Ratchet (Lock Progress)

The ratchet is what makes progress permanent. Once work passes the filter, it's locked—you can't go backward.

**Ratchet points in the system**:

| Action | What Gets Locked |
|--------|------------------|
| Merge to main | Code changes |
| Close beads issue | Task completion |
| Write to `.agents/` | Knowledge artifacts |
| Store MCP memory | Persistent insights |
| Update spec with learnings | Improved documentation |

**The property**: ratcheted work compounds. Each merge makes the codebase better. Each learning makes future work faster. Each pattern prevents future mistakes.

---

## The FIRE Loop

FIRE is the reconciliation engine that implements the Brownian Ratchet:

```
┌──────────────────────────────────────────────────────────────┐
│                         FIRE LOOP                            │
│                                                              │
│      FIND ────► IGNITE ────► REAP ────► ESCALATE             │
│     (state)    (chaos)    (ratchet)   (recovery)             │
│        │                                   │                 │
│        └───────────────────────────────────┘                 │
│                       (loop)                                 │
│                                                              │
│      EXIT when: all work reaped                              │
└──────────────────────────────────────────────────────────────┘
```

### FIND - Read State

Survey the battlefield. What's ready to ignite? What's currently burning? What's been reaped?

```bash
bd ready --parent=<epic>      # Ready to ignite
bd list --status=in_progress  # Currently burning
bd list --status=closed       # Already reaped
```

### IGNITE - Spark Chaos

Dispatch work to parallel polecats. Each polecat is an independent attempt—they don't coordinate, they just execute.

```bash
gt sling <issue1> <issue2> <issue3> <rig>
```

This is the **chaos** phase. Multiple agents, multiple paths, variance embraced.

### REAP - Harvest + Ratchet

Monitor for completion. When polecats finish, validate their work:

1. **Did they actually complete?** (beads status = closed)
2. **Is there a commit?** (git work product exists)
3. **Did it pass CI?** (filter approved)

Valid completions get merged. **Merge is the ratchet**—once in main, it's permanent.

<!-- STALE VOCAB: gt/convoy/polecat — superseded by Gas City (gc). See vocabulary note at top. -->
> **Stale callsite (illustrative only):** the `gt convoy` / `gt done` / `polecat/...` commands below describe the retired Gas Town surface. On the current Gas City backend, monitoring and completion route through `gc` (e.g. `gc status`, `gc session list`, `gc beads list`) and the daemon ledger, not `gt convoy`. The REAP step — validate completion, confirm a commit exists, confirm CI passed, then merge — is unchanged.

```bash
gt convoy status <id>         # Monitor (retired — use `gc status` / `gc session list`)
# Polecat runs: gt done → push → merge queue (now a Gas City agent + merge queue)
git merge origin/polecat/...  # Ratchet
```

### ESCALATE - Handle Failures

Not everything succeeds. Failed attempts have two paths:

1. **Retry** - Re-ignite with fresh polecat (back to chaos pool)
2. **Escalate** - After 3 failures, mark as BLOCKER and mail human

```bash
# Retry
gt sling <failed-issue> <rig>

# Escalate
bd update <issue> --labels=BLOCKER
gt mail send --human -s "BLOCKER: <issue> needs help"
```

The loop continues until all work is reaped or escalated.

---

## Applied to the Phased RPI Workflow

The historical RPI name now runs as a phased lifecycle:

```
DISCOVERY ──┬──► IMPLEMENTATION ──┬──► VALIDATION
            │                     │
            └── chaos + filter    └── ratchet + learning
```

| Phase | Chaos | Filter | Ratchet |
|-------|-------|--------|---------|
| **Discovery** | Multiple exploration paths and framing options | Research synthesis + pre-mortem review | Execution packet, scoped risks, and tracked issues |
| **Implementation** | Parallel fresh-context workers | Wave verification, tests, and `/vibe` | Accepted code plus ratchet checkpoints |
| **Validation** | Multi-aspect review and learning extraction | Council, findings registry, and flywheel closure | Knowledge stored, next work harvested, stronger prevention artifacts |

Each phase locks its output before the next phase begins. You cannot un-discover, un-verify, or un-learn without writing a new artifact that supersedes the old one.

---

## Skill Roles in the Ratchet

Every skill has a role in the pattern:

| Skill | Ratchet Role | What It Does |
|-------|--------------|--------------|
| `/research` | Chaos source | Broad exploration, divergent investigation |
| `/pre-mortem` | Pre-filter | Catch spec failures before implementation |
| `/plan` | Chaos + ratchet | Parallel exploration → locked epic |
| `/crank` | Full FIRE loop | Autonomous execution until complete |
| `/vibe` | THE filter | Quality gate that blocks bad code |
| `/implement` | Micro-ratchet | Single issue: open → closed (atomic) |
| `/swarm` | Parallel chaos engine | Fresh worker set per wave |
| `/post-mortem` | Knowledge ratchet | Learnings locked, never go backward |

---

## Why This Works

### 1. Failure is Expected, Not Catastrophic

In traditional development, a failed attempt means wasted time. In ratchet-based development, failed attempts are just filtered chaos—expected and cheap.

A polecat that fails:
- Costs ~10K tokens
- Teaches nothing (no human time wasted)
- Gets retried automatically

A human developer that fails:
- Costs hours of debugging
- Creates frustration and context loss
- Requires manual restart

### 2. Progress Compounds

Each ratchet point adds to the baseline:
- Merged code improves the codebase
- Stored learnings improve future research
- Captured patterns prevent repeated mistakes
- Updated specs improve future planning

This is the **knowledge flywheel**—every cycle makes the next cycle faster.

### 3. Parallelism is Natural

Chaos embraces parallelism. Instead of:
```
Issue 1 → Issue 2 → Issue 3 → Issue 4 (sequential)
```

You get:
```
Issue 1 ─┬─► merged
Issue 2 ─┼─► merged
Issue 3 ─┼─► merged (one failed, retried, succeeded)
Issue 4 ─┴─► merged
```

The FIRE loop naturally extracts maximum parallelism from available capacity.

### 4. Humans Handle the Hard Parts

Escalation ensures humans see only what matters:
- Blockers that need judgment
- Architectural decisions
- Ambiguous requirements

Routine work gets reaped automatically. Human attention is reserved for human problems.

---

## The Formula

```
Progress = ∫(Chaos × Filter) dt → Ratchet
```

Continuous application of filtered chaos accumulates as permanent progress.

Or more simply:

> **You can always add more chaos, but you can't un-ratchet.**

This is why token cost is front-loaded (more attempts early) but total cost is lower (no rework from bad foundations).

---

## Practical Application

### Starting an Epic

```bash
/plan <goal>              # Generate issues with dependencies
/pre-mortem <spec>        # Filter the plan before execution
/crank <epic>             # FIRE loop until complete
/post-mortem              # Extract learnings, close the loop
```

### During Execution

The orchestrator runs FIRE (on the current Gas City backend this is the `gc` dispatch layer + the `ao` daemon; "the Mayor" was the Gas Town-era name for the same role):
1. **FIND** - Check what's ready
2. **IGNITE** - Sling to polecats
3. **REAP** - Harvest completions
4. **ESCALATE** - Handle failures
5. Repeat until epic closed

### After Completion

```bash
/post-mortem <epic>       # Validate + extract learnings
```

Learnings get ratcheted:
- `.agents/learnings/` - What happened
- `.agents/learnings/` - What we learned
- `.agents/patterns/` - Reusable solutions
- MCP memories - Persistent insights

These feed the next `/research` cycle. The flywheel turns.

---

## Key Principles

1. **Embrace variance** - More attempts = more chances for optimal solutions
2. **Filter aggressively** - Cheap validation prevents expensive rework
3. **Ratchet permanently** - Locked progress compounds over time
4. **Escalate appropriately** - Humans handle human problems
5. **Close the loop** - Post-mortem feeds research feeds planning feeds execution

---

## The Forge Connection (Gas City)

> **Vocabulary note:** This was originally written for the **Gas Town** orchestration surface. The forge metaphor is unchanged, but the live backend is now **Gas City** — the names map one-to-one: Gas Town → Gas City, the Mayor → the `gc` dispatch/orchestrator role, polecats → fresh ephemeral Gas City worker agents. Commands are `gc …` (see `cli/cmd/ao/gc_bridge.go` and `skills/using-gc/SKILL.md`).

The forge is where chaos becomes product. FIRE is how the forge operates.

- **Worker agents** are the hands at the anvil—independent, ephemeral, expendable (fresh context per attempt)
- **The orchestrator** tends the FIRE loop—dispatching, monitoring, harvesting
- **Beads** are the work orders—tracked, statused, closed
- **Main branch** is the finished product—ratcheted, permanent, compounding

The forge runs until the work is done. Chaos in, quality out.

---

## The Ratchet-Flywheel Connection

The Brownian Ratchet and Knowledge Flywheel are complementary systems:

| Concept | Scope | What It Does |
|---------|-------|--------------|
| **Ratchet** | Single cycle | Extracts progress from chaos |
| **Flywheel** | Cross-cycle | Compounds knowledge over time |

### How They Connect

```
FIRE Loop (Ratchet)                    Knowledge Flywheel
═══════════════════                    ══════════════════
        │
  FIND → IGNITE → REAP → ESCALATE
              │
              ▼
    ┌─────────────────┐
    │  Ratchet Point  │──────────────→ Knowledge Artifact
    │  (merge/close)  │                      │
    └─────────────────┘                      ▼
              │                        ┌──────────────┐
              │                        │   Surface    │
              │                        │   (query)    │
              │                        └──────┬───────┘
              │                               │
              │                               ▼
              │                        ┌──────────────┐
              │                        │    Cite      │
              │                        │  (use it)    │
              │                        └──────┬───────┘
              │                               │
              │                               ▼
              │                        ┌──────────────┐
              │                        │   Promote    │
              │                        │ (tier up)    │
              │                        └──────┬───────┘
              │                               │
              │                               ▼
              │                        ┌──────────────┐
              │                        │ Better Rank  │
              │                        └──────┬───────┘
              │                               │
              ▼                               ▼
    ┌─────────────────────────────────────────────────┐
    │           NEXT FIRE CYCLE                       │
    │    (with better knowledge = faster/cheaper)     │
    └─────────────────────────────────────────────────┘
```

### The Unified Math

> For the formal Knowledge Flywheel differential equation (`dK/dt = I(t) - δ·K + σ·ρ·K`) and its parameters, see **[the-science.md](the-science.md#the-equation)** — the canonical home. The expressions below are intuition-level summaries, not the authoritative model.

**Ratchet (single cycle):**
```
Progress(t) = ∫(Chaos × Filter) dt → Ratchet
```

**Flywheel (knowledge value):**
```
Knowledge_Value = citations × confidence × freshness
```

**Combined (cumulative progress with compounding):**
```
Cumulative_Progress = Σ[ Progress(t) × Knowledge_Multiplier(t) ]

Where:
  Knowledge_Multiplier(t) = 1 + α × (flywheel_score / baseline)
```

### Why This Matters

Without the ratchet, the flywheel has nothing to compound:
- No merged code = no patterns to extract
- No closed issues = no learnings to store
- No validations = no quality evidence

Without the flywheel, the ratchet doesn't accelerate:
- Same mistakes repeated
- No institutional memory
- Linear progress instead of exponential

**Together:**
- Ratchet produces raw progress
- Flywheel amplifies future cycles
- System accelerates over time

### The Compounding Effect

| Cycle | Ratchet Output | Flywheel State | Effective Speed |
|-------|----------------|----------------|-----------------|
| 1 | 10 units | baseline | 1.0x |
| 2 | 10 units | +patterns | 1.2x |
| 3 | 10 units | +learnings | 1.4x |
| 4 | 10 units | +memories | 1.6x |
| N | 10 units | compounded | 2.0x+ |

The ratchet produces constant raw output. The flywheel multiplies it.

### Practical Integration

**After each FIRE cycle (ratchet point):**
1. Merge code (immediate ratchet)
2. Close beads issue (state ratchet)
3. Run `/post-mortem` (knowledge extraction)
4. Store learnings in `.agents/` (flywheel entry)
5. Store memories in MCP (semantic recall)

**Before each FIRE cycle (flywheel benefit):**
1. `/research` checks prior art (flywheel read)
2. Memory recall surfaces relevant patterns
3. Pre-mortem leverages past failures
4. Plan benefits from proven formulas

**The loop:**
```
Ratchet → Flywheel Entry → Better Knowledge → Faster Ratchet → ...
```

This is why `/post-mortem` is mandatory. Skipping it breaks the compounding.

---

## Validation Status

> **Canonical equation lives elsewhere.** The authoritative Knowledge Flywheel equation and its term-by-term explanation are in **[the-science.md](the-science.md#the-equation)**. The canonical baseline form is:
>
> ```
> dK/dt = I(t) - δ·K + σ·ρ·K
> ```
>
> with a scale-aware correction `… - φ·K²` (limits-to-growth) documented in the-science.md. **Cite that page rather than the divergent form below.**
>
> **Internal-WIP note (stale):** the `- B(K, K_crit)` barrier term and the `ol-rg3p` validation epic below were an internal work-in-progress framing that never landed in the canonical model (the-science.md uses `- φ·K²` for the saturation/limit term, not a `B(K, K_crit)` barrier). The `ol-rg3p` bead IDs and wave statuses below are not a current tracker — treat this section as historical. The general point still holds: **the flywheel math is itself validated by running FIRE on it** (literature + measured parameters), and that validation is owned by the-science.md.

The (historical / WIP) framing tracked the equation with an added barrier term:

```
dK/dt = I(t) - δ·K + σ·ρ·K - B(K, K_crit)    # WIP form — superseded by the-science.md
```

### What's Validated (Literature)

| Claim | Source | Evidence |
|-------|--------|----------|
| 17%/week knowledge decay | Darr et al. (1995) | Empirical, peer-reviewed |
| 85x decay reduction with DB | Boone et al. (2008) | Empirical, peer-reviewed |
| Ebbinghaus forgetting curve | Ebbinghaus (1885) | Classical, replicated |

### What's Being Validated (Epic ol-rg3p)

| Parameter | Status | Wave |
|-----------|--------|------|
| B(K, K_crit) barrier function | **Undefined** → Wave 0 | ol-rg3p.1 |
| δ decay measurement | Draft → Wave 0 | ol-rg3p.2 |
| σ retrieval effectiveness | Draft → Wave 0 | ol-rg3p.3 |
| ρ reinforcement rate | Draft → Wave 0 | ol-rg3p.4 |
| K_crit threshold | Draft → Wave 0 | ol-rg3p.5 |
| Empirical data collection | Pending → Wave 1-2 | ol-rg3p.6-13 |

### Falsification Conditions

The model is falsified if:
1. K(t) shows ceiling despite σρ > δ/100
2. Citations don't reduce decay
3. B(K, K_crit) is unmeasurable
4. δ_system > 20%/week consistently

### Validation Artifacts

- **Protocol:** `.agents/ao/validation/flywheel-validation-protocol.md`
- **Parameters:** `.agents/ao/validation/parameter-definitions.md`
- **Predictions:** `.agents/ao/validation/predictions.md`
- **Results:** `.agents/ao/validation/results-YYYY-MM.md` (pending)

**This is the Brownian Ratchet applied to itself.** We're running FIRE on our own philosophy.

---

*"The forge FIRES until the work is done."*
