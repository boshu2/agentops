# ADR-0008: `/evolve` Operating Model — Intelligent-Agile, Not Waterfall

- **Status:** Accepted (2026-05-22)
- **Author:** AgentOps maintainers
- **Tracking:** bead `soc-sfjx`
- **Builds on:** [ADR-0007](ADR-0007-deterministic-loop-only-operator-stops.md)
- **Origin:** ported from mt-olympus (`docs/decisions/2026-05-21-operating-model-intelligent-agile-not-waterfall.md`).

## Context

An autonomous loop fails in two opposite ways: it halts the moment work gets hard (over-cautious waterfall), or it confidently builds the wrong thing because nothing re-anchors it to operator intent (unanchored drift). The `soc-g2qd` session (2026-05-21) hit the second failure — it executed a bead queue as if it were validated spec, never re-derived the operator's actual goal, and shipped primitives no consumer used. This ADR defines the operating contract that prevents both.

## Decision

Three layers, read every cycle:

1. **Intent (operator-owned, durable, re-read each cycle).** `GOALS.md`, `PRODUCT.md`, `PROGRAM.md`, the ADRs in this directory, and `bd ready`. The operator signals by editing these + by git commit + by bead-status change — not by chat. Each cycle the loop checks for new operator signal (`git log --oneline -1`, `bd ready`) before selecting work. **The bead queue is a hypothesis, not validated spec** — every cycle re-confirms the work still serves the stated goal.

2. **Contract (locked before execution).** The architectural guardrail — agentops's 5 Bounded Contexts + the hexagonal `consumes`/`produces` skill frontmatter — is fixed up front. The loop shapes *within* the contract; it does not redraw it autonomously.

3. **Execution + shaping authority (agent-owned, within the contract).** The loop may: pick atomic primitives, file discovered-from sub-beads, decompose monolithic beads via scout-mode, update the recommended-next pointer, and write ADRs documenting decisions made during execution. **Bounded: one slice per cycle**, gated (build + test holds-or-rises + lint), reverted on red.

## The scope-precondition audit (anti-drift)

Before implementing a candidate, apply the Primitive Test: does the bead name files, have observable acceptance, and cite a sibling/predecessor pattern? **If the candidate is really an architecture decision disguised as bounded work, file a bead instead of implementing it.** This is the guardrail that would have caught `soc-g2qd`: "build six CLI primitives" was architecture masquerading as bounded beads, and no audit flagged that the consumer never wired up. An integration slice with an L2 test exercising the *consumer* is mandatory for any "build a primitive" arc.

## Consequences

- The loop ships continuously without halting, but cannot autonomously redirect outside operator-set intent.
- "Looks done" (green per-unit gates) is not "is done" — the consumer-calls-the-primitive L2 test is the close-gate, not unit-green alone.
- Decisions made mid-execution are captured as ADRs here, so the doctrine compounds.

## Evidence

mt-olympus's loop sustained a 10-cycle marathon (cycles 123–129) shipping 6 critical-path hops + 14 filed sub-beads with the operator confirming intent once — because this model let it shape within a locked contract while re-reading intent each cycle. agentops lacked the model on 2026-05-21 and drifted. See `.agents/research/2026-05-21-mt-olympus-evolve-loop.md`.
