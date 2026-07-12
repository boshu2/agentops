# Decision: make `ao pawl review --strict` satisfiable

- **Bead:** age-pawl-intent-zhndq.8 (F8a) — decision only; implementation is .9 (F8b).
- **Date:** 2026-07-11
- **Author (proposer):** Claude (Fable) — NOT a judge (no self-grading).
- **Judges (cross-family quorum, both distinct from the author):**
  - `codex` (gpt family) — **RECOMMENDATION: B**
  - `agy` (gemini family) — **RECOMMENDATION: C** (= B now, A deferred)
- **Outcome: CERTIFY-THEN-FLIP (the B core, on which both judges agree).**

## The problem
`--strict` is the advertised strongest gate for the highest-irreversibility doors, and it **can never
pass today**: it requires TWO DISTINCT strict-eligible **cold** families and only ONE exists (codex).
agy is A7-benched; there is **no cold claude adapter and never will be** — LAW 0 forbids `claude -p`.
So an operator reaching for maximum rigor gets a guaranteed exit-5 UNAVAILABLE.

## Decision
1. **Do NOT adopt warm-strict (option A) as the primary answer.** It requires operator-only NTM
   machinery, breaking strict's cold-portability, and the measured warm data says the bar may be
   unpassable anyway: the tri panel (cc cod agy) reached **full agreement 0/22 times** (p50 253s),
   versus dual (cc cod) at 10/19 (p50 100s). Both judges independently rejected A as primary.
2. **Certify `agy` as the second strict-eligible COLD family (option B) — but ONLY after it clears a
   measured bar, never as an unmeasured flag flip.** This preserves the cold-portable front door
   (a user with no NTM can still reach the strongest gate) and keeps LAW 0 intact.
3. **Until agy clears the bar, `--strict` stays honestly UNAVAILABLE (exit 5).** Never a silent
   single-family pass. (This is the status quo, so no code change is required to hold the line.)
4. **Option A is deferred, not dead** (agy's C): revisit warm-strict only after the age-xnet land-lane
   redesign, and only if per-family verdict recording lands.

## Why B and not D (keep it unavailable forever)
Live evidence from 2026-07-11: with codex rate-limited, agy served as the sole cross-family cold
reviewer for ~8 beads and produced **a real, correct defect catch** (reconcile falsely treated
REBOUND verdicts as unbound) — genuine independent value. It also produced **a false refute** (it
assumed `source_hash` was a raw sha256 when it is body-normalized; four deterministic gates proved it
wrong). So agy is *useful but noisier* than codex. Crucially, on a strict door a **false REFUTE fails
SAFE** (it HOLDs), while a false CONFIRM would compromise the gate — the asymmetry favors admitting a
noisy-but-real second family, provided its **false-CONFIRM rate is measured at zero**.

## Measurement plan (the bar agy must clear before the eligibility list is flipped)
Synthesized from both judges (codex's is stricter; adopt it, with agy's evidence-quality clause):
- **Sample:** ≥30 cold agy reviews over real held-out beads, **plus seeded known-good and
  known-defective diffs** (so both error directions are measurable).
- **Ground truth:** independent adjudication using the deterministic gates + the existing pawl verdict
  artifacts / gate logs (never agy's own self-report).
- **Pass bar (all three):**
  1. **ZERO false CONFIRMs** (the non-negotiable — a false CONFIRM on a strict door is the failure
     mode strict exists to prevent).
  2. **REFUTE precision ≥ 80%** (bounds the friction: a strict door that false-refutes constantly is a
     denial-of-service, the risk BOTH judges named).
  3. **≥ 80% detection of seeded material defects** (proves it adds real recall, not just noise).
  4. Evidence-quality: every run emits schema-conforming evidence with file:line findings.
- **Publish** the confusion matrix + prompts + evidence packets with the flip.

## Risks (both judges named the same one)
Even at zero false CONFIRMs, agy's noisy REFUTEs could make strict operationally frustrating —
"a safe but costly denial-of-service" (codex) — tempting operators to stop using `--strict` at all,
which would weaken safety overall. The ≥80% REFUTE-precision bar exists to bound exactly this.

## Consequences for .9 (F8b — implementation)
Rescope .9 to: **run the certification eval above** (build the seeded corpus, run the ≥30 reviews,
adjudicate, publish the matrix). Flip `PAWL_STRICT_ELIGIBLE_FAMILIES` to include agy **only if all
three bars pass**. If agy fails the bar, .9 closes with the measured evidence and `--strict` stays
honestly unavailable — a real outcome, not a failure. The honest-UNAVAILABLE path and the
"strict never degrades to one family" invariant are untouched either way.
