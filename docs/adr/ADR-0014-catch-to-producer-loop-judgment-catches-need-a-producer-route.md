# ADR-0014: The Catch→Producer Loop — Judgment-Class Catches Need a Producer Route, Not a Mechanical Check

- **Status:** Superseded by the Cathedral Cut (2026-07-14)
- **Author:** AgentOps maintainers
- **Builds on:** [ADR-0011](ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) (escape-corpus compounding unproven — data-starved), [ADR-0004](ADR-0004-corpus-moat-unproven-position-on-the-system.md) (corpus moat unproven). The EM spine (`escape → derived check → future block`) and `ao membrane {catch,triage,derive-checks}`.
- **Evidence:** historical catch data summarized below. The optional replacement
  contract is [`docs/contracts/producer-defect-register.md`](../contracts/producer-defect-register.md).

> This ADR records why the old membrane feedback machinery failed. Its commands,
> automatic routing, receipts, and producer-side mutation are not active product
> behavior. Learn may later analyze caller-supplied verdict collections, but it
> has no critical-path or lifecycle authority.

## Context

The doctrine says the membrane is self-improving: what it catches should feed back so the system produces fewer things to catch. We have tried this. The machinery exists and is wired: `ao membrane catch` is auto-invoked from the pawl on every REFUTE (`scripts/pawl-review.sh:182`), catches accrue to `.agents/yield/yield-ledger.jsonl`, `ao membrane triage` mines them, and `/post-mortem` Step BP.7 ("Two-Strikes Membrane Triage") reviews recurring classes. Yet the operator's lived experience is *"it's not working — the loop keeps catching the same kinds of things."* This ADR says why, from the production numbers.

### The numbers (2026-07-08 catch corpus)

- **512 gate-verdicts: 309 CONFIRMED, 201 REFUTED, 2 ESCALATE.** The membrane is catching a lot — **201 real catches**, not a data drought.
- `ao membrane triage`: **22 distinct catch classes, 13 recurring, 122 unclassified**, `Decision: CURATED`.
- **`Axis2Compilable: 0.00` — ZERO of the 13 recurring classes can be compiled into a mechanical check.** Coverage is 100%: 0 compilable, 13 not-compilable, 0 unassessed. Every recurring catch is **judgment-class**.
- **122 catches are UNCLASSIFIED** — "reason-less REFUTEDs are an UNCLASSIFIED floor, counted, never synthesized into a class." This has a specific mechanical cause: the gate-verdict record is pure metadata (`attempt, author_family, cross_family, difficulty, refuter_families…`) with **no defect text**, and `emit_pawl_catch` (`scripts/pawl-review.sh`) extracted the reason only from a `VERDICT: REFUTED <text>` sentinel — but the **multi-family panel** (the warm/good path) emits `PAWL <nonce> REFUTED` with no trailing reason, so the reviewer's actual finding (which sits as a `REFUTED: <finding>` prose line in the evidence) was dropped and the catch fell to a generic placeholder → the floor. **Fixed here:** `emit_pawl_catch` now salvages the `REFUTED: <finding>` line for routed/multi-family REFUTEs, so catches are born with a real reason and can be synthesized into a class. A catch the loop can't *read* can't be routed — this is the enabler that makes the rest non-vacuous.

### Two different feedback loops, aimed at two different targets

The failure is a **target mismatch**, and it hides behind one word ("self-improving") standing for two distinct loops:

| Loop | Input | Output | Target it improves | Status |
|---|---|---|---|---|
| **A. Escape → check** | *escapes* (a CONFIRMED later proven wrong) | a mechanical membrane detector | the **membrane** (catch more) | **data-starved** — ADR-0011: 0 escapes in 130 verdicts; a competent membrane catches at review, so escapes are structurally rare |
| **B. Catch → check** | *catches* (REFUTEs) | a mechanical membrane detector | the **membrane** (catch more) | **not data-starved, but blocked**: 201 catches exist, but `Axis2Compilable=0.00` — judgment defects can't be compiled into a deterministic check |
| **C. Catch → producer** | *catches* (REFUTEs) | a **producer-side rule** (skill standard / CLAUDE.md footgun / planning-rule / pre-mortem item) | the **operating loop / producer** (produce fewer) | **the missing route** — this is what the operator actually wants, and it is the only loop that can absorb a judgment defect |

Loops A and B both terminate in a *mechanical membrane check*. A is starved of input; B has input but the input is the wrong shape (judgment, not compilable). The existing Step BP.7 encodes loop B with binding preference **"gate > discovery checklist > prose"** — it frames the mechanical gate as the goal and the producer surfaces (checklist/prose) as the weak, explicitly-**UNMEASURED** fallback. But for a judgment defect, the producer surface is not a fallback — **it is the correct and highest feasible binding.** You cannot write a deterministic gate for "you enumerated bd children via `show`'s `dependents[]`, which bd omits"; you *can* write a producer rule that stops the agent from doing it again.

## Decision

Make **Loop C (catch → producer)** a first-class, measured route — the primary destination for the judgment-class catches that are ~100% of the recurring corpus. Concretely:

1. **Reframe Step BP.7.** For a recurring class (`HitCount ≥ 2`), pick the binding by the *class's nature*, not a fixed gate-first preference:
   - **Compilable** (rare) → a mechanical gate (Loop B). Keep this path.
   - **Judgment-class** (the common case) → a **producer-side fix**: a rule in the owning skill's standards, a `CLAUDE.md`/`AGENTS-*.md` footgun, a `/plan` planning-rule, or a `/discovery` pre-mortem check. This is Loop C, and it is *not* a downgrade — it is the right tool.
2. **Record every routing in a committed [producer-defect register](../architecture/producer-defect-register.md).** One row per acted-on recurring class: the defect, the producer fix + where it was bound, and — the honest part — the **recurrence count before vs. after** the fix. This is the measurement Step BP.7 always lacked.
3. **The proof-of-working is a recurrence DROP per class, not corpus size.** We do not claim a compounding moat (ADR-0004/0011 remain unproven and we do not market ahead of them). We claim exactly one falsifiable thing: after a producer fix binds for class X, class X's catch-rate should fall. The register makes that checkable; a class that keeps recurring after a fix means the binding was too weak (escalate it).

## Consequences

- **Judgment catches stop dead-ending.** The 13 recurring classes now have a destination (a producer surface) instead of waiting for a compiler that will never accept them.
- **Honest scope.** This does not resurrect the compounding-moat claim. It is a smaller, testable claim: a durable route from a caught defect to a producer rule, with a per-class recurrence measure. If recurrence does not drop, the register shows it and the ADR is falsified in practice.
- **The 122 unclassified catches are latent fuel** — classifying them is the backlog that feeds the register. Left unclassified they are invisible; that classification work is now the point of Step BP.7, not an afterthought.
- **Loops A and B are retained but demoted to their honest niche:** the rare compilable/escape case. They are not the mechanism for the judgment-class bulk.

## Seed (this session is the existence proof)

This very session ran Loop C by hand, which is why it is credible as a route:
- **Catch:** the multi-family panel REFUTED `age-f07z` — a bd fail-open (children enumerated via `show`'s `dependents[]`, which bd omits). **Producer fix:** a tracker-standards rule + the corrected skill. Judgment-class; a gate could not have expressed it.
- **Catches:** false-REFUTEs from a stale `cli/bin/ao` and a wrong-cwd `go` repro. **Producer fixes:** the fast-gate quick-wins (`age-jmfl`, `age-n8dt`) — these *were* mechanical (Loop B), and they landed. The mix is the point: route each class by its nature.

These are the register's first rows. The test of this ADR is whether, a month out, the register shows those classes recurring *less*.
