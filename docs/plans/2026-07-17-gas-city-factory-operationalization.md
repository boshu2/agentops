# AgentOps 3.3 Gas City Factory Reliability

```yaml
schema_version: agentops-plan.v1
plan_id: agentops-3.3-gas-city-factory-reliability
release: 3.3.0
status: duel-approved-implementation-gated
owner: AgentOps maintainers
intent_owner: this document
unit_of_work: bead
depends_on: []
first_acceptance_check: tests/scripts/gc-agentops-toolchain.bats
```

## Intent

Ship an optional AgentOps Gas City adapter that bootstraps repeatably, processes
multiple independent beads through fresh semantic validation, and delivers
admitted work through PR, hosted CI, moving-main rebase, and merge without
routine operator intervention.

This is a release behavior, not a rewrite of AgentOps or Gas City. AgentOps
continues to own one bounded Plan -> Implement -> fresh Validate -> durable
verdict experiment. Gas City supplies caller-side routing, sessions, pools,
health, events, and work persistence. The factory connects those primitives
without moving AgentOps lifecycle authority into the adapter.

## Product decision

The release has one agentic semantic-production loop followed by one
asynchronous mechanical delivery state machine, joined by one immutable
handoff:

1. **Semantic production:** intake bead -> Fable Mayor -> Sol plan review ->
   Terra/Opus implementation -> fresh Sol validation -> terminal semantic bead.
2. **Mechanical delivery:** `PASS` -> `admission-certificate.v2` ->
   replayable prepared handoff -> linked delivery bead -> crash-only
   deterministic delivery reducer on the native GC substrate ->
   PR/CI/rebase/merge -> landed receipt.

The original semantic bead closes after its exact candidate receives
`PASS | FAIL | NOT_PROVEN`. Delivery never reopens that bead. If moving `main`
or another admitted candidate makes delivery semantically incompatible, the
delivery bead becomes terminal and the Mayor receives a newly identified
successor bead.

`main` is never frozen. There is no branch-wide mutex and semantic validation
does not wait for the Refiner to merge anything.

The delivery state machine is not an optional embellishment. Removing it would
either keep the semantic bead open until delivery, coupling throughput to CI
and merge, or close the bead without a durable owner for admitted work. It is
not a second agentic loop or model-driven factory: it is one linked bead plus
short deterministic reconciliations. `Refinery` names the delivery subsystem;
`Refiner` names only the triggered Fable ambiguity role.

The routine delivery engine is typed `ao` Go product code invoked by thin GC
exec-order glue. Each invocation re-reads durable state, reconciles remote
reality, performs at most one effect, records one receipt, reduces one legal
transition, and exits. A periodic sweep is the correctness path. An event wake
is latency-only and ships only if the pinned GC/Beads capability gate proves
safe single-flight behavior.

## Stable-first boundary

The default toolchain is the official Gas City v1.3.5 release at
`8ffc009ded781a2ada2077f3a29bd712b2def0bf` paired with official Beads v1.1.0.
The lock records resolved commits and release-asset checksums.

Reuse these stable Gas City mechanisms instead of reproducing them in AgentOps:

- `gc init`, versioned imports, pack composition, and `packs.lock`;
- supervisor registration, reconciliation, health, doctor, and managed-Dolt
  recovery;
- durable `gc sling` routing through `gc.routed_to`;
- scale-from-zero pools, named sessions, and `gc hook --claim`;
- native session/worktree lifecycle, Orders, formulas, convoys, mail, events,
  API/SSE, dashboard, and OTel metrics/logs.

The bundled Gastown Refiner formula is the delivery-policy provenance baseline,
not a callable deterministic engine. Formula-derived current-target rebase,
target-regression distinction, zero-diff refusal, `force-with-lease`, PR
creation/adoption, and remote verification are AgentOps custom behavior unless
an unchanged official asset is executed directly. Map every reimplemented
block to exact pinned source lines and differential fixtures. Do not call that
official execution reuse.

The official lifecycle and PR terminal policy are not adopted unchanged. The
official Refiner expects an open implementation bead and closes PR-mode work at
PR publication. AgentOps instead closes the semantic bead after validation,
creates a linked open delivery bead, and keeps that delivery bead open until
the PR lands or delivery reaches a terminal outcome. Routine delivery beads
must not match the stock Refiner assignment query. The adapter reuses stable GC
supervision, Beads, Orders, health, teardown, and triggered session lifecycle;
it owns its deterministic delivery policy without modifying the GC Go fork.

Stable v1.3.5 has no executable propulsion subsystem. An explicitly slung,
routed bead wakes the Mayor through native demand and claim behavior. Do not
build a second scheduler or use nudges as work routing.

Gas City upstream PR #3985 at
`347c66b1caac551c31212d2d288847d9aff8fe04` is the only admitted fork candidate.
It may replace v1.3.5 only when the exact teardown reproducer fails on official
stable and passes on that commit. No unpublished or unsubmitted local fork is
releaseable. Fork `main` must remain identical to upstream `main`; verified bug
work lives only on contribution branches with an upstream issue or PR.

Moving the reducer into `ao` makes that binary part of the factory toolchain.
Qualification records its version, source commit, content digest, and resolved
`AO_BIN` path. A mismatch is a deterministic refusal and a changed reducer
digest creates a new qualification identity.

## Role and authority contract

| Role | Runtime policy | Allowed authority |
|---|---|---|
| Mayor | Fable 5, adaptive | Interpret intake, decompose, propose successors |
| Plan reviewer | Sol, high | Bind graph, scope, dependencies, and execution policy |
| Product executor | Terra, high | Default implementation writer |
| Challenger/overflow | Opus 4.8, medium | Mixed-runtime work and bounded overflow |
| Candidate Validator | Sol, high, fresh and read-only | Issue exact `verdict.v2` |
| Refiner | Fable 5, adaptive, min 0 / max 1 | One nonbinding ambiguity finding only; dispatch remains closed until GC33-4 proves process isolation |
| Luna | Luna, high, support-only | Read-only status, log, and CI summaries |

Remove legacy Sol Mayor, Opus plan-reviewer, and Sol Refiner routes. Use native
work queries and claim behavior unless a deterministic test proves a missing
stable capability. Every binding artifact records the requested and actual
model, reasoning, provider, context identity, and fallback. Silent fallback is
`NOT_PROVEN`. Claude-backed Fable and Opus roles use the bounded interactive
provider path; this plan never invokes `claude -p` or `claude --print`.

Luna has `min=0`, `max=1`, and no Git, verdict, queue, merge, or lifecycle
authority. Remove Luna from the default pack if the live canary cannot show one
unique support result that changed an operator or controller action.

## Refinery subsystem shape

`Refinery` names the asynchronous mechanical delivery state machine. `Refiner`
names only the Fable reasoning role triggered for genuine ambiguity within
that subsystem. There is no second model loop or resident Refiner worker pool.

The ordinary path is mechanical: validate the certificate, reconcile one
bead-native delivery state, construct a current-base epoch in an ephemeral
worktree, run deterministic repository gates, create or adopt one PR, require
nonempty hosted CI, execute the one selected protected-merge effect, and verify
the landed SHA/tree. It must not require Fable or Sol merely to move a known
state machine forward.

Routine `delivery.v1` beads and `ambiguity-request.v1` beads are disjoint staged
types. A new object is non-routable until all required metadata validates; one
publication transition makes it visible to exactly one selector. The fully
composed city config must prove the delivery selector intersects no model
`work_query` or `sling_query`. Qualification records zero Refiner starts and
zero model claims for clean delivery.

The reducer is crash-only. It holds no state across invocations, performs at
most one external mutation, reconciles before replay, and may be killed at any
instruction with the next invocation converging. No process, claim, worktree,
or fence is held across CI, model, forge, or human waits.

Wake Fable only for an ambiguity that deterministic facts cannot classify:

- ordering several admitted changes with a real interaction;
- distinguishing a mechanical conflict from changed product meaning;
- proposing the scope of a delivery-repair bead;
- recommending a successor shape to the Mayor.

Fable never owns the queue, edits code, runs Git effects, merges, closes beads,
or issues a verdict. The deterministic adapter may reject its proposal.

Delivery failures have three semantic lanes plus infrastructure replay:

| Failure class | Required transition |
|---|---|
| Clean rebase and green gates | Continue automatically without a reasoning wake |
| Mechanical conflict requiring changed bytes | Create a delivery-repair bead for the shared Terra/Opus writer fabric, then obtain a fresh Sol verdict over the integrated subject |
| Semantic incompatibility with current `main` or earlier landed work | Terminally stop the delivery bead and route a newly identified successor request to the Mayor |
| Infrastructure or forge interruption | Bounded idempotent replay of the same delivery identity; no semantic successor |

Sol is triggered after byte-changing repair or when current-base evidence shows
a shared interface, generated artifact, or behavioral interaction. A clean
tree-preserving rebase does not automatically spend a Sol judgment.

A byte-changing repair always creates a new exact subject and therefore
requires a fresh binding `verdict.v2` plus new admission evidence. The only
Fable ambiguity artifact is nonbinding `ambiguity-advice.v1`; until GC33-4
proves process isolation it cannot be dispatched or transition a bead.

Every waiting state has a finite operator-configured deadline. Expiry becomes
an operator-visible `stalled` state with a structured reason, never a silent
wait, retry storm, or automatic model wake. Status surfaces oldest delivery-WIP
age.

## Writer and queue contract

There are two logical writer queues and one shared Terra/Opus writer fabric:

- product work;
- byte-changing delivery-repair work.

The writer-admission policy applies:

- capacity 1: at most two repair admissions, then one product admission;
- capacity 2: one eligible slot per queue, with borrowing when one is idle;
- capacity 3: one slot per queue and a third favoring repair;
- capacity 4: one slot per queue and two additional slots favoring repair;
- within a queue: dependency readiness, oldest admission, stable bead ID.

Each writer receives its own worktree, branch, index, lease, and declared write
scope. Shared/generated paths and declared atomic groups serialize by conflict
domain. Stash, reset, peer-branch checkout, mutation of the primary worktree,
and writes outside the admitted scope fail the bead's process-policy check.

The delivery reducer is not a third writer queue and owns no general scheduler.
At 3.3 width two, one rig-scoped sweep enumerates ready delivery beads in stable
ready-time and bead-ID order and advances each selected bead by at most one
transition.

## Evidence and delivery interfaces

`program-graph.v1` binds the intent digest, nodes, dependencies,
`max_parallel`, role/model policy, write scopes and generated companions,
`delivery_group_id`, and `prefix_safety = safe | atomic_group |
externally_gated`.

Only `PASS` emits `admission-certificate.v2`. The certificate binds the source
bead, intent, immutable candidate/store identity, commit/tree/content digests,
complete changed-path manifest, verdict and evidence digests, author/Validator
attestation, delivery group, and prefix-safety policy.

The handoff derives a deterministic `handoff_id` and expected delivery-bead
identity, seals `handoff-prepared.v1`, terminalizes the semantic bead with exact
references, creates or discovers one initially non-routable delivery bead,
validates and publishes it, then seals `handoff-committed.v1`. The periodic
sweep reconciles every intermediate crash state. No transaction is claimed
across the evidence and Beads stores, and no delivery effect is legal before
terminal semantic PASS and committed handoff are both observable.

The reducer creates one linked delivery bead per certificate by default.
Explicit atomic groups may share one delivery unit. Delivery progresses by
base-sensitive epochs:

`queued -> preparing -> branch_ready -> pr_open -> ci_wait -> rebase_needed ->
preparing | merge_eligible -> merge_requested | merge_armed -> landed`

The exact release schema contains `merge_requested` or `merge_armed`, never
both. Waiting and terminal alternatives are `repair_wait`, `manual_review`,
`stalled`, `delivery_failed`, `successor_required`, and `cancelled`.
Artifacts are immutable evidence referenced by bead metadata; they are not a
second lifecycle ledger.

Deployment policy exposes:

- `delivery.mode = auto` by default: create/adopt PR, require hosted CI,
  reconcile moving `main`, rebase/rebuild, execute the selected single merge
  actor without admin bypass, and verify landed SHA/tree;
- `delivery.mode = manual`: create/adopt the PR, require hosted CI, and remain
  nonterminal in `manual_review` until external merge or cancellation is
  observed.

Remote mutations use stable idempotency keys and reconcile remote reality
before replay. The typed AgentOps reducer owns certificate admission,
linked-bead lifecycle, Git/PR/CI/merge effects, and landed verification while
stable GC supplies the runtime substrate. Fable and Sol remain triggered
judgment roles, not transition engines.

## OTel baseline

Remove the blanket `OTEL_SDK_DISABLED=true`. The deployment owns
`telemetry.mode = auto | required | off`, defaulting to `auto`, and always sets
both `GC_OTEL_METRICS_URL` and `GC_OTEL_LOGS_URL` together when enabled.

`auto` probes the documented VictoriaMetrics/VictoriaLogs endpoints and emits
one durable degraded result when unavailable. `required` is used for release
qualification. The 3.3 canary must observe lifecycle, sling/pool, and Beads
signals through both endpoints.

3.3 does not ship a collector stack, Grafana dashboards, distributed traces,
or model-cost analytics. Those belong to the 3.3.1 plan.

## Allowed write scope

Implementation is limited to these ownership classes:

- AgentOps GC deployment sources under `deploy/gc/`, including lock/provenance,
  bootstrap, teardown, reliability, and their generated qualification outputs;
- focused `ao gc delivery` command and internal Go packages under `cli/cmd/ao/`
  and `cli/internal/`, plus the generated command documentation owned by that
  surface;
- the `agentops-factory` and thin `agentops-executor` pack sources, including
  pack-generated schemas, command projections, role prompts, and doctor checks;
- Gas City architecture, execution-adapter, operations, release, migration,
  and plan documentation describing this optional adapter;
- focused Go, Python, Bats, schema, and release-gate tests asserting those
  surfaces;
- generated documentation/catalog projections only through their owning regen
  commands.

Gas City or Beads source is outside scope except for a separately reproduced,
upstream-filed bug branch. AgentOps core experiment semantics, unrelated
skills, unrelated CLI commands, and operator-owned dirty changes are outside
scope.

## Complexity admission

The delivery product surface is limited to one transition/reconciliation core,
one certificate/handoff adapter, one Beads adapter, one Git/worktree adapter,
one forge/CI/merge adapter, and immutable receipt/capsule schemas. The pack may
configure and invoke those commands; it may not own lifecycle Python.

The release prohibits a resident delivery daemon, webhook control plane,
general scheduler, second merge engine, private lifecycle ledger, default
trains, nested integration courts, dynamic delivery rigs, or model-authored
Git/queue/lifecycle transitions. A thin vertical slice measures the actual
surface before full implementation. A projection beyond roughly 3,000
non-test Go lines is a re-scope tripwire, not a correctness target.

Before each implementation bead freezes its scope, inspect:

- imported pack composition and any generated schema/command projections;
- mirrored or provider-specific role definitions and prompt templates;
- `skills-codex/` or other generated skill parity if a skill source changes;
- release notes, changelog, migration, command docs, and docs-site projections
  asserted by release gates;
- Python/Bats fixtures that embed schema versions, role names, CLI output,
  toolchain identities, or config snippets;
- generated-path companions and atomic groups in the target repository.

Scope admits the owning source plus every output of the owning regeneration
command as a class. Do not hand-enumerate whichever generated files happened
to change in one run.

## Finite bead graph

| Bead | Behavior and completion evidence |
|---|---|
| GC33-0 Clean baseline | Inventory forks, branches, worktrees, toolchains, processes, Dolt servers, and experiment roots. Keep the v17 city suspended. Give every known error an owner, reproducer, disposition, and release relevance before safely removing obsolete owned state. |
| GC33-1 Provenance/bootstrap | Select official v1.3.5/Beads v1.1.0 by default; qualify the single #3985 exception; pin the `ao` reducer binary; prove absolute binary identity and clean, idempotent bootstrap/teardown. |
| GC33-2 Substrate and contracts | Run the real Beads/GC capability envelope; choose in-place or successor-bead epochs; finalize graph, certificate, marker-first handoff, linked delivery type, terminal semantic behavior, liveness deadlines, and replay rules. Reconcile `docs/architecture/gas-city-factory.md` and `docs/contracts/gas-city-execution-adapter.md` to the selected Fable/Sol, bead-native deterministic delivery-state-machine contract. Stop if neither bead representation is safe without a private ledger. |
| GC33-3 Role pack | Install only the approved Fable/Sol/Terra/Opus/Luna matrix, native sling/claim paths, and exact runtime attestation. |
| GC33-4 Isolation/routing | Prove disjoint width-two writers, conflict-domain serialization, worktree/process containment, writer-pool capacity policy, disjoint delivery/ambiguity schemas, construction-interleaving safety, composed-config predicate isolation, and zero clean-path Refiner wakes. |
| GC33-5 Merge selection | Record forge protection and distinct identity authority; run bounded controller-merge versus forge-auto conformance probes on protected fixture branches; select exactly one engine and delete the loser. |
| GC33-6 Thin vertical slice | Implement certificate -> prepared handoff -> non-routable linked bead -> committed publication -> fake-forge branch preparation -> PR create/adopt. Enforce the complexity and prohibited-surface tripwires before continuing. |
| GC33-7 Crash-only Refinery | Complete the typed one-step reducer, moving-main epochs, current-base gates, PR adoption, selected merge actor, landed verification, auto/manual modes, liveness, repair/fresh-verdict, successor, and bounded infrastructure lanes. Port only allowlisted helpers and delete the old lifecycle center and obsolete routes before qualification. |
| GC33-8 OTel baseline | Enable both native signal paths, reachability/degraded policy, and qualification receipts without making telemetry lifecycle authority. |
| GC33-9 External qualification | Run deterministic transition, handoff, route, Git/forge, authority, replay, deadline, provenance-differential, inventory/deletion, kill-anywhere, and cold-resurrection gates from clean external Codex/shell. Qualification fails if opposite-provider binding validation, integration-train lifecycle, clean-rebase Sol revalidation, or other superseded v17 authority remains normative. Generate and verify the exact-subject qualification capsule. |
| GC33-10 Clean-city proof | Bootstrap a fresh disposable city twice with zero manual edits, nudges, session surgery, or ambient binary resolution; fully tear it down both times. |
| GC33-11 Width-two live canary | Run one Terra-high and one Opus-medium bead through Fable/Sol semantic flow and the exact selected moving-main PR/CI/merge path, including a predeclared crash point, zero routine Refiner wakes, and the Luna tripwire. |
| GC33-12 Release integration | Reconcile current non-GC 3.3 release blockers, integrate only admitted GC changes onto current `main`, run release gates, and obtain a fresh release-candidate verdict over the capsule-bound bytes. |

`GC33-1` depends on `GC33-0`. `GC33-2` through `GC33-5` depend on
successful `GC33-1`; they may run in parallel only where their declared write
scopes are disjoint. `GC33-6` depends on successful `GC33-2` through `GC33-5`.
`GC33-7` through `GC33-12` transitively depend on `GC33-6` and on the preceding
bead in that sequence. Any stop result makes every downstream bead
non-admissible.

The current large `factory.py` and its tests are salvage sources, not presumed
release artifacts. Characterize allowlisted helpers and fixtures, implement the
new reducer without importing the old entry point, prove obsolete commands and
schemas unreachable, then delete them before qualification. The factory under
qualification must never implement or repair its own release subject. GC33-11
permits no in-city repair; every failure returns to external Codex/shell
diagnosis, and any external fix creates a new qualification identity and reruns
all deterministic gates.

## Acceptance

### Clean bootstrap and mixed work

Given a clean host namespace, official locked toolchain, and two independent
intake beads, when bootstrap imports the pack and both beads are slung, then
Fable and Sol admit a width-two Terra/Opus wave, each writer remains isolated,
fresh Sol Validators terminally judge the exact candidates, and no operator
nudge or manual config edit is required.

### Semantic completion independent of delivery

Given an exact candidate with a fresh `PASS`, when its admission certificate is
created, then the semantic bead is terminal before its linked delivery bead is
processed, and later rebase, CI, merge, or incompatibility cannot alter that
verdict.

### Substrate-licensed bead authority

Given the pinned real Beads/GC/store boundary, when two claimants, metadata
writers, duplicate events, claim death, and clean restart exercise the delivery
contract, then exactly one effect owner and one live epoch are observable. The
release uses in-place epochs only if that contract passes; otherwise it uses a
single preselected successor-bead representation or stops without creating a
private ledger.

### Moving-main automatic delivery

Given a valid certificate and `delivery.mode = auto`, when `main` advances
before merge, then delivery creates a new epoch, reconstructs/rebases the exact
admitted delta, reruns deterministic gates, invokes Sol only for a real
interaction, adopts or updates one PR, requires hosted CI, merges, verifies the
remote landed SHA, and writes one landed receipt.

### Native-first happy path

Given a certified candidate that rebases cleanly and passes repository gates,
when its delivery bead runs, then native GC Orders invoke the crash-only typed
AgentOps reducer, stable GC/Beads remain the runtime substrate, formula-derived
Git behavior is traceable custom code, no routine bead matches a model query,
and the complete delivery records zero Fable/Sol session starts or claims.

### Triggered delivery judgment

Given delivery requires changed bytes or exposes a real semantic interaction,
when deterministic classification cannot continue, then Fable may propose
ordering or repair scope, a Terra/Opus delivery-repair bead owns any edit, and
fresh Sol judges the exact integrated subject before delivery resumes.

### Replay safety

Given any handoff, branch, PR, or selected-merge effect completed remotely but
the reducer died before recording success, when the next invocation or a cold
bootstrap reconciles durable evidence, Beads, and remote state, then it adopts
the same identity and does not create a duplicate bead, branch effect, PR,
merge intent, certificate, or semantic transition.

### One merge engine and lawful authority

Given the target repository's actual protection, approval, identity, and merge
capabilities, when the bounded selection probes complete, then exactly one
merge engine is selected, no self-approval or admin bypass is required, the
losing engine and states are absent from the release subject, and auto delivery
cannot land without the exact expected head and nonempty required hosted CI.

### Exact-subject proof

Given all deterministic and live checks have completed, when
`refinery-qualification.v1` is verified, then it transitively binds the exact
source, `ao` binary, pack, config, toolchain, store, forge policy, semantic
certificate, PR head, required checks, selected merge actor, landed SHA/tree,
zero-wake result, deletion inventory, clean-start receipts, and explicit
checked/not-checked scope. Any mismatch or missing edge is `NOT_PROVEN`.

### Fork discipline

Given an observed GC or Beads failure, when no deterministic official-version
reproducer and upstream contribution exist, then no fork source change is
admitted and the behavior is handled in AgentOps policy, external cleanup, or
the known-error disposition.

## Required checks

The first useful check is `tests/scripts/gc-agentops-toolchain.bats` because
every later result is invalid if binary and store provenance are ambiguous.

Before a live start, require:

1. exact GC/pack/Beads/store/forge/`ao` provenance and the
   official-versus-#3985 reproducer;
2. real Beads/GC capability-envelope results and the single chosen epoch
   representation;
3. pack composition, schemas, doctor, role matrix, staged route construction,
   selector truth table, and zero-wake attestation;
4. every legal/illegal transition, semantic/terminal immutability, marker-first
   handoff cut point, and liveness deadline;
5. subject/store identity, changed-path coverage, runtime attestation, model
   authority, and prompt-injection negatives;
6. worktree/Git/process isolation, conflict-domain writer policy, moving-main,
   zero-diff, lease failure, and one-PR adoption;
7. forge authorization plus selected-engine fake-forge ambiguity cases at
   marker/request/response/receipt/reduction boundaries;
8. required-CI/check-set/protection drift, auto/manual, repair/fresh-verdict,
   semantic-successor, infrastructure, and landed-identity cases;
9. provenance-map differential fixtures and inventory proof that the old
   lifecycle center, obsolete routes, and losing engine are unreachable;
10. kill-anywhere convergence and cold resurrection after destroying all
    disposable runtime and worktree state;
11. two clean bootstrap/work/teardown cycles; and
12. a deterministic verifier accepting the exact-subject qualification
    capsule and rejecting every negative release rule.

The live canary has a hard maximum of two starts total, regardless of whether
`live_effect_begins` is reached. That marker classifies effect exposure but
never authorizes a third start. A fix requires a new identity and a complete
deterministic-gate rerun. Stop after the second total start.

## Stop conditions

Stop and return to external diagnosis when:

- a GC/Beads change lacks a reproducer and upstream issue/PR;
- an ambient or mismatched binary/store is observed;
- Beads/GC cannot provide exclusive delivery ownership with either admitted
  representation without a private ledger or coordinator;
- bootstrap requires manual mutation or produces duplicate configuration;
- any published routine bead can match a model-session query or a clean path
  wakes/claims a Refiner;
- a writer escapes its worktree, branch, index, process, or write scope;
- a binding Validator authored or repaired the candidate;
- the subject changes after validation without a new verdict;
- delivery reopens the semantic bead or introduces a global `main` mutex;
- replay creates duplicate remote effects;
- cold resurrection cannot rediscover exact open work from declared durable
  authority alone;
- forge protection cannot be satisfied by declared identities without
  self-approval or admin bypass, or neither merge actor passes;
- both merge engines or their states remain in the release subject;
- auto mode can merge without nonempty required hosted CI;
- the thin slice requires a daemon, general scheduler, program executor,
  private ledger, old lifecycle import, or other prohibited surface;
- the old lifecycle center, obsolete route, lifecycle Python successor, or
  losing merge engine remains reachable;
- teardown leaves owned sessions, compiles, tmux servers, worktrees, or Dolt
  instances;
- the qualification capsule does not verify or the live subject differs from
  its bound identities;
- the two-attempt live budget is spent.

The existing `/Users/bo/dev/gc-agentops-v17-controller-city-20260719` remains
suspended throughout implementation and qualification.

## Release-ready verdict

`READY` requires the verified `refinery-qualification.v1` capsule over the exact
post-deletion subject, all deterministic checks, two clean bootstrap/teardown
cycles, the width-two moving-main canary within budget, terminal semantic beads
before delivery, one-PR crash and cold-resurrection replay, zero routine Refiner
wakes, hosted CI and the single automatic merge proof, OTel signal receipts,
clean fork provenance, resolved 3.3 wrapper/release gates, and a fresh
release-candidate verdict over the exact bytes.

Anything less is `NOT READY`. Earlier single-bead PR delivery is useful
component evidence but does not prove the full release behavior.

## Evidence anchors

- `.agents/research/gc-factory-two-loop-2026-07-20/DUELING_WIZARDS_REPORT.md`
- `.agents/research/gc-refinery-final-duel-2026-07-20/DUELING_WIZARDS_REPORT.md`
- `.agents/research/gc-factory-two-loop-2026-07-20/GAS_CITY_V1_3_5_CODEBASE_REPORT.md`
- `deploy/gc/known-errors.json`
- `docs/architecture/gas-city-factory.md`
- `docs/contracts/gas-city-execution-adapter.md`
- `docs/operations/gas-city-reliability.md`
