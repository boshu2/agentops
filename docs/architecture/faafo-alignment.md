# FAAFO Promise Framework

The FAAFO Promise defines the value proposition of Vibe Coding. When AI-assisted development is done correctly, teams achieve multiplicative gains across five dimensions.

**FAAFO = Fast + Ambitious + Autonomous + Fun + Optionality**

From Gene Kim and Steve Yegge's Vibe Coding book. These gains materialize only when the Three Loops are managed and failure patterns prevented.

---

## F - Fast (10-16x Velocity)

**Claim:** 10-16x velocity on appropriate tasks.

**Enablers:**
- Context bundles prevent re-research
- AI explores multiple approaches in parallel
- Precise plans reduce implementation decisions
- Inner Loop validation catches errors in seconds

**Measuring:** `Velocity Ratio = Time(Manual) / Time(AI-Assisted)`

**Breaks down when:**
- Context window >60% consumed
- Novel domains (L0-L1 trust level)
- Debug loop spirals emerge

---

## A - Ambitious (Solo Feasibility)

**Claim:** Individuals tackle projects previously requiring teams.

**Enablers:**
- AI explores option space humans couldn't cover
- Handles tedious cross-cutting concerns
- Provides expertise across multiple domains
- Substitutes stamina for repetitive work

**Option Value:** `(N x K x sigma) / t` - AI increases N (approaches) and K (parallel paths) while reducing t (time).

**Measuring:** `Complexity(Achieved) / Complexity(Typical Solo)`

**Breaks down when:**
- Multi-person coordination needed
- Deep domain expertise required
- Long-running projects accumulate AI-invisible context

---

## A - Autonomous (Team-Level Output)

**Claim:** Solo developers produce team-comparable output.

**Enablers:**
- AI fills junior developer, reviewer, tester roles
- Validation gates maintain consistent quality
- Cross-functional capability in single workflow
- 24/7 availability

**Team Equivalence:** Solo + AI matches team of 3-5 for velocity, review, coverage. Falls short for architecture decisions, stakeholder management.

**Measuring:** `Output(Solo+AI) / Output(Typical Team)`

**Breaks down when:**
- Specialized expertise needed (security, performance)
- Stakeholder communication required
- Accountability ownership unclear

---

## F - Fun (50% More Flow)

**Claim:** 50% increase in developer flow state time.

**Enablers:**
- Tedious tasks automated
- Immediate answers without context-switching
- Fewer stuck states
- Exploration becomes safe
- More visible progress per session

**Flow Protection:** Three Loops structure preserves flow - Inner (seconds) for momentum, Middle (hours) for focus, Outer (days) for decisions between sessions.

**Measuring:** Time in uninterrupted work, frustration frequency, session satisfaction.

**Breaks down when:**
- Failure pattern cascades
- Context rot frustration
- Trust calibration errors
- Tool instability

---

## O - Optionality (120x Options)

**Claim:** Explore 120x more solutions before committing.

**Enablers:**
- AI drafts approaches in minutes
- Multiple options compared simultaneously
- Tracer tests validate before full investment
- Rollback procedures preserve retreat options
- Bundle snapshots for later reconsideration

**Option Value Formula:**
```
(N x K x sigma) / t
N = Options (~10x with AI)
K = Parallel paths (~3-5x)
sigma = Uncertainty reduction (faster)
t = Time cost (~4x reduction)
Combined: ~120x exploration capacity
```

**Measuring:** Approaches per decision, time to prototype, successful pivots.

**Breaks down when:**
- Premature commitment (skipping research)
- Context exhaustion
- Integration constraints
- Sunk cost bias

---

## Cross-Cutting Enablers

All FAAFO dimensions require:

**The 40% Rule:**
- Below 40% context → 98% success, full FAAFO
- Above 60% context → 24% success, degraded FAAFO

**Three Loops Discipline:**

| Loop | FAAFO Impact |
|------|--------------|
| Outer (days) | Ambitious, Optionality |
| Middle (hours) | Fast, Fun |
| Inner (seconds) | Fast, Autonomous |

**Failure Pattern Prevention:** (full catalog in [The 12 Failure Patterns](failure-patterns.md); prevent/detect/correct mechanics in the [PDC Framework](pdc-framework.md))

| Pattern | FAAFO Damage |
|---------|--------------|
| Tests Passing Lie | Autonomous quality |
| Debug Loop Spiral | Fast velocity |
| Context Amnesia | All dimensions |
| Instruction Drift | Fun, Fast |

---

## FAAFO by Command

| Command | Primary FAAFO |
|---------|---------------|
| `/research` | Optionality, Ambitious |
| `/plan` | Fast, Autonomous |
| `/implement` | Fast, Fun |
| `/retro` | Optionality, Ambitious |

---

## Quick Reference

```
F - Fast:      10-16x velocity
A - Ambitious: Solo feasibility
A - Autonomous: Team output
F - Fun:       50% more flow
O - Optionality: 120x options

Conditional on: 40% Rule, Three Loops, failure prevention
```

---

## Further Reading

**Vibe Coding theory trio (architecture docs):**

- [PDC Framework](pdc-framework.md) - Prevent, Detect, Correct across workflow phases
- [The 12 Failure Patterns](failure-patterns.md) - failure catalog FAAFO prevention guards against

**External and command docs:**

- [Vibe Coding](https://itrevolution.com/product/vibe-coding-book/) - Gene Kim & Steve Yegge
- Command docs for FAAFO alignment tables: `/research`, `/plan`, `/implement`
