# age-cwo close: the self-improving membrane (flywheel) — exactly what is proven

> **Closing record for the flagship epic age-cwo** ("self-improving membrane: close the
> escape→check→re-measure loop, the provable flywheel"). Written 2026-06-23 after turning
> the membrane's *own* adversarial discipline on its *own* flagship proof. The headline:
> **the flywheel MECHANISM is real and demonstrated; its VALUE on the production membrane
> is empirically a no-op for escapes the frontier already catches.** Both halves are stated
> here so the close does not over-claim — over-claiming would itself be the fake-done the
> membrane exists to catch.

## How this close was decided (the membrane on itself)

The epic acceptance — *"demonstrate ONE real self-improvement cycle end-to-end with a measured
catch-rate delta on real data, do not fake-done it"* — was met by **age-1gl / cwo.1**
([cwo1-real-escape-self-improvement.md](cwo1-real-escape-self-improvement.md)). Before closing,
the proof was put through:

1. **A 7-lens adversarial panel** (Claude, fresh-context, each lens told to REFUTE). Result:
   **5 of 7 pillars REFUTED**, 2 SURVIVED. Synthesis: HOLD. The surviving core was
   `catch-delta-real` (verified across 57 transcripts — the 0/3→3/3 re-judging is genuine and
   non-tautological) and `epic-completeness`. The refuted pillars were framing/methodology:
   contrived single-draw escape, *deliberately weak* Haiku membrane, **domain-orthogonal
   false-alarm control**, synthetic e2e fixture, same-predicate "transfer".
2. **An independent cross-family codex verdict**: **CLOSE — "as scoped"** — proves one cycle
   works, not that the corpus compounds.
3. **A production-grade re-measurement** (this doc, §below) that drove the strongest refutation
   to ground: does the *production* membrane (codex) self-improve, and is the false-alarm
   control valid?

## What is PROVEN — the mechanism (cwo.1, weak membrane)

A real escape — a 3.8B Phi-4-mini producer's strict-schema bug (`required` = caller's subset),
**unanimously missed (3/3 ACK)** by a cross-family Claude-Haiku panel blind to a real
deterministic oracle — was turned into a derived check that flipped re-judging **0/3 → 3/3**,
with the held-out true-done control staying ACK. The escape→derive→re-measure **mechanism is
real, unmocked, and closes** (the `em-loop-donetest` also closes e2e on the shipped binary, on a
synthetic detector fixture). **This meets the epic acceptance: one real cycle, a measured delta,
on real data.**

## What is NOT proven — value on the production membrane (this session, codex)

The single most load-bearing objection was: *the cwo.1 blind spot is a property of a deliberately
weak Haiku membrane, not the membrane the product ships (codex).* So I re-ran the same escape
class against the **production membrane (codex / gpt-5.5, xhigh reasoning), N=3 per condition**,
with a **valid same-domain control** (the missing piece in cwo.1). Raw verdicts persisted in
`evals/membrane/harvest-2026-06-22/cwo-production-proof/verdicts.jsonl`:

| Condition | Code | Check? | codex verdict | Reading |
|---|---|---|---|---|
| **before** | nested-loose strict schema (real false-done) | no | **REFUTE 3/3** | codex **catches the escape UNAIDED** — *"nested address lacks required + additionalProperties:false"* |
| **after** | same false-done | + derived check | REFUTE 3/3 | the check **adds nothing** (already caught) |
| **control** | fully recursive-strict schema (real true-done) | + derived check | **ACK 3/3** | **0/3 false-alarm** on a valid same-domain true-done |

**Production-membrane catch-delta = 3/3 → 3/3 (NO delta).** The frontier membrane has **no blind
spot** for this escape, so the flywheel is a **no-op** on it. This both (a) **fixes** cwo.1's
invalid-control hole (the recursive-strict true-done is a real same-domain control the check
*could* over-reject, and codex correctly ACKed it 3/3), and (b) **confirms** the deepest
refutation: the escape-corpus adds value only to a membrane weak enough to *miss* — which the
production membrane, for frontier-catchable escapes, is not.

## Honest scope (what the close claims, and what it does not)

- **CLAIMED (proven):** the self-improvement *mechanism* is real — a real escape can be compiled
  into a derived check that upgrades a **weak/cheap** membrane from miss to catch, without
  false-alarm regression on a valid same-domain control.
- **NOT CLAIMED (unproven / shown false here):** that the *production* (frontier) membrane
  self-improves from this corpus — for the escapes measured, codex catches them unaided, so the
  flywheel is a no-op there. The **moat** (escape-corpus value on the shipped membrane) stays
  **unproven** ([ADR-0004](../adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md)); the
  corpus **compounding** over time stays **unproven** ([ADR-0011](../adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md)).
  n=1 escape class; quarantined eval lane.

**Where the flywheel has value, then:** upgrading a *cheap/weak* membrane tier to catch what it
would miss, and the (rare, hard-to-reproduce) escapes a frontier membrane genuinely misses — not
as a moat over the frontier membrane on escapes it already catches.

## Verdict

age-cwo's acceptance is **met** (mechanism demonstrated, one cycle, measured delta, real data),
and the close is scoped honestly: **mechanism proven, production-moat unproven (and a no-op for
frontier-catchable escapes).** Closing on the mechanism while explicitly *not* claiming the moat
is the discipline-consistent close — the membrane caught its own overstatement, and this record
states the result instead of burying it.

Provenance: adversarial workflow `wf_bc49738d-1fb`; codex seat `/tmp/codex-flywheel-verdict.txt`;
production re-measurement `evals/membrane/harvest-2026-06-22/cwo-production-proof/` (inputs +
`verdicts.jsonl`, codex/gpt-5.5, 9 reviews). Mechanism proof: [cwo1-real-escape-self-improvement.md](cwo1-real-escape-self-improvement.md).
