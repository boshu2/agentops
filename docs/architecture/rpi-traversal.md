# RPI traversal

This page owns the exact semantics of the RPI traversal — the standard path
through the federated integration graph — and the semantic work-and-proof
protocol it implements. AgentOps is the operations layer for agentic
engineering; it is not CI, a Git workflow, a tracker, a queue, or an
autonomous controller. Vocabulary lives in
[the ubiquitous language](../contracts/ubiquitous-language.md).

```text
intent
  -> anti-ceremony quick guard once before Plan
      -> STOP: NOT_PLANNED -> report and stop; dispatch no core phase
      -> CONTINUE: existing bead or caller source
          -> Plan
          -> risky write scope: one fresh judge reads the frozen plan before
             Implement, unless the caller waives that read
              -> blocking finding: NOT_PLANNED status -> report and stop
          -> one bounded implementation experiment
          -> runtime-derived subject-manifest.v1 + check receipts, including
             the orphaned-evidence receipt over the changed paths
          -> one fresh independent validation, plus the cross-family judge on
             a risky surface; a split is never PASS, and a split that survives
             repair is the orchestrator's decision, recorded in the report
          -> PASS | FAIL | NOT_PROVEN
          -> FAIL / NOT_PROVEN with findings: bounded repair under the
             convergence law (caller's repair_rounds), re-validate freshly
          -> a closed class of finding coming back on a new finding stops
             repair and returns the caller to Plan
          -> report when converged, stopped by the law, or out of rounds
```

One traversal is one experiment plus its bounded repair. The caller, a Goal, or
a factory decides whether to start another; the traversal never selects it.

## Selected Context Delivery Lifecycle

CDLC means **Context Delivery Lifecycle**. It maintains external context and
environment around disposable agents; it does not train weights or promise
deterministic inference. The selected contract has three delivery phases:

| Phase | Responsibility |
|---|---|
| Discovery | Retrieve authorized prior experience, resolve current intent and constraints, investigate uncertainty, challenge consequential choices, and shape the existing native work graph. Plan remains the shaping owner and freezes one experiment's acceptance. Research, Premortem and Council compose by selection; no new Discovery root or packet engine is required. |
| Implement | Execute that experiment, preserve episode associations before dispatch, collect factual evidence, and integrate only authorized disjoint work. |
| Validate | Independently judge the exact subject and unchanged acceptance; admitted repairs require fresh revalidation. Return the result to the caller or selected outer goal. |

Standalone RPI remains runnable with once-only Plan/Implement and bounded repair.
Selected-mode RPI is required to perform Recall at entry even when already shaped
intent skips further discovery. T14 owns that later Recall behavior and root;
this adoption does not enable an unavailable entrypoint
or bypass current skill checks. Selected recall is instruction-level composition,
not a new hard dependency. Learn's broader episode/curation behavior is also
later work. Only T25 may restore the small evolve skill and change its removal
guard, after native stop/continuation is proven.

An explicitly selected bounded outer goal may authorize a different experiment,
including a return to Discovery, within its accepted envelope. It retains native
budgets, stops, queue, work and delivery authority. RPI never extends that bound
or changes acceptance during a repair. AO gains no scheduler, second work store,
semantic workflow engine, lifecycle CLI or delivery ownership. The core hard
edges, specialist independence and retired command/schema tombstones remain.

Learning changes later context; it cannot retroactively change a product verdict.
Searchable supported references remain useful candidates even when they do not
justify method promotion. Default Recall supplies no optional knowledge without
an applicable admitted answer. Indispensable acceptance must remain available
or assembly reports overflow. There is no universal context percentage, numeric
utility score, page quota or required lesson per session.

[ADR-0016](../adr/ADR-0016-state-tiers.md) owns external memory, confidential
staging and legacy preservation. Exact factual support, destination disclosure
before any Git object/index/stash, and later observed utility are separate
claims. The current trial retrieved source references without requesting the
optional knowledge body: it showed no memory benefit. Mechanism success cannot
be reported as measured improvement, and a whole-trial acceptance cannot erase
a worker failure.

## Roles

| Role | Owns | Does not own |
|---|---|---|
| Caller | intent source, invocation, optional strategies, any later revision or delivery | semantic PASS unless acting in a fresh validator context |
| Anti-ceremony | one artifact-free pre-Plan dispatch guard | a planning artifact, acceptance change, or core-phase dispatch |
| Plan | refining acceptance and write boundary in the existing source | a duplicate planning artifact, scheduling, ownership, readiness |
| Implement | one subject change and factual evidence | validation, repair loop, Git, closure, delivery |
| Validate | exact identity, independent judgment, optional durable evidence | subject edits, retries, next actions, release |
| RPI | one ordered dispatch and report | a controller around repeated invocations |

One model may fill multiple roles across distinct contexts. PASS requires
nonempty distinct author and validator context IDs and an explicit freshness
attestation. The attestation is a declared trust fact, not cryptographic process
isolation.

## Pre-dispatch guard

RPI invokes Anti-Ceremony's artifact-free quick guard exactly once before Plan.
`STOP` dispatches none of Plan, Implement, or Validate, reports `NOT_PLANNED`
with the guard's one-sentence reason, and stops. `CONTINUE` creates no process
artifact and preserves the ordered Plan -> Implement -> fresh Validate
traversal.

## Intent source

Plan shapes one active behavior in the caller-owned bead, issue, or conversation.
That source records:

- acceptance examples where they reduce ambiguity;
- non-goals and required evidence;
- `write_scope.include` and `write_scope.exclude`, including generated companions;
- whether that scope reaches a risky surface;
- the evidence this change will orphan, as `bash scripts/evidence-orphans.sh`
  reports it over the write scope, budgeted as recapture work;
- a first acceptance command or artifact path;
- optional decomposition with no scheduling semantics.

A plan whose write scope reaches a risky surface exits through one premortem
before Implement: one fresh judge reads the frozen plan. A scope reaches a
risky surface when its permitted effects can alter acceptance or enforcement;
Validate owns the conservative path cues and the effect-based rule. Policy
written as documentation can require stronger review. Unknown, broad, or
unbounded scope takes the stronger path. A narrow wording correction proven to
have no behavioral effect may use the lighter path prospectively; no change
waives a review leg already required for itself.
A blocking finding ends the traversal at the `NOT_PLANNED` status, which is a
progress status and not a verdict, because the design is challenged before a
subject exists, and the plan goes back to the caller. The caller may waive that
read. Every terminal report says whether the read was not required, clean,
blocking, waived, or never finished, so a waived or dead read is never taken
for a clean one.

The runtime leaves a durable caller-owned source in place and carries its
reference plus the acceptance digest derived from its exact resolved bytes.
For standalone product proof, only when no durable source exists does it store those bytes under
`.agents/ao/intents/sha256/<digest>.intent`. This fallback makes
conversation-only intent available to a fresh validator. The model does not
author a second PlanPacket.

Selected CDLC knowledge/disclosure proof requires an explicit protected external
non-Git evidence root, including snapshots, manifests, verdicts and receipts,
with no consumer-workspace fallback. Existing helpers do not yet implement the
complete selected routing: their current commands and limits are described in
[Validate mechanics](../../skills/validate/references/mechanics.md). Do not send
restricted CDLC inputs through an incompatible entrypoint. The later Go owner
must establish shared conformance before this runtime path is advertised.

Owner, ready, claim, priority, attempt, wave, queue, lease, admission, next
action, close, release, and delivery fields are outside the contract.

## One bounded experiment

Implement consumes the resolved intent once. A behavior change captures a
right-reason RED, makes the smallest coherent change that turns it GREEN, and
refactors under the unchanged acceptance check. Docs-only and pure-refactor
work record an honest pre-change baseline.

The runtime derives the author context, subject manifest, actual changed paths,
coverage fact, and check receipts. After Implement, and again after every
repair round, run `bash scripts/evidence-orphans.sh <changed paths>` and put its
output in the check receipts the validator reads, so evidence the change
orphaned is named at Implement rather than discovered as a surprise at verify
time. A repair can orphan evidence the first pass did not, which is why it runs
each round, and its output separates the evidence this change orphaned from
drift that was already there. These facts can be passed directly to
Validate; the model does not transcribe a CandidatePacket. A failed check is
evidence, not loop authority.

## Content identity

`subject-manifest.v1` is independent of Git. It contains normalized relative
paths, file/symlink/deletion kinds, executable bits, content or target digests,
declared roots and exclusions, an optional base-manifest digest, and one
canonical manifest digest. Git commit/tree information may be attached as
read-only metadata.

The pure helper lives at `skills/validate/scripts/validate.py`. It makes no Git,
tracker, queue, network, release, or delivery call.

## Fresh Validate

Validate recomputes subject identity, confirms intent-source continuity and
complete changed-path coverage, compares actual changes with Plan scope, checks
the evidence, and judges every acceptance criterion.

- Proven out-of-scope change: `FAIL`.
- Incomplete path coverage, subject mutation, digest mismatch, missing/colliding
  identities, or missing freshness: `NOT_PROVEN`.
- Complete evidence satisfying every criterion, with nonempty checked scope and
  evidence references: `PASS`.

`not_checked` names in-scope acceptance surface that this validation did not
verify. PASS asserts that the whole declared acceptance surface was verified,
so a PASS carries no `not_checked` entries and any entry makes the result
`NOT_PROVEN`. That strictness never rewards deleting an honest caveat, because
each kind of scope limit has a home that survives inside a PASS: a bounded
proof of a criterion goes in `criteria[].reason`, a declared non-goal stays in
the intent source (optionally restated as an evidence-backed boundary
criterion), and residual risk goes in the caller-facing report. The full table
lives in `skills/validate/SKILL.md` under Scope disclosure.

Each finding carries a stable `class` beside its id: one short name for the
kind of defect, reused word for word when the kind recurs, so recurrence under
a fresh id stays visible. Causal evidence must distinguish a newly discovered
pre-existing defect from an introduced regression; new ids and counts alone do
not establish cause. Unknown cause and recurrence call for causal examination,
not an unsupported claim that the design is wrong. A `class` that is present and blank is a finding against the validator
that emitted it. A documentation sentence claiming something is published,
pinned, or proven is an acceptance criterion like any other: it needs a check
the validator can run, or it is `not_checked`. The `docs.claims-tracked` gate
covers the tracked-file half of that claim and nothing more.

For required diversity (risky surface or explicit caller acceptance), use the
[bounded model-dispatch contract](../../skills/agent-native/references/model-dispatch.md).
The fresh and cross-family legs receive independently supplied initial inputs
for the exact subject; required unavailable diversity is `NOT_PROVEN` with
`diversity_unsatisfied`, never a silent single-family PASS. Each reports its
own verdict and neither resolves a disagreement. The surface converges only
when both judges pass, so a split never certifies PASS and no finding leaves
the open set because one judge was preferred. A split that survives repair is
the orchestrator's decision, made in the open: both reads go in the report,
each with its own verdict, alongside what was decided and why. The traversal
convenes no third judge of its own; a caller who wants more reads before
deciding selects `council`, which rules on findings rather than verdicts and
closes nothing.

The validation result records criterion results, findings, evidence references,
checked and not-checked surfaces, identities, and freshness. It carries no
WARN, confidence, disposition, learning, owner, next action, retry, closure,
release, or delivery state. Returning that result to the caller completes
Validate; storage is not a precondition for semantic judgment.

When the caller requests machine-readable evidence or a declared downstream
consumer requires it, Validate alone may persist `verdict.v2`. Default storage
is `.agents/ao/verdicts/sha256/<digest>.json`; a caller may provide
`verdict_dir`. The digest is SHA-256 over canonical JSON without
`artifact_digest`. Writes are same-directory, flushed, fsynced, and atomically
renamed. Exact existing content is idempotent. Conflicting content is an
integrity failure and cannot produce PASS. Provenance may record a persisted
verdict afterward, but ledger availability never affects validity.

## Stop boundary and revision

RPI invokes the anti-ceremony guard exactly once. On `CONTINUE`, RPI invokes
Plan and Implement at most once; on `STOP`, it invokes none of them. Validate
repeats only inside the bounded repair phase (ADR-0017): a `FAIL` or
`NOT_PROVEN` with findings is repaired and re-validated freshly while the
convergence law admits another round, and RPI stops when converged, stopped by
the law, or out of the caller's `repair_rounds`. The law requires new
digest-bound evidence of a named acceptance gap actually closed; changed bytes
or a smaller finding count alone are insufficient. Evidenced pre-existing
discovery can increase the count, while introduced regression, unknown cause,
reopened ids, and returning closed classes stop repair for causal examination.
A recurrence alone proves neither design failure nor permission to reopen Plan.
The law is owned by `skills/rpi/SKILL.md`; the existing pure Python reference
consumes supplied receipt facts and dispatches no runtime or helper. RPI does
not replan, consult a helper, or escalate. `NOT_PLANNED` and `NOT_BUILT` describe
RPI progress only and are not verdict values.

The report says why the run ended, and it says what the change orphaned: the
evidence the plan budgeted to recapture, and the evidence the orphan receipt
named after Implement and after each repair round.

Informative FAIL or NOT_PROVEN may falsify a live hypothesis or resolve a
blocking uncertainty and justify a materially different outer experiment under
unchanged goal acceptance. A caller or selected bounded outer goal authorizes
that experiment within its remaining envelope, updates the existing bead or
caller intent, and starts a new invocation; a new subject never resets totals.
Necessary findings stay visible, and information gained is not achieved
capability. Changed acceptance requires caller authority in the intent source.

The selected outer goal owns causal HOLD on regression, unknown cause,
recurrence, oscillation, or its declared no-progress threshold. Exactly one
bounded fresh helper per incident fits inside the existing allowance; automatic
continuation of the same incident grants no additional helper. UNSTUCK needs a
different admissible experiment and discriminating check. An unhelpful helper
or ESCALATE stops implementation. Cancellation, explicit refusal/judgment, and
a genuinely spent hard time/cost/quota ceiling skip the helper. A retry count is
not itself a spent budget, and a helper never revives a consumed RPI bound.

Native controls own enforcement. Goal objective text and reports do not prove
pause or aggregate allowance enforcement; neither native operation is
demonstrated by these contracts. Reuse existing receipts and truthful handoff
for observed controls, measurement gaps, unresolved acceptance, and helper use.
The native controller's threshold for recording blocked is status bookkeeping,
not permission for more work. Any persisted verdicts and manifests remain
durable evidence; no new lifecycle schema, goal ledger, runtime, or budget
account is introduced. Beliefs may be revised or withdrawn as evidence changes;
provenance remains, and knowledge volume is not monotonic truth or progress.

## Optional ports

- Premortem, Postmortem, Council, and genie skills are judgment strategies the
  caller selects, with one exception the traversal asks for itself: a premortem
  at Plan exit on a risky write scope. An irreversible landing decision is
  `one-way-door`'s to classify, caller-selected and outside this traversal.
- `dispatch_once(explicit_disjoint_work, executor)` may dispatch explicit
  disjoint work exactly once. It does not select, queue, persist, retry,
  validate, integrate, close, or deliver.
- Learn may later inspect collections of durable verdicts. It cannot alter a
  verdict, plan, or core result.
- Consumer repository Git, CI, merge, rollback, and release mechanisms operate
  after and outside this loop.

## Active ADR dispositions for selected CDLC

These dispositions preserve the historical evidence and distinguish T04 contract
adoption from later behavior. The eight amended ADRs carry their exact replacement
and retained invariant at the top of each owner.

| ADR | Disposition |
|---|---|
| [0001](../adr/ADR-0001-ddd-hexagonal-adoption.md) | Retain DDD/effect seams; retire mandatory ExecutionPacket/package layout. Later Go effects belong to T06. |
| [0002](../adr/ADR-0002-agentops-3-hookless-cdlc-rearchitecture.md) | Proposed hookless CDLC hypotheses remain historical; no packet, phase engine or daemon revival. |
| [0003](../adr/ADR-0003-executable-spec-artifact-durability.md) | Retain executable acceptance and private holdout/public fixture separation; no automatic promotion. |
| [0004](../adr/ADR-0004-corpus-moat-unproven-position-on-the-system.md), [0011](../adr/ADR-0011-escape-corpus-compounding-unproven-structural-starvation.md) | Retain negative evidence and mechanism/benefit separation; a new cohort must establish any new lift claim. |
| [0005](../adr/ADR-0005-trace-link-convention.md) | Replace trace-walker/confidence closure prescriptions with exact native source identity and fresh acceptance evidence; T07 owns later mechanics. |
| [0007](../adr/ADR-0007-deterministic-loop-only-operator-stops.md), [0008](../adr/ADR-0008-evolve-intelligent-agile-operating-model.md) | Preserve doctrine and actual consumer enforcement; supersede unlimited/operator-only operation and CI-only delivery. T23/T24/T25 own later native control behavior. |
| [0009](../adr/ADR-0009-daemon-deletion-in-session-only.md) | Daemon stays deleted; external selected triggers may invoke bounded skills. T27 owns later trigger behavior. |
| [0010](../adr/ADR-0010-e6-session-log-miner-build-native.md) | Retain the tool-call event miner; it does not establish full raw-source coverage. T08/T09 own that later distinction in mechanics. |
| [0014](../adr/ADR-0014-catch-to-producer-loop-judgment-catches-need-a-producer-route.md) | Remains superseded; retain failure-to-improvement hypothesis without restoring its producer registry. T29 owns later methods. |
| [0015](../adr/ADR-0015-gas-city-fenced-steward.md) | Remains historical; selected upstream factories keep their native doors and authority. |
| [0016](../adr/ADR-0016-state-tiers.md) | Admit external reviewed memory, protected staging and preservation; retain source authority, verified BD routing and Go skill logic. |
| [0017](../adr/ADR-0017-loop-as-control-flow-not-knowledge.md) | Retain exact fresh validation/repairs; admit selected outer experiments, learning and bounded cross-family invocation. T25 alone changes evolve removal. |
| [0018](../adr/ADR-0018-retire-goals-shared-scope.md) | Keep goals/shared/scope retired; count projections change only with later Recall/evolve implementations. |
