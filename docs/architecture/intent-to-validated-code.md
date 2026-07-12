# Intent → Validated Code

> **What AgentOps is.** The product is an **operating loop** that turns declared
> intent into validated code with proof. **Skills** are the front door — the
> portable agent runtime that runs each move. The **membrane** is the close door
> (move 6 / S5): it binds an independent verdict to the slice's **acceptance
> behavior**, not to vibes on a diff. Without Gherkin (or an equivalent runnable
> acceptance contract), the membrane has nothing rigorous to validate against.
>
> Companion pages: [Operating Loop](operating-loop.md) (discipline),
> [Skills Matrix](../skills-matrix.md) (every skill placed on the loop),
> [3.0 north star](../3.0.md), [Skill Router](../SKILLS.md).

## One picture

```text
 INTENT (what "done" means)
   │
   ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │  1. Shape as BDD          /discovery  /product  /plan           │
 │     Feature + Given/When/Then (+ edge) + non-goals + evidence   │
 └───────────────────────────────┬─────────────────────────────────┘
                                 ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │  2. Track as a bead       /beads-br   (/goal-design upstream)   │
 │     Acceptance rides on the bead — no runnable contract, no bead│
 └───────────────────────────────┬─────────────────────────────────┘
                                 ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │  3. Slice vertically      /plan  (+ /behavior-first-planning)    │
 │     One behavior = one slice = one first-failing proof          │
 └───────────────────────────────┬─────────────────────────────────┘
                                 ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │  4. TDD the slice         /implement  (/test)                   │
 │     RED acceptance → smallest green → refactor-under-green      │
 └───────────────────────────────┬─────────────────────────────────┘
                                 ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │  5. Wave only if safe     /plan declares · /crank /swarm run    │
 │     Disjoint write scopes or stay sequential                    │
 └───────────────────────────────┬─────────────────────────────────┘
                                 ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │  6. Prove acceptance      /validate  /council  /pawl-review     │
 │     Every GWT → passing test + independent verdict              │
 │     no verdict = not done                                       │
 └───────────────────────────────┬─────────────────────────────────┘
                                 ▼
 ┌─────────────────────────────────────────────────────────────────┐
 │  7. Capture + ratchet      /post-mortem                          │
 │     Evidence always · learnings only if they change the next run│
 └───────────────────────────────┬─────────────────────────────────┘
                                 │
                                 └──► re-enter move 1 with evidence
                                      (re-plan the remaining route)
```

**One-tick wrapper:** `/rpi` runs research → plan → implement → validate over
one bead (one behavior, one acceptance proof). It does not replace the loop; it
*is* one pass through it. `/evolve` is N ticks toward a goal (experimental /
second-stage).

**Supporting CLI (not the front door):** `ao` bookkeeps, retrieves, and gates
(`ao beads`, `ao gate check`, `ao verify` as commit/pre-push ratchet, ledger).
Agents enter through **skills**.

## Why behavior comes before the membrane

The narrow-waist micro-cycle (canonical in [operating-loop.md](operating-loop.md)):

```text
small batch → BDD (Gherkin) → ATDD (acceptance RED) → green → refactor → membrane → mine back
```

| Without a behavior contract | With Gherkin → ATDD |
|-----------------------------|---------------------|
| `/validate` reviews a diff and invents the bar | `/validate` checks every Given/When/Then against fresh evidence |
| "Done" is a self-grade | "Done" is a runnable contract the author cannot redefine at close |
| Membrane is taste | Membrane is **acceptance proof** + independent judgment |

Rule from [`behavior-first-planning`](../../skills/behavior-first-planning/SKILL.md):
**no runnable acceptance test, no bead.**

`/validate` is the driving adapter for the `validate_acceptance` port: the
verdict binds to the **slice's** acceptance test (its ATDD contract), not only
the parent intent prose. See [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md).

## Full flow — artifacts and done signals

| Move | You produce | Skills (primary) | Done when |
|------|-------------|------------------|-----------|
| 1. Shape intent | Intent issue / packet with Feature + Scenarios | `/discovery`, `/product`, `/plan` | Happy path + ≥1 edge are testable; non-goals and evidence named |
| 2. Track | Bead carrying Gherkin or linking to it | `/beads-br`, `/goal-design` | Bead has acceptance + validation commands |
| 3. Slice | Slice list, one row per behavior | `/plan`, `/behavior-first-planning` | Each slice has first-failing proof + write scope |
| Pre-flight (optional but recommended) | Plan stress-test | `/pre-mortem`, `/council` | Plan HOLD/PASS before build |
| 4. Implement | RED → green → refactor commits | `/implement`, `/test` | Acceptance test failed for the right reason, then passes; refactor did not edit the contract |
| 5. Wave | Parallel ownership map | `/crank`, `/swarm` (+ `/agent-mail` if ≥2 writers) | Scopes disjoint or forced sequential |
| 6. Membrane | Criterion verdicts + roll-up | `/validate`, `/council`, `/pawl-review` | Every GWT mapped to fresh passing evidence; independent verdict recorded; **no verdict = not done** |
| Land | Commit-bound proof on the trunk path | `ao land` / pawl + `ao gate check` | CONFIRM bound to `head_sha`; gate green |
| 7. Ratchet | Evidence + promoted constraints | `/post-mortem` | Evidence cited; learnings only if next loop will read them |

Templates: [`intent-issue.md`](../templates/intent-issue.md),
[`slice-validation.md`](../templates/slice-validation.md).

## Three altitudes (same loop)

| Altitude | When | Skills in view | Still the same? |
|----------|------|----------------|-----------------|
| **Single tick** | One behavior, one session | `/plan` → `/implement` → `/validate` (or `/rpi`) | Yes |
| **Tracked project** | Multi-bead epic | `/discovery` → `/plan` → `/crank` → `/validate` → `/post-mortem` | Yes |
| **Orchestrated** | ≥2 lanes or unattended | `/swarm`, `/ntm`, `/agent-mail`, `/using-gc` (operator choice) | Yes — substrate dispatches a **whole** `/rpi` tick; it does not re-implement the loop |

Do not start at the third altitude. Shape 0 is single-agent with bookkeeping
([automation-shape-routing](../../skills/automation-shape-routing/SKILL.md)).

## First session (skills front door)

1. Install AgentOps skills for your agent runtime ([README](../../README.md)).
2. Optional: install `ao` for beads/gate/ledger.
3. In the agent, name a **small** intent.
4. Run `/plan` (or `/discovery` then `/plan`) until one Given/When/Then is frozen.
5. Run `/implement` on that slice (RED acceptance first).
6. Run `/validate` — the verdict must cite the scenario / acceptance evidence.
7. Only then optionally `/post-mortem` or scale to `/crank`.

Success signal: a PASS/CONFIRMED (or honest HOLD) that names the behavior that
was proved — not a green CI alone, and not a review with no acceptance mapping.

## Where to go next

| Need | Doc |
|------|-----|
| Every skill placed on the loop | [Skills Matrix](../skills-matrix.md) |
| "Which skill do I run?" decision tree | [SKILLS.md](../SKILLS.md) |
| Discipline detail (waves, ratchet, windshield) | [Operating Loop](operating-loop.md) |
| Ports / adapters for one turn | [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md) |
| Doctrine / waist | [3.0](../3.0.md) |
| Install + golden paths | [Getting Started](../getting-started/index.md) |
