# Merge-Door Duel — 2026-07-09 (proof packet)

Tracked mirror of the merge-door redesign duel that authorized the **async-membrane epic
(age-xnet)**: the "two-phase done, trunk-based push-to-main" design. The runtime originals
live under gitignored `.agents/` (plans, research, duel verdicts); this directory is the
durable audit copy, mirrored as part of age-ivoq (R0 of age-xnet).

## Contents

| Path | What it is |
|---|---|
| `2026-07-09-merge-door-SYNTHESIS.md` | The synthesis packet (Composite A, hardened) the duel judged |
| `research/merge-door-constraint-model.md` | R1 — measured Theory-of-Constraints / queueing model of the current door |
| `research/merge-door-rebaseline.md` | R2 — re-baseline of the door's guarantees |
| `research/merge-door-design-space.md` | R3 — design space of alternative doors |
| `plans/2026-07-09-merge-door-P1-product.md` | P1 — product plan |
| `plans/2026-07-09-merge-door-P2-architecture.md` | P2 — architecture plan |
| `plans/2026-07-09-merge-door-P3-operations.md` | P3 — operations plan |
| `duel-verdicts/r{1,2,3}/{claude,gpt}.json` | The 6 cross-family duel verdicts (2 families × 3 rounds) |

## Round history

| Round | claude | gpt | Outcome |
|---|---|---|---|
| r1 | WARN (mechanical) | **FAIL** | **REDO** — gpt: the LKG frontier-liveness rule cannot advance after the first REFUTED-after-land (no compensation/resolution edge rehabilitates later state); claude flagged the closure claim. |
| r2 | WARN (mechanical) | **FAIL** | **REDO** — A1's RESOLVED(c) fixed the fix-forward walk, but the promised mechanical-revert walk still had no RESOLVED base case for the revert commit (no second review, not provenance-only), and the evidence floor needed tightening. |
| r3 | **PASS** | **PASS** | **PASS both families** — A5–A8 close every r2 finding one-to-one: the deterministic git-diff inverse-equality check gives the revert commit an L0 verdict (verified-by-compensation base case); acceptance tests (a)–(e) implementable. `ao plan-pawl decide` → **RC 0**. |

Decision: RC 0 → the plan compiled into the age-xnet bead DAG. age-ivoq (this mirror's
carrier bead) is R0: fix the METER instrumentation so the Directive-17 ruler can measure
the before/after the epic claims.
