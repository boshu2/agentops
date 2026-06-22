# ADR-0011: Escape-Corpus Self-Improvement — Mechanism Proven, Compounding Unproven (Structural Data-Starvation)

- **Status:** Accepted (2026-06-22)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0004](ADR-0004-corpus-moat-unproven-position-on-the-system.md) (corpus moat unproven), the EM spine (`age-membrane-memory-arch-tz2s.2`, the escape→check→block loop proven e2e)
- **Evidence:** `docs/evals/membrane-escape-harvest-no-escape.md` (+ its Qwen2.5-Coder update), `docs/evals/catch-derived-fuel-council-and-probe.md`, `docs/evals/membrane-harvest-qwen-coder-2026-06-22-scorecard.json`, `docs/evals/catchfuel-transfer-2026-06-22-scorecard.json`, beads `age-cwo.3`…`age-cwo.6`

## Context

The doctrine (`AGENTS.md`) states the membrane "is self-improving … the escape-corpus
is the asset that compounds." The self-improvement **mechanism** is genuinely proven:
the EM spine demonstrates `escape → derived check → future block` end-to-end on the
shipped binary, travelling to CI. But "the escape-corpus *compounds*" is a claim about
**data accrual over time**, which was never measured — until this session put it under
the lab harness and the production ledger.

## Evidence (four independent confirmations, 2026-06-22)

1. **Production:** the yield ledger holds 591 total events; within them are **130 real
   gate-verdicts** (99 CONFIRMED + 31 REFUTED), of which **0 are escapes** (no CONFIRMED
   was ever overturned), and there are 0 escape-derived constraints. The 31 REFUTEDs are
   *catches at review* (the membrane working), not misses.
2. **Weak producer** (Phi-4-mini, 9 tasks): **NO_ESCAPE** — the cross-family Haiku
   membrane caught every false-done (the failures were loud / non-compiling).
3. **Strong producer** (Qwen2.5-Coder-32B — *exactly* ADR-0004's named "revival
   condition: weaker/cheaper base model"): shipped 2 **compiling-but-subtly-wrong**
   schema bugs (`rfd-codex-schema`, `rfd-nested-schema`, the OpenAI strict-mode class) —
   the genuinely-subtle samples the compounding thesis would need — and the membrane
   **caught both 3/3**. (Separately, one NON-compiling lab task, `hard-deep-merge`, was
   missed by a single reviewer 1/3 — a **lab-only noise escape**, explicitly NOT a
   production escape and not evidence of accrual; its derived check did not help on
   re-measure. Production escapes remain 0.)
4. **Catch-fuel** (the obvious un-starving alternative — learn from *catches*, not just
   misses): a cheaper reviewer tier caught the subtle bugs **6/6 unaided** (transfer
   lift 0). Even catches that could fuel improvement are already-handled.

**Root:** a *competent* membrane catches at review, so it structurally generates ~0 of
its own misses. **Self-improvement-from-escapes is anti-correlated with membrane
quality** — the better the membrane, the fewer escapes, the less the escape-corpus
accrues. The catch-fuel alternative is precision-safe ([TIERED_MIDDLE council](../evals/catch-derived-fuel-council-and-probe.md)) but value-null on available data.

## Decision

1. **The self-improvement MECHANISM stays stated as proven** — escape→check→block works
   e2e (the EM spine). This is not in question.
2. **"The escape-corpus *compounds*" is DEMOTED to an unproven hypothesis** — alongside
   the knowledge corpus / flywheel (ADR-0004), not stated as established fact. It faces
   a **structural data-starvation headwind** (a competent membrane self-starves its
   escape supply). `AGENTS.md` is hedged accordingly.
3. **Not disproven, calibrated.** The evidence is lab-heavy (9 toy Go tasks, a Haiku
   membrane) plus a thin, partly-synthetic production sample (591 events). It is
   **suggestive, not conclusive** for all production regimes. We record the dissonance;
   we do not claim the compounding thesis is dead.
4. **Stop manufacturing lab escapes.** The escape-harvest harness is built and reusable
   (`evals/membrane/harvest.sh` + workflows), but further weak/strong-producer lab runs
   to force an escape are diminishing returns — the result (a competent membrane catches
   even subtle bugs) is consistent across producers. Reserve a run only for a specific
   new question.

## Revival conditions (when the compounding claim can be re-opened)

Re-open if any of these produces real, accruing escape data:

- **Real production escapes accrue over time** — a deployed membrane tier *misses* a
  false-done that a stronger later check overturns, organically, repeatedly. (The
  capture path is wired; it just has ~0 input today.)
- **A deployed CHEAP tier with genuine blindspots** — if production runs a cheaper
  membrane than the strong reviewing tier, a strong-tier catch becomes a real blindspot
  for the cheap tier, and catch-fuel (advisory, source-tagged, gauge-isolated per the
  TIERED_MIDDLE council) gains demonstrable transfer value. The transfer test found no
  such blindspot in the tiers tested; a real cheap-deployment gap would change that.

## Consequences

- The product's **proven** claim is unchanged and strong: independent cross-family /
  deterministic verification — **no verdict = not done**. That is the membrane.
- The **moat** narrative must not lean on a compounding escape-corpus as established;
  both the knowledge corpus (ADR-0004) and the escape-corpus (this ADR) are demoted
  hypotheses. Position on the verification/control system that is proven, not on a
  self-improvement flywheel that the data has not yet shown accruing.
