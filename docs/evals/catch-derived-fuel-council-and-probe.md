# Catch-Derived Fuel for E5 — Cross-Family Council (TIERED_MIDDLE) + Precision Probe (2026-06-22)

> **The fork:** the membrane self-improves from ESCAPES (a CONFIRMED verdict later
> overturned = a proven MISS). But a *good* membrane catches at review, so it
> generates ~0 escapes (production: 591 yield events, 99 CONFIRMED + 31 REFUTED,
> **0 escapes**) — it structurally starves its own improvement governor (E5). The
> 31 REFUTEDs are *catches* (the membrane working). **Should catch-derived checks
> ALSO fuel E5's domain memory, or keep the corpus escape-only?**
>
> **Decision: TIERED_MIDDLE (cross-family council, unanimous). Precision falsification:
> PASSED (small). Value falsification (transfer test, on real subtle samples): NO LIFT —
> the catches were already caught by even a cheap tier, so catch-fuel is precision-safe
> but value-null on available data (escape-only-in-practice is the right default).**

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

## Value half — TRANSFER TEST: NO LIFT (the catches are already-caught)

The value half was **subsequently tested** once a stronger producer
(Qwen2.5-Coder-32B) supplied real compiling-but-subtly-wrong samples (see
[the stronger-producer run](./membrane-escape-harvest-no-escape.md#update--stronger-producer-qwen25-coder-32b-the-substrate-exists-but-the-membrane-still-catches)).
Qwen shipped two compiling subtle schema bugs (`rfd-codex-schema`,
`rfd-nested-schema`) that the Haiku **panel** caught 3/3. The transfer test:
does a catch-derived check help a **cheaper tier** (a single, terse "fast skim"
reviewer) catch them when it would otherwise miss?

| | cheap tier WITHOUT check | cheap tier WITH the derived check |
|---|---|---|
| catch on the 2 schema bugs | **6/6** | **6/6** (`transfer_lift = 0`) |
| false-alarm on a true-done control | 0/3 | 0/3 |

**NO LIFT.** The cheap single-reviewer tier already caught both bugs 6/6 *without*
the check — the bugs are **not a cheap-tier blindspot**. So catch-derived checks for
them add **no demonstrable catch-rate value**. This empirically confirms the
council's **Goodhart lens**, which predicted exactly this: "most catches encode
failure-modes the membrane ALREADY catches reliably." On the real data available,
catch-fuel is **precision-safe but value-null** — it would seed the corpus with
already-handled cases. Catch-fuel's value depends on finding a catch that is a
genuine *blindspot for the deployed tier*; none appeared here (the deployed tier,
even cheap, was competent).

## Earlier limit (now resolved): the precision probe was small

The precision probe is **small** (n=2 true-dones, 1 round, Haiku panel) and tested
only **precision** (catch-fuel doesn't hurt correct work). The **value** half — the
cwo.1 transfer test above — was the gap; it needed **compiling-but-subtly-wrong**
samples, which were the session's root blocker until the stronger producer supplied
them:

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
(the source-tagging policy does not). But the **value half now has a result, and it
is a null**: on real subtle samples, the catches were already caught by even a cheap
tier (`transfer_lift = 0`), so catch-fuel would seed the corpus with already-handled
cases — empirically confirming the Goodhart concern. **Net:** the DESIGN (if you ever
add catches, advisory-only + gauge-isolated) is sound and ready in the seam, but the
DATA does not (yet) justify adding catch-fuel — it is safe-but-valueless on what
exists. Catch-fuel becomes worthwhile only when a catch is a genuine *blindspot for
the deployed tier* (a strong tier catches what the deployed cheap tier misses); until
such a catch appears, **escape-only-in-practice** is the right default, and blocking
constraints stay escape-only regardless. This is the same root as the escape-harvest
floor: a competent membrane (even cheap) catches the subtle bugs, so neither escapes
nor *valuable* catches accrue.
