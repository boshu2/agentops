# SOL — sealed skill-system architecture proposal

Date: 2026-07-24
Mode: independent architecture review, research and planning only
Coverage: the frozen packet, all 49 canonical `skills/*/SKILL.md` files, and
the live Go, Python, schema, generator, and gate surfaces cited below

## Executive decision

Keep the evolved RPI membrane, but do not execute the migration in the order or
at the level of abstraction in the current plan.

The strongest target is a three-level semantic architecture supported by two
cross-cutting substrates:

```text
caller / product
  owns outcome, authority, terminal acceptance, and delivery
  |
  v
Goal campaign
  owns graph, experiment selection, monotonic budgets, ratchet, breakers,
  and terminal campaign report
  |
  | selects one exact experiment intent
  v
RPI evidence transaction
  exact intent bytes
    -> Plan once
    -> optional caller-selected pre-build challenge
    -> Implement once
    -> runtime-derived before/after identity, changed paths, and receipts
    -> fresh Validate once
    -> immutable verdict.v2
    -> rpi-report
    -> stop

Optional seams around the transaction
  product/fitness evidence | intent evidence | option shaping | plan review
  implementation methods | validation evidence | judgment strategies
  post-verdict analysis | capability-evolution proposals | runtime transport

Cross-cutting substrates
  skill-contract compiler + transactional projection publisher
  CLI safety/capability substrate
```

The plan correctly separates Goal from RPI, correctly keeps
`rpi -> {plan, implement, validate}` as the only hard skill graph, and correctly
treats strategies and runtimes as optional ports. It is not ready to implement
because the live evidence kernel, skill contract compiler, projection publishing
model, CLI policy substrate, and tranche boundaries are not yet coherent enough
to prove that architecture.

The five highest-leverage changes are:

1. make one exact-byte evidence transaction the executable RPI kernel;
2. replace dual loose frontmatter validation with one typed skill-contract
   compiler;
3. make projection generation a locked, staged, manifest-committed publication;
4. land the CLI audit as a prerequisite safety/capability substrate stage;
5. recut the portfolio and migration around semantic authority and proof shape,
   with explicit retirement/rename/conditional-retention decisions.

## What the live system actually says

### The recovered invariants

- `AGENTS.md`, `docs/architecture/operating-loop.md`, and
  `skills/rpi/SKILL.md` agree that one RPI invokes Plan, Implement, and a fresh
  Validate at most once and then stops.
- `skills/validate/SKILL.md` and `schemas/verdict.v2.schema.json` make Validate
  the sole `verdict.v2` writer. Strategies may advise an accountable validator;
  they cannot issue semantic PASS.
- `skills/craft-goal/SKILL.md` contains the campaign behavior that older RPI
  prose incorrectly absorbed: adaptive selection, monotonic ceilings, ratchets,
  breaker/HOLD state, and terminal campaign reporting.
- The non-core skills naturally fall into ports around the transaction, not
  additional phases inside it.
- Tool-specific panes, mail, Gas City, remote compilation, account rotation,
  hooks, and storage repair are execution or environment adapters. Their state
  is factual evidence, never experiment or campaign authority.
- Learn, Postmortem, Toil Mining, Pattern Mining, and Operationalize are
  distinguishable only if recurrence, causality, toil, earned abstraction, and
  packaging proposal remain separate outputs. None may self-promote a
  capability.

### Defects the current plan does not make first-class

1. **The reference RPI and Validate do not share an intent identity
   algorithm.** `skills/rpi/scripts/run_once.py:17-19,60` hashes canonical JSON
   of a model-returned mapping. `skills/validate/scripts/validate.py:210,546,571`
   hashes the exact intent bytes. The unit test at
   `skills/rpi/tests/test_run_once.py:31` mocks Validate with the RPI-local
   algorithm, so both test suites pass without exercising the real handoff.
   This violates the stated exact-source continuity contract.
2. **PASS coverage semantics are stricter than the prose and completion plan
   acknowledge.** `schemas/verdict.v2.schema.json:81` and
   `skills/validate/scripts/validate.py:266,389` prohibit any `not_checked`
   entry on PASS. The migration plan simultaneously requires platform-specific
   unavailable surfaces to remain explicit `not_checked`. The system needs to
   distinguish an acceptance-critical coverage gap from an explicitly
   out-of-contract platform surface; otherwise it must either lie by omission
   or make full-catalog PASS impossible.
3. **There are two live skill-frontmatter authorities.**
   `scripts/validate-skill-schema.sh:5,18` and
   `scripts/validate-manifests.sh:377` use v1, while
   `scripts/validate-skill-frontmatter.sh:3,15` uses v2. v1 requires
   `skill_api_version` and constrains more top-level fields; v2 requires
   metadata completeness but allows arbitrary additional top-level fields.
   Neither requires system layer, lifecycle seam, typed effects, or a binding
   output contract.
4. **Generated publication is conventionally serial, not transactionally
   safe.** `scripts/generate-skill-mesh.py:276-325` removes directories and
   directly writes many destinations. `scripts/codex-sync.sh:437-686` deletes
   and writes twins plus two shared catalogs. `scripts/regen-codex-hashes.sh:
   172-178` rewrites markers and the manifest. There is no shared lock, staged
   complete output tree, rollback journal, or final generation digest.
5. **The current gates can be green structurally while semantically stale.**
   On this worktree, `generate-skill-mesh.py --check` and
   `check-skill-mesh.py` pass for 49 skills, but strict frontmatter fails because
   `skill-builder` lacks `context_rel`, and
   `check-orchestration-skill-boundaries.sh` exits 2 on a deleted Go path before
   checking its stale phrase assertions. This is exactly the “appears complete”
   class the overhaul is meant to eliminate.
6. **Go skill consumers are not strict contract consumers.**
   `schemas/skill-catalog.schema.json` requires tier, disposition,
   capabilities, and effects, but `cli/internal/skills/catalog.go:30-43`
   does not model those fields, does not reject unknown fields, and does not
   validate the catalog schema version or count before answering queries.
7. **The CLI cannot yet be the proof substrate the skill plan assumes.**
   A current live capability projection still reports 84 command nodes, 74 with
   unknown args, 77 with no declared output, 74 with unknown effects, and 74
   with no exit map. The audit's path-containment, dry-run, output-negotiation,
   subprocess, cancellation, and temp-ownership findings are present in the
   cited live sources.

## Candidate generation ledger

I generated 40 candidate decisions before winnowing. “Bundle” means the item is
included in one of the five finalists; “retain” means the plan already has the
right decision; “defer/reject” means it should not drive this migration.

| # | Candidate improvement or alternative decision | Winnow |
|---:|---|---|
| 1 | Preserve one-RPI/one-verdict/report-and-stop as the non-negotiable membrane. | retain |
| 2 | Make exact resolved intent bytes, not a model-returned mapping, the only acceptance identity input. | bundle F1 |
| 3 | Add a real schema and validator for the RPI report and explicit pre-build stop reasons. | bundle F1 |
| 4 | Require a before manifest for changed-path derivation; do not infer complete coverage from a final manifest alone. | bundle F1 |
| 5 | Split acceptance-critical unchecked coverage from declared out-of-contract surfaces. | bundle F1 |
| 6 | Treat a caller-selected Premortem finding as a stop-before-build result, never an in-place Plan repair. | bundle F1 |
| 7 | Add `system_layer` and `lifecycle_seams` as independent generated dimensions. | bundle F2 |
| 8 | Add typed semantic authority separate from side effects and artifact flow. | bundle F2 |
| 9 | Replace free-form effect strings with typed effect, authorization, scope, and cleanup declarations. | bundle F2 |
| 10 | Make positive, negative, and ambiguity trigger cases structured contract data. | bundle F2 |
| 11 | Make every output contract declare binding/advisory status plus schema or executable validator. | bundle F2 |
| 12 | Collapse frontmatter v1/v2 into one versioned skill-contract schema and one parser. | bundle F2 |
| 13 | Make Go catalog readers strict on schema version, count, required fields, and unknown fields. | bundle F2 |
| 14 | Generate bounded-context membership from skill metadata while retaining hand-written context responsibilities. | bundle F2 |
| 15 | Add a per-skill proof descriptor and a generated structural-vs-behavioral coverage report. | bundle F2/F5 |
| 16 | Keep `context_rel` as DDD relationship data and dependencies as required delegation only. | retain |
| 17 | Introduce a single projection publisher with a repository-wide generation lock. | bundle F3 |
| 18 | Generate all projections into staging, validate there, then publish with a final digest manifest. | bundle F3 |
| 19 | For parallel skill edits, use isolated worktrees and prohibit generated writes in worker lanes. | bundle F3 |
| 20 | Regenerate once per frozen tranche source snapshot, not once per edited skill. | bundle F3 |
| 21 | Make check mode use the exact same staged compiler path as write mode. | bundle F3 |
| 22 | Land eval ID containment and owned-temp cleanup before executing migration probes. | bundle F4 |
| 23 | Enforce global dry-run centrally from command effect contracts. | bundle F4 |
| 24 | Unify `--json`/`-o json` through one negotiated output enum and rename file destinations. | bundle F4 |
| 25 | Create one bounded subprocess runner with caller cancellation and process-tree cleanup. | bundle F4 |
| 26 | Complete leaf CLI capability contracts before relying on CLI introspection as migration evidence. | bundle F4 |
| 27 | Add executable command-reference tests for help text. | bundle F4 |
| 28 | Rename the project-fitness skill from `goals` to `fitness` with a bounded alias window. | bundle F5 |
| 29 | Move `automation-shape-routing` from runtime to advisory routing; it selects no executor. | bundle F5 |
| 30 | Retire the empty `shared` skill and move genuinely shared references under explicit consuming owners. | bundle F5 |
| 31 | Move read-only/default-read-only Security beside validation evidence, not candidate producers. | bundle F5 |
| 32 | Split campaign/product migration from the core evidence transaction. | bundle F5 |
| 33 | Split the 18-skill runtime/support tranche by proof shape and mutation risk. | bundle F5 |
| 34 | Reject a universal 250-line kernel gate; use measured always-loaded context budget plus tested exceptions. | bundle F5 |
| 35 | Keep runtime-specific skills only when a distinct capability/unavailable/cleanup probe proves specialization. | bundle F5 |
| 36 | Merge all tool-specific runtime skills into `agent-native`. | reject: erases distinct hazards |
| 37 | Merge Learn, Postmortem, Pattern Mining, Toil Mining, and Operationalize. | reject: collapses distinct evidence claims |
| 38 | Add optional strategies as hard RPI dependencies for consistency. | reject: violates the core graph |
| 39 | Give Goal authority to strengthen or rewrite a prior verdict. | reject: collapses authorship and judgment |
| 40 | Make every one of 49 skills a separate RPI migration. | reject: ceremony without coherent integration proof |

The finalists won on authority leverage, ability to prevent false completion,
dependency ordering, falsifiable acceptance, and containment of migration
failure. Pure naming cleanup, blanket consolidation, and cosmetic CLI polish did
not survive.

## Finalist 1 — Make the RPI evidence transaction executable before migrating campaign policy

### 1. Precise claim

Create one runtime-owned `experiment-transaction.v1` interface whose identity
source is the exact resolved intent byte sequence and whose transitions are
enforced by code. Plan may refine the caller source once; Implement consumes the
resulting snapshot once; Validate consumes the same snapshot digest plus
runtime-derived before/final manifests, actual changed paths, and receipts once;
RPI emits a schema-validated report and stops.

Campaign migration must not land in the same tranche as this kernel. The
transaction is the substrate on which Goal depends and must be proven first.

### 2. Defect or ambiguity resolved

- RPI and Validate currently calculate acceptance identity differently.
- `rpi-report.v1` is prose and a Python dict shape, not a schema-backed durable
  contract.
- final subject identity is defined, but changed-path completeness depends on a
  before state that is not mandatory in the public manifest schema.
- an optional Plan challenge has no explicit machine stop reason.
- `not_checked` conflates acceptance gaps with intentionally excluded surfaces.
- T2 combines campaign policy, product/fitness, and the core transaction into
  one candidate, so a failure cannot localize whether the campaign or experiment
  membrane is wrong.

### 3. Evidence

- `skills/rpi/scripts/run_once.py:17-19,60,77-78`
- `skills/validate/scripts/validate.py:210,266,389,546,571`
- `skills/rpi/tests/test_run_once.py:31`
- `schemas/subject-manifest.v1.schema.json`
- `schemas/verdict.v2.schema.json:81`
- `docs/architecture/operating-loop.md`, especially “One bounded experiment,”
  “Fresh Validate,” and “Stop boundary and revision”
- `skills/implement/SKILL.md`, “Scope conflict rule,” which still permits one
  repair revision despite the operating contract
- plan rows for `rpi`, `plan`, `implement`, and `validate`, and T2 scenarios

### 4. Target architecture and ownership boundary

Use these runtime facts:

```text
intent_snapshot
  bytes
  sha256
  source_ref

experiment_baseline
  intent_digest
  before_subject_manifest_digest
  declared_scope_class
  author_context_id

implementation_result
  final_subject_manifest_digest
  actual_changed_paths
  coverage_complete
  check_receipts

validation_input
  all of the above, immutable
  validator_context_id
  freshness_attestation

verdict.v2
  acceptance and exact-subject judgment only

rpi-report.v2
  status
  stop_reason
  immutable refs/digests
  checked_required
  unchecked_required
  declared_exclusions
```

The runtime computes identities and paths. Plan owns only a source amendment;
Implement owns only subject mutation and factual receipts; Validate owns only
judgment and verdict persistence; RPI owns only ordered single dispatch and
reporting. A Premortem configured for this invocation may return advisory
findings between Plan and Implement. If adopting a finding would alter intent,
RPI reports `NOT_PLANNED` with `stop_reason: amendment_required` and stops.
Only the caller or Goal may amend and invoke another RPI.

Keep fail-closed semantics by defining:

- `unchecked_required`: an acceptance-relevant surface that could not be
  checked; nonempty means PASS is impossible.
- `declared_exclusions`: a surface explicitly outside acceptance or unavailable
  optional platform coverage; it remains visible but does not masquerade as a
  required gap.

Do not merely loosen the current `not_checked` rule.

### 5. Migration steps and dependencies

1. Freeze the current verdict corpus and add failing integration fixtures that
   pass a real byte snapshot from RPI to Validate.
2. Define `rpi-report.v2` and the required/excluded coverage terminology.
3. Add a baseline manifest or equivalent before-state digest requirement for
   mutating experiments.
4. Change `run_once.py` to accept an already-snapshotted intent reference and
   digest; delete its independent JSON digest function.
5. Remove Implement's “one repair revision” authority. Scope conflict returns a
   factual stopped result.
6. Update Validate writer, schema, Go reader, and golden corpus atomically.
7. Add an end-to-end pure fixture proving each phase call count, byte identity,
   path coverage, mutation detection, and durable report/verdict linkage.
8. Only then migrate `product`, renamed `fitness`, and `craft-goal` campaign
   behavior.

Dependencies: the verdict writer/reader corpus must change together. The skill
contract compiler in Finalist 2 may land before or in parallel in an isolated
tranche, but campaign skills depend on this finalist.

### 6. Proof / acceptance criteria

- Given intent bytes that serialize to the same semantic mapping but differ in
  whitespace, the snapshot digests differ and every phase uses the selected
  exact digest.
- The real RPI-to-Validate integration fixture passes without mocking a second
  digest algorithm.
- Plan, Implement, and Validate call counters are each `<= 1` for PASS, FAIL,
  NOT_PROVEN, NOT_PLANNED, NOT_BUILT, scope conflict, and plan-review stop.
- A subject mutation between validation start and end yields NOT_PROVEN.
- A proven changed path outside scope yields FAIL; incomplete path coverage
  yields NOT_PROVEN.
- Nonempty `unchecked_required` prevents PASS. A declared optional platform
  exclusion remains visible without being misclassified as checked.
- The RPI report digest/ref pair resolves to exactly one persisted verdict or to
  an explicit pre-verdict stop reason; it has no next-action field.

### 7. Failure modes and rollback / containment

- **Schema fork:** Python writer, Go reader, and JSON schema disagree. Contain
  with the existing cross-language golden corpus and change them in one tranche.
- **Compatibility break:** existing `rpi-report.v1` consumers fail. Keep a
  read-only v1 decoder and emit v2 only after fixture parity; do not dual-write
  semantically different reports.
- **Coverage laundering:** callers move required gaps to declared exclusions.
  Validate must compare exclusions to frozen acceptance and reject an exclusion
  that removes a criterion.
- **Runtime overreach:** transaction code starts selecting retries. State-machine
  tests assert terminal dispatch counts and absence of continuation fields.
- Rollback is a source revert to the prior writer/reader/schema corpus; existing
  content-addressed verdicts are immutable and require no rewrite.

### 8. Skills and CLI findings affected

Primary: `rpi`, `plan`, `implement`, `validate`.
Secondary: `scope`, `premortem`, `test`, `security`, `council`, `status`,
`handoff`, `craft-goal`.
CLI surfaces: `cli/internal/verdictcheck`, `ao status`, capability contracts for
commands that expose evidence. This proposal does not by itself fix the audit's
general dry-run/output/process findings.

### 9. Confidence and falsifier

**Confidence: 98%.** It would be falsified if a live integration path already
proved that RPI's canonical-mapping digest and Validate's exact-byte digest are
always the same identity. The source and passing isolated tests show the
opposite.

## Finalist 2 — Replace loose metadata with one typed skill-contract compiler

### 1. Precise claim

Introduce one authoritative `skill-contract.v3` schema and compiler. Every
canonical skill must declare enough structured data to decide placement,
authority, trigger separation, artifact flow, effects, failure behavior, and
proof. All catalogs, routers, context membership, CLI queries, runtime images,
and proof-coverage reports derive from that compiler output.

### 2. Defect or ambiguity resolved

The proposed `system_layer` and `lifecycle_seams` are necessary but
insufficient. A layer does not say whether a skill may mutate the subject,
write a verdict, select another experiment, or merely advise. Free-form effects
do not encode authorization, containment, or cleanup. A string
`output_contract` does not distinguish a binding schema from prose. Trigger
false-positive rules live only in bodies and cannot be exhaustively checked.
Two current schemas and several parsers already disagree.

### 3. Evidence

- `schemas/skill-frontmatter.v1.schema.json`
- `schemas/skill-frontmatter.v2.schema.json`
- `scripts/validate-skill-schema.sh:5,18`
- `scripts/validate-skill-frontmatter.sh:3,15`
- `scripts/validate-manifests.sh:377`
- `scripts/generate-skill-mesh.py:28-63`
- `schemas/skill-catalog.schema.json`
- `cli/internal/skills/catalog.go:23-43`
- plan D1, D4, catalog-wide acceptance 1–9, and T1
- current strict check: 49 structurally parsed, but `skill-builder` still lacks
  one optional field; structural success is not semantic completeness

### 4. Target architecture and ownership boundary

The contract should include:

```yaml
schema_version: 3
name: ...
description: ...
system_layer: product | campaign | experiment | judgment | evidence |
              implementation | evolution | runtime | support
lifecycle_seams: [...]
authority:
  - advise
  - read_evidence
  - refine_intent
  - mutate_subject
  - write_verdict
  - select_experiment
  - transport
effects:
  - kind: filesystem.write | process.start | external.mutate |
          credential.switch | runtime.session | host.configure
    scope: caller-declared symbolic scope
    authorization: caller | plan | implement | validate | goal
    cleanup: none | idempotent | required
inputs: [...]
outputs:
  - kind: ...
    semantics: factual | advisory | binding
    schema: path-or-null
    validator: command-or-null
triggers:
  positive: [...]
  negative: [...]
  ambiguous_with: [...]
failure:
  unavailable: ...
  partial_mutation: ...
  timeout: ...
proof:
  class: pure | read_only | mutating_leaf | strategy | runtime_adapter |
         router | core_phase
  command: ...
```

Compiler invariants:

- only `craft-goal` or a future explicit campaign engine may
  `select_experiment`;
- only Plan may `refine_intent` in the core;
- only Implement-authorized skills may `mutate_subject`;
- only Validate may `write_verdict`;
- a runtime transport may not claim campaign or semantic authority;
- advisory outputs cannot be labeled binding;
- an effectful skill cannot have an empty effects list;
- the only hard dependency edges remain
  `rpi -> {plan, implement, validate}`.

`docs/contracts/bounded-contexts.yaml` remains the hand-written owner of context
responsibilities and ports, but its skill memberships/centers are generated
from contract metadata. It must not remain a second skill-placement inventory.

### 5. Migration steps and dependencies

1. Add hostile fixtures for missing/contradictory authority, output, effect,
   trigger, seam, and proof declarations.
2. Implement one parser library used by validation and generation. Delete the
   v1/v2 split after all 49 source files migrate; do not indefinitely support
   two writable authorities.
3. Bump catalog schema and add strict generated-schema validation.
4. Update Go `CatalogEntry`, decoder, filters, graph projection, and capability
   output to reject incompatible versions/counts/unknown fields.
5. Add metadata to the 49 canonical sources without yet rewriting their bodies.
6. Generate the placement, authority, effect, output, trigger, and proof
   coverage matrices.
7. Rewrite bodies and local tests in later semantic tranches; compiler metadata
   may not claim behavioral proof until its named probe passes.

Dependencies: Finalist 3 should provide the publisher used by the compiler
before whole-catalog regeneration. The CLI capability substrate in Finalist 4
must consume the same effect/output vocabulary.

### 6. Proof / acceptance criteria

- Every source appears exactly once with one primary layer and at least one
  lifecycle seam.
- Every skill has one structured positive trigger, one negative boundary, one
  output, one failure contract, and one proof class.
- The compiler rejects a runtime adapter that claims `write_verdict`, a strategy
  that claims `select_experiment`, a mutating specialist authorized outside
  Implement/caller scope, and a binding output without validation.
- Go and Python decode the same golden catalog corpus and reject stale versions,
  count mismatch, missing required fields, duplicates, and unknown fields.
- Generated bounded-context membership equals source metadata; no handwritten
  skill list can drift.
- A proof report distinguishes “contract structurally valid,” “behavior probe
  passed,” and “platform unavailable/not checked.”

### 7. Failure modes and rollback / containment

- **Metadata theater:** all fields populated with generic values. Prevent with
  enum-specific negative fixtures and per-skill probe linkage.
- **Schema overfitting:** the schema encodes one runtime topology. Keep semantic
  authority separate from execution shape.
- **Migration lockout:** strict v3 blocks incremental work. Use an explicit
  migration mode over the frozen 49-entry ledger, but make full-catalog v3 a
  blocking exit criterion; never silently fall back to v1.
- **Unknown-field data loss:** older Go readers ignore new fields. Fail closed
  on schema version and unknown fields before publishing v3.
- Rollback preserves v2 sources and v3 work in a single tranche revert; never
  emit a v3 catalog from partially migrated sources.

### 8. Skills and CLI findings affected

All 49 skills. The largest semantic changes are `craft-goal`, `rpi`, `plan`,
`implement`, `validate`, `automation-shape-routing`, `security`,
`skill-builder`, every runtime adapter, and every mutating support skill.

CLI: generated skill catalog/graph consumers and the audit opportunity that 74
of 84 command nodes lack rich args/effects/exit contracts. It provides the
shared vocabulary for global dry-run and output policy but does not implement
those policies.

### 9. Confidence and falsifier

**Confidence: 96%.** It would be falsified if the existing v1/v2 schemas,
generator, Go catalog model, and gates already made authority/effects/output/
trigger/proof decisions unambiguous and rejected contradictions. They do not.

## Finalist 3 — Replace “serialize writes” with a projection publication transaction

### 1. Precise claim

All skill-derived projections must be produced by one locked publisher from one
frozen source snapshot. Worker lanes may edit canonical sources in isolated
worktrees but may not run projection writers. The publisher stages the complete
output set, validates it, publishes files by atomic replacement under one
generation ID, and writes a digest manifest last as the commit marker.

### 2. Defect or ambiguity resolved

Plan D3 correctly observes shared generated surfaces but treats serialization as
a scheduling convention. The live commands independently delete and rewrite
directories, twins, catalogs, hashes, images, and docs. A crash or accidental
parallel invocation can leave a mixture of generations. “Regenerate once”
does not prove the once was exclusive, complete, or all-or-nothing.

### 3. Evidence

- plan D3 and T7
- `scripts/generate-skill-mesh.py:249-325`
- `scripts/codex-sync.sh:437-686`
- `scripts/regen-codex-hashes.sh:172-178`
- `skills/skill-builder/scripts/build.sh:31-40`
- `skills/skill-builder/scripts/heal.sh`, fix mode
- `scripts/regen-all.sh`, whose step wrapper records failure and continues,
  allowing later generators to run after an earlier publication failure
- Go audit improvement: `cli/internal/evalsubstrate/atomic.go` uses a fixed temp
  name, another example of one-writer assumptions leaking into persistence

### 4. Target architecture and ownership boundary

`skill-builder` owns canonical skill package authoring. A new internal
projection publisher owns only derived surfaces:

```text
source snapshot digest
  -> acquire repository generation lock
  -> compile catalog in temp staging root
  -> render every projection/twin/image/doc into staging
  -> run schema, parity, link, and hash checks against staging
  -> compare expected output manifest to current tree
  -> atomic-replace each file / contained directory
  -> write generation-manifest.json last
  -> fsync destination directories
  -> release lock
```

The final manifest binds source digest, generator version, generation ID, and
every output path/digest. Consumers and `--check` reject a mixed generation.
Multi-file publication cannot be literally atomic across the whole tree, so the
last-written manifest is the transaction commit marker; absence/mismatch is
explicit drift, never success.

### 5. Migration steps and dependencies

1. Inventory every output of mesh generation, Codex sync/hash, Gemini image,
   GC projection, docs index, graph, CLI reference, and command surfaces.
2. Add a generated-owner map and fail on two owners for one path.
3. Extract render-to-directory modes from current writers; forbid direct
   in-repo writes in render mode.
4. Implement lock, staging, validation, publication, manifest, and crash
   cleanup.
5. Route `skill-builder build/fix`, `regen-all`, and changed-scope regeneration
   through the publisher.
6. Make worker instructions explicit: isolated worktree, canonical source only,
   no generated files; the integration lane merges sources then publishes once.
7. Add kill-at-each-publication-step tests and concurrent-invocation tests.

Dependencies: the output inventory should include Finalist 2's v3 catalog.
This should land before mass source metadata editing.

### 6. Proof / acceptance criteria

- Two concurrent publishers result in one waiting/refusing, never interleaved
  output.
- A forced crash after every replacement step leaves either the previous valid
  generation or a manifest mismatch that all checks report; it never reports
  current.
- `--check` and write mode use the same renderer and expected manifest.
- A second identical publication is byte-idempotent.
- An untracked stale generated file is removed only if the owner map says the
  publisher owns it.
- Worker-lane source edits can be merged without generated conflicts; one final
  publication produces zero drift on rerun.

### 7. Failure modes and rollback / containment

- **Over-broad deletion:** an owner map bug removes caller files. Stage and
  diff first; deletion is allowed only for previously manifested paths under
  explicit generated roots.
- **Stale lock:** record PID/start/generation and support read-only diagnosis;
  break only when process identity is proven dead.
- **Manifest-last crash:** outputs may be new with an old manifest, but checks
  fail closed and rerunning the publisher converges.
- **Bespoke Codex overwrite:** preserve explicit bespoke/excluded policy and
  test that staged generation never rewrites bespoke sources.
- Rollback restores the previous generation from the staging/backup manifest or
  re-renders it from the recorded source digest; product sources are untouched.

### 8. Skills and CLI findings affected

Direct: `skill-builder`, `converter`, `doc`, `bootstrap`, `shared` disposition,
and every skill because all have generated twins/catalog rows.
CLI: `ao skills` consumers, generated CLI docs, fixed-temp atomic-write audit
opportunity. It does not replace the CLI subprocess or dry-run fixes.

### 9. Confidence and falsifier

**Confidence: 95%.** It would be falsified if a repository-wide lock,
staged-complete render, crash-safe commit marker, and ownership manifest already
covered all the cited writers. No such mechanism appears in the live sources.

## Finalist 4 — Make the Go CLI audit a prerequisite substrate stage

### 1. Precise claim

The verified CLI audit findings belong in a prerequisite substrate stage after
baseline freeze and before the skill-contract and behavioral migration—not as a
separate program and not scattered opportunistically through T1/T6.

This stage contains three coherent RPI tranches:

1. containment and owned temporary resources;
2. command effect/output policy;
3. bounded subprocess lifecycle.

Cosmetic `-V` polish may remain outside; the verified safety and contract
findings may not.

### 2. Defect or ambiguity resolved

The migration intends to prove effect honesty, dry-run behavior, unavailable
runtime behavior, cleanup, bounded output, structured output, and CLI catalog
consumption. The current CLI violates or cannot describe those same properties.
Running skill behavioral probes on that substrate can mutate during dry-run,
escape eval roots, exhaust memory, orphan descendants, leak isolated homes, or
emit successful human text to a JSON caller.

Treating the audit as separate would let the skill overhaul declare success on
top of a known-unreliable proof runner. Folding findings into arbitrary skill
tranches would duplicate policy and make completion order-dependent.

### 3. Evidence

- eval traversal: `cli/internal/eval/task_service.go:64-84,188-198`,
  `suite_service.go:87-100`, `evalsubstrate/modelspec.go:12-20`
- dry-run: `cli/cmd/ao/root.go:135`,
  `commands/provenance/module.go:175`,
  `commands/gate/module.go:19-24,100-118`
- output fragmentation: root `--json` promise at `root.go:137-138`, local
  booleans throughout `commands/skills/module.go` and
  `commands/provenance/module.go`, eval's conflicting `--output`
- unbounded buffers: `goals/measure.go:159`, `gates/scriptrunner.go:136`,
  `eval/engine.go:224-225`, `eval/runtime.go:435-436`,
  `eval/expectations.go:527`
- lost context/process trees: `eval/core_service.go`, `eval/engine.go:203`,
  `eval/expectations.go:512`
- leaked isolation root: `eval/runtime.go:335`
- capability incompleteness: live 84/74/77/74/74 projection and
  `cli/internal/clicontract`
- audit stale-help finding and fixed-temp improvement

### 4. Target architecture and ownership boundary

**Containment tranche**

- one validated identifier type for task/suite/run/model-spec IDs;
- root-relative operations using `os.OpenRoot` or an equivalent
  descriptor-rooted API;
- symlink-parent, Windows-volume, separator, control-character, and traversal
  rejection;
- an owned-resource object for internally created runtime homes with guaranteed
  success/error/timeout cleanup;
- unique same-directory atomic temporaries.

**Command policy tranche**

- extend `CommandContract` with supported formats, effect classes, dry-run
  policy, and output destination semantics;
- root preflight rejects `--dry-run` on an effectful command that has neither a
  no-effect preview nor explicit unsupported rejection;
- one inherited output enum; local JSON flags become aliases into it;
- file outputs use unambiguous names such as `--out`/`--output-file`;
- every advertised read leaf has JSON-equivalence tests;
- command paths mentioned in help are executed in a reference test.

**Process tranche**

- one shared runner accepts caller context, deadline, cwd/env, bounded
  stdout/stderr policy, and cleanup mode;
- bounded ring/tail capture or capped spool with truncation telemetry;
- process-group/process-tree termination and `WaitDelay`;
- orphan-grandchild tests;
- goals, gate, deterministic eval, auto-detect, and live runtime consume it.

The CLI remains a repository tool, not an RPI lifecycle authority. These changes
make it safe and truthful; they do not let `ao gate` emit verdicts.

### 5. Migration steps and dependencies

1. Preserve audit reproducers as RED tests.
2. Land containment/owned-temp fixes first so later tests cannot escape or leak.
3. Land command policy and migrate dry-run/output behavior leaf by leaf,
   rejecting unsupported combinations during transition.
4. Land the process runner and migrate each call site with kill/noise tests.
5. Attach rich contracts to every command touched by the skill overhaul,
   then ratchet remaining unknown command nodes.
6. Re-run full Go tests, race, vet, lint, vuln, generated docs, and CLI
   equivalence matrix.

Dependencies: none on rewritten skill bodies. Finalist 2 should reuse the
effect/output vocabulary after this stage.

### 6. Proof / acceptance criteria

- traversal and symlink-parent attempts cannot read or write outside eval roots.
- every effectful leaf either performs zero effects under `--dry-run` or rejects
  the flag before any effect.
- `--json` and `-o json` produce equivalent valid JSON for every advertised read
  leaf; unsupported formats fail nonzero.
- a noisy child cannot make peak memory scale with its output.
- caller cancellation and timeout kill a child plus grandchild holding a pipe.
- internally owned isolation roots are removed after success, error, timeout,
  and partial setup.
- capability output has explicit args/output/effects/dry-run/exit metadata for
  all touched leaves and no false claims.

### 7. Failure modes and rollback / containment

- **Global-policy regression:** central preflight blocks previously tolerated
  combinations. Publish explicit errors and migrate tests before removing local
  flags.
- **Cross-platform kill failure:** keep platform-specific implementations behind
  one interface; Windows/Linux are explicit not-checked until their tests run.
- **Output compatibility break:** retain deprecated local aliases for one
  release while routing them to one value; never keep two renderers.
- **Runner semantic drift:** compare exit, tail, cancellation, and timeout
  results at each migrated call site.
- Roll back by call-site tranche; the shared runner remains unused until a
  consumer's parity tests pass.

### 8. Skills and CLI findings affected

Runtime/support: `account-rotation`, `agent-mail`, `agent-native`, `agy-native`,
`automation-shape-routing`, `bootstrap`, `cc-hooks`, `codex-exec`, `dcg`,
`handoff`, `ms`, `ntm`, `rch`, `sbh`, `status`, `swarm`, `using-gc`.
Also `goals`/`fitness`, `security`, `test`, `workflow-builder`,
`skill-builder`, and `validate` as CLI/process consumers.

It addresses every verified CLI audit finding and the command-contract,
fixed-temp, helper-main, and module-decomposition opportunities insofar as they
are necessary to land the substrate. Gate parallelism and `-V` are excluded.

### 9. Confidence and falsifier

**Confidence: 97%.** It would be falsified if the migration could prove effects,
dry-run, output, timeouts, cleanup, and CLI catalog semantics without exercising
the known affected CLI paths, or if the audited defects had already been fixed.
The live source and capability probe show they remain.

## Finalist 5 — Recut the portfolio and tranches around authority and proof shape

### 1. Precise claim

Retain most of the 49 capabilities, but require a one-sentence specialization
proof and an executable probe for each. Make four portfolio decisions now:

- rename `goals` to `fitness`;
- move `automation-shape-routing` to advisory judgment/support rather than
  runtime;
- move `security` to evidence/validation-evidence migration;
- retire the empty `shared` skill.

Conditionally retain tool-specific runtime adapters only if their distinct
unavailable/timeout/cleanup probe passes or is explicitly recorded as a
platform gap. Recut migration tranches so one RPI has one proof shape and one
architectural decision.

### 2. Defect or ambiguity resolved

The current plan has a good per-skill matrix but still groups:

- campaign/product and the evidence kernel in T2;
- read-only evidence, plan review, multi-judge strategy, post-verdict causality,
  and external reverse engineering in T3;
- Security with candidate-producing specialists in T4;
- 18 runtime and support skills with very different mutation/cleanup risks in
  T6.

It also keeps “goals” ambiguous, locates a pure router in runtime, and postpones
the empty shared root decision. The universal 250-line cap measures formatting,
not always-loaded cognitive cost.

### 3. Evidence

- the complete 49-skill matrix in the current plan
- `skills/goals/SKILL.md`: read-only project fitness
- `skills/craft-goal/SKILL.md`: actual persistent Goal campaign compiler/linter
- `skills/automation-shape-routing/SKILL.md`: “routes; it does not build or start
  a substrate”
- `skills/security/SKILL.md`: collection read-only by default; evidence output
- `skills/shared/`: only `SKILL.md`, no reference payload; its only apparent
  artifact-flow consumer is `bootstrap` frontmatter
- plan T2–T6 and the 250-line acceptance rule
- all runtime skills' distinct named hazards and tool-specific outputs

### 4. Target architecture and ownership boundary

Portfolio rule:

> A skill deserves a root only if it has a unique trigger branch, a distinct
> authority/effect boundary, a stable handoff contract, and at least one
> behavioral probe that would fail if it were merely a section of its nearest
> neighbor.

This preserves distinct evidence claims:

- Recon traces a codebase; Research answers a bounded question; Reverse
  Engineer produces an authorized external-system inventory/adoption
  recommendation.
- Premortem challenges a frozen plan; Council aggregates independent
  perspectives; Reality Check compares a claim to current evidence.
- Learn mines verdict recurrence; Postmortem tests causality; Toil Mining
  measures repeated friction; Pattern Mining earns an abstraction;
  Operationalize proposes a reusable shape.
- Agent Native coordinates explicit roles; NTM is a pane adapter; Swarm is an
  at-most-once dispatch port; Agent Mail carries coordination; Gas City is an
  explicitly selected factory adapter.

None of those distinctions imply hard dependencies.

Replace the line cap with:

- a measured always-loaded token/context budget;
- required progressive-disclosure routing;
- maximum kernel size as a default warning;
- a tested exception justified by trigger or safety latency.

### 5. Migration steps and dependencies

After the prerequisite and compiler stages, use this order:

| Stage | One architectural decision / proof shape | Skills |
|---|---|---|
| T0 | freeze source and evidence baseline | inventory and audit snapshot |
| T-1A | CLI containment and resource ownership | Go CLI |
| T-1B | CLI command effect/output policy | Go CLI |
| T-1C | CLI subprocess lifecycle | Go CLI |
| T1 | skill-contract compiler and projection publisher | `skill-builder`, schemas, generators, strict Go catalog |
| T2A | exact experiment transaction | `rpi`, `plan`, `implement`, `validate` |
| T2B | advisory pre-build scope/review contracts | `scope`, `premortem` |
| T2C | product/campaign admission and policy | `product`, `fitness` (renamed), `craft-goal`, `automation-shape-routing` |
| T3A | read-only intent/validation evidence | `cass`, `codebase-recon`, `domain`, `research`, `standards`, `security` |
| T3B | advisory option/judgment strategies | `council`, `idea-genie`, `reality-check`, `reverse-engineer` |
| T3C | post-verdict causal analysis | `postmortem` |
| T4A | code/test structural specialists | `refactor`, `test` |
| T4B | generators and clean-write specialists | `converter`, `doc`, `scaffold`, `workflow-builder` |
| T5 | capability evolution | `learn`, `operationalize`, `pattern-mining`, `toil-mining` |
| T6A | pure/one-shot runtime transport | `codex-exec`, `agy-native`, `ntm`, `rch` |
| T6B | multi-actor runtime transport | `agent-native`, `agent-mail`, `swarm`, `using-gc` |
| T6C | host/environment support mutations | `account-rotation`, `bootstrap`, `cc-hooks`, `dcg`, `handoff`, `ms`, `sbh`, `status`; retire `shared` |
| T7 | full-catalog convergence from one frozen source generation | all retained/renamed skills |

Split a row further if its RED fixtures reveal a separate authority question.
The table is an upper bound, not permission to force unrelated changes into one
RPI.

### 6. Proof / acceptance criteria

- Every current skill has one disposition and one specialization proof in the
  appendix below.
- `fitness` cannot be routed to persistent Goal control; `craft-goal` cannot
  mutate goals or run an RPI.
- Automation routing outputs one advisory route and starts no runtime.
- Security's read-only scan and any explicitly mutating baseline/policy mode
  have separate effect/authorization fixtures.
- `shared` has zero canonical root after consumers point to owned references.
- Every retained runtime-specific skill has unavailable, timeout, bounded
  output, and abnormal-cleanup results; unavailable platforms are labeled, not
  passed.
- No tranche mixes campaign selection, verdict authority, subject mutation, and
  host mutation.
- Kernel budget reports loaded bytes/tokens and reference routing, not only
  source lines.

### 7. Failure modes and rollback / containment

- **Over-consolidation:** removing a root hides a distinct hazard. The
  specialization/probe rule requires a nearest-neighbor comparison before
  retirement.
- **Rename breakage:** keep `goals` as a non-advertised compatibility alias for
  one bounded window; generated routers advertise only `fitness`.
- **Tranche explosion:** too many RPIs recreate 49-file ceremony. Group only
  where one contract/proof harness truly applies; integration proof remains one
  per coherent candidate.
- **Unavailable-runtime limbo:** retain source but mark behavior unproven rather
  than manufacturing PASS; the final completion report names the platform gap.
- Rollback is per semantic tranche. Renames preserve alias compatibility;
  retirement of `shared` occurs only after reference and generated-catalog
  checks show no consumer.

### 8. Skills and CLI findings affected

All 49 skills; specific disposition is in the appendix.
CLI: all audited findings indirectly affect proof shape; direct portfolio
touchpoints are `ao goals`, `ao skills`, `ao gate`, `ao status`, eval/runtime
processes, and generated command docs.

### 9. Confidence and falsifier

**Confidence: 91%.** The exact grouping is the most revisable finalist. It
would be falsified by RED fixtures showing two listed skills share neither a
contract nor proof harness, or by live usage evidence showing `shared` carries a
real payload/consumer or that renaming `goals` would cause unacceptable
compatibility cost.

## Tranche-ordering critique

The present sequence gets three things right: baseline first, factory before
mass edits, and full convergence last. Its central ordering error is T2:
campaign policy is built at the same time as the transaction it must consume.
That makes campaign tests capable of passing against a mocked or semantically
incompatible RPI. The live digest mismatch demonstrates why the kernel needs its
own tranche.

T3 is not one proof shape. `cass`/Research/Recon are evidence readers;
Premortem/Scope are pre-build challenges; Council is multi-perspective
judgment; Postmortem is post-verdict causal inference; Reverse Engineer has
authorization and external clone/write surfaces. A single integration verdict
could say the files cohere, but it would not falsify each authority boundary.

T4 misclassifies Security. Its normal collection is read-only and it supplies
validation evidence. Only explicit baseline/policy/suppression modes mutate.
Treating it as a candidate producer invites a validator to authorize security
policy edits under Implement without separate caller judgment.

T6 is too large and too heterogeneous. Account switching, hook installation,
storage recovery, read-only status, pane transport, multi-writer reservations,
and Gas City operation do not share one acceptance surface. Grouping them
creates exactly the “broad gate green” completion illusion the plan warns
against.

Finally, line count should not dictate tranche order. `cass` and `cc-hooks` are
large because they include volatile runbooks; the architectural issue is how
much text loads for a trigger and which behavior is executable. Measured context
cost and progressive disclosure are the useful constraints.

## Generated-write serialization ruling

**Keep the direction, change the mechanism.**

Keep:

- workers edit disjoint canonical sources only;
- shared projection surfaces serialize;
- generation happens after a tranche source set freezes;
- generated files are never hand-authored architecture.

Change:

- isolated worktrees are mandatory for concurrent source lanes;
- worker lanes never generate;
- one integration lane owns the publisher;
- one lock covers mesh, twins, hashes, images, catalogs, docs, and graph;
- render/check share the same staged compiler;
- validation happens before publication;
- a source/output digest manifest is written last;
- crash and concurrent-publication tests are blocking.

Reject:

- “run these generators in order” as sufficient serialization;
- direct deletion/rewrite of multiple shared roots without a generation commit
  marker;
- continuing later publication steps after an earlier generator fails;
- per-skill generated commits during a multi-skill tranche.

## CLI audit placement ruling

**The CLI audit is a prerequisite substrate stage.**

It is not a separate program because skill migration acceptance explicitly
depends on CLI effect truthfulness, dry-run safety, machine output, bounded
processes, cleanup, catalog consumption, and gates. It does not belong inside
later skill tranches because those tranches should consume a stable substrate,
not each invent local fixes.

The substrate stage must still obey one-experiment boundaries: containment/
resource ownership, command policy/output, and subprocess lifecycle are three
coherent RPIs, not one mega-patch. Cosmetic `-V`, optional gate parallelism, and
non-blocking module decomposition can remain separate.

## Three most dangerous false-completion modes

1. **The catalog says 49/49 while authority remains prose.** Both mesh checks
   pass today, yet strict frontmatter and the orchestration boundary gate fail,
   and the live schemas cannot reject a runtime adapter claiming semantic
   authority. A generated placement matrix can be perfectly complete and
   architecturally false.
2. **Every projection is byte-current but belongs to a mixed generation.**
   Direct multi-root writers, deletion, separate hash passes, and no shared lock
   can leave catalogs, twins, images, and docs individually well-formed but
   mutually inconsistent. A later hash pass can seal the wrong mixture.
3. **Every local unit test passes while the actual evidence handoff is
   impossible.** RPI tests use their own mapping digest; Validate tests use
   exact bytes; both pass. A real composed invocation can reject the intent
   continuity it was designed to prove. Similar false confidence would arise
   from CLI dry-run tests on supported families while unsupported effectful
   leaves still mutate.

## Explicit rulings on the current plan

| Plan element | Ruling | Reason |
|---|---|---|
| Goal outside RPI; RPI owns one experiment | **KEEP** | Correct semantic control split. |
| Only hard edge `rpi -> {plan, implement, validate}` | **KEEP** | Prevents optional strategies/adapters becoming core. |
| Layer and lifecycle seam are orthogonal | **KEEP, EXTEND** | Add typed authority, effects, output, failure, trigger, and proof. |
| One RPI per coherent migration tranche | **KEEP** | Correct; apply it more strictly to T2/T3/T6. |
| D4 proof by skill shape | **KEEP, EXTEND** | Link every source to an executable probe and distinguish structural from behavioral/platform coverage. |
| Catalog-wide effect/output/trigger/failure honesty | **KEEP** | These are strong acceptance dimensions. |
| T0 baseline and T7 convergence | **KEEP** | Necessary bookends. |
| T1 factory before mass skill editing | **KEEP, CHANGE** | Use one v3 compiler and transactional publisher; include strict Go consumers. |
| T2 campaign plus core experiment | **CHANGE** | Split exact transaction before campaign/product policy. |
| T3 evidence plus all optional judgment | **CHANGE** | Split by read-only evidence, pre-build review, option/judgment, and post-verdict causality. |
| T4 Security as candidate-producing specialist | **REJECT** | Security is evidence by default; explicit mutating modes need separate authority. |
| T5 capability evolution | **KEEP** | The four skills are distinct and correctly outside core. |
| T6 one runtime/support tranche | **REJECT** | Too heterogeneous for one bounded acceptance surface. |
| Generated writes merely “serialize” | **CHANGE** | Require lock, staging, manifest-last publication, and crash proof. |
| Kernels <=250 lines as catalog acceptance | **REJECT** | Replace with measured always-loaded context budget and progressive disclosure. |
| `goals` merely “prefer fitness terminology” | **CHANGE** | Rename the skill/surface with a bounded compatibility alias. |
| `shared` populate-or-retire late | **CHANGE** | Retire in the compiler tranche; no payload exists to justify a root. |
| CLI work only when placement fields cross boundary | **REJECT** | Known CLI safety/capability defects must be a prerequisite substrate stage. |
| Full completion may disclose unavailable platforms | **KEEP, CLARIFY** | Disclose as declared exclusions/platform gaps, not PASS-required unchecked scope. |

## Appendix A — one-row disposition for all 49 canonical skills

“Never own” is the semantic prohibition that matters even if a runtime or mode
can perform local effects.

| Skill | Disposition / primary placement | Why it exists as a skill rather than a section | Consumes -> produces; mutation boundary | Never own | Observable specialization proof |
|---|---|---|---|---|---|
| `account-rotation` | **KEEP**, runtime; cross-cutting/standalone | Credential switching has host/runtime-specific identity and stale-process hazards unlike general dispatch. | selected host/agent/profile -> observed identity/status; mutates credentials only with explicit caller authority | session restart, work continuation, tracker, verdict | fixture matrix discovers tool by host/agent, switches once, verifies identity in a new matching runtime, reports unavailable/stale process |
| `agent-mail` | **KEEP**, runtime transport | Message/reservation TTL, conflict, identity, and ACK semantics are not pane dispatch. | explicit identities/paths/thread/TTL -> message, reservation, ACK facts; external coordination mutations | work selection, lease/ownership semantics, completion | conflict/TTL/path-identity/degraded-surface fixtures plus proof single-writer route skips the adapter |
| `agent-native` | **CHANGE**, runtime transport | Role packet coordination and intervention ordering span replaceable runtimes; this is the coordinator contract. | explicit role packets -> runtime evidence/handoffs; manages selected sessions | “orchestrator” lifecycle authority, experiment selection, verdict | packet deadline/output/engagement/replacement/cleanup scenarios across at least two adapters or one adapter plus fake |
| `agy-native` | **CONDITIONAL KEEP**, runtime transport | AGY has live-capability discovery and session semantics distinct from Codex; otherwise it should be an `agent-native` profile. | explicit packet -> AGY run evidence; starts bounded AGY session | install/plugin mutation, retry, semantic result | black-box unavailable/timeout/output/abnormal-cleanup probe; merge into `agent-native` if no distinct probe can be maintained |
| `automation-shape-routing` | **CHANGE**, advisory judgment/support; standalone/cross-cutting | It decides semantic work shape before topology; no executor should contain that admission table. | task intent -> one advisory route line; no mutation | substrate startup, experiment selection, campaign control | exhaustive matrix for analysis vs one RPI vs Goal vs automation and for inline/fanout/persistent/factory topology |
| `bootstrap` | **KEEP**, support | Minimal never-overwrite initialization is a user-facing environment mutation with a unique idempotence contract. | requested missing docs/storage -> created/skipped paths; contained filesystem writes | Git, hooks, trackers, runtimes, RPI | disposable-tree byte-idempotence, refusal to overwrite, and no Git/tracker/runtime mutation |
| `cass` | **CHANGE**, evidence; plan input/post-verdict | Session archaeology has index freshness, source preservation, and provenance/recency behavior not covered by generic Research. | bounded query/corpus -> cited session hits/absence; derived index refresh only | source-session deletion, plan selection, semantic proof | direct/adjacent/verified-absence fixtures, stale-vs-broken behavior, bounded timeout, citation replay |
| `cc-hooks` | **KEEP**, support | Hook event/exit/output/recursion/config semantics are a distinct opt-in host-policy surface. | explicit hook policy -> installed/configured hook report; mutates settings/scripts only with authority | default installation, campaign/RPI enforcement, silent policy promotion | event-schema tests, silent happy path, fire/near-miss, recursion guard, safe removal, declared telemetry effects |
| `codebase-recon` | **KEEP**, evidence; plan input/validation evidence | Reusable entry-to-test repository model and baseline/delta identity are broader than one research answer. | repo/prior pack -> `codebase-recon.v1` + cited report; artifact writes only | verdict, completion decision, code plan | file:line citation resolution, complete lens flow or cut point, baseline/delta identity, inspected/uninspected coverage |
| `codex-exec` | **CHANGE**, runtime transport | One-shot Codex process invocation has sandbox/login/stdin/output/process-tree semantics distinct from a coordinator. | supplied prompt/workspace/sandbox -> structured process result; starts one process | work choice, retry, validation by itself | unavailable/login failure, closed stdin, deadline, bounded output, child cleanup, read-only sandbox fixture |
| `converter` | **KEEP**, implementation method | Cross-runtime parse/normalize/render/resource-parity is a real transformation pipeline. | canonical skill bundle -> target projection; clean-writes only explicit target | editing canonical source or generated projection as source, lifecycle control | disposable output containment, modular/inline cases, resource parity, stale-file removal, source unchanged |
| `council` | **KEEP**, judgment strategy | Independent multi-method perspective synthesis and dissent preservation differ from a single accountable validator. | question/evidence/digests -> advisory `council-report.v1`; report write | verdict, majority readiness, subject edits | distinct contexts/methods, echo-consensus downgrade, dissent preservation, missing-evidence and unsatisfied-diversity cases |
| `craft-goal` | **CHANGE**, campaign goal design | It is the compiler/linter for finite adaptive campaign policy, not the campaign runtime or RPI. | caller outcome/terminal evidence/authority -> goal contract or unsafe/use-RPI result; no campaign creation | running Goal, mutating graph, rewriting verdicts | single-experiment admission, bounded campaign, unsafe indefinite automation, monotonic ceiling, ratchet/breaker/terminal-report schema |
| `dcg` | **KEEP**, support | Destructive-command explanation and reversible-alternative routing has a human-only override boundary. | blocked command/rule -> risk and safe alternative; optional explicit config write | bypass generation/execution, approval laundering | discovered live syntax, rule explanation, reversible alternative, no-alternative human handoff, config safe-repeat |
| `doc` | **KEEP**, implementation method | Documentation generation has source-grounding, mode-specific writes, and conceptual-coverage proof unlike generic editing. | repo/mode -> docs + report; declared documentation writes | invented claims, product selection, verdict | project detection, mode fixtures, claim-to-source checks, no-overwrite OSS default, README prose/check pass |
| `domain` | **KEEP**, evidence/cross-cutting | Exact ubiquitous-language lookup protects authority-bearing terms from synonym drift. | term/context question -> cited definition/conflict/unknown; read-only | vocabulary promotion, lifecycle decisions | known/unknown/conflicting term fixtures resolving exact contract path/line |
| `goals` | **RENAME TO `fitness`**, product input/observation | Read-only product fitness measurement is distinct from persistent Goal campaign control. | goal document/selected measure -> stable measurement report; read-only except explicit export artifact | recommendations, work selection, Goal campaign | command/schema fixtures prove measurement-only behavior and reject campaign-control triggers; bounded `goals` alias window |
| `handoff` | **KEEP**, support/observation | Compact end-state evidence with digest/naming/read-without-consume semantics is a distinct write artifact. | caller-authored facts -> handoff artifact; contained write | inferred next action, ownership, status/verdict classification | schema/digest/naming fixture, missing-evidence rejection, repeat read leaves bytes unchanged |
| `idea-genie` | **KEEP**, judgment/option shaping | Evidenced option elicitation and sealed adversarial challenge share an idea portfolio contract not owned by Research or Council. | question/repo/portfolio -> portfolio or challenge; advisory artifact write | selection, readiness, scheduling, validation | elicit/duel schema parity, sealed contexts, novelty saturation, empty no-new-work, dissent/refutation retention |
| `implement` | **CHANGE**, core experiment | It uniquely owns one RED-to-GREEN subject mutation and factual evidence handoff. | exact intent snapshot/baseline -> final manifest/paths/receipts; subject mutation in scope | repair revision, validation, retry, Git/delivery | right-reason RED or honest baseline, one subject, scope-conflict stop, no placeholders, runtime-derived handoff |
| `learn` | **KEEP**, evolution/post-verdict | Cross-verdict recurrence analysis differs from single-event causality and skill promotion. | verdict collection -> advisory observations; report write | altering verdicts, on-path control, promotion | objective dedupe, sample/provenance/decay fields, failure overweight, dead-citation pruning, no-promotion |
| `ms` | **CHANGE**, support/cross-cutting | Skill retrieval/index administration has a unique canonical-source and stale-server hazard. | query or explicit admin request -> load/search/admin facts; index mutation only in admin mode | downstream authoring/validation, index as source | read/write mode split, MCP/CLI parity boundary, bounded helper cleanup, reindex source equivalence and stale-server reap |
| `ntm` | **CHANGE**, runtime transport | Persistent pane observation and truth-stack evidence are distinct from role coordination and work dispatch. | named session/pane/command/window -> robot/transcript/result facts; selected pane mutations | work selection, retries, semantic state | capability discovery, exactly-one send, artifact/transcript/robot liveness ordering, deadline, nudge/restart cleanup |
| `operationalize` | **CHANGE**, evolution/proposal | Packaging repeated expertise into the smallest reusable artifact follows evidence thresholds not owned by builders. | cited expertise -> `operationalization-proposal.v1`; advisory write | creating/promoting skill/gate/work, starting RPI | three-instance or authoritative-source rule, quote anchors, reapply proof, overlap search, deletion condition |
| `pattern-mining` | **KEEP**, evolution | Independent exemplars, holdout, and back-application establish earned abstraction, unlike general Research. | repo/question -> `pattern-mining.v1`; advisory artifact | self-promotion or packaging | reproducible search ledger, three independent exemplars, unseen holdout, back-apply, hypothesis no-action path |
| `plan` | **CHANGE**, core experiment | It uniquely refines one caller-owned intent and freezes acceptance/scope without a duplicate packet. | caller source/evidence -> in-place amendment or proposal + exact snapshot; authorized intent-source write | campaign graph, scheduling, second plan artifact | cold-context fixture, scope-class/generated companion admission, exact-byte snapshot, one amendment, no decomposition scheduling |
| `postmortem` | **KEEP**, judgment/post-verdict | Retrospective causal discrimination is not recurrence learning or acceptance validation. | verdict/evidence/causal question -> causal report; report write | rewriting proof, promotion, continuation | mechanism + discriminating evidence + counterfactual per supported cause; correlation/unknown paths remain visible |
| `premortem` | **CHANGE**, judgment/plan review | One fresh pre-build defeat attempt is distinct from Plan authoring and Council synthesis. | frozen intent/digest -> advisory review; report write | readiness, in-place repair, implementation | distinct judge, constructed/blocked defeat per finding, checked/not-checked, material amendment causes RPI pre-build stop |
| `product` | **KEEP**, product input | PRODUCT.md has user authority, preservation, evidence/aspiration separation, and stable non-goals. | product evidence/user decisions -> PRODUCT.md; authorized section writes | work selection, self-validation, delivery | preserve unselected sections byte-for-byte, label facts/aspirations/gaps, validate template |
| `rch` | **CHANGE**, runtime/implementation support | Remote compile classification and staged diagnostics have fail-open/local-fallback semantics unlike generic execution. | explicit build/diagnosis -> remote/local-fallback/failed facts; bounded remote process | semantic NOT_PROVEN verdict, retries, remote mutation without authority | capability, first-failing-stage, local-fallback classification, deadline/output/cleanup, no semantic verdict words |
| `reality-check` | **KEEP**, judgment/observation/plan input | Full vision-to-evidence coverage catches unbuilt goals and ambition drift, unlike generic status. | explicit claim/goals/repo -> advisory gap report; report write | creating work, planning, validation | one disposition per claimed goal, frozen-question repeats, built-world-bias and ambition-escalation fixtures |
| `refactor` | **CHANGE**, implementation method | Behavior-preserving structural change and neutrality gates are distinct from feature implementation. | preserved behavior/subject -> structural change + evidence; code writes under Implement/caller authority | behavior fixes, retry, semantic validation | pre/post identical tests/outputs/errors, isolated seam probes restored, failed-neutrality terminal result |
| `research` | **CHANGE**, evidence/plan input | One bounded current cited answer and multi-report synthesis are general evidence capabilities. | question/sources -> findings/report; optional report write, otherwise read-only | work selection, approval, recursive research control | capability flags, commit+file:line citations, observation/inference/conflict/unknown, checked scope, no next action |
| `reverse-engineer` | **CHANGE**, evidence/plan input | Authorized external-system inventory, reproducibility, and adoption mapping have legal/IP and lineage requirements beyond Research. | authorized repo/binary -> inventory/registry/specs/adoption recommendation; contained clone/artifact writes | adoption decision, proprietary reconstruction, implementation | authorization refusal, pinned SHA, registry lineage, secret scan, have/gap/adopt/park/reject evidence; one-way door routes to Plan |
| `rpi` | **CHANGE**, core experiment | It is the sole one-pass dispatcher/report boundary joining the three core phases. | exact intent source -> report/verdict ref or pre-verdict stop; dispatch effects only | campaign continuation, repair, budgets, queues, delivery | composed exact-byte transaction, each phase <=1, all terminal outcomes, no next action; delete continuation envelope |
| `sbh` | **KEEP**, support/standalone | Disk-pressure identity, protection vetoes, and irreversible recovery ordering are device-specific host operations. | exact mount/action authority -> before/after status; at most one authorized host mutation | automatic escalation/retry, wrong-mount success | device/volume binding, dry-run, protection vetoes, before/after bytes, one-action limit |
| `scaffold` | **CHANGE**, implementation method | Exemplar-based bounded generation, overwrite refusal, and one-way sync are a coherent creation capability. | target/mode/exemplar -> files/changed manifest/checks; contained filesystem writes | Git, work ownership, lifecycle state | mode fixtures, overwrite refusal, changed paths, build/test/lint, no twin-edit trap, safe repeat |
| `scope` | **CHANGE**, judgment/plan review | Axiom-to-pattern scope review and generated companion modeling are distinct from Plan authorship. | behavior/proposed scope -> advisory scope-review schema; no subject write | locks/claims, intent mutation, recovery execution | normalized patterns, axiom bijection, generated classes, ambiguity stop, byte-verified recovery recipe |
| `security` | **MOVE/CHANGE**, evidence; validation evidence/standalone | Authorized taxonomy coverage, scanner gaps, and reproducible findings are specialist evidence, not ordinary testing. | target/authorization/mode -> security report; read-only default, explicit artifact/baseline effects by mode | risk acceptance, policy approval, remediation, release | scanner unavailable gap, taxonomy ledger, empirical finding, fail-open probe, quiet-round bound, artifact validation |
| `shared` | **RETIRE**, no canonical skill root | It has no payload or unique trigger/procedure; shared doctrine belongs to owners and shared data belongs in explicitly consumed references. | currently claims reference docs but only `SKILL.md` exists | any authority or implicit dependency | retirement proof: no actual reference payload, consumers repointed, generated catalog/link/trigger checks pass |
| `skill-builder` | **CHANGE**, implementation/factory | Canonical skill package authoring and structural/projection contract compilation are a distinct meta-tool. | skill design/source -> package + reports + publisher invocation; scoped source/generated writes | scheduling, semantic candidate validation, promotion, Git | hostile v3 contract fixtures, create/check/fix isolation, source ownership, publisher transaction, second-run idempotence |
| `standards` | **CHANGE**, evidence; plan/implement/validate input | Focused normative guidance and mutation-safety findings are distinct from exact vocabulary lookup. | paths/language/risk -> cited findings; read-only | acceptance, edits, approval | live reference slug/currency checks, smallest-reference routing, pass/finding for mutation chokepoint/backup/ambition |
| `status` | **CHANGE**, support/observation | Evidence-store integrity and recency reporting is a read-only operational view, not a handoff or Goal controller. | evidence stores -> JSON/human snapshot; read-only | phase inference, queue/work choice, repair | corrupt/unavailable/arbitrary artifact rejection, JSON equivalence, recency-as-evidence only, checked/not-checked |
| `swarm` | **CHANGE**, runtime transport | Full-batch admission and at-most-once parallel dispatch over explicit packets are a reusable port independent of panes/factories. | explicit isolated packets/executor -> per-packet result; invokes executor | packet creation, partial launch, retry, integration, validation | reject whole invalid batch before launch, resource/effect isolation, call count exactly one, deadline/cancel/cleanup |
| `test` | **CHANGE**, implementation method + validation evidence | Behavioral test authoring, oracle strength, mutation-kill, and harness health form a specialist capability. | acceptance/subject/mode -> tests/coverage/evidence; writes tests/artifacts and product only in TDD under Implement | silent product fixes, semantic verdict, coverage-as-acceptance | authentic RED or kill proof, harness can fail, oracle tier, exact scenario linkage, declared writes/restoration |
| `toil-mining` | **CHANGE**, evolution/observation | Measured repeated operational friction and scoring are not general learning or pattern abstraction. | explicit history/window -> ranked evidence; read-only sources, optional report | work filing, scheduling, guessed priority | three cited occurrences, frequency×cost×error inputs, conservative extraction, unknown factor floor, no source mutation |
| `using-gc` | **CHANGE**, runtime transport | Explicit Gas City quest mapping and substrate-state isolation are unique to an operator-selected factory. | complete packets/city -> runtime evidence; operates selected city | automatic fallback, selection/retry, quest state as RPI state | explicit selection, packet/evidence schemas, disjoint workspace, deadline/cleanup, quest-state leakage negative fixture |
| `validate` | **CHANGE**, core experiment/judgment | It uniquely binds exact intent/subject identity and independent judgment into the durable semantic verdict. | immutable validation input -> `verdict.v2`; only verdict write | candidate edits, continuation, retries, Git/delivery | real cross-component corpus, distinct/fresh identities, before/end manifest, scope outcomes, criterion evidence, mutation quarantine |
| `workflow-builder` | **CHANGE**, implementation method | Authoring a thin one-shot typed adapter with at-most-once dispatch is distinct from executing Swarm. | explicit operations/runtime -> runnable workflow + schema; contained script writes | framework state, retry, selection, semantic validation | dry-run/fixture call counts, hostile inputs, timeout/sandbox/rollback, effect classification, per-operation errors |

## Appendix B — checked and not checked

Checked:

- all seven required frozen narrative/plan/audit files, completely;
- all 49 canonical `skills/*/SKILL.md` files, completely;
- skill frontmatter v1/v2 schemas, catalog schema, subject manifest schema, and
  verdict schema;
- RPI and Validate reference implementations and tests;
- skill mesh generator, frontmatter validators, Skill Builder build/heal paths,
  Codex sync/hash write surfaces, mesh checks, orchestration boundary gate, and
  gate registry;
- Go skill catalog/graph consumers, CLI contract projection, verdict reader,
  and all live source regions supporting the CLI audit findings;
- current focused probes: mesh generation/check PASS for 49; strict
  frontmatter FAIL for one missing optional field; orchestration boundary gate
  exits on a deleted source path; Python RPI/Validate unit suites PASS; 832
  focused Go tests PASS; live capabilities remain 84 commands with the audit's
  74/77/74/74 unknown/missing counts.

Not checked:

- external AGY, NTM, Agent Mail, Gas City, RCH, account, hook, or storage
  runtimes;
- Linux and Windows execution behavior;
- every skill reference file and every skill-local script line by line (the
  canonical SKILL kernels and directly relevant executable owners were read);
- performance benchmarks, hostile-output memory reproduction, and actual
  orphan-grandchild reproduction;
- any other wizard artifact, by design.

These gaps do not weaken the architecture claims above; they do prevent
claiming that runtime-specific behaviors already pass.
