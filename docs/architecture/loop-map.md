# The Loop Map

> One index for the loop. AgentOps describes its operating loop six ways, at different altitudes and for different audiences. They are not six competing models; they are six views of one control system. This doc names each view, says what it is for, and links its canonical source. It does not restate or replace those sources.

## Why this exists

Readers hit "the loop" in six places and reasonably wonder which is the real one. The honest answer: all of them, at different zoom levels. A reader landing on the operating-loop spine without the CDLC frame thinks the loop is about git moves; a reader landing on CDLC without the operating loop thinks it is about context phases. Each is half the picture. This map fixes the altitude so the views stop reading as contradictions.

The external world is now naming this too. Satya Nadella calls it "loopcraft"; the broader 2026 discourse calls it "loop engineering." The claim under all of it is the one this map encodes: the model commoditizes, and the loop around it is the part that compounds.

## The six views

| View | Altitude | Answers | Canonical source |
|---|---|---|---|
| **CDLC (7 phases)** | What context engineering *is* | Generate, Compile, Test, Distribute, Deliver, Observe, Adapt | [`docs/cdlc.md`](../cdlc.md) |
| **Operating loop (7 moves)** | How an agent *executes one turn* through those phases | BDD intent → bead → slice → TDD → wave → acceptance → ratchet | [`docs/architecture/operating-loop.md`](./operating-loop.md) |
| **RPI (4 steps)** | The tactical inner loop *inside a move* | Research → Plan → Implement → Validate | the `/rpi` skill; framed in [`docs/3.0.md`](../3.0.md) |
| **Control-loop model (2 timescales + governor)** | *Why* the loop converges and self-improves | Why route-back terminates (fast loop) + how the map improves without oscillating (slow loop + SPC governor) | [`docs/architecture/control-loop-model.md`](./control-loop-model.md) |
| **Operator model (6 primitives)** | The operational *discipline* the loop runs on | Stateful environment, replaceable actors, durable traces, selection gates, promotion loops, governance | 12-factor-agentops: `docs/explanation/operator-model.md` |
| **Three developer loops** | The *timescale* the loop runs at | Inner (seconds–minutes), Middle (hours–days), Outer (weeks–months) | 12-factor-agentops: `docs/explanation/three-developer-loops.md` |

## How they nest

The views are not parallel; they telescope. One outer-loop turn contains many operating-loop turns; one operating-loop move contains an RPI cycle; every RPI cycle moves the work through CDLC phases; the operator-model primitives are the substrate all of it runs on.

```
Three developer loops        the timescale (outer → middle → inner)
  └─ Operating loop          one turn: intent → … → ratchet
       └─ RPI                one move's executor: research → plan → implement → validate
            └─ CDLC phases   what each step does to context: generate … adapt
                 on top of
            Operator model   stateful env · actors · traces · gates · promotion · governance
```

A concrete trace of one operating-loop turn against the CDLC phases (the composition already stated in [`docs/cdlc.md`](../cdlc.md)):

```
BDD-shaped intent issue          ← Generate
  → vertical slices              ← Compile
  → TDD per slice                ← Test
  → conflict-free parallel wave  ← Distribute + Deliver
  → integrated bead completion   ← Observe
  → evidence + learning capture  ← Adapt
```

## Vocabulary reconciliation

The same mechanism wears different names across the sources. They are the same thing:

| Concept | Names in use | Where |
|---|---|---|
| The compounding feedback engine | **Knowledge Flywheel** · **Compound Knowledge** (12-factor) · **Adapt** (CDLC phase 7) | cdlc.md, operator-model.md, knowledge-flywheel (personal-site essay) |
| The one-way progress lock | **promotion ratchet** · **Brownian ratchet** | operating-loop.md move 7, 12-factor "Lock Progress Forward" |
| Independent verdict before progress | **selection gate** · **validation gate** · **bead-acceptance pawl** | operator-model.md, operating-loop.md, [`docs/contracts/pawls.md`](../contracts/pawls.md) |

12-factor references are by name, not roman numeral, on purpose: the factor numbering diverges between the doctrine repo and older citations (see Open items).

## The system, not the DAG

The reason there is a *loop* and not a *pipeline* is the one principle that ties the five views together, and it currently lives only inside the navigator section of [`docs/3.0.md`](../3.0.md). Stated as doctrine here:

**Orchestration determinism runs inverse to worker determinism.** A deterministic worker takes rails: a script or DAG is correct because the route *is* the track. A stochastic agent told one step has real probability of confidently doing the wrong thing. So you do not script the route; you fix the *map* (the role topology: the loop's stages, legal transitions, and gates) and let the *route* through it be chosen and re-chosen at each node. The map is deterministic and trusted; the route is dynamic and re-routed on failure. The gates are the windshield. The agent's signature failure is not a wrong turn on a real map; it is hallucinating a road that does not exist, and only ground-truth gates catch that.

This is a control system (reconciliation loop + error budgets + ratchet), not a workflow engine. Every view above is a slice of that system at a chosen altitude.

Two docs describe that control system on an axis orthogonal to the six loop views — not a seventh view of the loop, but the loop's **structure and behavior**: [the-agent-factory.md](the-agent-factory.md) names the **citizens** (the control-plane primitives + the adapter taxonomy: which named surface is the controller, the actuator, the instrument, or a guard), and [control-loop-model.md](control-loop-model.md) (also the control-loop view above) names how those citizens **behave** over the two timescales. The six views are *where you are in the loop*; these two are *what the loop is made of and why it converges*.

## What this doc is not

- Not a rewrite or rename of the five sources. Each remains canonical for its own altitude; this only indexes and relates them.
- Not new doctrine beyond elevating "system, not DAG" from an implicit `3.0.md` passage to a named principle.
- Not a code change.

## Open items (flagged, not fixed here)

- **12-factor numbering drift.** The doctrine repo orders factors in four phases (Prepare/Bound/Select/Govern) with the loop tier at Validate Externally / Lock Progress Forward / Extract Learnings / Compound Knowledge / Measure Outcomes; older citations (including the personal-site CDLC essay's external factor links) use a V–IX numbering. The external `12factoragentops.com/factors/0N-*` links in `cdlc.mdx` should be checked against the current numbering. File a bead if the live links resolve to renumbered factors.
