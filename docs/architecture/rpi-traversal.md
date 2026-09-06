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
risky surface when any path it permits sits on one, and a broad or unbounded
scope always does.
A blocking finding ends the traversal at the `NOT_PLANNED` status, which is a
progress status and not a verdict, because the design is challenged before a
subject exists, and the plan goes back to the caller. The caller may waive that
read. Every terminal report says whether the read was not required, clean,
blocking, waived, or never finished, so a waived or dead read is never taken
for a clean one.

The runtime leaves a durable caller-owned source in place and carries its
reference plus the acceptance digest derived from its exact resolved bytes.
Only when no durable source exists does it store those bytes under
`.agents/ao/intents/sha256/<digest>.intent`. This fallback makes
conversation-only intent available to a fresh validator. The model does not
author a second PlanPacket.

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
kind of defect, reused word for word when the kind recurs, so a design defect
returning under a fresh id is visible as a kind rather than counted as a new
finding. A `class` that is present and blank is a finding against the validator
that emitted it. A documentation sentence claiming something is published,
pinned, or proven is an acceptance criterion like any other: it needs a check
the validator can run, or it is `not_checked`. The `docs.claims-tracked` gate
covers the tracked-file half of that claim and nothing more.

On a risky surface the fresh judge and the cross-family judge each report their
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
the law, or out of the caller's `repair_rounds`. A class of finding closed in
an earlier round that comes back on a new finding stops repair and returns the
caller to Plan, because what failed is the design, not the patch. The law is
stated once, in `skills/rpi/SKILL.md`. RPI does not replan, consult a helper, or escalate. `NOT_PLANNED` and `NOT_BUILT`
describe RPI progress only and are not verdict values.

The report says why the run ended, and it says what the change orphaned: the
evidence the plan budgeted to recapture, and the evidence the orphan receipt
named after Implement and after each repair round.

If a caller wants another experiment, it updates the existing bead or caller
intent and starts a new invocation. Any persisted verdicts and manifests remain
durable evidence, but AgentOps does not require a model-authored revision
packet. Changed acceptance is represented once in the intent source.

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
