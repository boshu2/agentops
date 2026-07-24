---
id: plan-2026-07-24-skill-system-overhaul
type: plan
date: 2026-07-24
status: in_progress/T0-typed-pause-repair-candidate
goal: Overhaul every canonical AgentOps skill around the campaign-to-experiment architecture
architecture_ref: docs/contracts/skill-ports-and-adapters.md
duel_ref: docs/audits/skill-system-overhaul-duel-2026-07-24/README.md
cli_audit_ref: docs/audits/2026-07-24-go-cli-deep-audit.md
---

# Skill system overhaul plan

## Outcome

Every canonical skill has one distinct intent, one honest place in the
Goal-to-RPI system, typed inputs and outputs, explicit authority and effects, a
bounded procedure, and behavioral evidence appropriate to its shape.
Generated projections reproduce that contract without becoming another source
of truth.

The program is complete only when:

- all 49 current worktree skills have exactly one disposition below;
- the four-skill RPI kernel proves exact identity and independent judgment;
- the catalog compiler, runtime readers, and generated projections agree;
- every retained skill is behaviorally distinct and reachable in the
  portfolio;
- every migration tranche has a durable verdict under an activated proof
  contract; and
- the dated plan relinquishes its temporary authority to generated,
  source-owned contracts.

This revision incorporates the complete sealed SOL/FABLE duel and the parallel
Go CLI audit. The duel's launch-time packet and all raw artifacts are retained
under the linked audit directory.

## Intent and system boundary

AgentOps is not a generic agent factory. It turns one caller-owned intent into
one independently judged experiment:

```text
product boundary + fitness evidence
  -> optional Goal / Mayor campaign
       -> caller or Goal selects one experiment
       -> RPI: Plan -> Implement -> fresh Validate -> report and stop
       -> caller or Goal consumes the immutable result
  -> optional capability evolution
       -> evidence-backed proposal
       -> later ordinary RPI
```

The architectural unit is semantic, not process-shaped:

- a Goal owns a multi-experiment campaign, cumulative budgets, ratchets,
  breakers, and the choice of another experiment;
- RPI owns exactly one ordered experiment and no continuation;
- Plan refines or freezes one experiment's caller-owned intent;
- Implement produces one exact candidate and factual receipts;
- Validate judges the unchanged intent and subject once from a fresh,
  author-distinct context;
- runtime adapters execute supplied packets but own no semantic lifecycle;
- optional judgment and evidence skills advise an accountable owner and never
  mint their own PASS.

The only hard core graph remains:

```text
rpi -> plan
rpi -> implement
rpi -> validate
```

Everything else is an optional port or adapter. A skill's presence in a seam
does not grant the seam owner's authority.

## Evidence-backed baseline

The plan is based on the live worktree and executable behavior, not only on
`AGENTS.md` or architecture prose:

- the worktree exposes 49 `skills/*/SKILL.md` sources;
- current `origin/main` already contains commit `16d764b5a`: 28 of its 30
  paths remain byte-identical and the two generated divergences are explained
  by later `using-gc` and `skill-builder` source changes; the partial
  `craft-goal` state exists only in the preserved stale local worktree and is
  outside the landing baseline;
- `skills/rpi/scripts/run_once.py` hashes a canonical JSON mapping, while
  `skills/validate/scripts/validate.py` hashes the exact intent bytes;
- the RPI unit test mocks Validate with RPI's own digest function, so both unit
  suites pass without testing their composed identity contract;
- current PASS semantics prohibit unresolved required coverage, but narrative
  checked/not-checked language does not consistently distinguish required
  gaps from caller-declared exclusions;
- the skill frontmatter estate has two validation paths with different schema
  behavior, and the Go catalog reader does not strictly consume the claimed
  schema;
- `scripts/regen-all.sh` continues after a failed generation step, and shared
  generated surfaces are already dirty in the current worktree;
- the original stale local worktree's `make regen-check` exited 126 and direct
  Bash invocation reported `implement` twin, hash, GC projection, and pack
  drift; current `origin/main` already invokes the owner through Bash and
  contains the #988 projection set, so those facts are preserved as
  reconciliation evidence rather than copied into the landing candidate;
- `scripts/check-orchestration-skill-boundaries.sh` references a deleted Go
  path, is not wired into the effective gate set, and checks stale wording;
- the Go CLI has two high-severity defects and five additional verified
  defects, but most are not on the skill migration's proof path;
- installed skills, plugin bundles, symlink estates, and Homebrew CLI releases
  can advance on independent cadences.

Pre-existing worktree changes are caller-owned. No tranche may overwrite,
normalize, or restore them from HEAD.

## Decisions

### D1. Repair and activate the exact-byte kernel first

The first semantic migration atom is exactly:

```text
rpi plan implement validate
```

It lands one composed evidence transaction:

```text
single-mint intent snapshot
  -> exact intent digest
  -> before/final subject manifests
  -> complete actual changed paths + factual receipts
  -> one fresh Validate
  -> verdict.v3
  -> rpi-report.v2
  -> stop
```

The intent bytes are minted once. Every later phase or remote validator
receives the snapshot by digest reference; it may not re-fetch, reserialize, or
normalize the living intent source.

Each core phase runs at most once. A material Plan-review amendment, scope
conflict, candidate mutation after freeze, FAIL, or NOT_PROVEN terminates that
RPI. Another attempt is another explicit RPI with its own intent and report,
not a validation-only retry or hidden repair lane.

`rpi-report.v2` carries a narrow, size-bounded opaque correlation object for
caller or Goal identifiers. RPI preserves those values but never interprets
campaign state.

`verdict.v2` remains an immutable legacy statement and its existing schema is
never widened into a second shape. New writers emit `verdict.v3`; readers
dispatch strictly by `schema_version` and retain a read-only v2 branch.
Likewise, `rpi-report.v1` remains legacy-readable while new runs emit only
`rpi-report.v2`. New contracts use `intent_digest` for the digest of the whole
exact intent snapshot rather than preserving the misleading
`acceptance_digest` name.

### D2. Proof contracts advance by non-self-certifying epochs

Every binding verdict identifies the exact proof contract that produced it.
The identity includes, at minimum:

- validator contract and implementation digests;
- verdict schema digest;
- RPI report schema digest;
- subject-manifest schema digest;
- qualification-corpus digest; and
- active proof-contract transition digest.

Old verdicts are immutable statements under their original proof epoch. They
are never re-baselined, upgraded, or reinterpreted.

A candidate proof contract cannot activate itself. A
`proof-contract-transition.v1` record binds:

- the prior active proof-contract digest;
- the candidate proof-contract digest;
- the candidate's exact subject manifest;
- the qualification-corpus digest;
- a binding verdict minted under the prior active contract; and
- activation time plus fresh validator identity.

The candidate may emit shadow results during qualification, but those results
are nonbinding until activation. T0 freezes the pre-candidate Validate
implementation, schemas, and a standalone transition recorder as bootstrap
epoch 0. Its known composed RPI digest defect remains an explicit proof gap:
epoch 0 may judge the candidate directly under its frozen rules, but no
qualification may rely on the broken RPI dispatcher edge. The first qualified
and activated revision becomes epoch 1.

Fresh T0 validation rejected the first frozen root before it minted any PASS:
its recorder did not bind candidate components to live bytes, modes, and the
judged subject. That descriptor and FAIL remain immutable. An explicit
operator-authorized bootstrap-root replacement selects corrected epoch 0b;
this pre-activation escape hatch is unavailable after any PASS or epoch
transition.

Fresh validation of the first repair then rejected only its pause-state
criterion: the ledger still described its now-stable candidate as uncommitted,
and the T0 checker trusted `result: PASS` without checking lineage or progress
claims. That second FAIL is also immutable. A separate metadata-only invocation
added explicit lineage and hostile checks, but its fresh validator proved two
remaining semantic bypasses: contradictory progress could be added under
different prose, and a fabricated future active pointer was not bound to the
transition candidate or transition bytes. That third FAIL is immutable too.
A new typed, closed-world pause invocation owns only those two checker defects;
it does not reopen the accepted bootstrap repairs or start T1.

### D3. Required gaps and declared exclusions are different types

Acceptance criteria receive stable IDs at intent freeze. Evidence and
exclusions refer to those IDs so a validator cannot launder missing proof by
renaming a criterion.

- `unchecked_required` means an in-scope criterion lacks sufficient evidence;
  it forces NOT_PROVEN.
- `declared_exclusions` means the caller excluded a surface before the
  candidate froze; it may coexist with PASS only when it does not contain a
  required criterion.
- report-level `checked` and `not_checked` may disclose non-criterion surfaces,
  but they cannot weaken criterion coverage.

PASS requires a nonempty checked subject, top-level evidence, evidence for
every required criterion, complete changed-path coverage, an activated proof
contract, and an empty `unchecked_required` set.

### D4. One typed skill compiler owns contract grammar

`skills/<slug>/SKILL.md` remains the semantic source. One compiler owns the
machine grammar for:

- one primary layer:
  `product | campaign | experiment | judgment | evidence | implementation |
  evolution | runtime | support`;
- zero or more lifecycle seams:
  `product_input | goal_design | goal_observe | option_shaping | plan_input |
  plan_review | implement_method | validate_evidence | validate_strategy |
  post_verdict | runtime_transport | cross_cutting | standalone`;
- skill-grantable authority;
- structured local, process, host, credential, and external effects;
- typed consumed and produced artifacts;
- positive, negative, and ambiguity trigger cases;
- unavailable, timeout, partial-evidence, and cleanup behavior; and
- proof class plus executable probe.

Experiment selection is not skill-grantable authority. It remains caller or
Goal authority. `option_shaping` replaces the misleading
`experiment_select` seam. Core membership is expressed by the hard dependency
graph, not a `core_phase` label.

`standalone` means the skill may satisfy a bounded direct request without
joining RPI; it does not mean unbounded authority.

Existing `tier` and `disposition` fields do not become peer architectural
axes. Tier is derived or retired. Disposition is curation status only.
Taxonomy additions require a design review that proves the concept cannot be
derived from existing axes.

The `skill-contract.v3` compiler is a shadow readiness rail while legacy API1
source contracts and the generated `skill-catalog.v3` remain authoritative.
Owning tranches populate `metadata.contract_v3` data and fixtures without
publishing them as live authority. One all-catalog cutover promotes the strict
source shape, emits `skill-catalog.v4`, and retires the old writers; there is
never a period with two authoritative schemas or two incompatible catalogs
claiming version 3.

### D5. Proof follows transitive dependencies

T0 builds an acceptance-to-check graph and a proof-chain ledger. Every
deterministic check named by this plan is classified:

```text
GREEN | RED_FOR_CAUSE | DEAD
```

A DEAD check is repaired or retired before it supports acceptance. At least
one seeded negative witness must make each load-bearing check fail for the
intended cause.

Every CLI, process, generator, or adapter edge on a proof path is classified:

```text
USED_SOUND | USED_UNSOUND | PROVEN_UNUSED | UNKNOWN
```

`USED_UNSOUND` blocks the dependent tranche. `UNKNOWN` may coexist with
implementation work but cannot support PASS; the dependent result is
NOT_PROVEN. `PROVEN_UNUSED` keeps unrelated product defects out of the critical
path.

### D6. Observed effects must reconcile with declared authority

Typed declarations alone do not prove effect honesty. Effectful execution
emits `effect-receipt.v1`, reusing runtime facts where possible:

- actual changed paths;
- process tree and termination results;
- external adapter operation IDs and mutation result;
- before/after credential or host identity when relevant; and
- cleanup completion or explicit observation opacity.

Validate checks that observed effects are a subset of declared and authorized
effects and that every required cleanup has a receipt. If the platform cannot
observe an effect class, a no-effect or cleanup claim is
`unchecked_required`, not PASS.

### D7. Generated publication is a recoverable transaction

All shared projections have one owner map and one fail-fast `regen-all`
entrypoint. Worker lanes never generate shared views.

Before publication, the owner classifies every target:

```text
CLEAN_CURRENT | DIRTY_PRESERVE | MISSING | UNOWNED
```

It snapshots exact pre-run bytes, mode, and symlink target into a recovery
bundle; an unowned collision aborts. It renders the complete projection set
into staging, validates check/write parity and byte-idempotence, then replaces
live paths and writes the manifest last. Failure restores from the recovery
bundle, never from HEAD. The bundle is deleted only after the final manifest
validates.

Single-writer serialization is the default. If the caller explicitly permits
concurrent publishers, the same owner also takes a publication lock.

### D8. Per-skill validity must compose into portfolio usability

Before trigger descriptions change, T0 freezes roughly 30 realistic routing
scenarios and records current live-model behavior. Each skill later supplies
positive, negative, ambiguity, and nearest-neighbor fixtures.

At cutover, those fixtures compile into a global shadow-routing corpus with a
small adversarial cross-product. Current and v3 routing are compared for:

- canonical selection or required abstention;
- top-k reachability of every retained skill;
- ambiguity, fallback, and wrong-authority rates;
- alias equivalence; and
- progressive-disclosure payload selection.

Changed choices need not match the old router, but each change must follow the
new contract. No retained skill may be unreachable without a retirement
review. A small pre/post live-model behavioral sample tests the otherwise
unproven assumption that better skill text changes agent behavior.

### D9. Rename and retirement have semantic and physical phases

`goals` becomes the read-only `fitness` skill after vocabulary is pinned.
Generated discovery advertises only `fitness`; `goals` remains a bounded,
non-advertised compatibility alias. The `ao goals` CLI command is a separate
product surface and is not renamed by this plan.

`shared` is semantically retired only after runtime-neutral fresh-context and
model-identity contracts move to declared owners under `docs/contracts/`,
adapter-specific mechanics remain with adapters, and every prose/generated
consumer is migrated.

Static repository search is insufficient to authorize deletion. Compatibility
resolvers or typed tombstones record aggregate deprecation hits without prompt
content. Physical deletion requires zero observed hits across a declared
window, distribution-image and known-consumer scans, and owner confirmation.

Runtime adapters may be folded only when they fail to prove a distinct
unavailable, timeout, bounded-output, cleanup, or transport behavior.

### D10. Every tranche must be a safe resting state

One RPI covers one coherent tranche; a RED fixture that proves independent
authority or rollback risk forces a split before implementation.

Every tranche closes with:

- a durable verdict under the active proof epoch;
- green live gates and regenerated views that describe only landed state;
- no document delegating authority to unfinished work;
- a completion-matrix row keyed by verdict and proof-epoch digests;
- a pause drill showing a fresh context can identify landed, in-flight, and
  authoritative state; and
- a plan status update such as `in_progress/T3-complete`.

Stopping between tranches must be safe. A retry count alone never justifies
claiming completion or blockage.

## Catalog-wide acceptance

The final catalog must prove all of the following:

1. **Coverage:** every live semantic skill has one primary layer, legal seams,
   one disposition, and one matrix row.
2. **Core graph:** only RPI hard-depends on Plan, Implement, and Validate.
3. **Campaign separation:** no skill selects another experiment or owns
   cumulative campaign state.
4. **Verdict authority:** only Validate writes binding verdicts. Historical
   `verdict.v2` stays immutable and read-only; the activated exact kernel emits
   strict `verdict.v3`.
5. **Identity:** intent, subject, proof contract, and result lineage use exact
   digest references with single-mint transport.
6. **Criterion coverage:** every required stable criterion ID has evidence;
   exclusions cannot be introduced after freeze.
7. **Effect honesty:** observed effects and cleanup reconcile with declarations
   and caller authority.
8. **Output honesty:** every skill has a typed output contract or executable
   shape validator.
9. **Trigger separation:** positive, negative, ambiguity, alias, and
   nearest-neighbor behavior is globally routable.
10. **Failure semantics:** unavailable tools, timeouts, incomplete evidence,
    and partial mutation have explicit terminal outcomes.
11. **Behavioral proof:** each skill has shape-appropriate positive and
    hostile probes; phrase presence alone is never sufficient.
12. **Projection integrity:** the complete generated set publishes
    transactionally and reruns byte-identically with zero drift.
13. **Reference integrity:** practices, links, commands, schemas, and runtime
    facts resolve to living owners.
14. **Context discipline:** the current 250-line kernel cap remains an interim
    warning and gate until an always-loaded byte/token metric replaces it.
15. **Compatibility:** `skill-catalog.v4` readers negotiate legacy catalog
    v1/v2/v3 inputs across declared release channels, and deprecated names are
    removed only by observed-zero policy.
16. **Reachability:** every retained skill wins at least one justified
    portfolio scenario and cannot acquire forbidden authority through routing.

## Per-skill target matrix

Each skill appears exactly once. The curation disposition is CHANGE unless the
target explicitly says RENAME or RETIRE. “Target change” is the minimum
architectural correction; focused probes may expose additional required
repairs.

| Skill | Target placement | Target change and proof | Tranche |
|---|---|---|---|
| `account-rotation` | support; cross_cutting, standalone | Discover host/agent capability, declare credential effects, emit before/after identity plus cleanup receipt, and test unavailable and partial rotation. | T7b |
| `agent-mail` | runtime; runtime_transport | Type identity, reservation, ACK, TTL, conflict, and degraded-surface results; observe mutation effects without granting work ownership. | T7a |
| `agent-native` | runtime; runtime_transport | Constrain “orchestrator” to runtime coordination; type packets, deadlines, engagement, replacement, bounded output, and cleanup. | T7a |
| `agy-native` | runtime; runtime_transport | Probe live capability, bound sessions and output, define unavailable behavior, and prove abnormal process cleanup. | T7a |
| `automation-shape-routing` | support; cross_cutting, standalone | Make semantic admission advisory and separate from executor topology; exhaustively route analysis, one RPI, Goal, and automation without starting a runtime. | T3 |
| `bootstrap` | support; cross_cutting, standalone | Declare documentation/storage writes and prove byte-idempotence plus no Git, tracker, runtime, or undeclared host mutation. | T7b |
| `cass` | evidence; plan_input, post_verdict | Move volatile facts to discovery, preserve query/provenance/recency boundaries, and keep the kernel below the interim budget. | T4 |
| `cc-hooks` | support; cross_cutting | Move recipes to references, type hook/config effects, validate event schemas, and prove safe removal and required cleanup. | T7b |
| `codebase-recon` | evidence; plan_input, validate_evidence | Validate file-line citations, baseline/delta identity, checked coverage, and an explicit no-verdict boundary. | T4 |
| `codex-exec` | runtime; runtime_transport | Add wall-clock/output bounds, cancellation and process-tree cleanup, unavailable-runtime behavior, and a typed run result. | T7a |
| `converter` | implementation; implement_method | Reconcile modular/inlined defaults, constrain clean writes, emit exact changed paths, and reject direct projection edits. | T5 |
| `council` | judgment; plan_review, validate_strategy | Publish `council-report.v1`, preserve dissent, separate preferred from required model diversity, and forbid semantic PASS. | T4 |
| `craft-goal` | campaign; goal_design | Be the campaign-policy compiler/linter, not a runtime controller; type terminal acceptance, monotonic envelope, ratchet, and terminal report. | T3 |
| `dcg` | support; cross_cutting | Discover live command facts, emit risk and reversible alternatives, preserve human-only override, and type any config write. | T7b |
| `doc` | implementation; implement_method | Replace unbounded execution prose with a mode contract, declare writes, harden project detection, and validate generated claims and paths. | T5 |
| `domain` | evidence; plan_input, cross_cutting | Require exact citations, explicit unknown/conflict results, and a bounded vocabulary lookup result. | T4 |
| `goals` | product; product_input, goal_observe | Rename semantic skill to `fitness`, keep read-only measurement distinct from Goal campaigns, type its measurement report, and retain a measured compatibility alias. | T3 |
| `handoff` | support; goal_observe, cross_cutting | Type artifact name, digest, schema, and effects; prove read-without-consume behavior. | T7b |
| `idea-genie` | judgment; option_shaping, plan_input | Make elicit/duel outputs symmetric, bound novelty and saturation, and keep experiment choice outside the skill. | T4 |
| `implement` | experiment; core graph | Produce one exact candidate, before/final manifests, actual path coverage, check and effect receipts; stop on scope conflict with no repair revision. | T1 |
| `learn` | evolution; post_verdict | Preserve verdict identity, type recurrence/sample/confidence/decay, deduplicate by objective, and forbid self-promotion. | T6 |
| `ms` | support; cross_cutting, standalone | Split retrieval from write/admin modes, discover volatile corpus/host facts, and type authorization, effects, and cleanup. | T7b |
| `ntm` | runtime; runtime_transport | Require explicit roles, commands, scopes, deadlines, observation windows, bounded robot evidence, capability checks, and cleanup. | T7a |
| `operationalize` | evolution; post_verdict, plan_input | Type evidence-backed proposals, narrow authoritative exceptions, and route every implementation through a later caller-selected RPI. | T6 |
| `pattern-mining` | evolution; post_verdict, plan_input | Separate hypotheses from earned abstractions, retain exemplar/holdout lineage, and keep promotion advisory. | T6 |
| `plan` | experiment; core graph | Refine one caller intent, mint one exact snapshot, type amendments, preserve stable criterion IDs, and forbid campaign decomposition or duplicate plan packets. | T1 |
| `postmortem` | judgment; post_verdict | Type causal claims and report effects, preserve evidence/uncertainty, and keep proposed experiments advisory. | T4 |
| `premortem` | judgment; plan_review | Type a fresh pre-build challenge; a material finding stops the current RPI instead of silently editing intent. | T4 |
| `product` | product; product_input | Validate a stable product template, preserve user-authored sections, and distinguish evidence, aspiration, acceptance, and non-goals. | T3 |
| `rch` | runtime; implement_method, cross_cutting | Discover capability, bound remote execution, emit factual non-verdict states, and validate diagnostics and cleanup. | T7a |
| `reality-check` | judgment; goal_observe, option_shaping | Publish `reality-check-report.v1`, prove vision coverage, and preserve claim/evidence/unknown separation without selecting work. | T4 |
| `refactor` | implementation; implement_method | Require isolated seam probes, exact code effects, behavior-neutral before/after evidence, restoration checks, and a terminal failed-neutrality state. | T5 |
| `research` | evidence; plan_input | Narrow broad triggers, frontload question and source-quality constraints, validate citations and checked scope, and keep selection outside. | T4 |
| `reverse-engineer` | evidence; plan_input | Emit adoption recommendations rather than decisions, strengthen authorization and registry lineage, and modularize mechanics. | T4 |
| `rpi` | experiment; core graph dispatcher | Dispatch Plan, Implement, and fresh Validate at most once; use exact digest references; emit `rpi-report.v2`; preserve only opaque correlation; stop without continuation. | T1 |
| `sbh` | support; cross_cutting, standalone | Bind every action to exact device/volume identity, declare recovery effects, and emit a typed status/action result. | T7b |
| `scaffold` | implementation; implement_method | Declare filesystem effects, make tests mode-specific, emit exact changed paths, and prove refusal and overwrite boundaries. | T5 |
| `scope` | judgment; plan_review | Frontload constraints, type review findings, model generated companions as scope classes, and remain advisory. | T4 |
| `security` | evidence; validate_evidence, standalone | Separate read-only collection from explicitly authorized mutating modes, type taxonomy/gap/findings evidence, and forbid risk acceptance or policy approval. | T4 |
| `shared` | support; cross_cutting | Relocate neutral contracts to declared owners, migrate all consumers, semantically retire the empty skill root, and delay physical deletion until observed-zero evidence. | T7b |
| `skill-builder` | implementation; implement_method | Own `skill-contract.v3` placement support, semantic stocktake, effect/output/trigger grammar, accurate fix receipts, and hostile behavioral fixtures. | T2 |
| `standards` | evidence; plan_input, implement_method, validate_evidence | Repair practice slugs, validate reference currency, type cited findings, and distinguish advice from acceptance. | T4 |
| `status` | support; goal_observe | Move runtime-specific hints to projections, narrow triggers, type JSON output, and test corrupt or unavailable evidence stores. | T7b |
| `swarm` | runtime; runtime_transport | Require path, resource, and external-effect isolation; type admission, deadlines, cancellation, cleanup, and batch results. | T7a |
| `test` | implementation; implement_method, validate_evidence | Declare test/product/artifact effects, isolate mutation-kill proof, constrain production edits to Implement authority, and add real behavioral probes. | T5 |
| `toil-mining` | evolution; goal_observe, post_verdict | Normalize scoring and uncertainty, type ranked evidence, preserve history read-only, and forbid automatic work creation. | T6 |
| `using-gc` | runtime; runtime_transport | Type packet/evidence schemas, prove capability/isolation/deadline/cleanup, and prevent quest state from becoming RPI state. | T7a |
| `validate` | experiment; core graph | Judge the exact frozen intent and subject once, bind activated proof identity, distinguish gaps from exclusions, require criterion evidence, and issue the only verdict. | T1 |
| `workflow-builder` | implementation; implement_method | Declare script writes, classify effects, sandbox and time-bound execution, type rollback/artifacts, and exercise hostile fixtures. | T5 |

## Migration tranches

### T0 — Evidence lineage and safe starting state

Scope:

- freeze the 49-skill path and digest ledger;
- reconcile commit `16d764b5a`, the clean origin-based landing tree, and the
  preserved stale worktree as
  `PRESENT_IDENTICAL | PRESENT_DIVERGED | ABSENT | RESCHEDULED | OUT_OF_SCOPE`;
- capture the pre-change routing corpus and live-model baseline;
- classify every plan-named check for liveness and seed negative witnesses;
- build the transitive proof-chain ledger, including CLI/process edges;
- freeze the bootstrap epoch-0 Validate implementation, schemas, and standalone
  transition recorder outside the T1 candidate subject;
- record installed-estate delivery channels and current schema versions;
- repair or retire the dead orchestration boundary gate; and
- run a fresh-context pause drill.

Acceptance:

- no pre-existing path is overwritten or ambiguously owned;
- every load-bearing check demonstrably detects its intended negative;
- `make regen-check` invokes its owner successfully and fail-fast behavior is
  proven with a seeded generator failure;
- every proof edge is classified, with no UNKNOWN edge claimed as PASS support;
- the routing baseline can be replayed after descriptions change;
- the #988/craft-goal state and the stale local worktree have explicit,
  distinct dispositions; and
- stopping after T0 leaves current authority and generated views truthful.

### T1 — Exact kernel and proof epoch 1

Skills:

```text
rpi plan implement validate
```

Start RED with composed fixtures proving:

- exact source bytes survive Plan, Implement, remote transport, and Validate;
- whitespace or Unicode-different snapshots do not silently compare equal;
- a consumer cannot re-derive intent from a living source;
- all four current producer/consumer implementations agree on one digest;
- each core phase runs at most once;
- changed-path coverage includes all actual changes and generated companions;
- exclusions cannot absorb required stable criterion IDs;
- candidate mutation after freeze is terminal;
- FAIL and NOT_PROVEN emit a report and stop;
- duplicate unlinked verdicts over one intent/subject are rejected; and
- a candidate proof contract cannot activate itself.

Acceptance:

- the live RPI/Validate digest mismatch is reproduced RED and then fixed;
- `rpi-report.v2`, proof identity, effect receipt, and transition schemas have
  strict readers plus a shared golden corpus;
- epoch 0 directly judges the exact T1 candidate without traversing the broken
  RPI dispatcher edge;
- epoch 1 is then activated with `proof-contract-transition.v1`;
- all composed negative scenarios fail for the intended reason; and
- the kernel rests with no campaign dependency or retry lane.

### T2 — Contract compiler and transactional publisher

Skill:

```text
skill-builder
```

Scope:

- one `skill-contract.v3` grammar/compiler and hostile fixture corpus;
- primary layer, seams, authority, effects, typed artifacts, triggers, failure
  semantics, and proof class;
- explicit retirement/derivation of redundant taxonomy;
- a strict `skill-catalog.v4` reader and legacy-tolerant v1/v2/v3
  compatibility branches;
- a migration-readiness ledger for all 49 skills;
- one projection owner map and fail-fast regeneration command; and
- staging, pre-run recovery bundle, manifest-last publication, and fault
  injection.

Acceptance:

- legacy API1 source contracts and `skill-catalog.v3` remain the sole live
  authority while `skill-contract.v3` shadow validation is incomplete;
- unknown or forbidden authority, undeclared effects, bad artifacts, trigger
  collisions, and stale projections fail hostile fixtures;
- check and write modes render identical bytes;
- kill/failure injection restores exact dirty pre-publication bytes and modes;
- no worker lane writes shared projections; and
- the compiler can represent every current target without making the plan a
  second catalog.

### T3 — Product, campaign, and admission

Skills:

```text
product goals craft-goal automation-shape-routing
```

Required scenarios:

- one bounded request routes directly to one RPI without a Goal;
- a multi-experiment terminal outcome routes to a bounded Goal;
- read-only `fitness` cannot be mistaken for Goal campaign control;
- `craft-goal` compiles/lints policy but starts no runtime and selects no work;
- automation-shape routing advises semantic shape before topology; and
- the hidden `goals` alias resolves equivalently while only `fitness` is
  advertised.

Acceptance:

- product, fitness, campaign policy, and advisory admission have disjoint
  authority and trigger probes;
- `metadata.contract_v3` is readiness-complete but remains non-authoritative;
- live-model routing does not regress on the frozen T0 scenarios; and
- the `ao goals` CLI command remains explicitly outside the rename.

### T4 — Evidence and judgment

Skills:

```text
cass codebase-recon council domain idea-genie postmortem premortem
reality-check research reverse-engineer scope security standards
```

Acceptance:

- every advisory/evidence result names sources, omissions, uncertainty, and its
  downstream seam;
- no strategy emits readiness, risk acceptance, work selection, or
  a binding verdict;
- citations, authorization, disagreement, missing evidence, and false-positive
  boundaries are executable;
- `security` emits typed specialist evidence and separately authorizes any
  mutation;
- every observed effect reconciles to declarations; and
- a RED fixture, not family resemblance alone, is required to split a skill
  into another tranche.

### T5 — Candidate-producing specialists

Skills:

```text
converter doc refactor scaffold test workflow-builder
```

Acceptance:

- every mutation occurs under caller or Implement authority;
- exact changed paths, generated companions, factual checks, and effect
  receipts flow into the kernel;
- disposable isolation proves overwrite, rollback, mutation-kill, restoration,
  timeout, and hostile-input behavior;
- read-only and mutating modes are behaviorally distinct; and
- no specialist starts RPI, repairs a terminal result, or writes a verdict.

### T6 — Capability evolution

Skills:

```text
learn operationalize pattern-mining toil-mining
```

Acceptance:

- every observation preserves source verdict/history and proof-epoch identity;
- recurrence, sample size, uncertainty, decay, exemplars, and holdouts remain
  visible;
- proposals cannot promote themselves into skills, gates, tracker work, or
  campaign choices; and
- mutation requires a later caller/Goal-selected RPI.

### T7a — Runtime transports

Skills:

```text
agent-mail agent-native agy-native codex-exec ntm rch swarm using-gc
```

Acceptance:

- capability-unavailable behavior is explicit;
- packets, outputs, deadlines, cancellation, engagement, and cleanup are typed;
- resource and external-effect isolation is proven, not inferred from paths;
- transport state cannot become campaign, RPI, verdict, or work-ownership
  state;
- each retained adapter proves a distinct behavioral capability; and
- unobservable cleanup or external effects force NOT_PROVEN.

### T7b — Host and support surfaces

Skills:

```text
account-rotation bootstrap cc-hooks dcg handoff ms sbh shared status
```

Acceptance:

- host, credential, device, documentation, and configuration effects have
  authorization and observed receipts;
- support skills never steer experiments or mint semantic outcomes;
- neutral shared contracts have declared non-skill owners and tested consumers;
- `shared` is removed from advertising and authority without premature
  physical deletion; and
- compatibility/tombstone counters are ready for the observed-zero window.

### T8 — skill-contract.v3 / skill-catalog.v4 cutover and portfolio convergence

Scope:

- freeze all 49 `skill-contract.v3`-ready sources;
- compile the global routing corpus and adversarial cross-product;
- run legacy versus `skill-contract.v3` routing plus a pre/post live-model
  sample;
- perform the installed-estate/channel compatibility audit;
- transactionally publish every generated projection once;
- activate strict `skill-contract.v3` source authority, publish
  `skill-catalog.v4`, and retire old schema writers;
- produce the per-skill completion matrix keyed by verdict and epoch digests;
- conformance-check this matrix against the generated catalog; and
- remove the architecture contract's temporary pointer to this dated plan.

Acceptance:

- every retained skill is distinct, reachable, honest about authority and
  effects, and backed by the required probe;
- every changed routing decision is explained and wrong-authority behavior does
  not regress;
- catalog v4 and legacy v1/v2/v3 reader branches pass their support-window
  fixtures;
- regeneration is complete, byte-idempotent, drift-free, and recoverable;
- no canonical placement or lifecycle fact remains owned only by this plan;
- this plan is marked `superseded-by` the generated contract/catalog; and
- a fresh context can understand the final tree without campaign memory.

## Parallel Go CLI release program

The Go CLI audit is an expedited sibling program, not a blanket prerequisite
for rewriting skill text. It has three release tranches:

### G0 — Containment and owned temporary state

- validate and canonicalize eval identifiers before filesystem path
  construction;
- prove no traversal can escape the owned root;
- own, reuse, or clean automatically created runtime-isolation directories;
- add hostile traversal and cleanup tests.

### G1 — Command effect and output contract

- make global `--dry-run` suppress every declared mutation or reject the flag;
- unify `--json` and `-o json` semantics across read commands;
- give effectful commands a shared effect/output vocabulary with the skill
  compiler; and
- remove the stale eval help reference.

### G2 — Subprocess lifecycle

- stream output under a hard bound instead of truncating after unbounded
  buffering;
- thread caller cancellation through eval execution;
- terminate process groups and verify descendant cleanup; and
- test timeout, cancellation, high output, and abnormal exit.

A verified CLI defect blocks a skill tranche only when the proof-chain ledger
shows that tranche traverses it. `USED_UNSOUND` blocks; `UNKNOWN` forces
NOT_PROVEN; `PROVEN_UNUSED` permits independent progress. G0 remains urgent on
its own security merits.

## Proof requirements by skill shape

| Skill shape | Required evidence |
|---|---|
| Pure parser/schema/compiler | Unit tests, hostile fixtures, shared golden corpus, and strict-reader parity |
| Mutating specialist | Disposable-worktree integration, exact changed paths, effect receipt, and overwrite/rollback proof |
| Judgment strategy | Golden scenarios for disagreement, missing evidence, false positives, and forbidden authority |
| Runtime adapter | Unavailable, timeout, bounded output, cancellation, abnormal cleanup, and resource-isolation tests |
| Read-only evidence | Citation/coverage fixtures plus proof that declared sources and host state were not mutated |
| Router/admission | Positive, negative, ambiguity, abstention, alias, nearest-neighbor, and portfolio reachability corpus |
| Core kernel | Composed exact-byte state transitions, fresh identity, mutation detection, terminal outcomes, and epoch activation |

Phrase-presence and file-existence checks remain hygiene only.

## Write scope

Allowed:

- canonical skills and their owned references, schemas, scripts, and tests;
- proof-kernel contracts, schemas, readers, and golden fixtures;
- compiler, catalog, graph, and generated-projection owner sources;
- Go CLI consumers only where a typed skill/proof contract crosses that
  boundary;
- architecture and migration documents named by a tranche; and
- generated outputs produced transactionally by their declared owner.

Excluded unless a frozen tranche explicitly expands authority:

- unrelated CLI product features or delivery policy;
- Git history, branches, pushing, merge, release, or rollback decisions;
- external runtime installation or credential mutation;
- tracker closure or Goal creation; and
- hand-edits to generated projections.

## First useful checks

T0 decides which of these are live and load-bearing before any result relies on
them:

```bash
bash skills/skill-builder/scripts/heal.sh --check --strict skills/<slug>
bash skills/skill-builder/scripts/audit.sh --strict skills/<slug>
bash scripts/validate-skill-frontmatter.sh --strict
python3 scripts/generate-skill-mesh.py --check
bash scripts/validate-codex-generated-artifacts.sh --scope worktree
bash scripts/validate-codex-api-conformance.sh
bash scripts/validate-codex-override-coverage.sh
bash scripts/check-orchestration-skill-boundaries.sh
```

Image manifest parity is owned by `generate-skill-mesh.py --check`; the
retired standalone image-manifest command is not a second gate.

Each owning tranche adds its focused RED witnesses. A Go reader change also
runs focused package tests, `go test ./...`, `go vet ./...`, the pinned lint
wrapper, and the shared golden corpus.

## Completion evidence and residual risk

The final validator must inspect:

1. the exact 49-row matrix against the live generated catalog;
2. every per-skill disposition and focused evidence path;
3. kernel identity, mutation, criterion, effect, and proof-transition records;
4. the acceptance-to-check graph and transitive proof-chain ledger;
5. the staged publication fault-injection and zero-drift rerun;
6. global routing, alias, reachability, and live-model samples;
7. installed-estate version negotiation and deprecation observations;
8. completion rows keyed by verdict and proof-epoch digests; and
9. explicit checked, non-required not-checked, and
   `unchecked_required` surfaces.

Platform-specific or unavailable external runtimes remain explicit gaps. They
cannot be represented as passing behavior. The deepest residual risk is
stochastic: contract-conformant skill text may still fail to produce
contract-conformant agent behavior. The routing baseline and live behavioral
sample measure that risk; they do not pretend to eliminate it.
