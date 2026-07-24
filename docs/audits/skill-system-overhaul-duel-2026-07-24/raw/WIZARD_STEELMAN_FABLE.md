# WIZARD_STEELMAN_FABLE — steelman challenge

- Author: FABLE (`claude-fable-5`)
- Date: 2026-07-24
- Read for this phase: both rebuttals, completely.
- Selection: the two strongest opponent decisions are (1) the exact-byte
  executable experiment transaction — SOL's origination and the best finding of
  the duel — here strengthened by unifying it with their Phase-4
  `proof_contract` field; and (2) the UNKNOWN-crossing → NOT_PROVEN rule —
  SOL's Phase-4 sharpening of the CLI crossings model. Both steelmen end in
  genuine position changes on my side.

---

## Steelman 1 — The exact-byte experiment transaction, unified with first-class proof-contract identity

### The case, made stronger than its originator made it

SOL argued this from the defect (the digest mismatch) and the ordering (kernel
before campaign). The stronger argument is about *transitive de-risking*: the
transaction is the only component in the entire program that every other
decision reduces to. All twelve catalog acceptance criteria, every tranche
verdict, every epoch comparison, the eventual Goal runtime's trust in its
inputs — each bottoms out in one invariant: *the thing judged is byte-identical
to the thing intended, and the judgment is reproducible.* The live kernel
implements that invariant twice, differently, with unit tests that pass by
mocking the disagreement away (`run_once.py:60` digests a canonical-JSON
mapping; `validate.py` digests exact bytes; `test_run_once.py` supplies
Validate's answer using RPI's own function). A catalog of perfectly typed
contracts judged by an incoherent kernel is a well-organized fiction — no
metadata, tranche, or publication design can compensate.

The mismatch is also a *class* proof, not an instance. It demonstrates that an
invariant implemented by convention in two places will drift, and that isolated
green tests will conceal the drift. That generalization is what makes SOL's
Phase-4 "underrated decision 2" not an add-on but the same decision: carrying
epoch identity as an `evidence_refs` string convention — my proposal — is a
convention-implemented invariant of exactly the class that just failed. The
unified form is stronger than either of SOL's two presentations: **one atomic
kernel revision in which every evidence invariant becomes executable
contract** — identity algorithm (exact bytes, one mint), `rpi-report.v2`
(terminal `stop_reason`; `unchecked_required` vs `declared_exclusions`; a
size-bounded opaque `correlation` map), a required typed `proof_contract`
identity object (contract digest + verdict/report/manifest schema IDs), the
before-manifest requirement for changed-path completeness, the strict Go
reader, and the golden corpus — revised together, with no epoch-1 verdict
minted until every consumer enforces all of it. My bootstrap objection
(epoch 1 needs a carrier before the schema changes) dissolves under kernel-first
ordering: the kernel revision *is* the start of epoch 1; everything before it
is epoch 0 by absence of the field. One revision, five repairs, zero
convention-carried invariants.

There is a second under-argued strength: the campaign dogfoods its product.
Roughly eight to ten tranche verdicts follow the kernel tranche; each is minted
by the repaired instrument, exercising the composed fixture in anger and
accumulating exactly the verdict corpus that Learn and T7 consume. The
migration doesn't just use the kernel — it is the kernel's first sustained
production workload.

### Best incremental implementation path

1. Freeze the golden verdict corpus; write RED composed fixtures first: a real
   byte snapshot flows RPI→Validate with no second digest function;
   whitespace-differing intent bytes yield different digests; subject mutation
   between validation start and end yields NOT_PROVEN.
2. Delete `run_once.py`'s local `digest()`; it accepts a snapshotted intent
   ref + digest; `validate.py snapshot-intent` becomes the only mint.
3. Land the single atomic revision: report v2 + verdict schema rev
   (`proof_contract`, coverage split, correlation) + before-manifest + writer +
   Go `verdictcheck` reader + golden corpus, in one tranche.
4. Extend the pure reference layer with negative-authority state-transition
   tests: each phase dispatched ≤1 across all seven terminal outcomes,
   including `amendment_required`; continuation refused.
5. Strip rpi's continuation envelope and stale trigger in the same tranche,
   naming the **caller** as the standing continuation owner.
6. Pin epoch 1 as the digest over the revised validator scripts + schema
   tuple; epoch 0 is defined by field absence and never pools with epoch ≥ 1.

### Second-order benefits

- The composed RPI→Validate fixture becomes the permanent integration test the
  repo has never had — the standing cure for the isolated-green-tests disease.
- `unchecked_required` vs `declared_exclusions` simultaneously resolves the
  verified PASS/`not_checked` schema collision, gives T6 runtime adapters an
  honest platform-gap channel, and (see Steelman 2) provides the exact field
  that represents unresolved proof-chain edges.
- The `proof_contract` tuple gives T7's completion matrix a mechanical
  partition key and makes verdicts portable across repositories later.
- The bounded `correlation` map gives a future Goal runtime its correlation
  hook with zero campaign state entering the experiment report.

### Pre-empting the two strongest objections

**"Highest-risk change first — the campaign rebuilds its judge before judging
anything."** The risk is bounded on four sides: the surface is pure Python with
no Git/tracker/network calls; the golden corpus freezes expected behavior
before any edit; RED composed fixtures precede mutation; the revision is
atomic with a source-revert rollback and an intact epoch-0 history. The
alternative is not lower risk — migrating 49 contracts on an incoherent kernel
is deferred insolvency with interest.

**"Five repairs in one revision violates smallest-change discipline."** The
five repairs share one subject (the evidence-contract schema corpus) and one
acceptance surface (the composed fixture + golden corpus). Splitting them
multiplies schema revisions, and under the epoch rule every revision is a new
epoch — epoch proliferation in the campaign's first month is strictly worse
than one well-tested atomic cut. D2's one-decision-per-tranche rule is
satisfied: the decision is "the evidence contract becomes executable."

### Honest residual concern I cannot argue away

The unified revision concentrates schema-design judgment at the moment of
least usage data. The `declared_exclusions` comparison — "Validate must reject
an exclusion that removes a criterion" — has no designed mechanics: comparing
exclusions to acceptance criteria mechanically requires criteria to carry
stable IDs, which today they do not. If that comparison ships as prose
matching, coverage laundering returns through the side door; if criterion IDs
are added, the ripple reaches Plan's contract in ways neither wizard has
designed. This is the likeliest cause of an early epoch 2, and I cannot argue
it away — only flag it as the revision's sharpest open edge.

### Does steelmanning change my final position?

Yes, twice. Kernel-first was already conceded in Phase 3. New here: I withdraw
my `evidence_refs` epoch carrier entirely and adopt `proof_contract` as a
required typed field in the same atomic revision. SOL's argument — the digest
mismatch happened *because* an identity invariant was carried by convention —
is decisive against my own proposal, which was another convention.

---

## Steelman 2 — UNKNOWN acceptance-path crossings yield NOT_PROVEN, never PASS

### The case, made stronger than its originator made it

SOL grounded the rule in `AGENTS.md`'s evidence clause. The stronger framing:
this is not a new policy at all — it is the repository's *existing* verdict
semantics applied reflexively to the proof chain itself. The contract already
holds that incomplete changed-path coverage is NOT_PROVEN and that a green
shallow validator cannot stand in for behavioral proof. A tranche whose
acceptance evidence flowed through an edge that *might* reach a frozen-audit
finding has incomplete **evidence-integrity coverage** — the same class as
incomplete path coverage, one level up. A migration that demands fail-closed
evidence from every skill while minting its own completion claims fail-open
would be incoherent in exactly the way it accuses the old skills of being.
And the duel already produced the empirical proof that name-level traces
under-resolve: the orchestration boundary gate was a *named* check that was
simultaneously broken, wired to nothing, and phrase-stale — "a named
acceptance command is not automatically a live, trustworthy proof path" is not
a hypothesis, it is this repo, this week.

The under-argued masterstroke is the **decoupling of schedule from
assurance**. My Phase-4 objection to SOL's original position was critical-path
inflation; the labeling rule dissolves it. Implementation proceeds in
parallel; only the *claim* waits. And verdict immutability makes the mechanics
clean in a way SOL did not spell out: the tranche's verdict is honestly
NOT_PROVEN with the unresolved edges cited in `unchecked_required`; when an
edge resolves, a **new fresh validation over the same frozen subject manifest**
mints the PASS — no mutation, no replay, no epoch violation. The frozen
subject makes re-validation cheap and legitimate. The two steelmen compose:
Steelman 1's coverage split is precisely the field that represents Steelman
2's unresolved edges. That composition is stronger than either decision alone,
and neither of us stated it before now.

### Best incremental implementation path

1. T0: SOL's six-step transitive proof-chain ledger — enumerate every
   acceptance command, expand wrappers/children/output consumers, record
   effects/dry-run/buffering/cleanup per edge, cross-reference the frozen
   audit findings.
2. Classify every edge `USED_SOUND | USED_UNSOUND | PROVEN_UNUSED | UNKNOWN`;
   seed one negative fixture per matched finding and one witness per
   claimed-unused path.
3. Publish the UNKNOWN queue as parallelizable homework with owners —
   resolving an edge is usually one wrapper read plus one fixture.
4. Tranche validation template: cite the ledger; UNKNOWN edges land in
   `unchecked_required`; nonempty `unchecked_required` blocks PASS by the
   kernel's own rule — no new machinery needed.
5. Re-validation path: same subject manifest + updated ledger → new verdict;
   the NOT_PROVEN → PASS transition is two artifacts, both immutable.

### Second-order benefits

- The proof-chain ledger doubles as the seed data for CLI `CommandContract`
  effect/output metadata — one artifact feeds both programs and enforces the
  shared effect vocabulary.
- `unchecked_required` gets a real workload in its first month, proving the
  coverage split earns its schema seat.
- The UNKNOWN count is a natural operator-legible ratchet: a declining number
  that cannot be gamed by narrative.
- The rule generalizes to every future tool the proof chain adopts: admission
  by ledger, not by reputation.

### Pre-empting the two strongest objections

**"It proves too much — python3, git, and jq are also unaudited; universal
skepticism swallows everything."** UNKNOWN is defined *relative to the frozen
audit finding ledger*, not relative to all conceivable unsoundness. The
classifier fires only where a traced edge may reach a named finding. No
finding, no UNKNOWN. The rule is bounded by evidence that exists, which is
what separates it from paranoia.

**"PASS starvation — NOT_PROVEN tranches stack up and downstream decisions
stall, converting labeling back into schedule."** Three answers: resolution
effort per edge is small and parallelizable; the operating loop already treats
NOT_PROVEN as a valid, campaign-continuable result — the caller may select the
next tranche on an informative NOT_PROVEN, so work never stops; and the
acceptance chain is short (roughly ten commands, mostly shallow Python and
bash), so the UNKNOWN residue is likely small. The starvation scenario
requires a large, slow-resolving UNKNOWN set, which the T0 ledger will either
confirm or refute cheaply before anyone commits to the policy's cost.

### Honest residual concern I cannot argue away

If the UNKNOWN residue *is* large — deep dynamic dispatch,
platform-conditional paths — the campaign carries a long NOT_PROVEN tail, and
completion pressure will tempt a new laundering subclass: reclassifying
stubborn UNKNOWN edges as `declared_exclusions`. The kernel guards exclusion
laundering for *acceptance criteria*, but tool-edge laundering is a different
gate that nobody has designed. The rule's integrity depends on a guard that
does not yet exist; I can name it, but I cannot claim it is built.

### Does steelmanning change my final position?

Yes — full concession. My Phase-4 default ("proceed and PASS; repair only
proven crossings") is withdrawn. Adopted: implementation proceeds in parallel;
UNKNOWN edges flow into `unchecked_required`; affected tranches report
NOT_PROVEN until the edge resolves; re-validation over the frozen subject
mints the eventual PASS. This removes the CLI item from the operator's desk
entirely.

---

## Best combined architecture now visible — ten ordered decisions

1. **T0 — evidence baseline.** 49-skill inventory snapshot with per-file
   digests; the #988 three-state reconciliation ledger (present / absent /
   re-scheduled, byte-compared against `16d764b5a`); the executable-check
   liveness inventory (repair or formally retire the dead orchestration gate);
   the transitive proof-chain ledger with
   `USED_SOUND | USED_UNSOUND | PROVEN_UNUSED | UNKNOWN` classification; eval
   path containment expedited as an independent security release.
2. **Kernel tranche (4 skills).** The exact-byte experiment transaction in one
   atomic revision: unified identity, `rpi-report.v2` (stop_reason, coverage
   split, bounded opaque correlation), required `proof_contract` identity,
   before-manifest, strict Go reader, golden corpus; continuation envelope
   deleted with the caller named standing owner; negative-authority
   state-transition fixtures on the repaired pure reference layer; epoch 1
   pinned at close — prior verdicts are epoch 0 and never pool.
3. **Vocabulary and contract homes.** Machine-owned closed vocabulary
   (`option_shaping`; `standalone` defined; `core_phase` dropped; no
   skill-grantable `select_experiment`); campaign/Goal/fitness/ratchet terms
   enter the ubiquitous language; Campaign and Evolution bounded contexts with
   generated membership; model-dispatch split (neutral invariant →
   `docs/contracts/`, mechanics → adapters), six consumers repointed, the
   references class swept, `shared` retired; dead packet schemas removed after
   an external-consumer check.
4. **Compiler tranche.** Typed skill-contract compiler under the taxonomy
   budget: authority verbs (no selection), structured effects *referencing*
   authority, typed outputs, three-polarity trigger fixtures, axis retirement
   (tier derived or deleted; disposition curation-only), one parser, strict
   Python and Go consumers; grammar + fixtures + migration ledger only;
   population distributed to owning tranches as readiness data; **one all-49
   cutover publishes v3 authority exactly once** (SOL's barrier, adopted).
5. **Publication hygiene, with the one live disagreement preserved.** One
   serialized publisher (hardened `regen-all`): owner map, fail-fast,
   manifest-last marker, `--check` == render, byte-idempotence, worker lanes
   never generate. *Unresolved:* complete staged rendering before mass edits
   (SOL) versus owner-scoped git restore-on-failure until a
   concurrency/untracked-output trigger (FABLE) — decided by the agreed
   kill-at-each-step experiment on a scratch clone; SOL's own falsifier admits
   auto-restore if it recovers every seeded failure. Run it; adopt the winner.
6. **Campaign-layer tranche.** `craft-goal` as campaign-policy compiler/linter
   (never runtime, never selection); then `product`; then `goals` → `fitness`
   with a bounded non-advertised alias, after the vocabulary lands; `ao goals`
   CLI unchanged.
7. **Semantic tranches by proof shape, near the plan's original count.**
   Evidence + judgment (with `security` as evidence, mutating modes separately
   authorized); candidate specialists; capability evolution; transports; then
   host/support. Split further only on a failing authority/proof fixture.
   Every tranche verdict cites the proof-chain ledger; UNKNOWN edges →
   `unchecked_required` → NOT_PROVEN until resolved; re-validation over the
   frozen subject mints PASS.
8. **CLI program in parallel.** Three tranches — containment/owned temp,
   command effect/output policy, subprocess lifecycle — sharing the effect
   vocabulary with the compiler; a defect blocks exactly its proven consumers.
9. **T7 convergence.** One full regeneration from frozen source; the
   plan-matrix ↔ catalog conformance script; the contract's interim placement
   note expired with a `superseded-by` pointer; completion matrix keyed by
   verdict digests + epoch tuples; the 250-line kernel cap retained as an
   interim gate while the always-loaded token measure is built.
10. **Standing invariants.** Verdicts are immutable across epochs — compared,
    never rejudged; no skill owns continuation or selection; every named check
    stays live with a seeded-red proof; the net classification-axis count may
    not grow without a documented exception.

## Preserved unresolved evidence (not manufactured into agreement)

- **Staging vs restore-on-failure** (decision 5): open, empirical, experiment
  agreed by both sides' falsifier language.
- **`declared_exclusions` mechanics**: exclusion-vs-criterion comparison needs
  stable criterion IDs; undesigned by both wizards; likeliest early-epoch-2
  cause.
- **UNKNOWN-edge laundering guard**: the rule preventing stubborn UNKNOWN
  edges from being reclassified as exclusions exists only as a named risk.
- **Single-wizard items not yet cross-examined**: the #988 reconciliation
  ledger and the plan-authority expiry entered the combined architecture from
  my side without SOL engagement; they are in decisions 1 and 9 on my evidence
  alone and deserve the same adversarial reading everything else got.

*End of steelman. FABLE.*
