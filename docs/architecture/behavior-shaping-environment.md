# Behavior-Shaping Environment

> A behavioral lens on the loop — **not** the product's spine. The spine is the [Operating Loop](operating-loop.md) inside the SDLC control plane; this is a complementary frame: many AgentOps mechanisms can be read as operant conditioning — arranging antecedents and reinforcement so agents reliably perform the behaviors the operator and agent agree upon. Companion to [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md) (the ports) and [Ports and Adapters](ports-and-adapters.md) (the seams). Doctrine source: operator framing 2026-05-22 ("working with agents is like shaping a dog's behavior — arrange the environment, then reward/stop"); promote changes to that framing first.

You cannot compile a behavior into a non-deterministic model the way you specify a deterministic system. A prose spec assumes you can *dictate* behavior. You can't — a stochastic learner only acquires behavior through **observable examples and reinforcement**. So AgentOps does not try to specify what an agent will do; it **shapes** what an agent does, using the mechanics of operant conditioning. This is why behavior-driven development works so well here: a `.feature` scenario is not a description, it is a behavior added to a repertoire and reinforced until it is reliably exhibited.

## The ABC model

Operant conditioning runs on three terms — **Antecedent → Behavior → Consequence**. Every AgentOps surface is one of the three. The product already embodies this; this doc names it.

| Term | What it is | AgentOps mechanism |
|---|---|---|
| **Antecedent** (arrange the environment) | the context that makes the agreed behavior the *likely* one — set up *before* the behavior, not corrected after | `CLAUDE.md`/`AGENTS.md` (always-loaded context), `ao inject` (just-in-time, decay-ranked context), `GOALS.md` (standing intent), the corpus (`.agents/` — prior behaviors retrievable), skill frontmatter `consumes`/`produces` |
| **Discriminative stimulus** (the cue) | the signal that says *which* behavior to emit now | skill triggers, the intent/issue, the loop's current move |
| **Behavior** (what we agree on) | the discrete, observable action the agent should perform | `.feature` scenarios + bead-embedded `## Scenarios` (Given/When/Then) — each one a behavior, additive and composable, never a rewrite |
| **Consequence: reinforcement** (reward) | what makes the behavior more likely next time | passing gates (`/vibe`, `validation`, CI green), merge to main, citation/confidence feedback; the **ratchet** locks a reinforced behavior permanently (`can't un-ratchet` = a fixed habit) |
| **Consequence: stop / extinction** (punishment / non-reward) | what makes a behavior less likely or removes it | PreToolUse hook denials, halt-check STOP/kill markers, holdout-isolation gates, reverts (behavior not reinforced → not merged); **extinction** = deleting a scenario or gate so the behavior is no longer cued or rewarded |

## Why the lens is useful, not just a metaphor

- **Behavior is the unit of work** ([Operating Loop](operating-loop.md) principle 2) because behavior is the unit of *learning*. A vertical slice demonstrates one behavior; a layer demonstrates nothing an agent can be reinforced on.
- **The first failing test is the slice's contract** because red→green *is* shaping: the failing scenario is the target behavior, and the agent reinforces toward it through successive approximations until it is exhibited (green).
- **The promotion ratchet kills artifacts that don't change future behavior** because that is the definition of a reinforcer — if it doesn't change the next behavior, it isn't one.
- **`/evolve` is the shaping loop run continuously**: select a behavior to add or strengthen, run the slice, reinforce on green, lock with the ratchet, repeat.

## Governing principles

1. **Arrange the antecedent first.** The highest-leverage intervention is environmental: make the agreed behavior the easy, likely one *before* execution, rather than correcting drift after. Context arrangement (`ao inject`, GOALS, the corpus) is not overhead — it is the antecedent.
2. **Add and shape behaviors; do not big-design them.** Extend a skill by adding a scenario and shaping it to green, not by rewriting prose. Behaviors compose; designs collide.
3. **Every behavior needs a consequence.** A behavior with no gate is unreinforced and will drift. A behavior to remove needs extinction (delete its cue and reward), not a comment saying "don't."
4. **Reinforce on observable proof, never on status text.** Green is the reinforcer; "looks done" is not. This is the same discipline as [Operating Loop](operating-loop.md) move 7 (evidence under the ratchet).
5. **Stop is a first-class consequence.** The stop/extinction surfaces (hooks, STOP markers, kill switches, reverts) are not failure handling — they are the punishment/extinction half of the contract, and they belong to one coherent consequence system.

## Where this frame propagates

- **Domain vocabulary:** behavior-science is a domain axis alongside DDD (vocabulary), Hex (structure), and Gherkin (behavior) — see `skills/domain/references/`.
- **Acceptance:** every `.feature` is a shaped behavior; see [Intent-to-Loop Hexagon](intent-to-loop-hexagon.md) and `skills/*/references/*.feature`.
- **Reinforcement contract:** `GOALS.md` declares which behaviors are reinforced and which are extinguished; `ao goals measure` is the schedule check.
- **Positioning:** the outward thesis is "arrange the environment and reinforce the behaviors you both agree on" — see `README.md` / `PRODUCT.md`.
