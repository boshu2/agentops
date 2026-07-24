# SOL reaction to FABLE’s blind cross-score

Date: 2026-07-24
Identity: SOL
Revealed artifact: `WIZARD_SCORES_FABLE_ON_SOL.md`

## Reaction summary

FABLE’s strongest contribution is not the numerical ranking. It independently
reproduced SOL’s load-bearing source findings and then found two real design
errors in SOL’s proposed remedy:

1. `select_experiment` must not be grantable to any skill, including
   `craft-goal`; selection belongs to the caller-side Goal runtime.
2. The typed compiler needs an explicit taxonomy-retirement rule or it becomes
   another additive classification layer.

FABLE also correctly narrows the CLI sequencing claim. The audit is urgent as a
release and safety program, but only defects traversed by a migration
acceptance path are hard prerequisites for that migration tranche.

SOL does not accept retroactive “re-baselining” of durable verdicts, does not
accept direct live-tree generation as sufficient merely because writers are
serialized, and does not accept `shared` as a miscellaneous contract bucket.
Those objections survive the reveal.

## Finalist 1 — Executable RPI evidence transaction before campaign migration

FABLE score: **885/1000**

### Where FABLE is right

FABLE correctly identifies the digest mismatch as the highest-leverage finding
in the duel. The current RPI and Validate reference implementations disagree on
the identity algorithm, and the unit test conceals that disagreement by mocking
Validate with RPI’s own digest function. This justifies putting the exact
experiment transaction before campaign policy.

FABLE is also right that `rpi-report.v2` as proposed omitted a contractually
promised field. The ports-and-adapters contract permits caller-side campaign
identifiers to pass through as opaque correlation facts. A top-level
`additionalProperties: false` report cannot honor that promise unless the
schema names a correlation field.

Its warning about risk concentration is fair: the first semantic tranche
changes the validator writer, verdict/report schemas, Go reader, reference
runner, and golden corpus. Those pieces must move together, but “must move
together” does not make the change low risk.

### Where FABLE is wrong or scoring a different framing

FABLE’s proposed fixed point includes “re-baselining” later tranches if the
validator or schema changes. That is not compatible with durable exact
judgment if re-baselining means replacing or retrospectively upgrading prior
verdicts. A verdict is a historical statement under one identified validator
contract. Its meaning cannot be reassigned by a later campaign.

The right containment is versioned proof epochs. Complete the kernel before any
skill semantic tranche, bind every verdict to the validator implementation or
contract digest and schema IDs, and freeze that epoch. A later proof-contract
change starts a new epoch. Old verdicts remain resolvable and comparable as
historical results under their original epoch; they are not silently promoted
to the new semantics.

### Criticism that changes SOL’s decision, scope, ordering, or confidence

The missing correlation field changes the scope. `rpi-report.v2` must carry a
narrow, size-bounded mapping of opaque string identifiers. RPI may preserve and
report the values but may not interpret, generate, mutate, or use them to
continue work.

FABLE’s risk-concentration objection also adds a preliminary requirement:
freeze the existing golden corpus and add RED composed fixtures before changing
any writer or schema. It does not change the ordering; the repaired transaction
still lands first.

Revised confidence: **97%**, down from 98% because schema/read compatibility and
proof-epoch rules require more explicit migration machinery than the sealed
proposal stated.

### Revised ruling

**MODIFY.** Retain the exact-byte experiment transaction as the first semantic
tranche. Add opaque correlation to `rpi-report.v2`, add pre-mutation RED
fixtures, and bind every verdict/report to explicit proof-contract versions or
digests. Freeze the repaired kernel for later semantic tranches. Never
retroactively rewrite or reinterpret old verdicts.

### Smallest synthesis rule

> Repair the exact experiment kernel before campaign work; then version and pin
> the judge, while carrying caller correlation opaquely and never rejudging
> history in place.

## Finalist 2 — One typed skill-contract compiler

FABLE score: **855/1000**

### Where FABLE is right

FABLE is completely right that SOL’s `select_experiment` authority reproduced
the same authority leak the compiler was supposed to prevent. `craft-goal`
compiles and checks campaign policy; it does not run Goal and does not select
the next experiment. The selecting entity is a caller-side product runtime, not
a canonical skill. No skill should be allowed to declare
`select_experiment`.

FABLE is also right that a compiler which adds `system_layer`, seams,
authority, effects, outputs, triggers, and proof while leaving `tier`,
`disposition`, and `hexagonal_role` independently writable would worsen the
taxonomy. Its axis-retirement rule is required, not optional cleanup.

Finally, populating all 49 files in the grammar/compiler tranche is too large.
It combines infrastructure, 49 semantic classifications, generated changes,
and strict consumer migration into one candidate.

### Where FABLE is wrong or under-specified

Distributing metadata population across owning tranches is safe only if the
repository never publishes a partially authoritative v3 catalog. The migration
rail therefore needs two explicit states:

- a frozen 49-entry coverage ledger that can report v3 readiness without
  claiming v3 authority; and
- one final cutover where every retained source passes v3 and every generated
  consumer rejects stale or mixed versions.

Without that cutover rule, “populate later” can recreate the dual writable
authority already present in v1/v2.

FABLE’s objection is also narrower than the compiler’s role. The compiler is
not merely a frontmatter validator; it is the source-to-projection boundary
that must make Go, Python, routers, bounded-context membership, effects, and
proof coverage agree.

### Criticism that changes SOL’s decision, scope, ordering, or confidence

Three changes are accepted:

1. Remove `select_experiment` from the skill authority enum. Add an invariant
   that no skill may own caller continuation or experiment selection.
2. Derive or delete `tier`; constrain `disposition` to catalog curation; replace
   overlapping lifecycle meanings in `hexagonal_role` with generated views.
3. Land grammar, hostile fixtures, one parser, strict consumers, and migration
   ledger first. Populate source contracts in their semantic tranches, then
   perform one all-49 cutover. Do not publish partial v3.

Revised confidence: **97%**, up from 96% because the reveal supplies the missing
taxonomy constraint and makes the compiler architecture more coherent.

### Revised ruling

**MERGE.** Merge SOL’s typed compiler with FABLE’s one-seam vocabulary and
taxonomy budget. The result is one machine-owned contract grammar, no
skill-grantable continuation authority, generated placement views, structured
effects and outputs, positive/negative/ambiguity route fixtures, and explicit
axis retirement.

### Smallest synthesis rule

> Every new classification field must retire or derive an old ambiguity, and
> no skill contract may express caller-owned experiment selection.

## Finalist 3 — Projection publication transaction

FABLE score: **810/1000**

### Where FABLE is right

FABLE correctly separates the demonstrated hazard from speculative
concurrency. The live defect is that `regen-all.sh` continues after a failed
step, allowing later generators or hash writers to seal a mixed tree. The
repository contract defaults to one agent and one writer, so an elaborate
multi-writer journal and stale-lock protocol should not displace semantic work
without evidence that concurrent publication is planned.

The cheap half is immediately valuable: an owner map, fail-fast orchestration,
one renderer for check/write, byte-idempotence, and a manifest written only
after the complete projection set validates.

### Where FABLE is wrong or under-specified

Fail-fast direct writes plus a final manifest detect partial publication but do
not prevent it. A generator can mutate several live files and then fail. The
tree will correctly report drift, but subsequent checks and human work still
operate on a damaged mixed state until regeneration succeeds. Rendering the
complete set into a staging root and validating it before touching owned live
paths addresses validation failure, not only concurrency. It is therefore
load-bearing even under one writer.

Conversely, SOL’s original journal/backup language was under-specified and too
ambitious for the first cut. A manifest-last protocol can fail closed without a
general transaction journal, provided the staged set is complete and rerunning
is convergent.

### Criticism that changes SOL’s decision, scope, ordering, or confidence

SOL accepts a staged implementation:

- Before mass metadata edits: owner map, fail-fast, complete staging render,
  validation against staging, atomic per-file replacement, manifest last, and
  byte-idempotence.
- Only when explicit concurrent publishers are introduced: repository lock,
  stale-lock recovery, and the full concurrent/kill-step matrix.

Worker lanes remain forbidden from generating shared projections.

Revised confidence: **92%**, down from 95% because locking and full crash
journaling are not currently justified on the critical path.

### Revised ruling

**MODIFY.** Retain staged-complete rendering and manifest-last publication.
Demote the concurrency lock, recovery journal, and exhaustive crash matrix to a
ratchet triggered by actual concurrent publication. Do not demote staging
itself.

### Smallest synthesis rule

> One owner serializes publication, but no owned live projection changes until
> the complete staged generation validates; the manifest is written last.

## Finalist 4 — CLI audit as a prerequisite substrate stage

FABLE score: **765/1000**

### Where FABLE is right

FABLE’s strongest criticism is correct: SOL asserted that the skill migration
materially traverses the CLI’s defective paths without enumerating the actual
acceptance command graph. The worst verified defects cluster in eval,
provenance, goals measurement, generic dry-run policy, and subprocess capture.
The current migration’s strongest proof paths are primarily Python schema and
generation tools plus stronger Go surfaces such as skills, gates,
verdictcheck, and status.

The eval path-containment flaw is urgent on its own release and security merits.
It does not need an inflated dependency claim to justify immediate repair.

FABLE is also right to preserve SOL’s three-part decomposition:
containment/resources, command effect/output policy, and process lifecycle.
That is a coherent CLI program even if it is not a universal migration
prerequisite.

### Where FABLE is wrong or under-specified

“The current proof-relevant packages are among the strongest” does not prove
that no crossing exists. The migration proposes effect probes, runtime
unavailable/timeout behavior, generated CLI catalog consumption, and command
contract checks. Some later tranches may invoke the shared gate subprocess
runner or affected runtime/process surfaces. The dependency must be discovered
per acceptance command, not inferred from package-level audit grades.

The separate-program framing also needs one shared vocabulary boundary. CLI
command effects and skill effects must derive from compatible semantic types;
otherwise the two programs can both complete while disagreeing on what
“external mutation,” “dry run,” or “bounded process” means.

### Criticism that changes SOL’s decision, scope, ordering, or confidence

This criticism changes the placement materially. The CLI audit becomes an
**independent release program with named crossings**, not a blanket
prerequisite stage.

T0 must enumerate every acceptance command for every skill tranche and map it
to audited CLI packages. A verified defect blocks only a tranche that actually
traverses it. Eval containment is expedited immediately as a security release.
The effect/output vocabulary lands jointly with the contract compiler. The
remaining command-policy and process work can proceed in parallel and becomes a
hard dependency at its first proven consumer.

Revised confidence in the original placement: **72%**, down from 97%. Confidence
in the three-tranche CLI program itself remains **96%**.

### Revised ruling

**DEMOTE.** Demote “CLI as universal prerequisite.” Retain the audit
decomposition as a parallel release program, with immediate containment work
and an executable crossing matrix that can promote individual fixes to
prerequisites.

### Smallest synthesis rule

> A CLI defect blocks the first migration tranche whose acceptance command
> traverses it; otherwise it remains urgent parallel release work, not global
> critical path.

## Finalist 5 — Portfolio and tranche recut around authority and proof shape

FABLE score: **790/1000**

### Where FABLE is right

FABLE validates several of the portfolio’s strongest calls:

- move `security` to evidence/validation-evidence;
- retain runtime adapters only while a distinct behavioral probe justifies
  their root;
- put `automation-shape-routing` in advisory/support;
- require a one-sentence specialization proof and observable falsifier;
- preserve compatibility aliases for renames.

Its objections are also correct. Eighteen integration stages are too many as a
default plan. `postmortem` does not automatically deserve a one-skill tranche.
The `shared` retirement search missed six prose references to
`agent-native/references/model-dispatch.md`, including core `validate`.
Replacing the 250-line gate before defining and implementing a real
always-loaded context measure would remove a working proxy without a
replacement.

### Where FABLE is wrong or scoring a different framing

The tranche table was an upper bound with an explicit instruction to split only
when RED fixtures expose a separate authority question. Still, presenting
eighteen named stages predictably turns an upper bound into perceived ceremony;
FABLE is right about the operational effect even if the prose allowed merging.

The existence of cross-skill model-dispatch consumers proves the need for a
neutral contract owner, not the need for a canonical `shared` skill. Populating
`shared` would preserve a meta-skill whose trigger, output, and behavioral proof
remain unclear. The cleaner target is a runtime-neutral declared contract for
fresh-context/model identity, plus adapter-owned dispatch recipes.

The line-count cap should remain a warning/ratchet during migration, but it
should not become proof that a 249-line skill has acceptable loaded cognitive
cost. Both measures can coexist until the stronger metric is executable.

### Criticism that changes SOL’s decision, scope, ordering, or confidence

The portfolio changes in four ways:

1. Collapse the default migration to approximately the plan’s tranche count;
   split only on a failing authority/proof fixture.
2. Relocate the six cross-skill references before retiring `shared`. Neutral
   invariants move to `docs/contracts/`; runtime-specific recipes remain under
   adapters.
3. Pin Campaign/Goal/Fitness vocabulary before renaming the skill. Rename only
   the skill slug; keep the distinct `ao goals` product command stable unless
   its own contract requires change. Maintain a bounded, non-advertised
   `goals` skill alias.
4. Keep the 250-line cap as an interim warning or gate while implementing an
   always-loaded token/context measure; do not treat line count as the final
   architecture.

Revised confidence: **94%** in the portfolio decisions, **80%** in any exact
tranche table.

### Revised ruling

**MODIFY.** Retain the portfolio dispositions and specialization-proof rule.
Collapse the tranche table, make `shared` retirement conditional on contract
relocation, sequence `goals`/`fitness` after vocabulary, and retain the line cap
until a tested context-budget replacement exists.

### Smallest synthesis rule

> Keep a skill root only for a distinct tested contract, and change portfolio
> shape only after every consumer has an explicit new owner.

## Cross-plan decisions

### 1. Exact RPI transaction first versus a five-skill campaign-to-verdict atom

**Decision: exact RPI transaction first.**

FABLE’s reveal concedes the ordering, and SOL accepts the useful part of its
authority-transfer concern. The transaction that removes RPI continuation must
also assert the replacement owner: the external caller/Goal runtime owns
whether another invocation occurs. It must not name `craft-goal` as the runtime
owner, because `craft-goal` only compiles and checks campaign policy.

The safe sequence is:

1. repair exact-byte Plan/Implement/Validate/RPI identity;
2. delete RPI continuation and report terminally, naming caller ownership;
3. freeze the proof-contract epoch;
4. migrate `craft-goal` as an optional campaign-policy compiler;
5. migrate product admission and the renamed fitness observation.

This has neither double authority nor a zero-owner interval because the caller
exists throughout.

### 2. CLI audit prerequisite versus independent release program with crossings

**Decision: independent release program with executable crossings.**

The original prerequisite claim was too broad. T0 must produce a command-level
dependency matrix, not a narrative assertion. Eval containment proceeds
immediately as security work. Shared command/skill effect vocabulary is a
compiler dependency. Process, output, dry-run, and cleanup defects block the
first skill tranche that actually exercises them. Other CLI work proceeds in
parallel and cannot be used to delay the exact kernel or catalog grammar.

### 3. `shared` retirement versus population with cross-skill contracts

**Decision: relocate, then retire.**

The six model-dispatch references reveal a real contract class but not a useful
skill trigger. Move runtime-neutral fresh-context, validator-identity, and model
identity semantics to a declared document under `docs/contracts/`. Keep
runtime-specific invocation and dispatch under adapters. Update all consumers
and add a reference-integrity gate. Then retire the empty `shared` skill.

Do not turn `shared` into a catch-all merely because its name is available.

### 4. Transactional staged publication versus one serialized regeneration entrypoint

**Decision: serialized ownership plus staged validation and manifest-last
publication.**

One entrypoint answers who writes. Staging answers whether invalid partial
output reaches the live tree. Both are required before mass catalog edits.
Locking and crash journaling are deferred until concurrency is authorized, but
complete staging is not. Worker lanes never generate shared projections.

### 5. Typed compiler versus a taxonomy budget that retires axes

**Decision: merge them.**

The compiler is the mechanism; the taxonomy budget is a design invariant. The
machine grammar owns layers, seams, skill-grantable authority, structured
effects, typed outputs, route fixtures, failure semantics, and proof classes.
It must derive or retire overlapping `tier`, `disposition`, and role fields
rather than preserve them as independent truths.

The merged rule is:

> No new catalog axis lands without naming the ambiguity it eliminates, the old
> axis it derives or retires, and the executable contradiction it rejects.

### 6. Validator/report/schema evolution and comparable tranche verdicts

**Decision: versioned proof epochs, never retroactive re-baselining.**

The repaired kernel establishes an epoch identified by at least:

- exact validator contract/implementation digest;
- verdict schema ID;
- report schema ID;
- subject-manifest schema ID;
- exact intent and subject digests recorded by each result.

All semantic tranches should run under that frozen epoch. If a load-bearing
proof contract must change later, it gets its own RPI and starts a new epoch.
The completion matrix preserves each tranche’s original tuple and explicitly
marks the epoch boundary. Cross-epoch results can be compared only through a
declared compatibility relation; they are never silently pooled or rewritten.

This rule preserves durable history. It also creates a strong incentive to
finish the proof substrate before semantic migration.

### 7. `goals`/`fitness` naming and compatibility

**Decision: pin vocabulary, then rename the skill with a bounded alias.**

The ubiquitous language must distinguish:

- **Goal:** caller-side persistent campaign state and policy;
- **fitness:** read-only observation of project health/progress;
- **RPI experiment:** one terminal Plan/Implement/Validate/verdict transaction.

After that contract lands, rename the canonical skill from `goals` to
`fitness`. Generated routers advertise only `fitness`; `goals` remains a
non-advertised compatibility alias for one declared window and emits a
deprecation signal where the runtime supports one. The `ao goals` CLI product
surface remains named `goals` because it represents Goal runtime behavior, not
the old skill meaning. No dual canonical sources are permitted.

### 8. Security’s tranche placement

**Decision: evidence/validation-evidence, with mutation separately authorized.**

The primary `security` contract collects and structures security evidence. It
does not issue a verdict and does not authorize remediation. Validate may
consume its evidence under the exact subject identity.

If a security workflow can also mutate baselines, policies, or code, that mode
must declare a separate structured effect and run only under caller/Implement
authorization. Its evidence collection fixture and mutation fixture remain
distinct. This places the skill in the evidence tranche without allowing it to
become a shadow validator or autonomous implementation authority.

## Revised top three decisions

1. **Repair and version the exact RPI transaction first.** Add opaque
   correlation, prove exact-byte identity end to end, remove continuation, bind
   verdicts to one proof-contract epoch, and freeze it before campaign work.
2. **Merge the typed compiler with the taxonomy budget.** Forbid
   skill-owned experiment selection, retire redundant axes, type effects and
   outputs, and make every generated consumer strict.
3. **Publish projections from one complete staged render.** Fail fast, validate
   before live replacement, write the generation manifest last, and add locks
   only when concurrent publication is actually authorized.

## One disagreement that should remain visible to the operator

The operator should explicitly decide what happens if the validator/report
contract changes after semantic tranches have durable verdicts. FABLE’s
fixed-point language allows “re-baselining”; SOL rejects any interpretation
that replaces or upgrades earlier judgments. SOL’s recommendation is a visible
new proof epoch with immutable old verdicts and an explicit compatibility
statement. This is the remaining disagreement with the greatest risk to
evidence integrity.

## New consensus the current plan should adopt immediately

The original plan should incorporate these now, before implementation:

1. Repair the live RPI/Validate digest mismatch before campaign migration.
2. Add a schema-backed `rpi-report.v2` with narrow opaque correlation and
   explicit required-gap versus declared-exclusion semantics.
3. Remove skill-owned experiment selection from the catalog model; the caller
   retains continuation authority.
4. Pair the typed skill compiler with explicit axis retirement and a single
   all-49 cutover.
5. Make every named migration check live, seeded with a negative fixture, and
   mapped to acceptance; repair or retire the broken orchestration boundary
   gate.
6. Make `regen-all` fail fast, render and validate a complete staged set, and
   write its ownership/digest manifest last.
7. Move `security` to evidence, relocate cross-skill model identity contracts
   before retiring `shared`, and pin Goal/Fitness vocabulary before the skill
   rename.
8. Run the CLI audit as an expedited parallel release program with a
   command-level crossing matrix, rather than a blanket migration
   prerequisite.
