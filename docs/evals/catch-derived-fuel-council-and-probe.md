# Catch-Derived Fuel for E5 — Cross-Family Council (TIERED_MIDDLE) + Precision Probe (2026-06-22)

> **The fork:** the membrane self-improves from ESCAPES (a CONFIRMED verdict later
> overturned = a proven MISS). But a *good* membrane catches at review, so it
> generates ~0 escapes (production: 591 yield events, 99 CONFIRMED + 31 REFUTED,
> **0 escapes**) — it structurally starves its own improvement governor (E5). The
> 31 REFUTEDs are *catches* (the membrane working). **Should catch-derived checks
> ALSO fuel E5's domain memory, or keep the corpus escape-only?**
>
> **Decision: TIERED_MIDDLE (cross-family council, unanimous). Precision falsification: PASSED.**

## Council decision — TIERED_MIDDLE (Claude 4-lens panel + Codex, independent, agreed)

Catch-derived findings feed the **ADVISORY domain-memory tier ONLY** ("in domain D,
watch for failure-mode X"); the **BLOCKING gate-constraint corpus stays
escape-only** (only a *proven miss of the deployed tier* may hard-block).

The sharp argument (Claude thesis lens, grounded in the code): escape-only
**conflates two different things** — "the producer emitted a false-done" (the
failure worth learning from) and "*my* deployed membrane tier missed it" (an
artifact of membrane STRENGTH). A catch proves the producer failed *just as
conclusively* as an escape. So catch-derived advisory memory is the **same kind of
signal** from a different verdict — and escape-only "starves E5 exactly in the
SUCCESS case." This is also the **proven cwo.1 framing**: "the flywheel upgrades a
CHEAP membrane to catch what a STRONG one already catches" — a strong-tier catch is
the canonical fuel for a cheaper deployed tier.

**The guardrail SEAM already exists in code** (the policy on it does not yet).
`deriveFindingFromEscape` (`cli/cmd/ao/membrane.go`) compiles a finding to a
**blocking** gate constraint ONLY when `DetectorPattern != ""`
(`detectability: mechanical`); without it the finding stays advisory
(`compiler_targets: pre-mortem-check`). That mechanical-vs-advisory seam is
source-independent, so "catches → advisory, only escapes → blocking" is *expressible*
on top of it without new enforcement machinery — but the catch-vs-escape **policy is
not built**. Required additions: source-tagging (catch vs escape, so E5 can weight
escapes higher), gauge-isolation (catches must NOT enter `escape_rate`/`catch_rate`),
and decay/dedup.

## Precision falsification — PASSED (the dominant risk did not fire)

All 4 lenses named the same dominant risk: the advisory tier becomes *de-facto
blocking* through the inject/pre-mortem channel — a membrane primed to "watch for X"
might over-REFUTE borderline-correct work (`false_refute` regression, the exact
thing the cwo.1 control guards). The minimal falsification (the precision half):

| Condition | Membrane REFUTE on genuinely-correct true-dones |
|---|---|
| **WITHOUT** catch-findings | **0 / 6** (6/6 ACK) |
| **WITH** 8 catch-findings injected | **0 / 6** (6/6 ACK) |

Two oracle-verified correct true-dones (a correct non-mutating median; a correct
descending-sort `TopScores`), Haiku panel ×3 each, both conditions. The injected
catch-checks **included domain-relevant ones** ("sort DESCENDING", "no-mutate") — so
a primed reviewer had every opportunity to over-scrutinize. It didn't: the reviewers
*applied* the checks and correctly confirmed the code satisfies them (e.g. "returns
descending order... does not mutate the input"). On this small probe — **n=2
true-dones, 1 round, Haiku panel only** — catch-derived advisory findings were
**precision-safe** (no manufactured false-alarms). This is suggestive, not
conclusive; it shows the dominant risk did not fire on the cases tested, and
supports (does not prove) TIERED_MIDDLE.

## Honest limits — the value half is UNTESTED (blocked on subtle samples)

The probe is **small** (n=2 true-dones, 1 round, Haiku panel) and tests only
**precision** (catch-fuel doesn't hurt correct work on the cases tried), NOT
**value** (does a catch-derived check help a *cheaper* membrane catch a subtle miss
it would otherwise pass — the cwo.1 transfer test?). The value half needs
**compiling-but-subtly-wrong** samples, which is the session's root blocker:

- Production: **0 escapes**, and the 31 catches are **signal-less husks** (0/31
  carry domain/reason in the ledger row; none carry the reviewed code).
- Lab (Phi-4-mini producer): only **loud** failures (non-compiling) — a cheap
  membrane catches those trivially, so no transfer-lift to measure.
- The only source of subtle-wrong samples is a **stronger weak producer**
  (Qwen2.5-Coder-32B). It is **locally memory-infeasible** right now (the Mac has
  ~8 GB free vs the model's ~17 GB) — runnable only on bushido's GPU or by letting
  real production escapes accrue over time.

## Net

Catch-fuel is **council-blessed (TIERED_MIDDLE)** and **precision-safe on a small
probe**, and the mechanical-vs-advisory **seam it would ride already exists in code**
(the source-tagging policy does not). The un-starving path for E5 is clear and, so
far, safe on the precision axis. The remaining gate is **empirical value** (the
transfer-lift half), blocked on the same subtle-sample scarcity that blocks escape
harvest. When E5 is built, catch-derived advisory domain-memory (source-tagged,
gauge-isolated, decayed) is the candidate way to fuel it; blocking constraints stay
escape-only.
