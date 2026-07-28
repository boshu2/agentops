# SOL Cross-Score of FABLE Finalists

Date: 2026-07-24
Scorer: SOL
Subject: `WIZARD_IDEAS_FABLE.md`

## Scoring summary

| Finalist | Structural soundness | System coherence | Maintainability and proof | Migration feasibility | Total | Ruling |
|---|---:|---:|---:|---:|---:|---|
| F1. Proof substrate and frozen validator fixed point | 185 | 205 | 240 | 170 | **800** | Adopt with a precise modification |
| F2. One seam vocabulary owned by contract | 238 | 230 | 205 | 228 | **901** | Adopt with a precise modification |
| F3. Taxonomy budget and mechanical honesty | 238 | 245 | 235 | 197 | **915** | Adopt with a precise modification |
| F4. Authority-transfer tranche atom and harness | 145 | 195 | 210 | 195 | **745** | Subsumed; reject the five-skill atom |
| F5. Contract-tier cleanup and shared model dispatch | 165 | 185 | 205 | 220 | **775** | Subsumed with modifications |

The scores are intentionally discriminating. F2 and F3 are near peers because
they address two sides of the same architecture problem: F2 establishes a
single lifecycle language, while F3 keeps that language from becoming another
unbounded metadata layer. F1 contains the strongest proof discipline but also
the most consequential self-hosting flaw. F4 and F5 contain valuable work, but
their proposed bundles do not preserve the cleanest ownership boundaries.

## F1. Prove the proof substrate first: check liveness and a frozen validator fixed point

### Score

- Structural soundness: **185/250**
- System coherence: **205/250**
- Maintainability and proof: **240/250**
- Migration feasibility: **170/250**
- **Total: 800/1000**

### Strongest source-backed contribution

The check-liveness inventory is exceptionally strong. FABLE ties a plan-named
gate to a reproducible defect: `scripts/check-orchestration-skill-boundaries.sh`
still names deleted `cli/internal/adapters/agentworker_ntm/ntm.go`, is not
integrated into the normal gate surface, and can therefore look like proof
without executing as proof. Requiring every promised acceptance criterion to
map to an executable check, and requiring seeded negative cases, directly
attacks completion theater. The completion matrix with durable verdict digests
is also materially better than a prose claim that a tranche passed.

### Strongest objection or hidden cost

The “freeze, then rebaseline if changed” rule gives the proof machinery a
quasi-lifecycle of its own. The current kernel still has contract mismatches,
including the `rpi-report.v1` correlation mismatch FABLE itself identifies.
Freezing before the exact experiment identity path is repaired can faithfully
pin an incoherent kernel. Revalidating prior tranches after a validator change
is worse: a durable verdict should remain a statement made under an immutable,
identified validator contract, not be retrospectively refreshed to satisfy the
current campaign. That risks replacing exact evidence history with campaign
administration.

### Ruling

**Adopt with a precise modification.** Land the check-liveness inventory and
criterion-to-check matrix first. Before pinning, establish an end-to-end kernel
test over exact intent bytes, subject identity, changed-path coverage, fresh
validator identity, verdict, and report. Then version and digest the validator
and schemas used by each verdict. If the proof contract changes, begin a new
contract epoch; do not rebaseline or reinterpret old verdicts. A migration
summary may compare epochs, but it may not replace their original judgments.

### Evidence that would move the score by at least 150 points

An executable prototype showing that the proposed initial fixed point is
internally coherent end to end, and that “rebaseline” records a new comparison
without mutating, superseding, or reinterpreting any prior verdict, would add at
least 150 points. Conversely, evidence that tranche acceptance depends on
retroactively replacing old verdicts would remove at least 150 points.

## F2. Establish one contract-owned seam vocabulary without authority-implying names

### Score

- Structural soundness: **238/250**
- System coherence: **230/250**
- Maintainability and proof: **205/250**
- Migration feasibility: **228/250**
- **Total: 901/1000**

### Strongest source-backed contribution

FABLE identifies a real semantic split among the plan, the ports-and-adapters
contract, and the proposed catalog: `experiment_select` is assigned to
`idea-genie` even though Goal/campaign control owns experiment selection, while
`standalone` and `core_phase` are used without being defined by the contract
table. Renaming the advisory seam to `option_shaping`, and adding explicit
Campaign and Evolution responsibilities to the ubiquitous language and
component map, prevents a routing label from silently acquiring lifecycle
authority. This is exactly the kind of small naming defect that otherwise
becomes a second controller.

### Strongest objection or hidden cost

Making a narrative contract document the enum owner is not sufficiently
executable. It can simply move drift to a different file. Likewise, treating
`core_phase` as a seam mixes two dimensions: where a skill participates and
whether it is part of the invariant experiment kernel. Finally, a handwritten
`bounded-contexts.yaml` membership list can become yet another placement
inventory unless it is the machine source from which all other views derive.

### Ruling

**Adopt with a precise modification.** Put the closed lifecycle vocabulary in
one machine-readable schema or compiler source. Generate or conformance-check
the narrative contract and router projections from it. Keep `option_shaping`
as the advisory seam. Encode core-kernel membership or authority separately
from lifecycle seams. Add Campaign and Evolution to the responsibility model,
but generate skill membership rather than maintaining a parallel handwritten
list.

### Evidence that would move the score by at least 150 points

Because the score is already above 850, only contrary evidence can move it by
150. A complete consumer audit proving that seam values are never used for
routing, authorization, documentation, or model decisions—and that every
ambiguous term is already resolved by one other executable source—would lower
the score by at least 150 points.

## F3. Impose a taxonomy budget and make honesty sweeps mechanical

### Score

- Structural soundness: **238/250**
- System coherence: **245/250**
- Maintainability and proof: **235/250**
- Migration feasibility: **197/250**
- **Total: 915/1000**

### Strongest source-backed contribution

This is FABLE’s strongest finalist. The evidence is broad and concrete: the
system already has roughly six classification dimensions; approximately thirty
effectful skills declare empty effects; `consumes` and `produces` blur artifacts
with skill dependencies; runtime-specific hints leak into canonical skills; and
the plan proposes more placement fields without retiring redundant ones.
Deriving tier, restricting disposition to curation, making artifact flow
distinct from dependencies, typing output contracts, and testing triggers turns
the catalog from descriptive folklore into a compiler boundary.

The “net axis count stays flat” constraint is especially useful as a migration
review heuristic. It forces each new field to justify which older ambiguity it
eliminates.

### Strongest objection or hidden cost

A flat closed effects enum such as `filesystem_write` or `external_mutation`
does not by itself express the safety boundary. The architecture needs to know
scope, authorization source, cleanup/rollback expectations, and whether an
effect is required or merely possible. Otherwise the sweep can become
well-typed metadata theater. Similarly, a universal one-in/one-out axis rule is
a valuable budget, not an invariant: two genuinely independent dimensions
should not be collapsed merely to keep the count flat. Positive trigger probes
alone are also easy to game unless ambiguity and negative cases are included.

### Ruling

**Adopt with a precise modification.** Implement one typed catalog compiler
rather than layering fields onto v1/v2. Give effects structured kind, scope,
authorization, and containment semantics. Make output contracts distinguish
binding artifacts from advisory prose. Require positive, negative, and
ambiguity trigger fixtures. Derive tier and every generated projection from the
same intermediate representation. Treat the flat taxonomy budget as a
mandatory design-review test with an explicit exception process, not as a
schema law.

### Evidence that would move the score by at least 150 points

Evidence that real external consumers require the existing independent `tier`
or mixed dependency/artifact semantics, and cannot migrate through a
versioned compatibility reader, would lower the score by at least 150 points.
So would seeded tests showing that the proposed simple effects and trigger
fields pass while a skill still performs an unauthorized mutation or wins an
ambiguous route.

## F4. Re-cut tranches around an authority-transfer atom and an executable harness

### Score

- Structural soundness: **145/250**
- System coherence: **195/250**
- Maintainability and proof: **210/250**
- Migration feasibility: **195/250**
- **Total: 745/1000**

### Strongest source-backed contribution

The tranche critique contains two strong corrections. Splitting T6 into
transport/infrastructure adapters versus host/support skills removes a false
cohesion boundary. Reusing the pure reference implementations in
`skills/rpi/references/run_once.py`,
`skills/craft-goal/references/dispatch_once.py`, and
`skills/validate/references/validate.py` for negative authority scenarios is
far more probative than checking prose. Moving `product` and the `goals` name
collision out of the core tranche is also correct.

### Strongest objection or hidden cost

The proposed five-skill T2 atom
`{craft-goal, rpi, plan, implement, validate}` still joins campaign control to
the experiment kernel. FABLE argues that any other order creates double
authority or no authority, but the operating contract already supplies the
bridge: the caller owns whether another invocation occurs. The old continuation
behavior can be removed from `rpi` while the caller continues to select work;
`craft-goal` can then be added as an optional campaign controller. There is no
zero-owner interval.

The publication rule is also too weak. “One regeneration entrypoint” does not
provide locking, staging, crash recovery, or atomic replacement of multiple
generated surfaces. Per-skill lanes do not make shared generated paths safe in
a dirty worktree.

### Ruling

**Subsumed into F1/F3 and a recut migration; reject the five-skill atom.** Keep
the pure-script behavioral harness, the T6 split, and the relocation of
`product` and `goals`. Land the exact four-skill experiment kernel first. Then
land `craft-goal` as a separate campaign-layer migration with explicit
non-authority tests. Replace the single-regeneration convention with a locked,
staged, validated, atomic publisher. Parallelize only isolated, disjoint source
edits; serialize all shared projections.

### Evidence that would move the score by at least 150 points

A dependency proof and executable transition test showing that the caller
cannot safely own selection between the `rpi` cleanup and `craft-goal`
installation—and that every non-atomic order necessarily creates observable
double authority or no authority—would add at least 150 points. The present
contract says the opposite.

## F5. Restore declared-contract tier truth: remove dead schemas, add promised fields, and relocate model dispatch

### Score

- Structural soundness: **165/250**
- System coherence: **185/250**
- Maintainability and proof: **205/250**
- Migration feasibility: **220/250**
- **Total: 775/1000**

### Strongest source-backed contribution

FABLE’s dead-schema finding is important. `plan-packet.v1`,
`candidate-packet.v1`, and `revision-packet.v1` remain in the declared schema
tier despite having no live consumers beyond tombstone guards. Under the
repository’s source precedence, that is not harmless debris; it leaves obsolete
architecture in an authoritative location. The mismatch between the promised
opaque campaign correlation and `rpi-report.v1` with
`additionalProperties: false` is another concrete contract defect. The
six-skill dependency on
`skills/agent-native/references/model-dispatch.md` also exposes a real ownership
problem.

### Strongest objection or hidden cost

The finalist bundles four different ownership problems into one cleanup
proposal. In particular, moving `model-dispatch.md` into the canonical `shared`
skill risks turning runtime-neutral validator identity rules into a dependency
on a meta-skill. A shared skill can become the exact “miscellaneous” bucket that
the overhaul is trying to eliminate. The neutral fresh-context and model
identity contract belongs in a declared contract/reference owner, while
runtime-specific dispatch belongs in adapters.

The proposed correlation object also needs a deliberately narrow shape. An
arbitrary object can smuggle campaign state into the experiment report. Dead
schemas should be removed only after checking for external raw-repository
consumers and documenting the contract epoch; lack of internal generated-image
use is not complete consumer proof.

### Ruling

**Subsumed with modifications.** Remove or tombstone dead packet schemas as
part of the exact experiment-contract migration after an external-consumer
check. Add narrowly typed, size-bounded, opaque correlation identifiers to a
new report schema version, with an explicit rule that RPI does not interpret
them. Move neutral validator/model-identity rules to a declared contract owner
and adapter-specific dispatch to the relevant runtime adapter. Do not populate
`shared` as a canonical catch-all. Type `cc-hooks` and `dcg` outputs through the
catalog compiler from F3.

### Evidence that would move the score by at least 150 points

A live dependency audit proving that `shared` is already the intended stable
owner across all six consumers, that core validation can consume it without
acquiring an optional skill dependency, and that no external consumers use the
packet schemas would add at least 150 points. Discovery of active external
packet-schema consumers or campaign logic interpreting the new correlation
object would remove at least 150.

## Opponent-level assessment

### Strongest overall architecture

F3 is FABLE’s strongest architecture. It correctly sees that the overhaul is
not mainly a documentation rewrite; it is a schema-design problem whose output
must control routing, authority, effects, artifacts, and generated projections.
Its taxonomy budget prevents the migration from “solving” drift by adding more
overlapping labels. Combined with F2’s vocabulary correction, it provides a
credible product/campaign/experiment/evolution placement model for all 49
skills without granting lifecycle authority to the catalog.

### Weakest overall assumption

The weakest assumption is that campaign Goal and the one-shot experiment kernel
must move as one five-skill authority-transfer atom. That assumes the caller
ceases to exist during migration. The live operating contract says the caller
owns continuation before, during, and after every RPI invocation. Coupling
`craft-goal` to the kernel therefore does not avoid an authority gap; it makes
an optional campaign policy a prerequisite for proving the core experiment.

### Semantic matches with SOL finalists

| FABLE finalist | Semantic match in SOL architecture | Important distinction |
|---|---|---|
| F1 proof substrate | Exact-byte RPI transaction and executable acceptance | SOL repairs and versions the kernel before pinning; it does not rebaseline durable verdicts. |
| F2 seam vocabulary | Typed v3 skill compiler and explicit product/campaign/experiment/evolution placement | FABLE names the language more precisely; SOL requires the machine schema/compiler, not prose, to own it. |
| F3 taxonomy budget | Typed v3 compiler, derived projections, typed effects and outputs | This is the closest match. FABLE contributes the valuable net-axis budget; SOL requires richer effect and trigger semantics. |
| F4 tranche recut and harness | Portfolio recut, exact kernel first, serialized generated publication | Both split false-cohesion tranches and require behavioral tests. They disagree on the five-skill atom and on whether one regeneration command is sufficient. |
| F5 contract cleanup | Exact transaction contracts, source-authority cleanup, and compiler-owned output contracts | Both remove dead authority and repair report truth. They disagree on `shared` as the destination for model-dispatch semantics. |

### Genuine disagreements that must not be averaged away

1. **Core before campaign versus a five-skill atom.** The exact
   `plan -> implement -> fresh validate -> verdict -> report` kernel should be
   proven before `craft-goal`. `craft-goal` must remain an optional caller-side
   campaign controller. Averaging these positions would recreate a hidden
   continuation dependency.

2. **Versioned proof contracts versus rebaseline.** Old verdicts retain their
   original schema and validator identity. A new validator starts a new epoch;
   it does not cause earlier tranche judgments to be replayed as if history had
   used the new rules.

3. **Transactional publication versus one regeneration entrypoint.** Shared
   generated outputs require a lock, staging directory, validation of the
   complete staged set, and atomic publication. A social convention around one
   command does not contain crashes or concurrent writers.

4. **CLI audit placement.** FABLE places the CLI audit in a separate program
   with only explicit crossings. SOL treats the critical safety and proof-chain
   findings as prerequisite substrate stages: path containment and resource
   limits first; command policy/output contracts next; subprocess lifecycle
   hardening next. Unrelated CLI feature work can remain separate or run in
   parallel, but migration tranches may not claim behavioral proof while the
   gate, goal, process, or output substrate they execute is known-unsound. This
   is a dependency decision, not a request to absorb every CLI finding into the
   skill migration.

5. **`shared` as owner versus neutral declared contracts.** A cross-skill
   reference needs a stable owner, but a canonical miscellaneous skill is not
   automatically that owner. Fresh-context/model-identity invariants belong to
   the experiment contract; runtime dispatch belongs to adapters.

6. **Security placement.** FABLE keeps `security` in capability evolution.
   SOL places its binding evidence obligations with experiment evidence while
   leaving reusable security-method evolution outside the loop. A single
   disposition cannot represent both roles; the skill must expose an advisory
   method and a separately typed evidence output without becoming the validator.

## Effect of FABLE’s 49-skill appendix on SOL dispositions

One appendix item materially sharpens SOL’s disposition:

- **`shared`:** FABLE’s source-backed observation that six canonical skills
  reference `skills/agent-native/references/model-dispatch.md` changes the
  sequencing from “retire the empty `shared` skill” to “first relocate the
  shared invariant to a neutral declared-contract owner, update and test all
  six consumers, then retire `shared` as a skill.” It does **not** change the
  target architecture to “populate `shared`”; it changes the retirement
  precondition and prevents deleting a currently hidden cross-skill contract.

No other appendix row changes SOL’s target disposition. Several corroborate it:
`product` remains outside the experiment, `goals` needs terminology separation,
`craft-goal` is campaign control, `learn` and pattern evolution remain
post-verdict, runtime tools remain adapters, and the four core skills remain
inside exactly one bounded experiment.
