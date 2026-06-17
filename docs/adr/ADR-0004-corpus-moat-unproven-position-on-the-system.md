# ADR-0004: Corpus Moat Unproven — Position on the Verification System, Keep Forge Prose-Default

- **Status:** Accepted (2026-06-17)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0002](ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md)
- **Tracking:** epic `age-kf-s1-close-loop-0ly`, beads `age-kf-s1-close-loop-0ly.4` (moat proof), `age-xgi` (scenario headroom audit), `age-jrq` (forge default flip — cancelled by this ADR)
- **Evidence:** `docs/evals/agentops-effectiveness-evidence.md`, `docs/evals/applied-ood-claim-rule.md` (locked prereg), `.agents/evals/scenario-ab-004-enforce-ceiling-2026-06-17.json`

## Context

The "knowledge corpus improves agent reliability" claim (the *corpus moat*) was put
under a pre-registered A/B ruler (`ao eval scenario-ab`: control without-gold vs
treatment with-gold, cross-family judge, deterministic gate). By 2026-06-17 the ruler
itself is **built and valid**:

- Arm filesystem isolation — `age-9a9` CLOSED (macOS `sandbox-exec` deny-read; the
  control cannot grep the corpus off disk).
- Gold retrieval — `age-r3w` CLOSED (gold findings surface via `ao lookup --gold`).
- Ceiling pre-screen (`age-707`), per-arm absolute grading (`age-oe2`), fail-closed
  moat aggregation (`age-sb0`) — all working; the gate fails LOUD.

With a valid ruler, the blocker became **empirical, not code**. Across **5 diverse
applied-OOD scenarios** — reinforcement-gate, merge-sha-after-push, single-writer-per-file,
codex-stdin, and enforce-via-deterministic-gate-not-instructions (s-2026-06-17-004) —
**codex-cli 0.139.0 scored without-gold 0.8–1.0 unaided**, ceiling-violating every time.
The locked claim rule (`applied-ood-claim-rule.md`) bans the only cheap alternative
(corpus-only *facts* → tautological `fact-recall`, `moat_eligible=false`). The squeeze:
obscure-enough-to-fail trends to a banned leakable fact; rich-enough-to-be-real trends
to "the frontier model already does it."

## Decision

1. **The corpus moat is UNPROVEN, not disproven.** A frontier model is the *worst*
   case to prove it on — it already applies AgentOps doctrine ≥0.8 unaided on
   cheaply-constructible single tasks, so the corpus shows no measurable marginal
   uplift *at that altitude*. We do not claim a corpus moat.

2. **Position AgentOps on what is PROVEN: the verification/control system** — the
   deterministic gates, arm/corpus isolation, append-only hash-chained provenance,
   and fail-loud rulers (including this one, which correctly refused to certify an
   unproven claim). The value proposition is reliability *infrastructure* for
   stochastic agentic work, not "our corpus beats a frontier model on a task."

3. **Forge stays prose-default.** The typed-extraction engine (epic `age-mra`) is
   built, live-proven to run, and shipped **opt-in** (`--typed` / `AGENTOPS_FORGE_TYPED`,
   default OFF). Flipping the default (`age-jrq`) was gated on a valid positive delta;
   with none, **`age-jrq` is cancelled by this ADR** — do not flip the forge default
   on current evidence.

4. **Stop grinding frontier single-task scenarios.** 5 ceiling violations across
   distinct doctrines is conclusive at this altitude; further such scenarios are
   diminishing returns and must not be authored to manufacture a positive (the
   prereg is locked).

## Revival conditions (when to reopen the moat claim)

Reopen `age-jrq` / re-run the ruler ONLY if one of these produces a **valid** positive
(per the locked publication rule — isolated, ceiling-cleared, non-author-reviewed):

- **Weaker base model:** the same A/B against a weaker/cheaper worker (e.g. bushido
  Qwen) — the regime where a navigator/corpus should actually help a stochastic agent.
- **Longitudinal/compounding eval:** a new eval that measures flywheel *accumulation*
  across many tasks/sessions (a single-task A/B structurally cannot capture it).

## Consequences

- No marketing or doc may assert a corpus-delta moat. The effectiveness register stays
  the honest source of truth.
- The typed extraction engine remains available + proven opt-in; no work is wasted —
  it is simply not the default until earned.
- Anyone tempted to flip the forge default or re-grind frontier scenarios should read
  this ADR first.
