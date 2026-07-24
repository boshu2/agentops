# SOL steelman of FABLE’s strongest architecture

Date: 2026-07-24
Identity: SOL
Inputs read completely: `WIZARD_REBUTTAL_SOL.md`,
`WIZARD_REBUTTAL_FABLE.md`

## Decision 1 — Bracket the migration with provenance at entry and authority expiry at exit

This decision combines and strengthens three FABLE-originated ideas:

- T0’s executable check-liveness and criterion-to-check inventory;
- the complete reconciliation ledger for commit `16d764b5a` / #988;
- T7’s plan-to-catalog conformance check and explicit expiry of the plan’s
  interim placement authority.

### Stronger case

A migration is not coherent merely because every intended edit eventually
appears in the tree. It needs a chain of custody:

```text
known starting content and provenance
  -> named transformation tranches
  -> live acceptance witnesses
  -> one authoritative final projection
  -> explicit retirement of temporary authority
```

The current overhaul lacks both brackets.

At entry, #988 is neither wholly present nor wholly absent. Its `craft-goal`
half is represented in the dirty worktree while its RPI/report half is absent,
and the planned kernel work overlaps the missing half. A normal dirty-tree
snapshot records bytes but not why those bytes exist or which reviewed change
they partially represent. Without a reconciliation ledger, “preserve existing
work” cannot distinguish intentional divergence, missing reviewed content,
rescheduled work, and accidental omission.

At exit, the ports-and-adapters contract temporarily delegates placement truth
to a dated plan matrix. If T7 publishes a correct catalog but leaves that
delegation alive, the migration creates two legitimate answers on its final
day. The next skill edit can update canonical frontmatter and generated views
while the contract still blesses the stale plan.

The liveness inventory connects the brackets. It proves that every claimed
transformation has an executable witness and that every final acceptance row
traces back to a known input. This is not project-management ceremony; it is
the migration’s evidence lineage.

### Best incremental implementation path

1. **Create a generic migration-input ledger.** Seed it with every path in
   `16d764b5a`, not merely the currently visible `craft-goal` paths. Record the
   commit blob digest, HEAD state, worktree digest, and one classification:
   `PRESENT_IDENTICAL`, `PRESENT_DIVERGED`, `ABSENT`,
   `RESCHEDULED_BY_TRANCHE`, or `OUT_OF_SCOPE`.
2. **Make every nontrivial classification attributable.** A rescheduled entry
   names the exact plan row/tranche; a diverged entry records whether the
   divergence is caller-owned and must be preserved.
3. **Inventory acceptance checks as a graph.** For every plan criterion record
   check ID, owning source, invocation, transitive wrapper/process path,
   expected evidence artifact, negative witness, and current liveness state.
4. **Repair, replace, or retire dead witnesses.** The broken orchestration
   boundary gate cannot remain a named proof. Retirement must also remove every
   plan or contract claim that relies on it.
5. **Require tranche manifests to cite ledger and check IDs.** This turns
   starting provenance and proof liveness into normal inputs, without creating
   a second lifecycle.
6. **Generate a normalized placement matrix at T7.** Diff plan intent against
   the compiled all-49 catalog by stable skill ID, layer, seams, authority, and
   disposition.
7. **Expire interim authority in the same T7 candidate.** Edit the live
   contract to point only to the generated/source-owned placement view. Mark
   the dated plan `superseded-by` that view without rewriting its historical
   substance.
8. **Add a recurrence gate.** No current contract may delegate normative
   placement to a dated plan, and no superseded plan may be treated as a
   writable catalog source.

### Second-order benefits

- **Safer rollback and conflict resolution.** A later merge of #988 can be
  classified as already applied, intentionally diverged, or still missing
  path by path instead of guessed from commit ancestry.
- **Honest dirty-worktree preservation.** The operator can distinguish
  caller-owned edits from migration outputs and reviewed-but-unmerged inputs.
- **Executable audit closure.** A future audit can answer both “what did the
  migration start from?” and “which source became authoritative?” without
  reconstructing history from prose.
- **Less gate theater.** Dead, orphaned, or phrase-stale checks become visible
  before their green names appear in a completion report.
- **Reusable migration discipline.** The ledger format can handle future
  partially adopted commits or patches; #988 becomes the first instance, not a
  permanent special case.

### Pre-empting the two strongest objections

**Objection 1: this creates another plan packet or lifecycle.**
It does not select work, own retries, authorize mutation, or issue verdicts.
The entry ledger is a factual snapshot/provenance map. The check graph is a
factual proof-supply inventory. The expiry step removes temporary authority
rather than adding it. One RPI still owns each bounded
Plan/Implement/Validate/verdict experiment.

**Objection 2: historical plans should be immutable.**
Preserve the plan body and decisions. Add only explicit status metadata or an
append-only supersession notice, and edit the *live contract* that currently
points at it. Historical fidelity is improved when a reader can see when the
plan stopped being authoritative.

### Honest residual concern

Repository evidence cannot enumerate external consumers that copied the plan
matrix or #988 content outside the tree. The ledger and expiry gate can prove
local closure, not global absence of unofficial dependencies. T0/T7 therefore
still need one operator disclosure of known external consumers; unknown
external copies remain residual risk.

### Does steelmanning change SOL’s final position?

**Yes.** SOL now treats the provenance/authority bracket as a first-class
migration invariant, not appendix hygiene. The #988 ledger belongs in T0
acceptance, and plan-authority expiry belongs in T7 acceptance. FABLE’s
criterion-to-check graph is the evidence spine connecting them.

## Decision 2 — Make the taxonomy budget the design constitution of the typed compiler

This decision originated in FABLE’s taxonomy-budget finalist and was sharpened
through the duel: one machine vocabulary, explicit axis retirement, structured
effects referencing authority, three-polarity route fixtures, and no
skill-grantable experiment selection.

### Stronger case

A compiler can eliminate syntax drift while institutionalizing semantic drift.
If it faithfully compiles eight overlapping classification axes, it makes the
contradictions faster, stricter, and harder to remove.

The taxonomy budget prevents that failure by treating the skill contract as a
minimal ontology, not a metadata wish list. Every canonical field must answer
one distinct reader question:

| Reader question | Canonical semantic |
|---|---|
| Where does this capability primarily belong? | `system_layer` |
| At which lifecycle boundary may it participate? | `lifecycle_seams` |
| What semantic act may it perform? | `authority` |
| What world change may execution cause, under whose authorization and cleanup duty? | structured `effects` |
| What does it consume and emit? | typed artifact inputs/outputs |
| When should routing select or reject it? | positive/negative/ambiguity triggers |
| What observable behavior proves specialization? | proof class and probe |

Everything else must be derived, limited to curation, or retired. `tier` is a
view over architectural criticality/loading policy, not an independent truth.
`hexagonal_role` is a projection of layer, seam, and authority. `disposition`
is a temporary portfolio-curation decision, not runtime semantics.
`core_phase` is not a seam. `experiment_select` is not a skill seam or
authority at all; only the external caller/Goal runtime selects another
experiment. `option_shaping` names the advisory behavior without borrowing
authority.

The budget is therefore not aesthetic minimalism. It bounds the number of
semantic combinations the system must make coherent. Each independent boolean
or enum multiplies possible states; redundant axes create structurally valid
but contradictory entries. Axis retirement is a reliability feature.

### Best incremental implementation path

1. **Inventory axes and readers before writing v3.** For every current and
   proposed field, list each Python, Go, shell, generated-doc, router, and human
   decision that consumes it.
2. **Build a reader-question decision table.** Mark fields that answer the same
   question, fields that combine independent questions, and fields with no live
   reader.
3. **Approve the minimal semantic basis.** Define machine-owned layer and seam
   vocabularies, skill-grantable authority verbs, structured effects, artifact
   contracts, trigger fixtures, failure semantics, and proof declarations.
4. **Record derivation and retirement rules.** Generate legacy tier/role views
   only where a bounded consumer still requires them. Keep disposition
   curation-only. Set deletion dates for obsolete writers and readers.
5. **Implement hostile fixtures before the compiler.** Reject a runtime adapter
   writing verdicts, a strategy selecting experiments, an effect restating
   contradictory authority, a binding output without a validator, an ambiguous
   route with no discriminator, and a specialization claim with no failing
   probe.
6. **Build one parser and one intermediate representation.** Python and Go
   golden-corpus consumers must agree and reject stale versions, unknown
   fields, count mismatch, and mixed authority.
7. **Populate contracts in owning semantic tranches.** During this period v3
   fields are readiness data, not published authority.
8. **Perform one all-49 cutover.** Publish v3 only when every retained source,
   strict consumer, generated membership view, and projection passes against
   the same frozen ledger.
9. **Delete the superseded axes and writable readers.** Compatibility views may
   remain read-only for one bounded window, but there is one canonical
   intermediate representation.
10. **Govern future extensions.** A new field requires a named reader question,
    contradiction fixture, derivation/retirement impact, and a documented
    exception if it increases the independent axis count.

### Second-order benefits

- **Skill-sprawl control becomes falsifiable.** A root survives because its
  trigger, authority/effect boundary, output, and proof differ observably from
  its nearest neighbor.
- **CLI and skill effects share semantics.** The parallel CLI program can
  consume the same effect kinds without giving the CLI lifecycle authority.
- **Generated bounded contexts stop drifting.** Responsibilities remain
  hand-authored; membership derives from the same contract basis as routers and
  catalogs.
- **Runtime adapters become honestly optional.** Their distinct unavailable,
  timeout, cleanup, and output probes justify specialization without turning
  tool topology into the semantic schema.
- **Future vocabulary changes get cheaper.** `goals`/`fitness`,
  `option_shaping`, and Security’s evidence/mutation split change one semantic
  source and regenerate compatible views.
- **Review quality improves.** Reviewers debate the reader question and
  contradiction, not whether a new YAML field “seems useful.”

### Pre-empting the two strongest objections

**Objection 1: a taxonomy budget will collapse genuinely independent
dimensions.**
The budget is a review invariant, not a fixed numeric schema law. A new
dimension is allowed when it answers a distinct live reader question and has a
fixture proving that derivation from existing fields would lose meaning. The
burden is evidence, not an arbitrary one-in/one-out quota.

**Objection 2: retiring fields will break generated or external consumers.**
Generate bounded read-only compatibility views from the new intermediate
representation, publish deprecation and parity fixtures, and remove old
writable inputs immediately. External consumers get a compatibility window,
but they never preserve a second source of truth. The strict all-49 cutover
prevents mixed semantics.

### Honest residual concern

The chosen ontology can ossify the architecture. A schema designed around
today’s 49 skills may make a genuinely new product layer or effect model
unnecessarily expensive to express. The exception process contains but cannot
eliminate that risk. Periodic evidence-based review of actual reader questions
is necessary; otherwise the taxonomy budget can become institutional
conservatism.

### Does steelmanning change SOL’s final position?

**Yes.** The compiler is no longer the primary design decision with a taxonomy
check attached. The taxonomy decision must precede compiler coding and governs
its intermediate representation. SOL also elevates axis retirement and
reader-question evidence into T1 acceptance, not later cleanup.

## Best combined architecture now visible

1. **T0 establishes evidence lineage.** Freeze the 49-skill ledger; reconcile
   every #988 path and dirty input; inventory live checks; map each acceptance
   criterion to an owner, invocation, negative witness, and transitive
   CLI/process path. Preserve `UNKNOWN` crossings rather than calling them
   safe.
2. **Repair the four-skill experiment kernel first.** Unify RPI and Validate on
   exact intent bytes; require before/final subject identity and changed-path
   coverage; remove continuation and repair authority; distinguish
   `unchecked_required` from `declared_exclusions`; add narrow opaque
   correlation.
3. **Mint proof epoch 1 explicitly.** Add a required typed `proof_contract`
   identity in the same schema/writer/Go-reader revision as report v2; bind all
   semantic tranche verdicts to that epoch and never reinterpret old verdicts.
4. **Approve the minimal skill ontology before implementing v3.** One
   machine-owned vocabulary, no skill selection authority, structured effects
   referencing authority, typed artifacts, three-polarity triggers, proof
   declarations, and explicit derivation/retirement of redundant axes.
5. **Build the compiler and migrate through a non-authoritative readiness
   rail.** Use one parser/IR and hostile cross-language fixtures; populate
   source contracts in owning tranches; make one strict all-49 authority
   cutover.
6. **Repair contract homes and portfolio boundaries.** Move cross-skill
   invariants to `docs/contracts/` and mechanics to adapters, then retire
   `shared`; pin Goal/Fitness vocabulary before the bounded skill alias; place
   Security evidence and separately authorized mutation honestly; retain
   adapters only with distinct probes.
7. **Run the CLI audit as an expedited parallel release program.** Fix eval
   containment immediately; share effect/output vocabulary with the compiler;
   block a tranche only on a `USED_UNSOUND` crossing. An `UNKNOWN` crossing may
   not support PASS, but need not stop unrelated implementation.
8. **Decide projection containment by experiment, not assertion.** Immediately
   add owner mapping, fail-fast regeneration, manifest-last commit, and
   check/render parity. On a scratch clone inject failure after every writer
   mutation. If owner-scoped restore returns every tracked output to the exact
   prior generation, it may precede full staging; any untracked or
   unrecoverable surface requires complete staged rendering before mass
   publication. Concurrency still triggers a repository lock.
9. **Migrate compact semantic tranches by one authority/proof shape.** Keep
   campaign after the kernel, evidence separate from verdict authority,
   evolution after verdict collections, and runtime/host adapters outside the
   loop. Split a tranche only when a RED fixture proves a distinct authority
   question.
10. **T7 closes authority, not merely generation.** Publish and verify one
    catalog/projection generation, resolve every criterion/check and platform
    gap by proof epoch, diff the plan matrix against the generated catalog,
    retire the contract’s interim plan pointer, and mark the plan superseded.
    Report and stop.
