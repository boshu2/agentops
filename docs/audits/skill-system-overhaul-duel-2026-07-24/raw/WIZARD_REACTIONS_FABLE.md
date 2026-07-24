# WIZARD_REACTIONS_FABLE — reveal-phase reaction

- Author: FABLE (`claude-fable-5`)
- Date: 2026-07-24
- Read for this phase: `raw/WIZARD_SCORES_SOL_ON_FABLE.md`, plus re-reads of both
  proposals and my own score file.
- Prior commitments carried in: in Phase 2 I verified SOL's digest-mismatch and
  PASS/`not_checked` findings against source and conceded both. Nothing in this
  phase walks that back.

Factual accuracy note on SOL's score file, before the reactions: it contains
one misread and three misfiled citations, none of which changes their
substance. (1) Their disagreement #6 says I "keep `security` in capability
evolution" — my appendix row 39 placed `security` in the **evidence** layer and
kept only its *tranche* at T4 per the plan; evolution was never my position,
and my Phase-2 score had already adopted their move. (2) Their F4 section cites
`skills/rpi/references/run_once.py`, `skills/craft-goal/references/dispatch_once.py`,
and `skills/validate/references/validate.py`; the live paths are
`skills/rpi/scripts/run_once.py`, `skills/swarm/scripts/dispatch_once.py`
(swarm's, not craft-goal's), and `skills/validate/scripts/validate.py`.

---

## F1 — Proof substrate: check-liveness inventory + frozen validator fixed point (SOL: 800, adopt with modification)

**Where SOL is right.** Two places, both important. First, freezing before
repair can faithfully pin an incoherent kernel — with the digest mismatch and
the report-schema gap live, "freeze now" preserves defects at high fidelity. I
had already conceded the ordering in Phase 2; their score confirms the
consequence for my F1 specifically. Second, and this is the sharper catch:
my phrase "re-baselining of prior tranche verdicts" was a genuine design
error, not a wording slip. The natural implementation of re-baselining —
refreshing or replaying earlier verdicts under the new contract — violates the
architecture's own invariants (content-addressed immutable verdicts; a
post-verdict consumer never mutates what it reads). Their epoch model is
strictly better: each verdict permanently names the proof-contract version it
was minted under; a contract change opens a new epoch; a migration summary may
*compare* epochs but never reissues old judgments. That preserves evidence
history as history.

**Where SOL is under-specified or scoring a different framing.** The 170
feasibility score prices the rebaseline hazard as if it were the mechanism
rather than a repairable defect in my mechanism; with epochs substituted, the
remaining machinery (a digest record per verdict, an inventory JSON, a
criterion→check matrix) is among the cheapest items in either proposal. One
gap in their modification: where the epoch identity lives. `verdict.v2` has no
validator-contract field, so the epoch digest must ride in `evidence_refs` or
the freshness-attestation notes until the one scheduled schema revision — that
detail should be pinned so epochs don't start as prose.

**Criticism that changes my position.** Both: repair-before-pin (ordering) and
epochs-instead-of-rebaseline (mechanism). Confidence in the *inventory* half
is unchanged at ~92%; confidence in my original fixed-point *wording* drops to
zero — it is replaced, not defended.

**Revised ruling: MODIFY (merge with SOL F1).** Sequence: liveness inventory
and criterion→check matrix at T0, unchanged; SOL's kernel repair lands; then
the repaired validator scripts + schemas are digest-pinned as **epoch 1**.
Verdicts record their epoch. The one scheduled schema revision (report v2 with
correlation) exercises the epoch mechanism exactly once, proving it works.

**Smallest synthesis rule.** *Every verdict permanently names the digest of the
proof contract it was minted under; changing that contract opens a new epoch
and never reopens, replays, or reinterprets a prior verdict.*

---

## F2 — One seam vocabulary owned by the contract (SOL: 901, adopt with modification)

**Where SOL is right.** The ownership polarity: a narrative document as enum
owner just relocates drift. My proposal did include a mechanical conformance
check binding the schema enum to the contract table, but I had the arrow
backwards — the machine source (schema/compiler) should own the enum and the
narrative surfaces should be generated from or conformance-checked against it.
Also right: `core_phase` mixes participation with kernel membership; kernel
membership is already fully captured by the hard dependency graph plus
authority typing, so a `core_phase` seam is redundant and mildly misleading.
And generated bounded-context membership (hand-written responsibilities,
machine-derived membership) closes a second-inventory hole my C7 left open.

**Where SOL is wrong or scoring a different framing.** Nothing material. The
one thing their score under-credits is that the conformance check was already
in my acceptance criteria — the distance between our positions was polarity,
not presence. That affects no ruling.

**Criticism that changes my position.** Polarity of ownership; deletion of
`core_phase` from the seam enum.

**Revised ruling: MODIFY.** The closed vocabulary lives in the compiler/schema
source; `ubiquitous-language.md`, the contract's seam table, and every
generated view are conformance-checked against it in CI. `option_shaping`
rename retained. `standalone` gets its contract definition. `core_phase`
dropped as a seam; kernel membership expressed only via hard edges + authority.
Campaign and Evolution enter the responsibility model with generated
membership.

**Smallest synthesis rule.** *One machine-readable vocabulary source; every
prose surface that restates it is conformance-checked in CI; no seam name may
denote an authority.*

---

## F3 — Taxonomy budget + mechanical honesty sweeps (SOL: 915, adopt with modification)

**Where SOL is right.** Three refinements, all accepted. A flat effects enum
under-expresses the safety boundary — structured effects (kind, scope,
authorization, cleanup) are what acceptance #5 actually needs; my flat enum
optimized for sweep mechanics over expressiveness. The one-in/one-out axis rule
is a budget with an exception process, not a schema law — two genuinely
independent dimensions must not be collapsed to keep a count flat (this was my
framing's intent, but their formulation is the enforceable one). Positive-only
trigger probes are gameable; negative and ambiguity fixtures are required,
which matches the structured-triggers shape from their own F2.

**Where SOL is under-specified.** One guard from my Phase-2 scoring of *their*
F2 must survive the merge: `effects[].authorization` has to **reference** the
authority verbs, never restate them, or the compiler acquires two places to
disagree about who may mutate. And their "implement one typed compiler rather
than layering fields onto v1/v2" is right, but it silently absorbs my
population question; my position — compiler tranche carries grammar, parser,
fixtures, and strict Go consumers, while the 49-file population distributes to
each skill's owning tranche — was not contested and stands.

**Criticism that changes my position.** Structured effects; budget-as-review-
test; trigger negatives.

**Revised ruling: MERGE.** F3 merges into the typed compiler (SOL F2) as its
governing constraint set: axis retirement (tier derived or deleted;
`SKILL-TIERS.md` re-keyed or retired; disposition curation-only), structured
effects referencing authority, typed outputs, trigger fixtures in all three
polarities, population distributed. `select_experiment` stays deleted from the
skill-grantable authority verbs — my open modification to their compiler,
which their own craft-goal appendix row supports.

**Smallest synthesis rule.** *No new classification field lands without naming
the field it retires or the reader question only it can answer — enforced as a
design-review gate on the compiler schema with a documented exception path.*

---

## F4 — Authority-transfer atom, harness, T6 split, regen entrypoint (SOL: 745, subsumed; atom rejected)

**Where SOL is right.** On the atom: fully. Their argument is the correct
recovery from the operating contract — the **caller** is the standing owner of
continuation ("AgentOps does not own what the caller does next"), and the
Goal/Mayor is an optional *delegation* of that authority. Deleting rpi's
continuation envelope therefore returns authority to its permanent owner; it
does not orphan it. My "zero-owner interval" assumed authority lives in skill
prose; it lives in the caller by contract, and prose only describes
delegations. The phantom interval cannot justify coupling an optional campaign
policy to the kernel's proof. I concede the atom without residue — including
the softer Phase-2 version ("delete the envelope in the same verdict that
names craft-goal") — the deletion verdict need only name the **caller** as
owner, which the contract already does; pointing at craft-goal is informational.

**Where SOL is wrong or scoring a different framing.** Two notes. First, the
145 structural score prices the whole finalist at its weakest sub-claim; the
bundle contained four claims and they adopted three (T6 split, product/goals
relocation, pure-script harness). That is a fair penalty for *my bundling*,
not for the surviving content — lesson taken. Second, on publication their
objection ("one entrypoint does not provide locking, staging, crash recovery")
is true but partially aimed past my Phase-2 position, which had already
adopted the manifest-last commit marker and fail-fast — the cheap half of
their machinery. The genuine residual is timing of the expensive half; that
lives in the cross-plan section below.

**Criticism that changes my position.** The atom is withdrawn. Kernel tranche
is the four core skills; `craft-goal` migrates in a subsequent campaign-layer
tranche with explicit non-authority tests.

**Revised ruling: DEMOTE AND SPLIT.** The finalist dissolves: atom withdrawn;
T6a/T6b split, product/goals relocation, and the negative-authority harness
(now explicitly built on the *repaired* kernel, per SOL F1) are retained as
adopted items inside the merged migration; regen items move to the publication
synthesis below.

**Smallest synthesis rule.** *Continuation authority's standing owner is the
caller; skill migrations edit only delegations — so no tranche ever needs a
campaign co-tenant to keep authority owned, and every tranche that edits a
delegation names the standing owner in its acceptance.*

---

## F5 — Declared-contract tier truth: dead schemas, correlation, model-dispatch, missing contracts (SOL: 775, subsumed with modifications)

**Where SOL is right.** The destination critique lands. Moving
`model-dispatch.md` into `shared` would make core `validate` depend on a
canonical meta-skill — trading a core→adapter reference for a core→grab-bag
reference, and re-seeding the miscellaneous bucket the overhaul exists to
kill. Their split is the correct decomposition: the runtime-neutral invariant
(fresh-context identity, model-identity recording, disclosure rules, the
`claude -p` prohibition) belongs in a **declared contract** under
`docs/contracts/`; runtime-specific dispatch mechanics stay with the adapters
that execute them. Notably, my own Phase-2 score file had already sketched
exactly this third option ("home cross-skill runtime recipes as a
docs/contracts/ document — a synthesis neither of us proposed"); SOL reached
it independently in their score file. Convergence from both sides is the
strongest signal in this duel. Also right: the correlation object must be
narrow — typed, size-bounded, opaque identifiers with an explicit
"RPI does not interpret them" rule — or the field becomes a campaign-state
smuggling channel; and dead-schema deletion deserves an external-consumer
check plus an epoch note first.

**Where SOL is wrong or scoring a different framing.** The bundling penalty is
fair; one under-credit: their "no other appendix row changes SOL's
dispositions" passage accepts my six-consumer evidence as a *sequencing*
change only, but it is slightly more — it establishes the **class** (cross-
skill contracts currently homed in skill references), and the class predicts
more members. The migration should sweep `skills/*/references/` for other
multi-consumer contracts before declaring the class closed; one relocation is
a fix, an unswept class is a recurrence.

**Criticism that changes my position.** Destination (contracts home, not
`shared`), correlation narrowing, deletion diligence.

**Revised ruling: MODIFY.** (a) Dead packet schemas removed inside the kernel-
contract migration after an external-consumer check, tombstone guards kept.
(b) Report v2 gains a size-bounded, typed, opaque correlation map; RPI never
interprets it. (c) `model-dispatch.md` splits: invariant → `docs/contracts/`,
mechanics → adapter references; all six consumers repoint; then `shared` is
retired — SOL's retirement now has its precondition satisfied, and my
"populate" is withdrawn. (d) A one-shot sweep of `skills/*/references/` for
other multi-consumer contracts joins the same tranche. (e) `cc-hooks`/`dcg`
output contracts land through the compiler.

**Smallest synthesis rule.** *Cross-skill invariants live in declared
contracts, never in any skill's `references/`; adapters keep only mechanics;
a skill root with no unique behavior is retired once its references have
contract homes.*

---

## Cross-plan disagreements, addressed explicitly

**1. Exact RPI transaction first vs the five-skill atom.** Resolved — I
concede. Kernel first (`rpi`, `plan`, `implement`, `validate`) with the
exact-byte identity repair; `craft-goal` in a following campaign tranche with
non-authority fixtures. The caller-as-standing-owner argument is the correct
reading of the contract, and it removes the only structural justification my
atom had.

**2. CLI audit: prerequisite substrate vs independent program with
crossings.** Substantially narrowed, honestly not closed. SOL's score file
retreats from "the audit is a prerequisite stage" to "migration tranches may
not claim behavioral proof while the substrate they *execute* is
known-unsound" — which is my crossings model with a stricter default. The
residual is exactly that default: prerequisite-unless-proven-unused (SOL) vs
parallel-unless-proven-used (me). Synthesis that contains both: make the
**proof-chain trace a T0 deliverable** — enumerate every command the
migration's acceptance actually executes (it is a short list: frontmatter
validators, mesh generator, codex-sync, `validate.py`, bats, focused `go
test`, `ao skills`, `ao gate check`) and mark each sound/unsound against the
audit. Unsound-and-used paths block their consumers; everything else
proceeds in parallel. Eval containment is expedited under both positions and
was never in dispute. What remains genuinely open is only what happens if the
trace is contested — see "single visible disagreement" below.

**3. `shared`: retire vs populate.** Resolved by convergence — split
model-dispatch into contract + mechanics, repoint six consumers, sweep the
class, then retire the empty root. Neither original position survives intact;
the synthesis is better than both.

**4. Transactional staged publication vs one serialized entrypoint.**
Narrowed. Adopted now, jointly: owner map, fail-fast `regen-all` (the
verified record-and-continue defect dies), manifest-last generation marker,
`--check` == render, byte-idempotence test, worker-lanes-never-generate.
Residual: whether the full lock/staging/crash-kill publisher must precede mass
edits (SOL) or is triggered by the first tranche that actually runs parallel
lanes near generated surfaces (me). I hold my position with a testable
trigger: serial tranches + manifest marker close every *observed* hazard; the
lock earns its cost the day parallelism is real. If SOL's crash-kill matrix
finds a single-writer failure the manifest marker misses, I fold.

**5. Typed compiler vs taxonomy budget.** Resolved by merge — compiler owns
grammar and enforcement (SOL's mechanism); the budget governs axis count and
retirement (my constraint); structured effects reference — never restate —
authority verbs (my guard); `select_experiment` is not a skill-grantable verb
(my catch, standing unrebutted and supported by their own craft-goal row);
population distributes to owning tranches (my placement, uncontested).

**6. Validator/report/schema evolution and verdict comparability.** Resolved
on SOL's epoch model, with my machinery around it: proof-contract digest
recorded per verdict, liveness inventory, criterion→check matrix, and the
single scheduled report-schema revision as the epoch mechanism's first live
exercise. Rebaselining is dead; epochs are the law.

**7. `goals`/`fitness`.** Resolved: rename skill-side to `fitness` using SOL's
bounded, non-advertised alias mechanics, executed once the campaign/fitness
vocabulary lands in the ubiquitous-language contract (my gate — a T0/T1 item,
so the timing difference is days, not tranches). `ao goals` CLI stays.

**8. Security placement.** Resolved, with the correction recorded: my position
was evidence-layer all along (tranche-following-the-plan was my error, adopted
away in Phase 2). SOL's dual-role refinement is adopted: advisory method and
typed evidence output are separately declared through the compiler; mutating
baseline/policy modes carry their own authorization fixtures; the skill never
becomes the validator.

---

## Revised top-three decisions

1. **Repair, then pin, the kernel — with epochs.** SOL's exact-byte
   `experiment-transaction` work first (four skills only), including the
   narrow correlation field and `unchecked_required` vs `declared_exclusions`;
   then digest-pin the repaired validator+schemas as epoch 1; every verdict
   names its epoch; prior verdicts are never reopened. My T0 liveness
   inventory and criterion→check matrix land before any of it.
2. **One typed skill-contract compiler under a taxonomy budget.** Machine-owned
   vocabulary; authority verbs without `select_experiment`; structured effects
   referencing authority; trigger fixtures in three polarities; typed outputs;
   tier/disposition retirement; strict Go consumers; 49-file population
   distributed to owning tranches.
3. **Contract-tier truth with proper homes.** Dead packet schemas out after an
   external-consumer check; model-dispatch split into a `docs/contracts/`
   invariant plus adapter mechanics; six consumers repointed; the
   references-class swept; `shared` retired; `security` dual-typed;
   `cc-hooks`/`dcg` contracts through the compiler.

## The single disagreement that should remain visible to the operator

**The CLI-audit scheduling default.** Both wizards now agree on the
resolution procedure (a T0 proof-chain trace; unsound-and-used paths block
their consumers; eval containment expedited regardless). What remains is the
default when the trace is ambiguous or contested: SOL blocks migration
tranches on substrate soundness; I proceed in parallel and repair only proven
crossings. This is a real schedule-vs-assurance trade with weeks of critical
path at stake, and it should be the operator's call, not averaged.

## New consensus the original plan should adopt immediately

1. **The kernel identity repair is missing from the plan entirely.** The
   RPI↔Validate digest-algorithm mismatch (verified by both wizards) must be
   the first work item of the core tranche; no T2 scenario currently covers
   it.
2. **Split plan-T2: kernel (4 skills) first, campaign after; move `product`
   and `goals` out.** Reached independently by both wizards; the atom dispute
   is settled on kernel-first.
3. **T0 gains an executable-check liveness inventory and a
   criterion→check matrix**; the orchestration boundary gate is repaired or
   formally retired (it is broken, orphaned, and phrase-stale today).
4. **Verdict epoch rule**: verdicts permanently record their proof-contract
   digest; contract changes open epochs; no retroactive reinterpretation.
5. **`experiment_select` → `option_shaping`**, `standalone` defined,
   `core_phase` dropped; no skill-grantable selection authority anywhere in
   the vocabulary.
6. **PASS coverage semantics**: adopt `unchecked_required` vs
   `declared_exclusions` — the schema's `not_checked: maxItems 0` on PASS
   collides with T7's platform-gap disclosure duty as written.
7. **Split T6** into transports vs host/support (both wizards independently).
8. **Publication hygiene now**: fail-fast `regen-all`, manifest-last
   generation marker, owner map, `--check`==render — the record-and-continue
   defect is verified and cheap to kill.
9. **Model-dispatch relocation + `shared` retirement** in the split-home form
   above.
10. **`security` re-tranched as evidence** with dual-typed outputs and
    separate authorization fixtures for its mutating modes.

*End of reaction. FABLE.*
