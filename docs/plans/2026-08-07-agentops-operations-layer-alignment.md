---
id: plan-2026-08-07-agentops-operations-layer-alignment
type: implementation-plan
date: 2026-08-07
status: proposed
goal: Align AgentOps behavior, contracts, skills, generated surfaces, and public identity around its actual role as the operations layer for agentic engineering
inputs:
  - AGENTS.md
  - PRODUCT.md
  - GOALS.md
  - PROGRAM.md
  - docs/architecture/operating-loop.md
  - docs/contracts/ubiquitous-language.md
  - docs/contracts/skill-ports-and-adapters.md
  - docs/contracts/registry-as-derived.md
  - docs/adr/ADR-0016-state-tiers.md
review:
  method: one read-only NTM Claude Fable review
  status: complete
  verdict: READY
  material_revisions_integrated: 5
---

# AgentOps operations-layer alignment

## Outcome

Make every current, authoritative product surface tell the same true story:

> AgentOps is the operations layer for agentic engineering.

The technical description is:

> AgentOps is a portable semantic integration and judgment layer that connects
> intent, coding agents, software factories, context sources, and independent
> validation without taking ownership of their state or delivery lifecycle.

The architecture uses four distinct terms:

| Term | Meaning |
|---|---|
| **Operations layer** | The product category: the portable layer that makes heterogeneous agentic engineering systems interoperate semantically. |
| **Federated integration graph** | The topology: caller-owned intent, source systems, agents, factories, checks, and judgments remain separate nodes joined by typed handoffs. |
| **Semantic work-and-proof protocol** | The interoperability contract: exact intent, exact subject, evidence, fresh judgment, and honest outcomes. |
| **RPI traversal** | One standard path through the graph: Plan -> Implement -> fresh Validate -> report and stop. |

The shortest explanation is:

> The traversal is RPI. The graph is the topology. The protocol defines
> interoperability. The operations layer is the product.

This is a product-alignment change, not a new orchestration system. It must
remove live contradictions as well as update prose.

## Why this change is necessary

The repository has already narrowed the AgentOps semantic boundary, but its
surfaces still describe several older products at once:

| Current contradiction | Evidence surface | Required disposition |
|---|---|---|
| AgentOps is called an "operating loop" | `README.md`, skills, docs, workflows | Replace current-authority usage with operations-layer language; retain historical usage only in dated records. |
| RPI is presented as the whole system | `README.md`, `AGENTS.md`, workflow copy | Name RPI as one bounded traversal through a wider federated graph. |
| AgentOps appears to orchestrate work itself | README and adapter wording | State that factories and callers own execution, scheduling, retries, work state, and delivery. |
| A knowledge flywheel is still a live CLI/product feature | `ao flywheel`, eval help, release metadata | Retire the command family and product claims; keep learning an optional evidence consumer. |
| `.agents/` is described and written as a knowledge store | CLI defaults, skills, scripts, ADR residue | Enforce source-owned state plus requested proof, scratch, and rebuildable projections. |
| A seven-move retrying workflow remains active | `workflows/operating-loop.js` | Replace it with an explicit compatibility tombstone that routes humans to the correct distinct workflow without translating arguments. |
| Generated projections and historical artifacts compete with current sources | routers, CLI docs, plans, audits | Edit declared source owners, regenerate projections, and leave dated history intact. |

Copy-only rebranding is insufficient. Completion requires public language,
runtime behavior, state writers, skills, generated artifacts, and gates to
agree.

## Product boundaries that must remain true

AgentOps owns:

- portable skills and semantic contracts;
- one bounded RPI traversal when explicitly invoked;
- exact intent and subject identity;
- deterministic repository checks;
- fresh independent judgment;
- optional caller- or consumer-requested proof persistence;
- adapters that translate explicit packets to caller-selected runtimes.

AgentOps does not own:

- a global control plane, scheduler, queue, retry loop, or autonomous campaign;
- Beads, Git, CASS, CM, NTM, Gas City, Agent Mail, CI, merge, release, or deploy
  state;
- the next action after `PASS | FAIL | NOT_PROVEN`;
- a merged "knowledge base" that silently outranks source systems;
- automatic promotion from observation to policy;
- proof merely because a runtime, workflow, or factory reports completion.

## State and context authority

The integration graph is federated. AgentOps links source identities; it does
not absorb their authority.

| Information | Authority | AgentOps treatment |
|---|---|---|
| Work, status, dependencies, close reasons | Beads or the caller's tracker | Query directly; reference stable IDs; never create a second work index. |
| Source content and delivery history | Git and repository policy | Bind exact content when useful; never infer semantic PASS from a commit or merge. |
| Past agent sessions | CASS | Retrieve cited episodes on demand; treat search output as evidence, not policy. |
| Curated cross-session memory | CM or a caller-selected memory system | Retrieve by explicit need; preserve provenance and freshness. |
| Runtime execution | NTM, Gas City, Agent Mail, cloud agents, or another selected factory | Read and report native state; never reinterpret runtime completion as validation. |
| Checks and test output | The executable that produced them | Store factual receipts or references; fresh Validate judges meaning. |
| Product rules and architectural decisions | Current docs and accepted ADRs | Edit deliberately in their source-owner files. |
| Requested proof | `.agents/ao/` | Persist only when the caller asks or a declared consumer requires it. |
| Disposable investigation | `.agents/scratch/` | Rebuild, promote to a real owner, quarantine, or expire. Never treat as authority. |
| Mined/linked views | `.agents/projections/` | Rebuildable output with generator, inputs, exact source identities, citations, and freshness. |

A projection is useful only when it has a named consumer and is cheaper than
re-querying its sources. Deleting projections must not change semantic
behavior. A projection must never become a hidden source of truth.

The earlier Graphify experiment remains historical evidence. It does not
justify a graph database, a required Graphify dependency, or a graph-first
ritual. A future Graphify adapter would have to satisfy the same projection
contract and demonstrate an actual consumer. The ignored `graphify-out/`
artifact is not cleaned up by this change.

## Non-goals

- Do not build an AgentOps scheduler, daemon, service, graph database, or
  software factory.
- Do not add retry, queue, claim, lease, merge, release, or deployment
  ownership to RPI.
- Do not create a second plan, design packet, per-phase report, or new Beads
  hierarchy from this plan.
- Do not rewrite dated plans, audits, evidence, releases, provenance, or the
  changelog to use new terminology retroactively.
- Do not delete or migrate the operator's current `.agents/` data. This change
  fixes current writers and checks; destructive cleanup requires separate
  explicit authorization.
- Do not hand-edit `skills-codex/**`, generated routers, catalogs, graphs,
  manifests, or `cli/docs/COMMANDS.md`.
- Do not change GitHub topics or the project homepage.
- Do not make optional context miners or runtime adapters hard dependencies of
  the core RPI skills.
- Do not require persistent `verdict.v2` for ordinary interactive validation.

## Definition of done

1. A new reader can state the product category, topology, protocol, and
   standard traversal without calling all four a loop.
2. Current authoritative surfaces use the canonical product description and
   preserve the small ownership boundary.
3. RPI still performs exactly one Plan -> Implement -> fresh Validate -> report
   traversal with no automatic continuation.
4. Runtime/factory completion, repository checks, and semantic validation are
   visibly different states.
5. Current code no longer advertises or computes the old knowledge-flywheel
   product status.
6. Current writers create only requested proof, scratch, or declared
   projections under `.agents/`; no writer creates a parallel Beads, Git,
   CASS, or CM authority.
7. Every generated surface is regenerated from its declared source and the
   regeneration check is clean.
8. Historical records remain historically accurate and resolve through
   compatibility links where a current path moved.
9. The repository's fast and full local gates pass.
10. One fresh validator judges the exact implementation against these criteria
    and returns `PASS`; otherwise the change does not merge.

## Source ownership map

Edit owners first. Regenerate or update consumers afterward.

| Claim | Source owner | Important consumers |
|---|---|---|
| Product category and boundary | `PRODUCT.md` | `README.md`, package metadata, GitHub About |
| Fitness properties | `GOALS.md` | tests, status and product docs |
| Repository operating contract | `AGENTS.md` | `docs/agent-workflow-reference.md`, agent sessions |
| Canonical vocabulary | `docs/contracts/ubiquitous-language.md` | architecture docs, skills, public docs |
| RPI semantics | `docs/architecture/rpi-traversal.md`, `schemas/*.schema.json` | core skills, workflows, tests |
| Skill behavior and metadata | `skills/<slug>/SKILL.md` | router, catalogs, graphs, Codex projections |
| CLI behavior | `cli/cmd/ao/`, `cli/internal/commands/` | generated CLI reference, compatibility fixtures |
| State tiers | `docs/adr/ADR-0016-state-tiers.md` | state writers, hygiene docs, gates |
| Generated skill inventory | source skill metadata | `registry.json`, `skills/catalog.json`, routers and graphs |
| Public package copy | product owner plus package manifest | plugin stores, Homebrew, docs site |

## Dependency order

```text
canonical language and state contract
  -> live CLI/workflow/state-writer behavior
  -> source skills and AGENTS contract
  -> public and package surfaces
  -> generated projections
  -> conformance gates
  -> fresh semantic validation
  -> post-merge GitHub About update
```

Run the work sequentially because the phases share generated surfaces and
terminology. Parallel writers would create avoidable conflicts.

## Implementation plan

### 0. Establish the exact baseline

Purpose: prevent the cloud agent from treating stale plans or local projections
as current authority.

- [ ] Start from the commit containing this plan on an otherwise clean branch
  from current `main`.
- [ ] Read `AGENTS.md`, `PRODUCT.md`, `GOALS.md`, `PROGRAM.md`, the active ADRs,
  and the owning contracts named in this plan before editing.
- [ ] Record, in the PR description rather than a new artifact, the current
  outputs of:

  ```bash
  git status --short --branch
  (cd cli && go run ./cmd/ao --help)
  (cd cli && go run ./cmd/ao flywheel status)
  bash scripts/regen-all.sh --check
  ```

- [ ] Inventory active non-test writers of `.agents/` with `rg`; distinguish
  normal execution paths from historical comments, fixtures, and migration
  compatibility.
- [ ] Treat every finding in old plans/audits as a hypothesis until confirmed
  in current source.

Stop and re-scope if a current accepted ADR contradicts the category or state
model in this plan. Do not paper over the conflict in downstream copy.

### 1. Make the vocabulary and architecture canonical

Purpose: give every later edit one stable set of meanings.

#### 1.1 Canonical vocabulary

- [ ] Update `docs/contracts/ubiquitous-language.md` with the four definitions
  from the Outcome section.
- [ ] Define these supporting terms precisely:
  `context source`, `execution orchestrator`, `software factory`, `projection`,
  `fresh judgment`, `runtime completion`, and `repository check`.
- [ ] State that `loop` may describe a local control structure but is not the
  product category or global architecture.
- [ ] Add the forbidden conflations:
  operations layer != execution orchestrator; RPI result != factory result;
  check success != semantic PASS; projection != source authority.

#### 1.2 Rename the core architecture page

- [ ] Move `docs/architecture/operating-loop.md` to
  `docs/architecture/rpi-traversal.md` and make it the exact semantic protocol
  owner.
- [ ] Leave `docs/architecture/operating-loop.md` as a short compatibility
  page that says the former name described only the traversal and links to the
  new owner. It must contain no duplicate contract.
- [ ] Update current references in `mkdocs.yml`,
  `scripts/generate-documentation-index.py`, `docs/architecture/index.md`,
  contract indexes, and current tests.
- [ ] Let historical plans, audits, releases, evidence, and changelog entries
  keep their original wording and old link; the compatibility page preserves
  resolution.

#### 1.3 Architecture and state contracts

- [ ] Update `docs/architecture/component-map.md` and
  `docs/architecture/ports-and-adapters.md` so callers, source systems,
  execution factories, AgentOps semantics, checks, and validators are distinct
  graph nodes with typed handoffs.
- [ ] Update `docs/contracts/skill-ports-and-adapters.md` to describe RPI as a
  traversal and external Goal/Mayor/factory systems as callers or execution
  orchestrators, never AgentOps-owned control levels.
- [ ] Update `docs/contracts/bounded-contexts.yaml`; replace "optional corpus
  and provenance" product language with the operations-layer boundary.
- [ ] Amend `docs/adr/ADR-0016-state-tiers.md` in place:
  - Beads/tracker owns work;
  - Git owns source/delivery history;
  - CASS and CM remain source systems;
  - `.agents/ao/` is requested proof, not a general knowledge lake;
  - `.agents/scratch/` is disposable work;
  - `.agents/projections/` contains named-consumer, manifest-stamped derived
    views;
  - no stored index duplicates a queryable source without measured need.
- [ ] Reconcile `docs/agents-dir-hygiene.md` with the amended ADR.
- [ ] Clarify in `docs/contracts/agents-documentation-authority.yaml` that the
  tracked root `MEMORY.md` is a temporary machine-consumed projection, not an
  AgentOps authority; retire it only after its current consumers are audited.

Acceptance for phase 1:

- One page owns each live claim.
- The compatibility page contains no normative copy beyond its redirect.
- No architecture page implies that AgentOps owns a factory's campaign or
  work graph.
- The state contract names authority, lifetime, provenance, and deletion
  semantics for every AgentOps-owned file class.

### 2. Remove live behavior that contradicts the product

Purpose: ensure the new category follows from what the repository actually
ships.

#### 2.1 Retire the knowledge-flywheel command family

- [ ] Remove `ao flywheel status` and `ao flywheel compare` from the live
  command tree, including `cli/internal/commands/flywheel/**` and their
  composition registration.
- [ ] Remove the complete live flywheel implementation and contract surface:
  - `cli/internal/flywheelapp/**`;
  - flywheel-only functions and tests in
    `cli/internal/quality/metrics_run.go` and `metrics_ops.go`;
  - flywheel-only types in `cli/internal/types/types.go`;
  - `clicontract.ProfileFlywheel` in
    `cli/internal/clicontract/metadata.go` and every command profile set that
    carries the bit;
  - `cli/testdata/compatibility-baseline/families/flywheel/`.
  Confirm no current import or profile consumer remains before deleting shared
  metric primitives. Keep historical evidence only where it explains removed
  behavior.
- [ ] Update `cli/internal/commands/eval/module.go` so optional eval manifests
  feed a declared evidence consumer, not a Knowledge Flywheel.
- [ ] Update CLI compatibility fixtures to record an intentional command-family
  retirement rather than freezing the old product claim forever.
- [ ] Add an explicit release-note fragment or current release documentation
  for the removal according to repository convention; do not rewrite the
  changelog's history.

#### 2.2 Contract the workflow adapters

- [ ] Replace `workflows/operating-loop.js` with a small compatibility
  tombstone that fails with a deterministic migration message:
  - one experiment -> `workflows/rpi.js`;
  - multi-Bead delivery -> `workflows/ship-beads.js` or a selected factory.
- [ ] Do not translate the legacy workflow's incompatible seven-move arguments
  into RPI automatically.
- [ ] Update `workflows/README.md` and workflow drift tests so the tombstone is
  not advertised as an active conveyor. The owning check is
  `scripts/check-workflow-drift.sh`; update its report-only workflow inventory
  because only `bdd-foundry.js` is currently blocking.
- [ ] Reword `workflows/rpi.js` as one traversal. Preserve its durable verdict
  behavior only by declaring the workflow return contract as the downstream
  consumer that requires `verdictPath`.
- [ ] Describe `workflows/ship-beads.js` as repository delivery orchestration
  outside the AgentOps semantic core.

#### 2.3 Fix current state writers

- [ ] Change `ao init` to create only current required state paths. Remove
  automatic scaffolding for generic session transcripts, search indexes,
  provenance ledgers, or handoff stores that have no declared consumer.
- [ ] Begin the writer audit at the known live entry points instead of
  rediscovering them:
  - `cli/internal/initapp/initapp.go` creates proof paths and gitignore entries
    for `index/`, `sessions/`, and `provenance/`;
  - `cli/internal/storage/**` and `cli/internal/commands/session/**` own the
    current session/index/provenance stores;
  - `cli/internal/config/config.go` carries legacy defaults for
    `.agents/learnings`, `.agents/patterns`, and `.agents/research`.
- [ ] Audit CLI config defaults and active commands that write
  `.agents/learnings`, `.agents/patterns`, `.agents/research`,
  `.agents/handoff`, or similar legacy roots. For each active writer, choose
  exactly one disposition:
  1. requested proof under `.agents/ao/`;
  2. disposable output under `.agents/scratch/<writer>/...`;
  3. manifest-stamped output under `.agents/projections/<consumer>/...`;
  4. caller-selected output path;
  5. retire the writer because it has no consumer.
- [ ] Update tests and config defaults with the chosen owner. Historical test
  vectors may retain legacy strings when they specifically exercise migration
  or denial behavior.
- [ ] Do not migrate or delete the live ignored `.agents/` tree.

#### 2.4 Remove copy-pinning ceremony

- [ ] Delete `scripts/check-thesis-stability.sh` and remove its grandfather
  entry from `scripts/.preamble-grandfather`. It pins narrative copy to a local
  `.agents/reconcile` snapshot and turns product evolution into artifact
  ceremony. There is no Makefile, CI, hook, or gate registration to unwind.
- [ ] Strengthen existing product-boundary tests instead of creating a new
  snapshot, decision packet, or gate framework.
- [ ] Update `cli/internal/commands/demo/module.go` from "one-pass AgentOps
  evidence loop" to "one RPI traversal" while preserving demonstrated
  behavior.

Acceptance for phase 2:

- `ao --help` has no live `flywheel` command.
- No live command computes or reports `COMPOUNDING`, `DECAYING`, or escape
  velocity as AgentOps product state.
- The legacy workflow refuses clearly and points to distinct supported shapes.
- A fresh fixture initialized by `ao init` contains no undeclared knowledge,
  session, handoff, or index store.
- No cleanup touches the operator's current `.agents/` contents.

### 3. Align `AGENTS.md` and source skills

Purpose: make agent behavior operationalize the architecture instead of merely
describing it.

#### 3.1 Repository operating contract

- [ ] Change the opening of `AGENTS.md` to the operations-layer definition.
- [ ] Rename "Core loop" language to "Standard RPI traversal" while keeping
  the exact one-pass Plan, Implement, fresh Validate, report-and-stop rules.
- [ ] Add the federated source-authority table in compact form.
- [ ] Make runtime adapters, factories, and context miners explicit peers in
  the graph whose native state AgentOps does not own.
- [ ] Preserve authority, mutation, freshness, independent-judgment,
  source-precedence, and single-writer constraints.
- [ ] Update `docs/agent-workflow-reference.md` as the detailed consumer of the
  operating contract.

#### 3.2 Core and product skills

- [ ] Update `skills/rpi/SKILL.md` description, triggers, and capability
  wording from "run/feed through the loop" to "coordinate one RPI traversal."
- [ ] Update `skills/domain/SKILL.md` to load the new canonical vocabulary.
- [ ] Update `skills/product/SKILL.md`, `skills/bootstrap/SKILL.md`, and
  `skills/doc/SKILL.md` so generated product/docs copy starts from the
  operations-layer category and preserves the ownership boundary.
- [ ] Keep `plan`, `implement`, and `validate` behavior unchanged unless a live
  phrase or output path conflicts with the new contracts.

#### 3.3 Context and evidence skills

- [ ] Update `skills/plan/SKILL.md`, `skills/research/SKILL.md`, and
  `skills/idea-genie/SKILL.md` to hydrate only the sources needed for the
  current decision and to return cited evidence, not a merged context store.
- [ ] Update `skills/cass/SKILL.md` so CASS supplies episodic evidence; remove
  claims that it feeds a corpus or owns a learning loop.
- [ ] Update `skills/status/SKILL.md` to report tracker, Git, factory/runtime,
  deterministic-check, and semantic-validation state separately.
- [ ] Move defaults in these source skills to canonical state locations:
  - `skills/codebase-recon/SKILL.md` -> `.agents/scratch/codebase-recon/...` or
    a caller-selected path;
  - `skills/handoff/SKILL.md` -> caller-owned handoff location or explicit
    requested proof, not a permanent generic store;
  - `skills/reverse-engineer/SKILL.md` and its existing implementation ->
    `.agents/scratch/reverse-engineer/...` or a caller-selected output;
    generated comparative views require projection manifests.

#### 3.4 Runtime/factory adapter skills

- [ ] Audit `skills/using-gc/SKILL.md`, `skills/using-flywheel/SKILL.md`,
  `skills/agent-native/SKILL.md`, `skills/swarm/SKILL.md`,
  `skills/ntm/SKILL.md`, and `skills/agent-mail/SKILL.md`.
- [ ] Preserve their legitimate orchestration language about the selected
  external runtime.
- [ ] State that the adapter cannot select AgentOps semantics, issue a binding
  verdict, or turn factory completion into delivery or validation proof.

#### 3.5 Whole-catalog audit

- [ ] Scan all 51 canonical `skills/*/SKILL.md` sources for inaccurate uses of
  `operating loop`, `knowledge flywheel`, `corpus`, `control plane`, and
  `orchestrator`.
- [ ] Edit only skills whose behavior or placement is inaccurate. Historical
  examples that are clearly labeled may remain.
- [ ] Update source metadata when descriptions, capabilities, effects,
  consumes, or produces change.
- [ ] Do not edit `skills-codex/**` or generated catalogs by hand.

Acceptance for phase 3:

- An agent following `AGENTS.md` chooses a source, traversal, runtime, and
  validator without assigning AgentOps external lifecycle authority.
- Source skills agree on the distinction between graph, traversal, protocol,
  and product.
- Context skills return evidence with source identity and freshness.
- Runtime skills expose native facts without minting semantic PASS.

### 4. Align public, package, and current documentation surfaces

Purpose: make the product understandable before installation and consistent
after installation.

#### 4.1 Public entrypoints

- [ ] Rewrite the top of `README.md` with this copy:

  > AgentOps is the operations layer for agentic engineering.
  >
  > AgentOps connects intent, coding agents, software factories, context
  > sources, and independent judgment through portable skills and evidence
  > contracts without replacing the systems that own work, execution, or
  > delivery.

- [ ] Show the federated graph before introducing RPI; label RPI as the
  standard one-experiment traversal.
- [ ] Replace "installs the corpus" with accurate skill-bundle language.
- [ ] Keep quickstart, install choices, software-factory options, evidence
  contract, and CLI boundary, but remove wording that makes AgentOps the
  factory.
- [ ] Update `PRODUCT.md`, `GOALS.md`, and `PROGRAM.md` only where needed to
  adopt the category and topology; preserve their already-correct small
  boundary.

#### 4.2 Package and website metadata

- [ ] Update `.claude-plugin/plugin.json`,
  `.claude-plugin/marketplace.json`, `.codex-plugin/plugin.json`, and
  `mkdocs.yml` with the canonical description and keywords.
- [ ] Remove `knowledge-flywheel` and equivalent obsolete product keywords.
- [ ] Update `.goreleaser.yml`, `cli/Formula/agentops.rb`, and `cli/README.md`
  from "Knowledge Flywheel CLI" to a narrow description of the optional `ao`
  checks/linking CLI.
- [ ] Update `docs/templates/README.md`, `docs/software-factory.md`,
  `docs/trust-factory.md`, `docs/dependencies.md`, and
  `docs/seed-definition.md` where they currently teach the old category.
- [ ] Add a short current-state note to `docs/origin-story.md` if needed; do not
  rewrite the historical narrative.

#### 4.3 GitHub About copy

After the implementation PR merges, an explicitly authorized operator runs:

```bash
gh repo edit boshu2/agentops \
  --description "The operations layer for agentic engineering — portable skills and contracts connecting intent, agents, software factories, and independent judgment."

gh repo view boshu2/agentops --json description,homepageUrl,url
```

This is a post-merge external mutation. The implementation agent must not run
it while working the branch.

Acceptance for phase 4:

- README, product contract, package manifests, Homebrew copy, docs site, and
  planned GitHub description all use the same category.
- Public copy names external systems positively without claiming ownership of
  them.
- No current entrypoint calls the skill collection a corpus or AgentOps a
  knowledge flywheel.

### 5. Regenerate every owned projection

Purpose: update derived views only after all source owners are stable.

- [ ] Run:

  ```bash
  bash scripts/regen-all.sh
  ```

- [ ] Review every generated diff. Expected families include:
  - `docs/documentation-index.md`;
  - `docs/SKILL-ROUTER.md`;
  - skill graph, domain map, and context map;
  - `skills/catalog.json` and `registry.json`;
  - `skills-codex/**` and Codex manifests/hashes;
  - `cli/docs/COMMANDS.md`;
  - command headings and CLI surface inventory.
- [ ] Reject any generated old claim by fixing its source owner and rerunning;
  never patch the projection.
- [ ] Prove idempotence:

  ```bash
  bash scripts/regen-all.sh --check
  git diff --check
  ```

Acceptance for phase 5:

- Generation is clean and repeatable.
- Generated docs refer to `rpi-traversal.md`, not the compatibility page.
- Generated skill descriptions carry the new vocabulary without new hard core
  dependencies.

### 6. Strengthen existing conformance checks

Purpose: keep the alignment true without creating another governance system.

- [ ] Update these existing checks and fixtures:
  - `tests/scripts/agents-operating-contract.bats`;
  - `tests/scripts/agentops-product-boundary.bats`;
  - `scripts/check-cathedral-cut-conformance.py`;
  - `scripts/check-doc-skill-refs.sh` and its tests;
  - `scripts/check-workflow-drift.sh` and its tests/fixtures;
  - CLI contract/compatibility fixtures affected by command retirement;
  - documentation authority/index checks affected by the renamed page.
- [ ] Add positive assertions for:
  - the operations-layer category;
  - federated integration graph;
  - semantic work-and-proof protocol;
  - RPI traversal;
  - external lifecycle ownership;
  - canonical `.agents/` writer destinations.
- [ ] Add negative assertions over current-authority surfaces for:
  - AgentOps as an operating loop, operating system, global control plane, or
    execution orchestrator;
  - Knowledge Flywheel as a current product or CLI state;
  - automatic retry, continuation, delivery, promotion, or next-action
    authority;
  - active writers to noncanonical `.agents/` roots.
- [ ] Keep historical allowlists path-based and narrow. Do not fail dated
  records for accurately preserving old terminology.
- [ ] Test projection deletion/rebuild in a temporary fixture. Never prove
  disposability by deleting the operator's live ignored data.

Acceptance for phase 6:

- Every new negative check has a planted-negative test proving it can fail.
- Checks judge current owners rather than frozen narrative snapshots.
- No new process packet or manual approval artifact is required.

### 7. Validate the exact final change

Run focused checks first:

```bash
python3 scripts/check-cathedral-cut-conformance.py
bats tests/scripts/agents-operating-contract.bats
bats tests/scripts/agentops-product-boundary.bats
bash scripts/check-doc-skill-refs.sh
bash scripts/check-contract-compatibility.sh
bash scripts/check-honest-voice.sh
bash scripts/regen-all.sh --check
make docs-check
make local-ci-fast
```

Before merge, run the complete repository gate:

```bash
make local-ci
```

Then use one fresh, author-distinct Validate context over the exact final
subject. Its report must answer:

1. Do public claims follow from shipped behavior?
2. Can a reader distinguish the operations layer from an execution
   orchestrator or software factory?
3. Is RPI clearly one traversal rather than the global topology?
4. Are Beads, Git, CASS, CM, factories, checks, and semantic judgment assigned
   distinct authority?
5. Can disposable projections be rebuilt from named sources with identities,
   citations, and freshness?
6. Are factory-complete, checks-green, and AgentOps-PASS visibly distinct?
7. Did the change avoid rewriting history or touching live ignored data?
8. Does every in-scope criterion have direct evidence and no unchecked
   surface?

`PASS` is required. `FAIL` or `NOT_PROVEN` stops the merge; it does not
authorize an automatic repair loop.

## PR shape and closeout

Use one branch and one PR because this is one identity migration whose shared
copy, command tree, skill metadata, and generated projections must remain
coherent. Commit by coherent implementation phase if helpful, but do not split
authority changes and generated consequences across independently mergeable
PRs.

The PR description should contain only:

- the old and new category in one sentence;
- the live behaviors retired or redirected;
- the state-authority boundary;
- generated surfaces changed;
- exact checks run and outcomes;
- the fresh validation result;
- residual risk, especially external consumers of removed `ao flywheel` or
  the legacy workflow.

After merge, update the GitHub About description with the authorized command in
phase 4.3 and verify it. No other post-merge automation is part of this plan.

## Residual risks to disclose, not solve with ceremony

- External users may still call `ao flywheel`; the release note and intentional
  compatibility-fixture change make the break explicit.
- Historical terminology will continue to appear in dated evidence and plans;
  this is accurate history, not current drift.
- Existing ignored `.agents/` directories may remain messy until the operator
  authorizes a separate, recoverable migration.
- A consumer may still depend on root `MEMORY.md`; keep it until that consumer
  audit proves removal safe.
- The word "graph" can tempt implementation of a graph database. The product
  needs typed relationships and source identity, not a new mandatory storage
  engine.
- The operations-layer category can tempt control-plane expansion. The
  non-ownership tests and fresh validation must reject that drift.
