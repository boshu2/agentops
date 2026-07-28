# FABLE program-level validation — skill-overhaul, intermediate

- **Date:** 2026-07-28
- **Author:** FABLE (`claude-fable-5`), independent program-level validator/helper. Did not author any candidate report, plan, or design.
- **Nature:** the operator-requested **intermediate** validation. **Not** the final dueling-idea-wizards Sol/Fable duel. **No binding PASS, no verdict, no repo/ref/session edit, no answer to the T4 authority question, no implementation/merge/push.**
- **Landing (verified):** `/Users/bo/dev/agentops-worktrees/skill-overhaul`, branch `codex/skill-overhaul-20260724`, HEAD `0088c6e3824da201eabb1e751ac8e976599e0b5c`, tree `c0c43eefb8042af5a6a7877c0f7f0de80149ffc6`, clean; `origin/main f66ab953a` fully ancestral; **root HEALTHY epoch 1**.

---

## Disposition

**PROCEED ON THE EXECUTION TRACK; FREEZE THE AUDIT TRACK.** The program's identity discipline is genuinely strong and its RPI-boundary fidelity is sound. But the four skill-group audits and the Go-CLI audit have become a self-referential **provenance-correction treadmill** — 39 audit revisions and 29 Sol reviews that converged technically rounds ago and now loop on their own bookkeeping — while the actual product path (S0→H0→T2, T3/T4/T5, integration of accepted prerequisites) is less mature and carries a live execution blocker. The shortest trustworthy finish reallocates review budget from the audit loop to the execution lanes.

---

## 1. Is the orchestration faithful to the RPI product boundary and live behavior?

**Substantially yes, with one drift.**

- The **execution designs** (D0 decision, active-proof-gate, S0/H0, T3, T4, T5) are RPI-shaped: single-mint intent snapshots, records-first ordering, Implement-derived Candidate packets, fresh author-distinct Validate, `verdict.v2/v3`, and explicit disclaimers of Git/merge/release authority. The D0 (`d4a9b131…` verdict) and active-proof-gate (`67c6d24e…` verdict) prerequisites went through **real** RPI validation with distinct author/validator context IDs. This is faithful to the operating loop (`docs/architecture/operating-loop.md`), which I re-read against `kernel_v3.py`/`validate_v3.py`.
- The **skill/CLI audits** are advisory analysis documents, correctly and repeatedly disclaimed as "not a `verdict.v2`/`verdict.v3`." They are the program-intent-#1 deliverable (analyze 49 skills). As *analysis*, faithful.
- **The drift:** the audits have become an end in themselves — auditing the audit's own ledger/chain/seal — rather than feeding findings into scoped RPI repairs. The ~213 catalogued findings (28 core + 72 workflow + 35 outcomes + 78 adapters) are real and Sol-confirmed, but the conversion *findings → RPI repair experiments* is under-invested while the analysis is massively over-invested. Analysis is not the product; validated change is.

## 2. Are accepted / pending / rejected identities kept honest?

**The discipline is honest and rigorously enforced; the artifacts keep introducing dishonesty that the discipline then has to catch.** Both halves matter.

Honest and well-drawn:
- Every Sol review pins exact SHA/line/byte at open and close and distinguishes **"unreviewed ≠ rejected"** — T4 explicitly corrects the roadmap's stale "v2 rejected" mislabel (v2 was only preflighted); T5 v4 states its historical `APPROVE` is `NOT_PROVEN`/non-integration-ready. This is exactly right.
- Accepted (off-landing, integration-ready): **active-proof-gate repair3** (Sol PASS, `verdict.v2` minted) — and I verified `cli/internal/proofconsistency/` **is present on landing** (its base landed; repair3 is a further hardening in a separate worktree). **D0 decision repair5** (Sol PASS) — I verified **zero `d0-decision` paths on landing**, so it is accepted but **not integrated**.
- Pending: outcomes-12 v9, adapters-13 v10/v11, go-cli v6 (all Sol `REQUEST_CHANGES`); S0/H0 v9 (`REPAIR_BEFORE_SOL`); T5 v4 (`READY_FOR_SOL`); T4 v4 (unreviewed, authority-blocked).

The recurring dishonesty the reviews catch (all report-integrity, none changing a finding):
- outcomes-12 v9: main body says "this pass"/"re-executed this pass" for probes §9.1/§10 say were **not** rerun in v9.
- adapters-13 v10: labels the v8→v9 block "this revision" (current is v10); splits a historical markdown table; leftover `most`/`several`.
- go-cli v6: claims "all 71 package rows byte-identical" when **five changed**; labels inputs the v4 pair not v5; D0 worktree "verified live" vs "carried from v5" conflict.
- S0/H0 v9: terminates in **`PLAN_V8_COMPLETE`** and sources every adapter from **`/tmp/agentops-v8-adapter.py`** in a v9 subject.

**Net:** identities are kept honest *by exhausting review effort on artifacts that keep lying about their own provenance*, not by clean artifacts. The honesty is real; its cost is unsustainable (see §4).

## 3. Shortest critical path to a trustworthy finish

The audits are **not** on the critical path to a working product — they are a findings catalogue. The critical path is the execution track plus integration:

1. **Freeze the audit track** at each group's last substantively-correct revision (core-12 v8 and workflow-12 v8 are already Sol-PASS; outcomes/adapters/go-cli are substantively PASS with only provenance defects). Accept them as advisory findings with a one-line "provenance bookkeeping may trail the current revision" caveat. Stop commissioning revisions to fix "this pass" labels and mis-flattened YAML path names.
2. **Repair S0/H0 v9 B-1** (before-manifest.json — see §4) before any Sol review or execution; then get **S0 accepted and built**. S0 is the root dependency: H0b, T5, and the Go-CLI's sole P0 (N2) all rest on it.
3. **Integrate the two accepted prerequisites** (active-proof-gate repair3, D0 decision) into landing via caller-owned Git, records-first, each re-verified for exact identity post-merge.
4. **Execute the port lanes in dependency order** — S0 → H0a(`contractv3`) → H0b(`skillprobe`) → H0c(cutover) → T2 merge; T3/T4/T5 per their own edges — each with fresh exact-content Validate. This clears the ungrandfathered `contract_v3.py` ratchet blocker that gates the T2 lane.
5. **Escalate T4's operator-authority question** (do not answer it — §5) so the T4 lane is unblocked or explicitly deferred.
6. **Then** the final Sol/Fable duel → fresh Fable execution validation → combined-tree validation → refresh/push/verify.

## 4. Overbuilt / under-specified / circular / loop-prone artifacts (adversarial)

**OVERBUILT & LOOP-PRONE — the audit treadmill (the dominant program risk).** Revision counts I measured: core-12 **7→v8**, workflow-12 **7→v8**, outcomes-12 **8→v9**, adapters-13 **11→v10/v11**, go-cli **6→v6**; ~29 Sol reviews total. Each is an 800–1,300-line document whose self-describing correction-ledger, lineage-cardinality, severity-history, and seal machinery **generates a new provenance defect every revision** that the next review catches. The last five `REQUEST_CHANGES` (outcomes v9, adapters v10, go-cli v6) are **entirely** provenance/qualitative-word/markdown-table defects — Sol states each changes "no repository finding, severity, or count." This is textbook "another repair loop," and it is the clearest thing to stop.

**UNDER-SPECIFIED with an EXECUTION blocker — S0/H0 v9.** The cross-preflight found `freeze_before_manifest` is defined once and called only in i0a/i0b/s0 — **H0a, H0b, H0c consume `before-manifest.json` that no recipe authors**, `recipe_gate` Part 1 doesn't test for it, and v9's own self-check falsely asserts "all five authored by all six recipes." This fails at runtime **inside `derive-effect`, after the barrier has committed the candidate** — i.e., it corrupts exact identity. Plus B-2 (`PLAN_V8_COMPLETE`) and load-bearing B-3 (adapter sourced from a v8-named path in a plan whose own ledger records a *fatal* v8-era adapter defect). Disposition correctly `REPAIR_BEFORE_SOL`.

**UNDER-SPECIFIED (precision) — T5 v4.** `READY_FOR_SOL`, twelve D-items closed and six independently measured, G1 froze against the live kernel — but **zero JSON blocks**: scope-class `patterns[]` are named, not transcribed, so the preflight instantiated G1 by *inferring* patterns. That inference decides `undeclared_paths` and therefore the verdict. Supply the packet data before execution or a competent implementer diverges. Also correctly flagged: T5 rests on S0 (a candidate), so it cannot begin until S0 is accepted.

**CIRCULAR by nature, correctly refused — T4.** The impossibility proof is **correct**: a candidate-local self-attestation cannot establish execution provenance against an author who controls both producer and witness; the shipped `repin_self` fixpoint (`verify_claim_h1_semantics.py:239`) is the concrete demonstration. T4 v4 correctly **deletes the producer** and routes to fresh-validator independent re-derivation, with sound records-first wiring (T-1..T-6 fixes over v3). **But it changes the accepted property** (execution-provenance → independent re-derivation), which `operating-loop.md` places with the **caller**, and T4 v4 explicitly holds no such authorization (§4, §7). T4 has run **4 rounds** (v1 rejected, v2/v3/v4 preflight loops) and **cannot be unblocked by more design** — only by the operator decision. It is honestly blocked, not defective.

## 5. Does the integration/implementation order preserve exact identity, author-distinct validation, and caller-owned Git?

**The mechanisms do; the order has two live hazards.**

- Exact identity: single-mint intent snapshots, digest-referenced transport, and the kernel's own dual `verify_manifest_v2(final, repository)` gate are used correctly across designs; T4 v4 even re-derives the T-1/T-2/T-5 lesson that runtime artifacts must live under the manifest-excluded packet, never under `docs/`. Author-distinct Validate: enforced (distinct context IDs, records→RED→source ordering) and proven in the D0/active-proof verdicts. Caller-owned Git: every artifact disclaims merge/push and defers integration to the caller. T5's historical Python lane is a read-only oracle, never merged — verified (`262e9d49`/`9f1983f4` are non-ancestors of HEAD).
- **Hazard 1 — executing S0/H0 v9 as written breaks exact identity** (the B-1 unauthored `before-manifest.json`): the candidate would be frozen with an incomplete packet, defeating the very identity the plan exists to preserve. The order is sound only *after* B-1 is repaired.
- **Hazard 2 — no single dependency-ordered integration sequence exists.** Five parallel execution designs at five maturity levels, plus two accepted-but-unintegrated prerequisites, plus a scatter of audits, with **no one integrating artifact** that sequences them into one critical path and one combined-tree validation. Program intent #3/#5 assume that sequence; today it must be assembled. Until it is, "execute in dependency order" is aspirational.

## 6. What Fable should help with next — and what to stop

**STOP now:**
- The skill-audit and Go-CLI-audit revision loops. They are substantively complete; further revisions to fix provenance labels, qualitative-word sweeps, and markdown tables are pure waste that starves the execution track of review budget. Freeze and accept.

**Fable should actively HELP with (execution critical path):**
- Validate the **S0/H0 B-1 repair** (before-manifest authoring + `recipe_gate` file test) — this is the execution blocker and the highest-leverage next check.
- Fresh exact-content validation of **S0** once repaired (root dependency), then **H0a/H0b/H0c** and the port lanes in dependency order.
- Assembling/validating the **single dependency-ordered integration sequence** (prerequisites → S0/H0 → T2 → T3/T4/T5 → T8) and the combined-tree validation the program lacks.
- At program-intent-#5, the **fresh Fable execution validation** and exact combined-tree validation before refresh/push.

**Fable should ESCALATE, not answer:**
- The **T4 operator-authority question** — authorizing the property change (execution-provenance → independent re-derivation) and superseding the T4E-1..T4E-4 family is a caller one-way-door decision. Surface it plainly; do not decide it. (This report does not decide it.)

## Blockers vs optimizations

**Blockers (must clear before a trustworthy finish):**
- **BL-1** S0/H0 v9 B-1: `before-manifest.json` consumed by H0a/H0b/H0c, authored by none; corrupts exact identity at runtime. Repair before Sol/execution.
- **BL-2** S0 unbuilt/unaccepted: root dependency of H0b, T5, and Go-CLI N2 (the sole P0). Nothing downstream can validate until S0 is accepted.
- **BL-3** Accepted prerequisites off-landing: active-proof-gate repair3 and D0 decision are accepted but not integrated; integration is caller-owned and pending.
- **BL-4** `contract_v3.py` ungrandfathered governed Python: blocks the T2 lane merge until the `contractv3` port (H0a) lands. (Head-scope ratchet passes today; the block is the future merge.)
- **BL-5** T4 operator-authority: T4 cannot proceed without the operator property-change decision. (Escalate, do not answer.)
- **BL-6** No single dependency-ordered integration + combined-tree sequence exists yet.

**Optimizations (improve but do not block):**
- OP-1 Freeze/accept the audit track; stop the provenance loops (net *reduces* waste).
- OP-2 Supply T5 v4's per-slice packet JSON so `undeclared_paths` isn't left to implementer inference.
- OP-3 S0/H0 v9 B-2/B-3 label repairs (B-3 load-bearing but cheap).

## Checked

- Live landing HEAD/tree/clean status, origin/main ancestry, root health (epoch 1), and the 17 commits since the P0-recovery landing (ratchet/JSONL/regen fail-closed repairs, all landed).
- Integration state: `proofconsistency` present; `d0-decision`, `strictjson`, `contractv3`, `skillprobe` absent; zero `t4-` paths; ratchet PASS/24-grandfathered; `contract_v3.py` ungrandfathered.
- In full: the two accepted-prerequisite Sol reviews (active-proof-gate repair3, D0 repair5); the four accepted/in-flight skill-audit Sol reviews (core-12 v8, workflow-12 v8, outcomes-12 v9, adapters-13 v10); the Go-CLI v6 Sol review; the S0/H0 v9 preflight; the T5 v4 preflight; the T4 v4 design (lines 1–794 of 1311, covering the impossibility proof, authority separation, and mechanism).
- The live operating-loop/kernel ownership contract and the ratchet, cross-checked against the reviews' claims.
- Revision-count census of every audit and design lineage in `/tmp`.

## Not checked

- **No binding PASS, no verdict, no repo/ref/worktree/session edit, no implementation/merge/push, and no answer to the T4 §7 authority question.**
- The full audit **bodies** (core/workflow/outcomes/adapters, ~800–1,300 lines each) and the S0/H0 v9 plan body (3,083 lines) — I relied on their exact Sol reviews/preflights, which I read in full and cross-checked against live facts, rather than re-deriving every internal claim.
- T4 v4 lines 795–1311 (later mechanism sections); the impossibility, authority, and structural facts I judged are in the read portion.
- I did not re-execute any Sol probe, freeze any packet, or build any Go owner; the execution-track maturity judgments rest on the preflights plus the live integration-state facts I verified.
- Linux/Windows behavior; the final combined-tree state (does not yet exist).

## Seal

This report is sealed at publication. Its whole-file SHA-256, line count, byte count, and mode are computed after sealing and reported out of band (a file cannot contain its own whole-file digest). No repository file, ref, index, worktree, projection, or input artifact was modified. **PROGRAM_VALIDATION_V1_COMPLETE.**
