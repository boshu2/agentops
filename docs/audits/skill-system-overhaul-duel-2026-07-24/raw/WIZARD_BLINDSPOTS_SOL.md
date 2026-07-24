# SOL blind-spot probe

Date: 2026-07-24
Identity: SOL
Evidence reviewed: both sealed proposals, both blind score files, both reveal
reactions, both rebuttals, and both steelmen, all completely

The five items below were absent from both original proposals. Some became
visible only because the later duel artifacts composed mechanisms that the
sealed proposals had considered separately.

## Blind spot 1 — Proof epochs have identity but no trust-activation protocol

### Why both wizards missed it

Both proposals treated proof comparability as a static identity problem:
repair the kernel, hash the validator and schemas, freeze them, and label later
verdicts by epoch. Neither proposal specified who is trusted to activate a new
proof contract when the candidate being judged includes the validator, schemas,
golden corpus, or reader that will define the next epoch.

The later steelmen expose a second half of the same gap. One suggests converting
a terminal `NOT_PROVEN` into PASS by fresh re-validation over the same frozen
subject. The core contract has no validation-only continuation lane, and the
verdict schemas have no lineage rule for multiple judgments over the same
intent/subject pair. Exact artifact identity alone does not say which judgment
is authoritative.

### Affected architecture

- Layer: experiment/evidence kernel
- Seams: fresh Validate, durable verdict, proof-contract evolution
- Skills: `rpi`, `validate`, `plan`, `implement`
- Schemas/readers: verdict, RPI report, subject manifest, Go verdict reader
- Migration controls: kernel tranche, proof epochs, T7 verdict collection

### Failure mechanism and earliest observable signal

The candidate epoch-1 validator judges the change that creates epoch 1, so it
can approve its own weakened freshness or evidence rules. All later verdicts
carry an exact epoch digest, but the epoch itself entered the trust chain
circularly.

Alternatively, a tranche records `NOT_PROVEN`, an evidence gap later closes,
and a standalone fresh Validate emits PASS for the same intent and subject.
Nothing declares whether the second judgment is a new RPI experiment, a
replacement, an additional opinion, or an impermissible continuation. A
consumer can select the favorable verdict.

The earliest signals are:

- the first binding verdict naming proof contract E1 was itself produced only
  by E1 while E1’s files were in the candidate subject; or
- two verdicts share acceptance and subject digests but have no typed
  transition/predecessor relation and no distinct RPI intent.

### Falsifier

This risk is falsified if a currently active, immutable validator outside the
candidate subject already:

1. qualifies a candidate proof contract;
2. records old and new contract identities;
3. activates the new contract only after a binding old-contract verdict; and
4. rejects unlinked duplicate judgments over one intent/subject.

No duel artifact identified such a mechanism.

### Smallest non-churning mitigation

Add a narrow `proof-contract-transition.v1` activation record, not a new skill
or lifecycle. It binds:

- prior active proof-contract digest;
- candidate proof-contract digest;
- exact candidate subject manifest;
- qualification corpus digest;
- binding verdict minted under the prior active contract;
- activation timestamp and fresh validator identity.

The candidate contract may emit shadow results during qualification, but they
are nonbinding until activation.

Do not add an implicit validation-only retry. Resolve known proof-chain
`UNKNOWN` edges before starting the RPI whose verdict must count at T7. A
terminal `NOT_PROVEN` remains terminal. Any later attempt is a new explicit RPI
with its own intent and report; T7 preserves both results and never silently
upgrades the first.

### Classification

**Tranche blocker — exact kernel/proof-epoch tranche.**

## Blind spot 2 — Rollback has a third baseline: dirty pre-publication bytes

### Why both wizards missed it

The sealed proposals debated direct serialized regeneration versus a staged
publisher. Their rollback models implicitly chose between Git HEAD and the last
valid generated manifest. Neither modeled the state already present in this
repository: generated owner paths can be tracked, modified, and intentionally
different from both HEAD and the last committed generation before publication
starts.

The later restore-on-failure proposal makes the gap concrete:
`git restore --source=HEAD` can erase pre-existing caller-owned or
reviewed-but-unmerged generated changes. Staging prevents validation-time live
writes, but a crash during multi-file replacement still needs the exact
pre-publication state, not an assumed clean baseline.

### Affected architecture

- Layer: migration operations and generated projection publication
- Surfaces: mesh outputs, Codex/Gemini twins, catalogs, hashes, manifests,
  router/docs projections
- Skills/tools: `skill-builder`, `converter`, `bootstrap`, `regen-all`
- Migration controls: owner map, rollback, staging experiment, T7 regeneration

### Failure mechanism and earliest observable signal

The publisher fails after replacing some owned paths. Its recovery restores
HEAD or re-renders the last manifested source. Pre-existing dirty generated
bytes disappear, even though the migration was required to preserve unrelated
work. The resulting tree is internally consistent and therefore hides the
loss.

The earliest signal is an owner-mapped path marked modified before generation,
or a preflight digest that matches neither HEAD nor the current generation
manifest. FABLE’s rebuttal already observes modified tracked projection
surfaces in the current worktree; neither original rollback design classified
that state.

### Falsifier

The risk is falsified if every owner-mapped generated path is proven clean,
tracked, reproducible from the active source digest, and equal to the current
generation manifest before every publication attempt—and the publisher refuses
to run otherwise.

### Smallest non-churning mitigation

Before any live replacement:

1. classify each owned path as `CLEAN_CURRENT`, `DIRTY_PRESERVE`, `MISSING`, or
   `UNOWNED`;
2. snapshot exact pre-run bytes plus file mode and symlink target for every
   `DIRTY_PRESERVE` or currently owned path into a temporary recovery bundle;
3. abort on unowned collisions;
4. restore failures from that bundle, never from HEAD by default; and
5. delete the bundle only after the manifest-last commit validates.

Use the same exact pre-run bundle in the staging-versus-restore fault-injection
experiment. This adds a truthful rollback baseline without choosing the
larger publication design in advance.

### Classification

**Tranche blocker — projection publisher / first catalog-wide regeneration.**

## Blind spot 3 — Locally valid triggers can produce a globally unusable portfolio

### Why both wizards missed it

Both proposals improved per-skill trigger contracts: positive cases, negative
boundaries, ambiguity declarations, and “rank first for the skill’s own
phrases.” That tests each root in isolation or against named neighbors. It does
not test the user-facing portfolio under realistic compound requests,
abstention, aliases, and progressive disclosure.

The metadata overhaul changes descriptions, routing keys, layers, seams,
aliases, and generated router organization simultaneously. A catalog can be
49/49 valid while generic skills dominate selection and specialist roots become
practically unreachable.

### Affected architecture

- Layer: product/support routing and portfolio usability
- Seams: admission, option shaping, standalone invocation, adapter selection
- Skills: all 49, especially high-overlap families such as
  `research`/`codebase-recon`/`reverse-engineer`,
  `premortem`/`scope`/`council`/`reality-check`, and runtime adapters
- Surfaces: compiler trigger data, generated `SKILL-ROUTER`, aliases,
  `ao skills` discovery

### Failure mechanism and earliest observable signal

Every skill passes its own fixture, but multi-intent requests route to a broad
generic skill, route to two incompatible authorities, or start a runtime
adapter when an advisory route was required. A retained specialist has a
distinct contract and probe yet receives no realistic selection, so the
portfolio keeps the cognitive cost of a root without delivering its
specialization.

The earliest signals are:

- a retained skill has zero top-k reachability in the portfolio corpus;
- the new router materially increases ties, fallback, or wrong-authority
  selections versus the current router;
- old and new aliases select different canonical behaviors; or
- compound requests bypass the required clarification/abstention path.

### Falsifier

This risk is falsified if every skill is invoked only by an exact explicit slug
and no generated description, router, model, alias, or CLI discovery surface
participates in selection. Otherwise a portfolio-level interaction exists and
must be tested.

### Smallest non-churning mitigation

Compile the existing per-skill trigger fixtures into one global shadow-routing
corpus. Add only a small adversarial cross-product for nearest-neighbor and
multi-intent collisions. For current and v3 routers record:

- canonical selection or required abstention;
- top-k reachability per retained skill;
- ambiguity and fallback rates;
- wrong-authority selections;
- alias equivalence; and
- progressive-disclosure payload selected.

T7 need not demand identical routing, but every changed decision must be
explained by the new contract and no retained skill may be unreachable without
an explicit retirement review.

### Classification

**T7 convergence blocker.**

## Blind spot 4 — Static consumer searches do not prove a rename or retirement is unused

### Why both wizards missed it

Both proposals relied on repository grep, generated-reference integrity,
nearest-neighbor probes, and bounded compatibility aliases. Those checks prove
local reference closure. They do not observe direct skill invocations, saved
prompts, shell automation, other repositories, raw schema URLs, cached runtime
images, or consumers that construct a slug dynamically.

This affects more than `goals` to `fitness`. It applies to retiring `shared`,
conditionally folding runtime adapters, deleting old packet schemas, removing
legacy fields, and eventually deleting compatibility views.

### Affected architecture

- Layer: portfolio evolution and release compatibility
- Seams: skill discovery/routing, external schema consumption, generated image
  loading
- Skills/contracts: `goals`/`fitness`, `shared`, conditionally retained runtime
  adapters, dead packet schemas, retired tier/role views
- CLI/projections: `ao skills`, generated routers, Codex/Gemini images,
  compatibility readers

### Failure mechanism and earliest observable signal

T7 proves no in-repo reference remains and hard-deletes the old surface. A
downstream automation still invokes `goals`, loads `shared`, reads a raw packet
schema, or expects the legacy catalog field. Its first post-release execution
fails, often outside the repository where the migration’s gates cannot see it.

The earliest observable signal is a hit on a deprecated alias/tombstone, a
downstream not-found/schema error, or a compatibility reader observing a legacy
field request. Without an observation surface, the earliest signal is merely a
user report after removal.

### Falsifier

The risk is falsified if the project can enumerate every distributed runtime
image, repository, automation, and schema consumer; prove all invocation is
static and locally searchable; migrate them all; and show zero legacy
resolution across one representative usage window.

### Smallest non-churning mitigation

Separate **semantic retirement** from **physical deletion**:

- T7 removes the old surface from advertising and authority.
- A generated compatibility resolver maps the old slug/field to the canonical
  replacement, or returns a typed tombstone naming the new contract when no
  equivalent exists.
- The resolver records only aggregate local deprecation counters—no prompt
  content—and the release process scans every declared distribution image and
  known consumer repository.
- Physical deletion requires zero observed hits for a declared window plus
  owner confirmation; otherwise it remains a post-migration compatibility
  shim with an explicit removal date.

This preserves one canonical source and does not revive retired semantics.

### Classification

**T7 convergence blocker for claiming hard deletion.** Soft retirement may
converge while the compatibility shim enters a bounded watch window.

## Blind spot 5 — Declared effects are not reconciled to observed effects

### Why both wizards missed it

Both proposals made effect metadata much stronger: typed kind, scope,
authorization, cleanup, shared CLI vocabulary, and hostile compiler fixtures.
They still treated effects primarily as declarations plus skill-specific
behavior probes. Neither required the runtime evidence transaction to compare a
skill’s declared effects with what execution actually did.

The current kernel derives repository changed paths, but host configuration,
credential changes, spawned process trees, external mutations, runtime
sessions, and cleanup obligations live outside that manifest. A perfectly
typed declaration can still be false in production.

### Affected architecture

- Layer: implementation, runtime, support, and evidence
- Seams: Implement delegation, runtime transport, host mutation, fresh Validate
- Skills: the roughly 30 effect-honesty repairs, especially
  `account-rotation`, `bootstrap`, `cc-hooks`, `dcg`, `ms`, `rch`, `sbh`,
  `swarm`, `using-gc`, generators, and mutating Security modes
- CLI surface: shared process runner, command effects/dry-run contracts,
  capability output

### Failure mechanism and earliest observable signal

A skill declares read-only or a contained repository write. Its adapter starts
an undeclared child, modifies host state, switches credentials, contacts an
external service, or fails required cleanup. The compiler and local happy-path
probe pass. Validate sees only the repository manifest and accepts the claimed
effect boundary.

The earliest signal is any observed changed path, process, adapter call,
credential identity, host configuration delta, or cleanup result not covered by
the declared and authorized effect set. A missing observation channel is itself
an early `UNKNOWN`, not proof of no effect.

### Falsifier

The risk is falsified if the selected runtime already sandboxes or records every
declared effect class, binds the observations to the exact invocation, and
rejects execution whose observed effects exceed the contract. No original
proposal identified such enforcement.

### Smallest non-churning mitigation

Add an `effect-receipt.v1` to the existing implementation/runtime evidence
handoff. Reuse facts already available where possible:

- runtime-derived changed paths;
- process IDs/tree termination results from the shared runner;
- adapter operation IDs and external mutation result;
- before/after credential or host identity for relevant support skills; and
- cleanup completion or explicit opacity.

Validate checks that observed effects are a subset of declared, authorized
effects and that every `cleanup: required` effect has a receipt. If an effect
class is not observable on the current platform, the result is
`unchecked_required`/`NOT_PROVEN` for a no-effect or cleanup claim, not PASS.
This is an evidence extension, not a runtime policy engine or second validator.

### Classification

**Tranche blocker — every tranche that claims effect honesty for effectful
skills.**

## Single unproven assumption the final plan still depends on

The combined plan still assumes that the known-incoherent current proof kernel
can safely bootstrap the first trusted proof epoch. Neither sealed proposal,
nor the later consensus, identifies an already trusted external validator that
can bind the transition from epoch 0 to epoch 1 without allowing the candidate
validator to certify itself. Until that trust anchor or activation protocol is
demonstrated, every later exact digest can identify a proof contract precisely
without proving that the contract entered the chain legitimately.
